package mq

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ============ 核心类型定义 ============

// Config RabbitMQ 连接配置
// 为什么用 struct 而不是直接传参数？
// → 连接参数多（地址/用户/密码/重试），struct 便于扩展——以后加 TLS/心跳配置不用改函数签名
type Config struct {
	Host     string // RabbitMQ 地址，如 localhost
	Port     int    // AMQP 端口，默认 5672
	User     string // 用户名
	Password string // 密码
	VHost    string // 虚拟主机，默认 "/"，用于多租户隔离
}

// DSN 生成 AMQP 连接字符串
// 作用：把零散的配置字段拼接成 amqp://user:pass@host:port/vhost 格式
// 为什么是方法而不是函数？绑定到 Config 上，调用方只需 cfg.DSN()，语义更清晰
func (c *Config) DSN() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		c.User, c.Password, c.Host, c.Port, c.VHost,
	)
}

// Connection RabbitMQ 连接管理器
// 为什么需要这个封装而不是直接用 amqp.Dial？
// → 生产环境需要处理断线重连——直接用 amqp.Dial 断连后 Channel 全部失效，
//
//	Connection 封装了重连逻辑和并发安全的 Channel 获取
//
// 为什么只存 conn 不存 ch？
// → RabbitMQ 的设计：一个 TCP 连接可以有多个独立的 Channel。
//
//	Producer 需要 Confirm 模式的 Channel，Consumer 需要 Consume 模式的 Channel，
//	DLX Manager 也需要独立 Channel——它们不能共用同一个 Channel。
//	每次调用 Channel() 都创建新 Channel，各组件互不干扰
type Connection struct {
	config      Config           // 连接配置，重连时需要复用
	conn        *amqp.Connection // 底层 TCP 连接
	mu          sync.RWMutex     // 读写锁保护 conn 的并发访问
	reconnect   bool             // 是否启用自动重连
	done        chan struct{}    // 关闭信号——为什么用 chan struct{} 而不是 chan bool？ → struct{} 不占内存（0 字节），用作信号通道最省资源
	reconnectCh chan struct{}    // 重连完成通知——close(channel) 会唤醒所有阻塞的读取者
}

// ============ 构造函数 ============

// NewConnection 创建连接管理器
// 作用：初始化 Connection 结构体，但不建立实际连接（延迟到 Connect() 调用时连接）
// 为什么不在构造函数里直接连接？
// → 分离"创建对象"和"建立连接"两个关注点。
//
//	构造函数只做配置，Connect() 做连接——如果连接失败，调用者可以拿到 error 决定重试策略
func NewConnection(cfg Config) *Connection {
	return &Connection{
		config:      cfg,
		done:        make(chan struct{}),
		reconnectCh: make(chan struct{}),
	}
}

// ============ 核心方法 ============

// Connect 建立 RabbitMQ 连接
// 作用：拨号到 RabbitMQ → 启动重连监听
//
// 错误处理策略：连接失败直接返回 error，让上层决定是否重试
// 为什么不在 Connect 里循环重试？
// → 连接失败可能是配置错误（密码错/VHost不存在），此时重试毫无意义。
//
//	运行时断连（网络闪断）才需要自动重连，那个逻辑在 notifyReconnect 里
func (c *Connection) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error

	// 1. 建立 TCP 连接
	c.conn, err = amqp.Dial(c.config.DSN())
	if err != nil {
		return fmt.Errorf("amqp dial: %w", err)
	}

	c.reconnect = true

	// 2. 启动后台重连监听 goroutine
	go c.notifyReconnect()

	log.Println("[MQ] ✅ 连接成功")
	return nil
}

// ============ Channel 获取（每次都创建新 Channel） ============

// Channel 创建新的 AMQP Channel（带 QoS 配置和重连等待）
// 为什么每次都创建新 Channel 而不是复用同一个？
// → RabbitMQ 的 Channel 是轻量级的（一个连接可以有几万个 Channel）。
//
//	不同操作需要不同模式的 Channel：
//	- Producer 需要 Confirm 模式（ch.Confirm(false)）
//	- Consumer 需要 Consume 模式（ch.Consume(...)）
//	- DLX Manager 需要独立的 Channel 做拓扑声明和死信消费
//	它们不能共用——Confirm 模式的 Channel 无法 Consume
//
// 每个 Channel 自动配置 QoS(PrefetchCount=1)：
// → 防止慢消费者积压，保证公平调度
//
// ctx 参数的作用：支持超时控制。如果重连一直没完成，ctx 超时后返回 error
func (c *Connection) Channel(ctx context.Context) (*amqp.Channel, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	// 快速路径：连接正常，直接创建新 Channel
	if conn != nil && !conn.IsClosed() {
		ch, err := conn.Channel()
		if err != nil {
			return nil, fmt.Errorf("创建 Channel: %w", err)
		}
		// 每个 Channel 独立设置 QoS
		if err := ch.Qos(1, 0, false); err != nil {
			ch.Close()
			return nil, fmt.Errorf("设置 QoS: %w", err)
		}
		return ch, nil
	}

	// 慢速路径：连接不可用（正在重连中），等待重连完成或 ctx 超时
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("等待重连超时: %w", ctx.Err())
	case <-c.reconnectCh:
		// 重连完成，重新获取连接并创建 Channel
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()
		if conn == nil || conn.IsClosed() {
			return nil, fmt.Errorf("重连后连接仍不可用")
		}
		ch, err := conn.Channel()
		if err != nil {
			return nil, fmt.Errorf("重连后创建 Channel: %w", err)
		}
		if err := ch.Qos(1, 0, false); err != nil {
			ch.Close()
			return nil, fmt.Errorf("重连后设置 QoS: %w", err)
		}
		return ch, nil
	}
}

// ============ 自动重连 ============

// notifyReconnect 后台监听连接关闭事件，自动重连
// 为什么用 goroutine 后台运行？
// → 连接断开是异步事件（网络闪断/服务重启），不能阻塞业务线程。
//
//	后台 goroutine 监听到断连后自动重连，业务层通过 Channel() 的 ctx 机制
//	等待重连完成
func (c *Connection) notifyReconnect() {
	// NotifyClose 注册一个 channel，连接关闭时会发送错误到这个 channel
	closeCh := c.conn.NotifyClose(make(chan *amqp.Error))

	for {
		select {
		case <-c.done:
			// 主动关闭，退出重连循环
			return
		case err, ok := <-closeCh:
			if !ok {
				// Channel 被关闭了（正常关闭也会触发），退出
				return
			}
			log.Printf("[MQ] ⚠️ 连接断开: %v，开始重连...", err)

			// 重连循环：最多重试 60 次，每次间隔 2 秒
			// 为什么不是无限重试？
			// → 如果 RabbitMQ 挂了 10 分钟，无限重试会刷屏日志也毫无意义。
			//    有限重试 + 告警是更好的策略
			for i := 0; i < 60; i++ {
				select {
				case <-c.done:
					return
				default:
				}

				if newErr := c.reconnectOnce(); newErr != nil {
					log.Printf("[MQ] 重连失败 (%d/60): %v", i+1, newErr)
					time.Sleep(2 * time.Second)
					continue
				}

				// 重连成功：通知所有等待 Channel 的 goroutine
				// close(c.reconnectCh) 会唤醒所有阻塞在 <-c.reconnectCh 的 goroutine
				// 为什么 close 而不是发信号？
				// → 发信号只能唤醒一个等待者；close channel 后所有读取者都会收到零值
				//    但 close 后的 channel 不能再发信号，所以需要重建 reconnectCh
				close(c.reconnectCh)
				c.reconnectCh = make(chan struct{})

				// 重新注册断连通知（新连接有新 closeCh）
				closeCh = c.conn.NotifyClose(make(chan *amqp.Error))
				log.Println("[MQ] ✅ 重连成功")
				break
			}
		}
	}
}

// reconnectOnce 执行一次重连尝试
// 作用：Dial 新连接即可（不再创建单 Channel，由 Channel() 按需创建）
func (c *Connection) reconnectOnce() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := amqp.Dial(c.config.DSN())
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	// 原子替换：先关旧的，再赋新的
	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = conn

	return nil
}

// ============ 资源管理 ============

// Close 优雅关闭连接
// close(c.done) 的作用：通知 notifyReconnect goroutine 退出
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	close(c.done) // 通知重连 goroutine 退出

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("关闭连接: %w", err)
		}
	}

	log.Println("[MQ] 🔌 连接已关闭")
	return nil
}
