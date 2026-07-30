package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ============ 死信消息结构 ============

// DeadLetter 死信消息（RabbitMQ 会在原消息头注入死亡信息）
// 为什么需要这个结构体？
// → 消息变成死信后，RabbitMQ 在 Header 中自动添加 x-death 数组，
//
//	记录死因、死亡时间、来源 Queue 等信息。解析这些信息才能做正确的补偿决策
type DeadLetter struct {
	OriginalMessage Message        // 原始消息体
	DeathInfo       DeathInfo      // 死亡信息（从 x-death header 提取）
	RawHeaders      map[string]any // 完整的 AMQP Header，用于调试
}

type DeathInfo struct {
	Reason      string    // 死因: "rejected" / "expired" / "maxlen"
	Queue       string    // 死在哪个 Queue
	Time        time.Time // 死亡时间
	Count       int64     // 在这个 Queue 内死了几次（循环死信的防护）
	Exchange    string    // 原本发往的 Exchange
	RoutingKeys []string  // 原始 Routing Key
}

// ============ DLX 配置 ============

// DLXConfig 死信队列配置
type DLXConfig struct {
	// 死信交换机（DLX）配置
	// 为什么 DLX 需要独立的 Exchange？
	// → 死信是一种特殊的消息路由——正常 Exchange 按 RoutingKey 分发业务消息，
	//    DLX 专门收集"死亡"消息。分开可以独立管理、独立监控、独立权限
	DLXExchangeName string // 死信 Exchange 名称，如 "order.dlx"
	DLXQueueName    string // 死信 Queue 名称，如 "order.dlq"
	DLXRoutingKey   string // 死信路由键，通常用 "#" 接收所有死信

	// 业务 Queue 的死信配置（在声明业务 Queue 时传给 args）
	// 为什么 DLX 配置放在业务 Queue 的 args 里？
	// → RabbitMQ 的设计：每个 Queue 可以有自己独立的 DLX。
	//    Queue A 的死信可能发到 DLX-A，Queue B 的可能发到 DLX-B
	TTL       int32 // 消息 TTL（毫秒），0 表示不设过期。消息超时未消费 → 自动变死信
	MaxLength int   //  Queue 最大消息数，0 表示不限制。队列满 → 溢出消息变死信
}

// ============ DLX 管理器 ============

// DLXManager 死信队列管理器
// 职责：声明 DLX 拓扑 + 消费死信 + 死信补偿策略
type DLXManager struct {
	conn   *Connection
	config DLXConfig

	// 补偿策略：死信的处理方式（记录/重试/告警）
	// 为什么是链式？
	// → 一条死信可能需要多个补偿动作：记录日志 → 发告警 → 尝试重试
	compensationHandlers []DeadLetterHandler
}

// DeadLetterHandler 死信处理函数
// 为什么返回 bool 而不是 error？
// → 死信处理可以链式调用。return true 表示"已处理，不需要后续 handler 处理"
type DeadLetterHandler func(ctx context.Context, dl DeadLetter) (handled bool)

// NewDLXManager 创建死信队列管理器
func NewDLXManager(conn *Connection, config DLXConfig) *DLXManager {
	return &DLXManager{
		conn:   conn,
		config: config,
	}
}

// ============ 核心方法 ============

// Setup 创建死信 Exchange + Queue
// 作用：声明 DLX 和 DLQ，确保死信路由拓扑存在
// 为什么要独立于业务 Queue 创建 DLX？
// → 解耦：DLX 的声明是全局的（多个业务 Queue 可以共享同一个 DLX），
//
//	Setup 可以在应用启动时一次性创建，业务 Queue 声明时只需引用 DLX 名字
func (m *DLXManager) Setup(ctx context.Context) error {
	ch, err := m.conn.Channel(ctx)
	if err != nil {
		return fmt.Errorf("获取 Channel: %w", err)
	}

	// 1. 声明死信 Exchange
	// 为什么死信 Exchange 用 Topic 类型？
	// → Topic 最灵活：可以按业务模块路由死信（如 order.# → order死信处理, user.# → user死信处理）
	//    即使现在只用 "#" 接收所有死信，Topic 保留未来细粒度路由的可能
	err = ch.ExchangeDeclare(
		m.config.DLXExchangeName,
		"topic", // 用 Topic 做死信路由最灵活
		true,    // durable=true — 死信不能丢！
		false, false, false, nil,
	)
	if err != nil {
		return fmt.Errorf("声明 DLX Exchange: %w", err)
	}

	// 2. 声明死信 Queue
	_, err = ch.QueueDeclare(
		m.config.DLXQueueName,
		true, // durable=true
		false, false, false, nil,
	)
	if err != nil {
		return fmt.Errorf("声明 DLQ: %w", err)
	}

	// 3. 绑定 DLX → DLQ
	// RoutingKey="#" 的含义：接收所有死信，不按路由键过滤
	err = ch.QueueBind(
		m.config.DLXQueueName,
		m.config.DLXRoutingKey, // 通常是 "#"
		m.config.DLXExchangeName,
		false, nil,
	)
	if err != nil {
		return fmt.Errorf("绑定 DLX → DLQ: %w", err)
	}

	log.Printf("[DLX] ✅ 死信拓扑已创建: %s → %s (RK=%s)",
		m.config.DLXExchangeName, m.config.DLXQueueName, m.config.DLXRoutingKey)
	return nil
}

// DeadLetterArgs 生成业务 Queue 的 DLX args
// 作用：返回一个 amqp.Table，在声明业务 Queue 时作为 args 参数传入
//
// 调用示例:
//
//	args := dlxManager.DeadLetterArgs()
//	ch.QueueDeclare("order.queue", true, false, false, false, args)
//
// 为什么 DLX 参数用 map 传而不是 struct？
// → RabbitMQ 的 Queue.Declare 接受 amqp.Table（map[string]interface{}），
//
//	这是 AMQP 协议规定的扩展参数传递方式
func (m *DLXManager) DeadLetterArgs() amqp.Table {
	args := amqp.Table{
		// x-dead-letter-exchange: 告诉 RabbitMQ "这个 Queue 的死信发到哪个 Exchange"
		"x-dead-letter-exchange": m.config.DLXExchangeName,
		// x-dead-letter-routing-key: 死信的路由键
		// 为什么单独指定而不是用原消息的 routing key？
		// → 可以区分不同类型的死信——比如 payment.paid 的死信路由到 payment.dlq，
		//    而不是和 order.canceled 的死信混在一起
		"x-dead-letter-routing-key": m.config.DLXRoutingKey,
	}

	// 如果配置了 TTL，添加消息过期时间
	// 为什么用 x-message-ttl 而不是在 Publish 时设 Expiration？
	// → Queue 级别的 TTL 对所有消息生效，Publish 级别的 Expiration 对单条消息生效。
	//    Queue 级别 TTL 更合理——同一 Queue 的消息通常有相同的时效要求
	if m.config.TTL > 0 {
		args["x-message-ttl"] = m.config.TTL
	}

	// 如果配置了 Queue 最大长度
	// 为什么 Queue 的长度限制很重要？
	// → 防止消费者宕机时消息无限堆积撑爆磁盘。
	//    x-max-length 限制 Queue 中最多存多少条消息，超出后最早的消息自动变死信
	if m.config.MaxLength > 0 {
		args["x-max-length"] = m.config.MaxLength
	}

	return args
}

// ============ 死信消费 ============

// StartConsumeDeadLetters 开始消费死信队列
// 作用：启动一个 Consumer 监听 DLQ，收到死信后调用补偿链处理
//
// 为什么要为死信单独起 Consumer？
// → 死信是异常路径——Consumer 和 DLQ Consumer 应该独立运行：
//
//	正常 Consumer 宕机 → 死信继续堆积在 DLQ → DLQ Consumer 独立告警
func (m *DLXManager) StartConsumeDeadLetters(ctx context.Context) error {
	ch, err := m.conn.Channel(ctx)
	if err != nil {
		return fmt.Errorf("获取 Channel: %w", err)
	}

	// 声明拓扑（幂等）
	if err := m.Setup(ctx); err != nil {
		return err
	}

	// 开始消费 DLQ
	deliveries, err := ch.Consume(
		m.config.DLXQueueName,  // consumer tag
		"dead-letter-consumer", // consumer tag
		false,                  // autoAck=false — 死信也要 Manual Ack！
		// 为什么死信也要 Manual Ack？
		// → 死信也需要被可靠处理。如果补偿逻辑失败了就 Nack（不进队），
		//    这条死信会被 DLQ 的 DLX（如果配置了的话）再次路由——形成死信的死信
		false, false, false, nil,
	)
	if err != nil {
		return fmt.Errorf("消费 DLQ: %w", err)
	}

	log.Printf("[DLX] 🎧 开始监听死信 Queue='%s'", m.config.DLXQueueName)

	// 死信消费循环
	for {
		select {
		case <-ctx.Done():
			log.Println("[DLX] ⏹️ Context 取消，停止死信消费")
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				log.Println("[DLX] ⚠️ 死信消费 Channel 已关闭")
				return nil
			}

			dl := m.parseDeadLetter(delivery)
			log.Printf("[DLX] 💀 收到死信: Reason=%s, Queue=%s, OrigID=%s",
				dl.DeathInfo.Reason, dl.DeathInfo.Queue, dl.OriginalMessage.MessageID)

			// 执行补偿链
			m.runCompensation(ctx, dl)

			// 补偿完 Ack——死信处理完后从 DLQ 中删除
			// 为什么补偿失败了也要 Ack？
			// → 补偿失败说明系统出了大问题（数据库也挂了等），此时
			//    1) 已记录了详细日志（留做人工排查）
			//    2) 不 Ack 会导致死信堆积，可能拖垮 RabbitMQ
			//    正确的做法：Ack + 写日志/发告警 + 人工介入
			delivery.Ack(false)
		}
	}
}

// parseDeadLetter 从 AMQP Delivery 中提取死信信息
// RabbitMQ 在消息变死信时，会在 Header 里追加 x-death 数组。
// x-death[0] 是最新一次的死亡信息
func (m *DLXManager) parseDeadLetter(delivery amqp.Delivery) DeadLetter {
	dl := DeadLetter{
		RawHeaders: make(map[string]any),
	}

	// 解析原始消息体
	if err := json.Unmarshal(delivery.Body, &dl.OriginalMessage); err != nil {
		log.Printf("[DLX] ⚠️ 死信消息体无法解析: %v", err)
	}

	// 提取 x-death header
	// 为什么 x-death 是数组？
	// → 消息可能"死"多次——比如业务 Queue → DLQ → 消费失败 → DLQ 的 DLX → ...
	//    每次死亡都在 x-death 数组头部插入一条记录，数组长度就是死亡次数
	if deaths, ok := delivery.Headers["x-death"]; ok {
		if deathList, ok := deaths.([]any); ok && len(deathList) > 0 {
			// 最新一条死亡记录
			if death, ok := deathList[0].(amqp.Table); ok {
				m.parseDeathInfo(death, &dl.DeathInfo)
			}
		}
	}

	// 复制所有 Header 用于调试
	for k, v := range delivery.Headers {
		dl.RawHeaders[k] = v
	}

	return dl
}

// parseDeathInfo 解析单条死亡记录
func (m *DLXManager) parseDeathInfo(table amqp.Table, info *DeathInfo) {
	if v, ok := table["reason"].(string); ok {
		info.Reason = v
	}
	if v, ok := table["queue"].(string); ok {
		info.Queue = v
	}
	if v, ok := table["count"].(int64); ok {
		info.Count = v
	}
	if v, ok := table["exchange"].(string); ok {
		info.Exchange = v
	}
	// routing-keys 是 []interface{} → []string
	if v, ok := table["routing-keys"].([]any); ok {
		for _, rk := range v {
			if s, ok := rk.(string); ok {
				info.RoutingKeys = append(info.RoutingKeys, s)
			}
		}
	}
	// 死亡时间
	if v, ok := table["time"].(time.Time); ok {
		info.Time = v
	}
}

// ============ 补偿链 ============

// AddCompensationHandler 添加死信补偿处理器
// 处理器按添加顺序执行——第一个返回 handled=true 的处理器终止后续处理
func (m *DLXManager) AddCompensationHandler(h DeadLetterHandler) {
	m.compensationHandlers = append(m.compensationHandlers, h)
}

// runCompensation 执行补偿链
// 链式调用：handler1 → handler2 → handler3 → ...
// 为什么是链式而不是并发？
// → 补偿操作有优先级——先尝试自动修复（如重试），修复失败再记录告警。
//
//	如果是并发执行，重试和告警同时触发，可能造成混乱
func (m *DLXManager) runCompensation(ctx context.Context, dl DeadLetter) {
	// 如果没有注册任何处理器，至少打印完整死信信息
	if len(m.compensationHandlers) == 0 {
		m.defaultCompensation(ctx, dl)
		return
	}

	for i, handler := range m.compensationHandlers {
		if handler(ctx, dl) {
			log.Printf("[DLX] 🔧 补偿成功 (handler-%d): msgID=%s", i, dl.OriginalMessage.MessageID)
			return
		}
	}
	// 所有 handler 都没处理 → 兜底
	log.Printf("[DLX] ⚠️ 无补偿处理器成功处理: msgID=%s", dl.OriginalMessage.MessageID)
}

// defaultCompensation 默认补偿：序列化记录日志
// 这是最后的兜底——至少死信信息被完整记录下来，方便人工排查
func (m *DLXManager) defaultCompensation(ctx context.Context, dl DeadLetter) {
	b, _ := json.MarshalIndent(dl, "", " ")
	log.Printf("[DLX] 📝 死信详情:\n%s", string(b))
}

// ============ 预置补偿处理器 ============

// LogCompensationHandler 日志记录处理器（始终返回 false，不终止链）
// 为什么返回 false？
// → 日志只是记录，不是真正的"处理"。返回 false 让后续处理器继续执行
func LogCompensationHandler(ctx context.Context, dl DeadLetter) bool {
	log.Printf("[DLX] 💀 死信详情: Reason=%s, OrigQueue=%s, Count=%d, MsgID=%s, Payload=%v",
		dl.DeathInfo.Reason, dl.DeathInfo.Queue, dl.DeathInfo.Count,
		dl.OriginalMessage.MessageID, dl.OriginalMessage.Payload)
	return false // 不终止链
}

// RetryCompensationHandler 重试处理器（把死信重新发回原 Queue）
// 限制重试次数不超过 maxRetry，防止死循环
// 为什么不用 x-death count 来判断重试次数？
// → x-death 是 RabbitMQ 自动注入的——当消息被 Republish（而非 Requeue）后，
//    它是全新的消息，不再有 x-death 历史。消费者拒绝后 RabbitMQ 重新添加
//    x-death，count 始终为 1，导致永远无法触发"超过最大重试"的停止条件。
//    解决方案：用自定义 Header (x-retry-count) 在消息体重试时递增计数
func RetryCompensationHandler(producer *Producer, maxRetry int64) DeadLetterHandler {
	return func(ctx context.Context, dl DeadLetter) bool {
		// 从原始消息 Payload 或 Header 中读取重试计数
		retryCount := getRetryCount(dl)
		if retryCount >= maxRetry {
			log.Printf("[DLX] ❌ 超过最大重试次数(%d/%d): msgID=%s",
				retryCount, maxRetry, dl.OriginalMessage.MessageID)
			return false // 交给下一个 handler（如告警）
		}

		// 递增重试计数并写入消息 Payload
		setRetryCount(&dl.OriginalMessage, retryCount+1)

		// 重新发回原始 Exchange
		err := producer.Publish(ctx, dl.DeathInfo.RoutingKeys[0], dl.OriginalMessage)
		if err != nil {
			log.Printf("[DLX] ⚠️ 重试发布失败: %v", err)
			return false
		}

		log.Printf("[DLX] 🔄 已重试: msgID=%s, attempt=%d/%d",
			dl.OriginalMessage.MessageID, retryCount+1, maxRetry)
		return true
	}
}

// getRetryCount 从消息 Payload 中提取重试计数
func getRetryCount(dl DeadLetter) int64 {
	if payload, ok := dl.OriginalMessage.Payload.(map[string]any); ok {
		if count, ok := payload["_retry_count"]; ok {
			switch v := count.(type) {
			case float64:
				return int64(v)
			case int64:
				return v
			}
		}
	}
	return 0
}

// setRetryCount 将重试计数写入消息 Payload
func setRetryCount(msg *Message, count int64) {
	if payload, ok := msg.Payload.(map[string]any); ok {
		payload["_retry_count"] = count
	}
}
