package enhanced

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ==================== Pool ====================

type Pool struct {
	workerCount int
	taskQueue   chan Task
	resultQueue chan Result
	workers     []*Worker
	wg          sync.WaitGroup
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	started     bool
}

// Option — 函数式选项模式
type Option func(*Pool)

func WithWorkers(n int) Option {
	return func(p *Pool) { p.workerCount = n }
}

func WithTaskBuf(n int) Option {
	return func(p *Pool) { p.taskQueue = make(chan Task, n) }
}

func WithResultBuf(n int) Option {
	return func(p *Pool) { p.resultQueue = make(chan Result, n) }
}

func NewPool(opts ...Option) *Pool {
	p := &Pool{
		workerCount: 4,
		taskQueue:   make(chan Task, 100),
		resultQueue: make(chan Result, 100),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// ==================== Start / Submit（error 返回）⭐⭐ ====================

func (p *Pool) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return fmt.Errorf("Worker Pool 已经启动")
	}

	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.workers = make([]*Worker, p.workerCount)

	for i := 0; i < p.workerCount; i++ {
		w := NewWorker(i+1, p.taskQueue, p.resultQueue)
		p.workers[i] = w
		p.wg.Add(1)
		go func(worker *Worker) {
			defer p.wg.Done() // ⭐ defer 标记完成
			worker.Run(p.ctx)
		}(w)
	}

	p.started = true
	fmt.Printf("🚀 增强版 Pool 启动: %d workers\n", p.workerCount)
	return nil
}

// Submit 提交任务（⭐ 返回 error，不 panic！）
func (p *Pool) Submit(task Task) error {
	if task == nil {
		return fmt.Errorf("不能提交 nil 任务") // ⭐ 输入校验 → error
	}

	select {
	case p.taskQueue <- task:
		return nil
	case <-p.ctx.Done():
		// ⭐ context 取消 → 返回明确错误，调用方可以判断
		return fmt.Errorf("Pool 正在关闭，任务 %s 被拒绝", task.ID())
	default:
		// ⭐ 队列满 → 返回错误，不阻塞
		return fmt.Errorf("任务队列已满，任务 %s 被拒绝", task.ID())
	}
}

// Results 返回结果 channel（只读）
func (p *Pool) Results() <-chan Result {
	return p.resultQueue
}

// Stats 返回统计
type Stats struct {
	Workers   int
	QueueLen  int
	ResultLen int
	Running   bool
}

func (p *Pool) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock() // ⭐ defer 释放锁

	var completed, failed int64
	for _, w := range p.workers {
		c, f := w.Stats()
		completed += c
		failed += f
	}
	return Stats{
		Workers:   p.workerCount,
		QueueLen:  len(p.taskQueue),
		ResultLen: len(p.resultQueue),
		Running:   p.started,
	}
}

// ==================== Shutdown（带超时）⭐⭐ ====================

func (p *Pool) Shutdown(timeout time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock() // ⭐ defer 释放锁

	if !p.started {
		return fmt.Errorf("Pool 未启动")
	}

	fmt.Println("🛑 优雅关闭中...")

	// 1. 通知所有 worker 停止接受新任务
	p.cancel()

	// 2. 关闭任务队列（worker 会处理完剩余任务再退出）
	close(p.taskQueue)

	// 3. 等待所有 worker 完成（带超时）
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("✅ 所有 Worker 正常退出")
	case <-time.After(timeout):
		log.Printf("⚠️ 关闭超时(%v)，强制停止", timeout)
		for _, w := range p.workers {
			w.Stop()
		}
		p.wg.Wait() // 再等一次
	}

	// 4. 关闭结果队列
	close(p.resultQueue)
	p.started = false
	return nil
}

// ==================== Demo ====================

func Demo() {
	fmt.Println("====================================")
	fmt.Println(" Day 34: 增强版 Worker Pool")
	fmt.Println(" interface + error + defer")
	fmt.Println("====================================")
	fmt.Println()

	// —— Part A: 接口多态 ——
	fmt.Println("--- Part A: 接口多态 ---")
	fmt.Println("了解原则：只要实现了 Task 接口的三个方法，就是 Task 类型")

	// 验证：所有任务类型都满足 Task 接口（编译时检查）
	var _ Task = (*ComputeTask)(nil) // ComputeTask 满足 Task ✅
	var _ Task = (*FailingTask)(nil) // FailingTask 满足 Task ✅
	var _ Task = (*PanicTask)(nil)   // PanicTask 满足 Task ✅
	var _ Task = (*SlowTask)(nil)    // SlowTask 满足 Task ✅
	fmt.Println("✅ 编译时检查通过: 4 种类型都满足 Task 接口")
	fmt.Println()

	// —— Part B: 启动 Pool ——
	fmt.Println("--- Part B: 创建并启动 Pool ---")
	pool := NewPool(
		WithWorkers(3),
		WithTaskBuf(10),
		WithResultBuf(10),
	)

	// ⭐ Start 返回 error，必须检查
	if err := pool.Start(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}

	// —— Part C: 提交多种任务（接口多态） ——
	fmt.Println()
	fmt.Println("--- Part C: 提交多种类型任务 ---")

	// 正常计算任务（3 个）
	for i := 1; i <= 3; i++ {
		err := pool.Submit(&ComputeTask{
			TaskID:   fmt.Sprintf("calc-%d", i),
			TaskName: fmt.Sprintf("计算 Fib(%d)", i*10),
			N:        i * 10,
		})
		if err != nil {
			fmt.Printf("❌ 提交失败: %v\n", err)
		}
	}

	// 会失败的任务
	err := pool.Submit(&FailingTask{
		TaskID:      "fail-1",
		TaskName:    "模拟失败",
		FailMessage: "数据库连接超时",
	})
	if err != nil {
		fmt.Printf("❌ 提交失败: %v\n", err)
	}

	// 会 panic 的任务（defer recover 会兜底）
	err = pool.Submit(&PanicTask{
		TaskID:   "panic-1",
		TaskName: "触发 Panic",
	})
	if err != nil {
		fmt.Printf("❌ 提交失败: %v\n", err)
	}

	// 提交 nil 任务 → 验证 error 返回
	err = pool.Submit(nil)
	fmt.Printf("提交 nil: err=%v\n", err) // 预期：不能提交 nil 任务

	fmt.Println()

	// —— Part D: 收集结果 ——
	fmt.Println("--- Part D: 收集结果 ---")
	var results []Result
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done() // ⭐ defer 保证 Done
		for r := range pool.Results() {
			results = append(results, r)
		}
	}()

	// —— Part E: 关闭 ——
	time.Sleep(2 * time.Second) // 等任务执行完
	fmt.Println()

	stats := pool.Stats()
	fmt.Printf("📊 统计: Workers=%d, Queue=%d, Results=%d, Running=%v\n", stats.Workers, stats.QueueLen, stats.ResultLen, stats.Running)

	if err := pool.Shutdown(3 * time.Second); err != nil {
		fmt.Printf("关闭异常: %v\n", err)
	}
	wg.Wait()

	// —— Part F: 总结 ——
	fmt.Println()
	fmt.Println("=== 执行结果汇总 ===")
	success, fail := 0, 0
	for _, r := range results {
		fmt.Println("  " + r.String())
		if r.Error != nil {
			fail++
		} else {
			success++
		}
	}
	fmt.Printf("\n成功: %d, 失败: %d (含 panic 恢复)\n", success, fail)

	fmt.Println()
	fmt.Println("====================================")
	fmt.Println(" Day 34 知识点回顾:")
	fmt.Println(" 1️⃣  interface: 4 种 Task 实现，无需 implements")
	fmt.Println(" 2️⃣  error 处理: Submit 返回 error, Execute 返回 error")
	fmt.Println(" 3️⃣  defer: LIFO 顺序 / 参数快照 / panic recover / 锁释放")
	fmt.Println("====================================")
}
