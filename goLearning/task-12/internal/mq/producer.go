package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ============ 消息结构定义 ============

// Message 业务消息体
// 为什么封装一层而不是直接用 amqp.Publishing？
// → amqp.Publishing 是 RabbitMQ 的通用结构体，包含太多底层字段。
//
//	Message 只暴露业务关心的字段（Type + Payload + MessageID），
//	底层字段（DeliveryMode/ContentType/Timestamp）由 Publish 方法自动填充，
//	调用者不需要知道这些 RabbitMQ 细节
type Message struct {
	Type      string `json:"type"`       // 消息类型，如 "order.created"、"user.registered"
	Payload   any    `json:"payload"`    // 消息体，用 any 支持任意类型——json.Marshal 能处理就行
	MessageID string `json:"message_id"` // 全局唯一 ID，用于幂等去重。为什么 Producer 生成而不是 Broker？→ Broker 不保证 ID 唯一性；Producer 用 UUID 自行生成最可靠
}

// ============ Producer 声明配置 ============

// DeclareConfig Exchange/Queue/Binding 的声明参数
// 为什么把这三者放在一起？
// → 声明 Exchange + Queue + Binding 是一组原子操作——三者缺一无法正常工作
type DeclareConfig struct {
	// Exchange 配置
	ExchangeName string // Exchange 名称
	ExchangeType string // 类型：direct / topic / fanout
	Durable      bool   // 是否持久化（重启不丢）

	// Queue 配置
	QueueName    string // Queue 名称
	QueueDurable bool   // 为什么 Queue 也有 Durable？和 Exchange 的 Durable 一样——声明是独立的

	// Binding 配置
	RoutingKey string // Exchange → Queue 的路由键
}

// ============ Producer 结构体 ============

// Producer 消息生产者
type Producer struct {
	conn      *Connection            // 连接管理器
	ch        *amqp.Channel          // 专属 Channel（开启 Confirm 模式，发布消息用）
	exchange  string                 // 默认 Exchange 名称
	confirmCh chan amqp.Confirmation // Publisher Confirm 通知 channel
	// 为什么 Producer 需要独立的 Channel（存储在 ch 字段）？
	// → Confirm 模式绑定在具体 Channel 上——ch.Confirm(false) 之后，
	//    Broker 的确认通知只会发到 ch.NotifyPublish 注册的 channel。
	//    如果 Publish() 时换一个新 Channel 发布，确认通知永远不会到达
	//    旧的 confirmCh，导致 Publish 永久阻塞。所以 Channel 必须复用。
}

// NewProducer 创建消息生产者
// exchange: 默认发往的 Exchange
// 为什么不在 NewProducer 时就声明 Exchange/Queue？
// → 分离"创建对象"和"声明拓扑"——一个 Producer 可以向多个 Queue 发消息，
//
//	拓扑声明放在 PublishWithDeclare 更灵活
func NewProducer(conn *Connection, exchange string) (*Producer, error) {
	// 获取 Channel（带 3 秒超时，因为可能在重连中）
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, err := conn.Channel(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取 Channel: %w", err)
	}

	// 开启 Publisher Confirm 模式
	// 为什么必须开这个？
	// → 默认模式下 Publish 是"发后不管"，消息可能在网络传输中丢失。
	//    Confirm 模式让 Broker 在收到消息后发回确认，Producer 才知道消息真正到达了
	// 为什么 Confirm Channel 有 10 的 buffer？
	// → 如果 Producer 很快连续发消息，confirm 通知可能积压。
	//    10 的 buffer 避免 Broker 因 confirmCh 满而阻塞
	// NotifyPublish 注册确认监听，返回的 channel 就是传入的 channel（不返回 error）
	confirmCh := ch.NotifyPublish(make(chan amqp.Confirmation, 10))

	// Confirm 模式需要显式调用——NotifyPublish 只是注册通知 channel
	if err := ch.Confirm(false); err != nil {
		return nil, fmt.Errorf("Confirm 模式: %w", err)
	}

	log.Printf("[Producer] ✅ 已创建，默认 Exchange: %s", exchange)
	return &Producer{
		conn:      conn,
		ch:        ch, // 保存 Channel，Publish() 复用以确保 Confirm 通知正确路由
		exchange:  exchange,
		confirmCh: confirmCh,
	}, nil
}

// ============ Exchange/Queue 声明 ============

// Declare 声明 Exchange + Queue + Binding（幂等操作）
// 作用：确保消息拓扑存在。如果已存在且参数一致，RabbitMQ 不会重复创建
//
// 为什么要把声明和发送分开？
// → Producer 只管"发送"，拓扑声明可以由部署脚本或 Producer 初始化时做。
//
//	分开的好处：生产环境 Exchange/Queue 可能由运维预先创建好，
//	代码中重复声明不会报错（幂等），但分开让职责更清晰
func (p *Producer) Declare(ctx context.Context, cfg DeclareConfig) error {
	ch, err := p.conn.Channel(ctx)
	if err != nil {
		return fmt.Errorf("获取 Channel: %w", err)
	}

	// 1. 声明 Exchange
	// 为什么要先声明 Exchange？
	// → 消息拓扑是自上而下的：Exchange → Queue → Binding。
	//    Queue 声明后需要 Binding 连到 Exchange，所以 Exchange 必须先存在
	err = ch.ExchangeDeclare(
		cfg.ExchangeName, // name — Exchange 名字
		cfg.ExchangeType, // type — direct/topic/fanout
		cfg.Durable,      // durable — 持久化，重启不丢
		false,            // autoDelete — 所有 Queue 解绑后自动删除？false=不自动删
		false,            // internal — 仅用于 Exchange 间转发？false=正常使用
		false,            // noWait — 不等服务器确认？false=等确认
		nil,              // args — 扩展参数（如 alternate-exchange）
	)
	if err != nil {
		return fmt.Errorf("声明 Exchange '%s': %w", cfg.ExchangeName, err)
	}

	// 2. 声明 Queue
	_, err = ch.QueueDeclare(
		cfg.QueueName,    // name — Queue 名字
		cfg.QueueDurable, // durable — 持久化
		false,            // autoDelete — Queue 无消费者时自动删除？false=不删
		false,            // exclusive — 只允许当前连接使用？false=共享
		false,            // noWait
		nil,              // args — 扩展参数（x-message-ttl/x-dead-letter-exchange 等）
	)
	if err != nil {
		return fmt.Errorf("绑定 '%s' → '%s': %w", cfg.ExchangeName, cfg.QueueName, err)
	}

	log.Printf("[Producer] 📋 拓扑已声明: Exchange=%s(%s) → Queue=%s (RK=%s)",
		cfg.ExchangeName, cfg.ExchangeType, cfg.QueueName, cfg.RoutingKey)
	return nil
}

// ============ 消息发送 ============

// Publish 发送消息到默认 Exchange（带 Publisher Confirm）
// 作用：把业务消息序列化为 JSON → 封装成 amqp.Publishing → 发布 → 等待 Broker 确认
//
// 返回值：发送成功返回 nil，失败返回 error（包括 Confirm 超时）
// 为什么 Publish 是同步等待确认而不是异步？
// → 保证消息至少到达 Broker。如果业务允许异步，可以加一个 PublishAsync 方法
func (p *Producer) Publish(ctx context.Context, routingKey string, msg Message) error {
	// 1. 序列化消息体为 JSON
	// 为什么用 JSON 而不是 Protobuf？
	// → 消息队列中的消息体需要可读可调试，JSON 可以直接在 RabbitMQ 管理界面查看。
	//    Protobuf 更省空间但不可读，适合高频内部通信
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息: %w", err)
	}

	// 2. 使用 Producer 专属 Channel（已经在 Confirm 模式）
	// 为什么用 p.ch 而不是再创建新 Channel？
	// → Confirm 模式绑定在 p.ch 上，换新 Channel 发布的话，
	//    Broker 会发确认到新 Channel 的 NotifyPublish，而不是 p.confirmCh，
	//    导致 Publish 永久阻塞等待确认
	ch := p.ch

	// 3. 发布消息
	// PublishWithContext 是 Publish 的升级版，支持 context 超时控制
	err = ch.PublishWithContext(ctx,
		p.exchange, // exchange — 发到哪个 Exchange
		routingKey, // routing key — 消息的路由键
		false,      //  mandatory — 找不到匹配 Queue 时是否返回消息？false=直接丢弃
		false,      // immediate — 已废弃，RabbitMQ 3.x 不支持
		amqp.Publishing{
			// ContentType: 消息体的格式，JSON 就是 application/json
			ContentType: "application/json",
			// DeliveryMode: 2 = Persistent（持久化），消息写入磁盘而非只在内存
			// 为什么必须设为 2？
			// → 消息不丢失三要素之一。设为 1 (Transient) 的话 Broker 重启消息就没了
			DeliveryMode: amqp.Persistent, // = 2
			// MessageId: 全局唯一 ID，消费者用来做幂等去重
			// 为什么 Producer 设 MessageId？
			// → 消费者需要 MessageId 判断是否重复消费，Producer 最清楚消息的唯一标识
			MessageId: msg.MessageID,
			// Timestamp: 消息创建时间，消费者可以做延迟监控
			Timestamp: time.Now(),
			// Body: 消息体字节
			Body: body,
		},
	)
	if err != nil {
		return fmt.Errorf("发布消息: %w", err)
	}

	// 4. 等待 Publisher Confirm
	// 为什么必须等确认？
	// → Publish 返回成功只表示消息写入了 TCP socket 缓冲区，
	//    Broker 可能还没来得及处理就挂了。Confirm 是 Broker 的"回执"——
	//    只有收到 Confirm，才能肯定消息到了 Broker
	select {
	case <-ctx.Done():
		return fmt.Errorf("等待 Confirm 超时: %w", ctx.Err())
	case confirm := <-p.confirmCh:
		if !confirm.Ack {
			// Broker 拒绝（Nack）——极少发生，通常是磁盘满或队列限制
			return fmt.Errorf("Broker Nack 消息 %s", msg.MessageID)
		}
	}

	log.Printf("[Producer] ✅ 消息已确认: Type=%s, ID=%s, RK=%s", msg.Type, msg.MessageID, routingKey)
	return nil
}

// ============ 便捷方法 ============

// PublishWithDeclare 声明拓扑 + 发送消息（一次性搞定）
// 为什么提供这个方法？
// → 开发/测试阶段频繁调用，一步步 Declare + Publish 太啰嗦。
//
//	生产环境建议分开——拓扑声明放 init，Publish 放业务代码
func (p *Producer) PublishWithDeclare(ctx context.Context, cfg DeclareConfig, msg Message) error {
	if err := p.Declare(ctx, cfg); err != nil {
		return fmt.Errorf("声明: %w", err)
	}
	return p.Publish(ctx, cfg.RoutingKey, msg)
}
