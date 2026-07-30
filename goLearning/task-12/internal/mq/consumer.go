package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ============ 消息处理器类型 ============

// Handler 消息处理函数签名
// 为什么返回 error 而不是 panic？
// → 处理失败不应该让整个 Consumer 崩溃。
//
//	返回 error 让 Consumer 框架决定是 Nack（进死信）还是 Requeue（重试）
type Handler func(ctx context.Context, msg Message) error

// ============ Consumer 配置 ============

// ConsumeConfig 消费者配置
type ConsumeConfig struct {
	QueueName    string // 监听的 Queue 名称
	ExchangeName string // 对应的 Exchange（用于声明拓扑）
	ExchangeType string // Exchange 类型
	RoutingKey   string // Binding 的 routing key
	Durable      bool   // Queue 是否持久化

	// ConsumerName: 消费者标签，用于在 RabbitMQ 管理界面识别
	// 为什么需要名字？
	// → 多个 Consumer 实例监听同一 Queue 时，名字帮助运维区分各个实例的处理状态
	ConsumerName string

	// Concurrency: 并发处理的 goroutine 数量
	// 为什么需要并发？
	// → <-ch 默认串行消费，如果一个消息处理需要 100ms，串行只能 10 QPS。
	//    开启并发后多个 goroutine 并行处理，提升吞吐量
	//    但注意：并发会增加 Ack 顺序复杂度（消息 A 比消息 B 后处理完）
	Concurrency int
}

// ============ Consumer 结构体 ============

// Consumer 消息消费者
type Consumer struct {
	conn      *Connection    // 连接管理器
	config    ConsumeConfig  // 消费配置
	handler   Handler        // 消息处理函数（业务逻辑注入点）
	handlerMu sync.RWMutex   // 保护 handler 的并发读写（支持热更新）
	done      chan struct{}  // 关闭信号
	wg        sync.WaitGroup // 等待所有处理 goroutine 退出
}

// NewConsumer 创建消费者
// handler: 业务处理函数，由调用方注入。为什么通过参数注入而不是 Consumer 内部实现？
// → 依赖注入原则——Consumer 只负责"如何消费"（拉取/Ack/Nack），不管"消费了什么"（业务逻辑）。
//
//	这样同一个 Consumer 框架可以处理订单、通知、日志等多种消息
func NewConsumer(conn *Connection, config ConsumeConfig, handler Handler) *Consumer {
	return &Consumer{
		conn:    conn,
		config:  config,
		handler: handler,
		done:    make(chan struct{}),
	}
}

// ============ 消费者启动 ============

// Start 开始消费消息（阻塞，直到 ctx 取消或 Close 被调用）
// 为什么 Start 是阻塞的？
// → 消费是一个持续的过程，Start 启动后一直循环处理消息，
//
//	直到外部取消（ctx.Done）或主动关闭（Close）
//	通常放在一个独立的 goroutine 中运行：go consumer.Start(ctx)
func (c *Consumer) Start(ctx context.Context) error {
	// 1. 声明拓扑（和 Producer 的 Declare 逻辑一样，保证 Exchange/Queue/Binding 存在）
	if err := c.declareTopology(ctx); err != nil {
		return fmt.Errorf("声明拓扑: %w", err)
	}

	// 2. 获取 Channel
	ch, err := c.conn.Channel(ctx)
	if err != nil {
		return fmt.Errorf("获取 Channel: %w", err)
	}

	// 3. 开始消费
	// Consume 返回一个 <-chan amqp.Delivery，每收到一条消息就会发送到这个 channel
	deliveries, err := ch.Consume(
		c.config.QueueName,    // queue — 监听哪个 Queue
		c.config.ConsumerName, // consumer tag — 消费者标签
		false,                 // autoAck — ⚠️ false = Manual Ack 模式！
		// 为什么必须 Manual Ack？
		// → Auto Ack 意味着 Broker 发出消息就立即删除——如果处理流程后面失败了，
		//    消息已经没了，无法重试。Manual Ack 让你在处理完后显式确认，失败时 Nack
		false, // exclusive — 独占 Queue？false=共享消费
		false, // noLocal — 不接收自己发的消息（仅支持 RabbitMQ 集群）
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("开始消费: %w", err)
	}

	log.Printf("[Consumer] 🎧 开始监听 Queue='%s', 并发=%d", c.config.QueueName, c.config.Concurrency)

	// 4. 启动处理循环
	// 为什么用 goroutine pool 模式而不是每个消息起一个 goroutine？
	// → 每个消息一个 goroutine 的话，消息量大时 goroutine 爆炸（10万 QPS = 10万 goroutine）。
	//    goroutine pool 用固定数量的 worker，通过 channel 分发，资源可控
	if c.config.Concurrency <= 1 {
		// 串行模式：一个 goroutine 顺序处理
		c.processSerial(ctx, deliveries)
	} else {
		c.processParallel(ctx, deliveries, c.config.Concurrency)
	}

	return nil
}

// ============ 拓扑声明（内部） ============

// declareTopology 确保 Exchange + Queue + Binding 存在
// 为什么 Consumer 也要声明拓扑？
// → 防御性编程——Consumer 独立声明拓扑，不依赖 Producer 先启动。
//
//	即使 Producer 还没创建 Exchange，Consumer 启动时也能正常工作（声明是幂等的）
func (c *Consumer) declareTopology(ctx context.Context) error {
	ch, err := c.conn.Channel(ctx)
	if err != nil {
		return err
	}

	// 声明 Exchange
	err = ch.ExchangeDeclare(
		c.config.ExchangeName,
		c.config.ExchangeType,
		c.config.Durable,
		false, false, false, nil,
	)
	if err != nil {
		return fmt.Errorf("Exchange: %w", err)
	}

	// Queue 由 main.go 在启动时声明（带 DLX args），这里不再重复声明
	// 为什么 Consumer 不声明 Queue？
	// → Queue 的 DLX 参数（x-dead-letter-exchange 等）必须在声明时传入，
	//    如果 Consumer 用 nil args 重新声明同名 Queue，RabbitMQ 会返回
	//    "inequivalent arg" 错误。Queue 的声明统一在 main.go 中管理

	// 绑定 Exchange → Queue
	err = ch.QueueBind(
		c.config.QueueName,
		c.config.RoutingKey,
		c.config.ExchangeName,
		false, nil,
	)
	if err != nil {
		return fmt.Errorf("Binding: %w", err)
	}

	return nil
}

// ============ 串行处理 ============

// processSerial 串行处理消息（一个 goroutine，顺序处理）
// 优点：严格保序——消息 A 一定在消息 B 之前被处理完
// 缺点：吞吐量受单消息处理时间限制
// 适用场景：订单状态流转等必须保序的业务
func (c *Consumer) processSerial(ctx context.Context, deliveries <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[Consumer] ⏹️ Context 取消，停止消费")
			return
		case <-c.done:
			log.Println("[Consumer] ⏹️ 收到关闭信号，停止消费")
			return
		case delivery, ok := <-deliveries:
			if !ok {
				// Channel 被关闭了（可能是重连），退出后由上层决定是否重启
				log.Println("[Consumer] ⚠️ 消费 Channel 已关闭")
				return
			}
			// 处理消息
			c.handleDelivery(ctx, delivery)
		}
	}
}

// ============ 并行处理 ============

// processParallel 并行处理消息（固定数量 worker goroutine）
// 原理：主 goroutine 从 deliveries channel 读取消息，通过内部的 jobs channel
//
//	分发给 worker goroutine 池。worker 数 = Concurrency
func (c *Consumer) processParallel(ctx context.Context, deliveries <-chan amqp.Delivery, concurrency int) {
	// 创建任务分发 channel，buffer 大小 = worker 数
	// 为什么 jobs channel 需要 buffer？
	// → 避免主 goroutine 等待 worker 取任务时阻塞 deliveries channel 的消费。
	//    有 buffer 后主 goroutine 可以快速读完消息放入 jobs，然后继续从 RabbitMQ 拉取
	jobs := make(chan amqp.Delivery, concurrency)

	// 启动 worker goroutine 池
	for i := 0; i < concurrency; i++ {
		c.wg.Add(1)
		go func(workerID int) {
			defer c.wg.Done()
			for delivery := range jobs {
				c.handleDelivery(ctx, delivery)
			}
			log.Printf("[Consumer] Worker-%d 退出", workerID)
		}(i)
	}

	// 主循环：从 deliveries 读取消息，分发到 jobs channel
	for {
		select {
		case <-ctx.Done():
			close(jobs) // 关闭 jobs 后 workers 会处理完剩余任务后退出
			c.wg.Wait() // 等待所有 worker 完成
			log.Println("[Consumer] ⏹️ Context 取消，所有 Worker 已退出")
			return
		case <-c.done:
			close(jobs)
			c.wg.Wait()
			log.Println("[Consumer] ⏹️ 主动关闭，所有 Worker 已退出")
			return
		case delivery, ok := <-deliveries:
			if !ok {
				close(jobs)
				c.wg.Wait()
				log.Println("[Consumer] ⚠️ 消费 Channel 已关闭")
				return
			}
			// 将消息分发给 worker
			jobs <- delivery
		}
	}
}

// ============ 消息处理核心 ============

// handleDelivery 处理单条消息（Ack/Nack 决策）
// 这是整个 Consumer 最核心的方法——它决定了消息的最终命运
//
// 处理流程:
//  1. 解析消息体 → 2. 调用业务 Handler → 3. 根据结果 Ack 或 Nack
//
// 为什么不用 defer recover？
// → Handler 由业务方实现，如果 Handler panic 了，用 recover 兜底
//
//	防止一个坏消息导致整个 Consumer 崩溃
func (c *Consumer) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	c.handlerMu.RLock()
	handler := c.handler
	c.handlerMu.RUnlock()

	// recover 兜底：业务 Handler 可能 panic，不能让它炸掉 Consumer
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Consumer] ❌ Handler panic: %v, msgID=%s", r, delivery.MessageId)
			// panic 的消息不能丢——Nack 且不重入队（让死信队列兜底）
			// 为什么 requeue=false？
			// → panic 通常意味着代码有 bug，重试也会继续 panic，不如进死信队列
			//    让人工或补偿逻辑处理
			delivery.Nack(false, false) // multiple=false, requeue=false
		}
	}()

	// 1. 反序列化消息体
	// 为什么只解析成 Message 结构体，Payload 保留为 json.RawMessage 或延迟解析？
	// → 这里用 json.Unmarshal 一步到位。如果 Payload 非常大，可以考虑只解析 Type+MessageID
	//    然后 Payload 传给 Handler 按需解析（延迟反序列化）
	var msg Message
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		log.Printf("[Consumer] ❌ 消息反序列化失败: %v, body=%s", err, string(delivery.Body))
		// 反序列化失败 → 消息格式有问题，重试也没用 → 直接 Nack 不进队
		delivery.Nack(false, false) // multiple=false, requeue=false → 进死信队列
		return
	}

	// 2. 创建带超时的处理 context
	// 为什么每一条消息独立超时？
	// → 防止某个 Handler 卡死阻塞整个 Consumer。30 秒是经验值——大部分业务处理在百毫秒级
	handlerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 3. 调用业务 Handler
	log.Printf("[Consumer] 📨 收到消息: Type=%s, ID=%s", msg.Type, msg.MessageID)
	if err := handler(handlerCtx, msg); err != nil {
		// 处理失败 → Nack 且不重入队 → 消息进死信队列
		// 为什么 requeue=false 而不是 true？
		// → requeue=true 会立刻把消息放回 Queue 头部，如果错误是持续的（如数据库连接失败），
		//    消息会被重复投递-Nack-重入队，形成死循环，阻塞后续正常消息
		//    正确的做法是 Nack 进死信，由死信消费者决定重试策略（延迟重试/人工介入）
		log.Printf("[Consumer] ❌ 处理失败: %v, msgID=%s → 转入死信", err, msg.MessageID)
		delivery.Nack(false, false)
		return
	}

	// 4. 处理成功 → Ack
	// 为什么 Ack 是最后一步？
	// → 业务流程: 获取消息 → 解析 → 处理 → 写库 → 只有全部成功才 Ack
	//    如果先 Ack 再写库，写库失败时消息已删除，数据就丢了
	delivery.Ack(false) // multiple=false，只 Ack 这一条
	log.Printf("[Consumer] ✅ 处理成功: Type=%s, ID=%s", msg.Type, msg.MessageID)
}

// ============ 生命周期管理 ============

// Close 优雅关闭消费者
// 为什么 Close 不直接关闭 Channel？
// → 要先通知主消费循环退出（close done），等所有 worker 处理完当前消息，
//
//	再关闭连接。如果不等待直接关 Channel，Ack 可能失败导致消息丢失
func (c *Consumer) Close() {
	close(c.done) // 发送退出信号给消费循环
	c.wg.Wait()   // 等待所有处理中的消息完成
	log.Printf("[Consumer] 👋 已关闭")
}

// SetHandler 热更新 Handler（运行时替换业务逻辑）
// 为什么需要热更新？
// → 生产环境可能需要在不停服的情况下更新消息处理逻辑（如紧急关闭某个处理流程）
//
//	通过 SetHandler 替换后，新消息走新逻辑，旧消息继续旧逻辑处理完
//
// 用写锁保护：确保更新是原子的
func (c *Consumer) setHandler(handler Handler) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.handler = handler
	log.Println("[Consumer] 🔄 Handler 已热更新")
}
