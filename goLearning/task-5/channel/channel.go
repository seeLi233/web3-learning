package channel

import (
	"fmt"
	"time"
)

// ==================== 1. 无缓冲 vs 有缓冲 channel 行为对比 ====================

func UnbufferVsBuffered() {
	fmt.Println("=== 无缓冲 vs 有缓冲 - channel ===")

	// 无缓冲: 发送必须等接收
	fmt.Println("--- 无缓冲 channel ---")
	ch1 := make(chan string)
	go func() {
		time.Sleep(500 * time.Millisecond)
		fmt.Println("  接收者准备接收...")
		msg := <-ch1
		fmt.Printf("  接收者收到: %s\n", msg)
	}()
	start := time.Now()
	fmt.Println("  发送者阻塞等待...")
	ch1 <- "无缓冲消息"
	fmt.Printf("  发送者解除阻塞 (等待了 %v)\n\n", time.Since(start))

	// 有缓冲: 缓冲未满时发送立即返回
	fmt.Println("--- 有缓冲 channel (cap=3) ---")
	ch2 := make(chan string, 3)
	start = time.Now()
	ch2 <- "消息1"
	ch2 <- "消息2"
	ch2 <- "消息3"
	fmt.Printf("  发送3条消息耗时: %v (几乎瞬间)\n", time.Since(start))

	// 第4条会阻塞（缓冲区满了）
	go func() {
		time.Sleep(500 * time.Millisecond)
		<-ch2 // 取走一条，腾出空间
	}()
	start = time.Now()
	ch2 <- "消息4" // 阻塞，直到上面取走一条
	fmt.Printf("  第4条消息耗时: %v (因为要等缓冲区有空位)\n\n", time.Since(start))
}

// ==================== 2. close(channel) 的行为验证 ====================

func CloseBehavior() {
	fmt.Println("=== close(channel) - 行为验证 ===")

	// 验证1: 从已关闭的 channel 读取（还有数据时）
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	ch <- 3
	close(ch)

	fmt.Println("从已关闭但有数据的 channel 读取:")
	for i := 0; i < 5; i++ {
		v, ok := <-ch
		fmt.Printf("  <-ch = %d, ok = %v\n", v, ok)
	}
	// 前3次: v=1/2/3, ok=true
	// 后2次: v=0, ok=false（零值 + 失败标记）

	// 验证2: 向已关闭的 channel 发送 → panic
	fmt.Println("\n向已关闭的 channel 发送会 panic:")
	// ch <- 4  // 取消注释会 panic: send on closed channel

	// 验证3: 反复 close → panic
	// close(ch) // 取消注释会 panic: close of closed channel

	fmt.Println("  (panic 演示已注释，取消注释可观察)")
	fmt.Println()
}

// ==================== 3. nil channel 的行为 ====================

func NilChannelBehavior() {
	fmt.Println("=== nil channel - 行为验证 ===")

	// nil channel 的发送和接收都会永久阻塞
	// 但 select 中 nil channel 的 case 永远不会被选中（不是阻塞，而是跳过）

	var nilCh chan int

	// 演示：在 select 中使用 nil channel
	ch := make(chan string, 1)
	ch <- "hello"

	for i := 0; i < 3; i++ {
		select {
		case msg := <-ch:
			fmt.Printf("  从 ch 收到: %s\n", msg)
		case v := <-nilCh:
			fmt.Printf("  永远不会执行: %d\n", v)
		default:
			fmt.Println("  default 分支执行（因为 nilCh 无法就绪）")
		}
		time.Sleep(100 * time.Millisecond)
	}
	// 第一次: ch 有数据，执行 ch case
	// 后续: ch 空了，nilCh 永远不就绪，走 default

	fmt.Println()
}

// ==================== 4. select 随机性验证 ====================

func SelectFairness() {
	fmt.Println("=== select 多路就绪时的随机选择 ===")

	ch1 := make(chan int, 10)
	ch2 := make(chan int, 10)

	// 两个 channel 都有数据
	for i := 0; i < 10; i++ {
		ch1 <- 1
		ch2 <- 2
	}

	count1, count2 := 0, 0
	for i := 0; i < 10; i++ {
		select {
		case <-ch1:
			count1++
		case <-ch2:
			count2++
		}
	}
	fmt.Printf("  ch1 被选中: %d 次, ch2 被选中: %d 次\n", count1, count2)
	fmt.Println("  注意: 每次运行结果可能不同（伪随机选择）")
}

func ChannelDeepDive() {
	UnbufferVsBuffered()
	CloseBehavior()
	NilChannelBehavior()
	SelectFairness()
	fmt.Println("=== channel 深度解析完成 ===")
}
