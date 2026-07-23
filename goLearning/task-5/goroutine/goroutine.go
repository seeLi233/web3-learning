package goroutine

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// ==================== 1. GOMAXPROCS 对并发性能的影响 ====================

// GOMAXPROCS 控制同时执行 Go 代码的 OS 线程数（即 P 的数量）
// 默认 = CPU 核心数

func GOMAXPROCSDemo() {
	fmt.Println("=== GOMAXPROCS 演示 ===")
	fmt.Printf("当前 GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))
	fmt.Printf("CPU 核心数: %d\n", runtime.NumCPU())

	// 临时改为 1，观察并发变成"串行交错"
	old := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(old)

	fmt.Println("\n在 GOMAXPROCS=1 下运行 CPU 密集型任务：")
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sum := 0
			for j := 0; j < 10_000_000; j++ {
				sum += j // CPU 密集型操作，一直占用 M
			}
			fmt.Printf("goroutine %d 完成 (sum=%d)\n", id, sum)
		}(i)
	}

	wg.Wait()
	fmt.Printf("总耗时: %v\n\n", time.Since(start))
}

// ==================== 2. goroutine 的调度点 ====================

// goroutine 在以下时机被调度（主动让出 CPU）：
// 1. 函数调用（编译器插入的抢占检查点）
// 2. channel 操作（发送/接收）
// 3. 系统调用（如 time.Sleep）
// 4. 显式调用 runtime.Gosched()

func PreemptionDemo() {
	fmt.Println("=== 抢占调度演示 ===")
	var wg sync.WaitGroup

	// 演示 1: 没有任何函数调用的死循环 → 在 Go 1.14+ 会被信号抢占
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		// Go 1.14+ 引入异步抢占，即使没有函数调用也会被抢占
		// 这里加个 time.Sleep 确保其他 goroutine 能运行
		for i < 1_000_000 {
			i++
		}
		fmt.Println("纯循环 goroutine 完成")
	}()

	// 演示 2: 主动让出 CPU
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			fmt.Printf("Gosched 演示: 第 %d 次\n", i)
			runtime.Gosched() // 显式让出 CPU，让其他 goroutine 运行
		}
	}()

	wg.Wait()
	fmt.Println()
}

// ==================== 3. goroutine 泄漏检测 ====================

// 演示常见的 goroutine 泄漏场景

func LeakDemo() {
	fmt.Println("=== goroutine 泄漏演示 =====")

	// 场景 1: channel 发送方泄漏（没有接收者）
	ch1 := make(chan int)
	go func() {
		ch1 <- 42 // 永远阻塞！没有接收者
		fmt.Println("永远不会执行")
	}()
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("场景1后 goroutine 数: %d (期望: 泄漏了1个)\n", runtime.NumGoroutine())

	// 场景 2: channel 接收方泄漏（没有发送者）
	ch2 := make(chan int)
	go func() {
		<-ch2 // 永远阻塞！没有发送者
		fmt.Println("永远不会执行")
	}()
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("场景2后 goroutine 数: %d (期望: 泄漏了2个)\n", runtime.NumGoroutine())

	// 场景 3: 正确的退出方式 — 使用 done channel
	ch3 := make(chan int)
	done := make(chan struct{})

	go func() {
		fmt.Println("这个 goroutine 会正确退出")
		for {
			select {
			case v := <-ch3:
				fmt.Printf("收到: %d\n", v)
			case <-done:
				fmt.Println("收到退出信号，goroutine 退出")
				return // 正确退出
			}
		}
	}()

	close(done)
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("正确退出后 goroutine 数: %d (之前泄漏的不算)\n", runtime.NumGoroutine())
	fmt.Println()
}

// ==================== 4. 查看 goroutine 调度信息 ====================

func SchedulerTrace() {
	fmt.Println("=== 调度器状态 ===")

	// 获取 goroutine 数量
	fmt.Printf("当前 goroutine 总数: %d\n", runtime.NumGoroutine())

	// 获取 CPU 核心数
	fmt.Printf("NumCPU: %d\n", runtime.NumCPU())

	// 获取 GOMAXPROCS
	fmt.Printf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))

	// 获取 Go 版本
	fmt.Printf("Go 版本: %s\n", runtime.Version())

	fmt.Println()
}

func GoroutineDeepDive() {
	SchedulerTrace()
	GOMAXPROCSDemo()
	PreemptionDemo()
	LeakDemo()
	fmt.Println("=== goroutine 深度解析完成 ===")
}
