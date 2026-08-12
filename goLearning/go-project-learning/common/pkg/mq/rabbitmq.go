package mq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-project-learning/project/common/pkg/errorcode"
	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/common/pkg/redis"
	"github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

// QueueDecl 描述一个完成的队列声明 （交换机 + 队列 + 绑定）
type QueueDecl struct {
	ExchangeName string
	ExchangeType string
	QueueName    string
	RoutingKey   string
	QueueArgs    amqp091.Table // 死信队列的 TTL 参数
}

var (
	Channel    *amqp091.Channel
	conn       *amqp091.Connection
	confirmCh  chan amqp091.Confirmation
	mu         sync.Mutex  // 保护 Channel 的并发安全
	queueDecls []QueueDecl // 所有注册的队列声明
)

type handler func(ctx context.Context, body []byte) error

func InitRabbitMQ(username, password, host string, port int) {
	if err := connect(username, password, host, port); err != nil {
		logger.Error("RabbitMQ 连接失败", zap.Error(err))
	}

	// 监听连接断开，自动重连
	go watchConnection(username, password, host, port)
}

func connect(username, password, host string, port int) error {
	mu.Lock()
	defer mu.Unlock()

	// cfg := configs.Conf.RabbitMQ
	mqURI := fmt.Sprintf("amqp://%s:%s@%s:%d/", username, password, host, port)

	// 创建连接
	var err error
	conn, err = amqp091.Dial(mqURI)
	if err != nil {
		logger.Error("RabbitMQ 连接失败", zap.Error(err))
	}

	// 创建通道
	Channel, err = conn.Channel()
	if err != nil {
		logger.Error("RabbitMQ 通道创建失败", zap.Error(err))
	}

	Channel.Confirm(false)
	// 注册一次，全局复用
	confirmCh = Channel.NotifyPublish(make(chan amqp091.Confirmation, 1))

	logger.Info("RabbitMQ 连接成功")
	return nil
}

// watchConnection 监听连接断开, 自动重连
func watchConnection(username, password, host string, port int) {
	for {
		// NotifyClose 在连接关闭时发送 error，连接正常时组赛
		err := <-conn.NotifyClose(make(chan *amqp091.Error))
		if err == nil {
			logger.Info("AMQP 连接正常关闭")
			return
		}

		logger.Error("AMQP 连接断开，开始重连...", zap.Error(err))

		// 指数退避重连
		for i := 0; ; i++ {
			delay := time.Duration(1<<min(i, 6)) * time.Second // 1s, 2s, 4s, 8s, 16s, 32s, 64s
			time.Sleep(delay)

			if err := connect(username, password, host, port); err != nil {
				logger.Error("AMQP 重连失败", zap.Error(err), zap.Int("attempt", i+1))
				continue
			}

			logger.Info("AMQP 重连成功")

			// 重连后需要重新声明队列和绑定 （因为 channel 是新的）
			if err := redeclareAll(); err != nil {
				logger.Error("重新声明队列失败", zap.Error(err))
			}

			// 重连后需要重启消费者
			if onReconnect != nil {
				onReconnect()
			}

			// 继续监听下一次断线
			break
		}
	}
}

// onReconnect 重连后的回调，由外部注册（启动消费者）
var onReconnect func()

// SetReconnectCallback 设置重连后的回调
func SetReconnectCallback(fn func()) {
	onReconnect = fn
}

// RegisterQueue 注册一个队列声明, 重连时会自动声明
func RegisterQueue(decl QueueDecl) {
	queueDecls = append(queueDecls, decl)
}

// redeclareAll 重连后重新声明所有注册过的交换机/队列/绑定
func redeclareAll() error {
	for _, d := range queueDecls {
		if err := Channel.ExchangeDeclare(d.ExchangeName, d.ExchangeType, true, false, false, false, nil); err != nil {
			return fmt.Errorf("声明交换机 %s 失败: %w", d.ExchangeName, err)
		}
		if _, err := Channel.QueueDeclare(d.QueueName, true, false, false, false, d.QueueArgs); err != nil {
			return fmt.Errorf("声明队列 %s 失败: %w", d.QueueName, err)
		}
		if err := Channel.QueueBind(d.QueueName, d.RoutingKey, d.ExchangeName, false, nil); err != nil {
			return fmt.Errorf("绑定队列 %s 失败: %w", d.QueueName, err)
		}
	}
	return nil
}

func DeclareAll() error {
	return redeclareAll()
}

// DeclareExchange 声明交换机 （项目启动时调用）
func DeclareExchange(exchangeName, exchangeType string) error {
	mu.Lock()
	defer mu.Unlock()
	return Channel.ExchangeDeclare(
		exchangeName, // 交换机名称
		exchangeType, // 类型： direct/topic/fanout/headers
		true,         // 持久化
		false,
		false,
		false,
		nil,
	)
}

// DeclarQueue 声明队列
func DeclarQueue(queueName string, args amqp091.Table) (amqp091.Queue, error) {
	mu.Lock()
	defer mu.Unlock()
	return Channel.QueueDeclare(
		queueName,
		true,  // 持久化
		false, // 自动删除
		false, // 排他
		false,
		args,
	)
}

// QueueBind 绑定队列 + 交换机 + 路由键
func QueueBind(queueName, routingKey, exchangeName string) error {
	mu.Lock()
	defer mu.Unlock()
	return Channel.QueueBind(
		queueName,
		routingKey,
		exchangeName,
		false,
		nil,
	)
}

// Public 发送消息 （生产者）
func Public(ctx context.Context, exchange, routingKey string, body []byte) error {
	mu.Lock()
	ch := Channel
	cc := confirmCh
	mu.Unlock()

	if Channel == nil || Channel.IsClosed() {
		return errorcode.NewBizError(errorcode.ThirdApiErr, "AMQP channel 已关闭，等待重连")
	}
	// 将 Trace 上下文注入到 AMQP Headers
	headers := amqp091.Table{}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	for k, v := range carrier {
		headers[k] = v
	}

	// confirmCh := Channel.NotifyPublish(make(chan amqp091.Confirmation, 1))

	err := ch.Publish(
		exchange,
		routingKey,
		false,
		false,
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent, // 消息持久化 （重启不丢）
			Body:         body,
			Timestamp:    time.Now(),
			Headers:      headers, // <-携带 Trace 上下文
		},
	)

	if err != nil {
		return err
	}

	select {
	case ack := <-cc:
		if !ack.Ack {
			logger.Warn("RabbitMQ NACK")
			return fmt.Errorf("message nacked")
		}
	case <-time.After(5 * time.Second):
		logger.Warn("等待 RabbitMQ ACK 超时")
		return fmt.Errorf("public ack timeout")
	}
	logger.Info("消息发送成功并已确认" + string(body))
	return nil
}

// PublicReplyWithTrace 发送响应消息
func PublicWithTrace(ctx context.Context, exchange, routingKey, correlationId string, body []byte) error {
	mu.Lock()
	ch := Channel
	mu.Unlock()

	headers := amqp091.Table{}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	for k, v := range carrier {
		headers[k] = v
	}

	return ch.Publish(
		exchange, routingKey, false, false,
		amqp091.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp091.Persistent,
			Body:          body,
			Timestamp:     time.Now(),
			Headers:       headers,
			CorrelationId: correlationId,
		},
	)
}

// Consume 消费消息队列 （消费者）
func Consume(queueName string, handler handler, maxRetries int) error {
	mu.Lock()
	ch := Channel
	mu.Unlock()

	if ch == nil || ch.IsClosed() {
		return fmt.Errorf("AMQP channel 已关闭，等待重连")
	}

	msgs, err := ch.Consume(
		queueName,
		"",    // 消费者标签
		false, // 自动确认
		false, // 排他
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// 异步监听消息
	go func() {
		for msg := range msgs {
			// 从 Headers 中提取 Trace 上下文
			carrier := propagation.MapCarrier{}
			if msg.Headers != nil {
				for k, v := range msg.Headers {
					if str, ok := v.(string); ok {
						carrier.Set(k, str)
					}
				}
			}
			// 将提取的上下文作为 parent， 创建新 span
			msgCtx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
			tracer := otel.Tracer("mq-consumer")
			msgCtx, span := tracer.Start(msgCtx, "MQ-Consume-"+queueName)
			span.SetAttributes(attribute.String("messaging.system", "rabbitmq"))
			span.SetAttributes(attribute.String("messaging.destination", queueName))

			// 重试
			retryCount := 0
			success := false

			// 添加幂等 Redis 处理
			key := "redis:processed:" + string(msg.Body)

			exists, err := redis.Exists(msgCtx, key)
			if err != nil {
				logger.Error("Redis 获得幂等消息出错", zap.Error(err))
				continue
			}

			if exists {
				// 成功 ACK
				_ = msg.Ack(false)
				logger.Info("消息已处理")
				continue
			}

			// 重试
			for retryCount <= maxRetries {
				// 执行业务
				err := handler(msgCtx, msg.Body)

				if err == nil {
					// 成功 ACK
					_ = msg.Ack(false)
					logger.Info("消息处理成功")
					success = true
					break
				}

				// 失败： 重试计数
				retryCount++
				if retryCount > maxRetries {
					break
				}

				// 指数退避 1s -> 2s -> 4s -> 8s -> 16s
				delay := time.Duration(1*(1<<retryCount)) * time.Second
				logger.Info(fmt.Sprintf("失败, %v 后重试 (%d/%d)\n", delay, retryCount, maxRetries))
				time.Sleep(delay)
			}

			if !success {
				// 达到最大次数 -> 死信 （不 ACK，不重回队列）
				_ = msg.Nack(false, false)
				logger.Info("超过最大次数，进入死信队列")
			}
			span.End()
		}
		// msgs channel 关闭说明连接断了，goroutine 退出
		// watchConnection 会重连并重启消费者
		logger.Warn("消费者 goroutine 退出: " + queueName)
	}()

	return nil
}

// 关闭连接
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if Channel != nil {
		_ = Channel.Close()
	}
	if conn != nil {
		_ = conn.Close()
	}
	logger.Info("RabbitMQ 连接已关闭")
}
