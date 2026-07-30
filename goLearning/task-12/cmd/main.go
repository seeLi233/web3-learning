package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/seeLi/go-learning/task-12/internal/mq"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Println("🚀 RabbitMQ 消息队列学习 Demo 启动")

	// ============ 1. 连接 RabbitMQ ============

	conn := mq.NewConnection(mq.Config{
		Host:     "localhost",
		Port:     5672,
		User:     "admin",
		Password: "admin123",
		VHost:    "/",
	})

	if err := conn.Connect(); err != nil {
		log.Fatalf("❌ 连接 RabbitMQ 失败: %v", err)
	}
	defer conn.Close()

	// ============ 2. 创建 DLX 管理器 ============

	dlxManager := mq.NewDLXManager(conn, mq.DLXConfig{
		DLXExchangeName: "order.dlx", // 死信 Exchange
		DLXQueueName:    "order.dlq", // 死信 Queue
		DLXRoutingKey:   "#",         // 接收所有死信
		TTL:             10000,       // 消息 10 秒未消费 → 变死信（仅演示用，生产设大些）
		MaxLength:       100,         // Queue 最多 100 条消息
	})

	ctx := context.Background()

	// 创建 DLX 拓扑
	if err := dlxManager.Setup(ctx); err != nil {
		log.Fatalf("❌ 创建 DLX 拓扑: %v", err)
	}

	// ============ 3. 创建 Producer ============

	producer, err := mq.NewProducer(conn, "order.events")
	if err != nil {
		log.Fatalf("❌ 创建 Producer: %v", err)
	}

	// ============ 4. 声明业务 Queue（带 DLX 参数） ============
	// 为什么业务 Queue 的声明放在 main 而不是 Producer.Declare？
	// → 业务 Queue 需要 DLX 参数（DeadLetterArgs），这个参数来自 DLXManager，
	//    在 main 中组装最合适——各个组件的耦合只在 main 层

	declareCfg := mq.DeclareConfig{
		ExchangeName: "order.events",
		ExchangeType: "topic",
		Durable:      true,
		QueueName:    "order.queue",
		QueueDurable: true,
		RoutingKey:   "order.created",
	}

	// 先创建 Producer 的基础拓扑（Exchange + Queue + Binding）
	// 但要覆盖 Queue 的 DLX 参数——需要直接操作 Channel
	ch, err := conn.Channel(ctx)
	if err != nil {
		log.Fatalf("❌ 获取 Channel: %v", err)
	}

	// 声明 Exchange
	if err := ch.ExchangeDeclare(declareCfg.ExchangeName, declareCfg.ExchangeType, true, false, false, false, nil); err != nil {
		log.Fatalf("❌ Exchange: %v", err)
	}

	// 声明 Queue（带 DLX 参数）
	// 这里是关键：args 参数来自 dlxManager.DeadLetterArgs()，
	// 把 x-dead-letter-exchange 等参数注入到业务 Queue 中
	dlxArgs := dlxManager.DeadLetterArgs()
	if _, err := ch.QueueDeclare(declareCfg.QueueName, true, false, false, false, dlxArgs); err != nil {
		log.Fatalf("❌ Queue: %v", err)
	}

	// 绑定
	if err := ch.QueueBind(declareCfg.QueueName, declareCfg.RoutingKey, declareCfg.ExchangeName, false, nil); err != nil {
		log.Fatalf("❌ Binding: %v", err)
	}

	log.Println("✅ 业务拓扑已创建（带 DLX 参数）")

	// ============ 5. 创建 Consumer（正常消息处理） ============

	// 正常消息处理器：订单创建成功返回 nil，其他类型的消息返回 error 模拟处理失败
	// 为什么这样设计？
	// → 真实场景中不同消息类型的处理逻辑不同——有的成功有的失败
	//    这里用 Type 字段区分，演示正常 Ack 和 Nack→死信两条路径
	normalHandler := func(ctx context.Context, msg mq.Message) error {
		log.Printf("[Handler] 🔧 处理消息: Type=%s, ID=%s, Payload=%v", msg.Type, msg.MessageID, msg.Payload)

		// 模拟处理时间
		time.Sleep(50 * time.Millisecond)

		// 类型为 "order.fail" 的消息模拟处理失败
		if msg.Type == "order.fail" {
			return fmt.Errorf("模拟订单处理失败: %v", msg.Payload)
		}

		log.Printf("[Handler] ✅ 订单处理成功: %s", msg.MessageID)
		return nil
	}

	consumer := mq.NewConsumer(conn, mq.ConsumeConfig{
		QueueName:    "order.queue",
		ExchangeName: "order.events",
		ExchangeType: "topic",
		RoutingKey:   "order.created",
		Durable:      true,
		ConsumerName: "order-processor-1",
		Concurrency:  2, // 2 个 worker 并发处理
	}, normalHandler)

	// ============ 6. 启动正常消费者（后台） ============
	// 放在 goroutine 里，主线程继续发送消息

	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()

	go func() {
		if err := consumer.Start(consumerCtx); err != nil {
			log.Printf("⚠️ 消费者退出: %v", err)
		}
	}()

	// 等消费者就绪
	time.Sleep(500 * time.Millisecond)

	// ============ 7. 发送测试消息 ============

	log.Println("\n📤 ====== 开始发送测试消息 ======")

	// 发送 3 条正常消息 + 2 条会失败的消息
	testMessages := []mq.Message{
		{
			Type:      "order.created",
			MessageID: uuid.NewString(),
			Payload:   map[string]any{"order_id": 1001, "amount": 99.9},
		},
		{
			Type:      "order.created",
			MessageID: uuid.NewString(),
			Payload:   map[string]any{"order_id": 1002, "amount": 199.9},
		},
		{
			Type:      "order.fail", // 这条会触发 Nack → 死信
			MessageID: uuid.NewString(),
			Payload:   map[string]any{"order_id": 9999, "reason": "库存不足"},
		},
		{
			Type:      "order.created",
			MessageID: uuid.NewString(),
			Payload:   map[string]any{"order_id": 1003, "amount": 299.9},
		},
		{
			Type:      "order.fail", // 这条也会触发 Nack → 死信
			MessageID: uuid.NewString(),
			Payload:   map[string]any{"order_id": 8888, "reason": "支付超时"},
		},
	}

	for i, msg := range testMessages {
		log.Printf("[Producer] 📤 发送消息 %d/%d: Type=%s, ID=%s", i+1, len(testMessages), msg.Type, msg.MessageID)
		if err := producer.Publish(ctx, "order.created", msg); err != nil {
			log.Printf("[Producer] ❌ 发送失败: %v", err)
		}
		time.Sleep(200 * time.Millisecond) // 稍微间隔，方便看日志
	}

	log.Println("✅ 所有消息已发送，等待消费者处理...")
	time.Sleep(2 * time.Second) // 等消费者处理完

	// ============ 8. 启动死信消费者 ============

	log.Println("\n💀 ====== 启动死信消费者 ======")

	// 死信处理器：记录 + 自动重试
	dlxManager.AddCompensationHandler(func(ctx context.Context, dl mq.DeadLetter) bool {
		return mq.LogCompensationHandler(ctx, dl)
	})
	dlxManager.AddCompensationHandler(mq.RetryCompensationHandler(producer, 3))

	dlxCtx, dlxCancel := context.WithCancel(context.Background())
	defer dlxCancel()

	go func() {
		if err := dlxManager.StartConsumeDeadLetters(dlxCtx); err != nil {
			log.Printf("⚠️ 死信消费者退出: %v", err)
		}
	}()

	// 等死信消费者处理
	time.Sleep(3 * time.Second)

	// ============ 9. 优雅关闭 ============

	log.Println("\n🛑 ====== 优雅关闭 ======")

	// 先停消费者，让正在处理的消息完成
	consumer.Close()

	// 等待 Ctrl+C 或超时
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("收到信号 %v，正在关闭...", sig)
	case <-time.After(2 * time.Second):
		log.Println("超时，强制关闭")
	}

	dlxCancel()
	log.Println("👋 Demo 结束")
}
