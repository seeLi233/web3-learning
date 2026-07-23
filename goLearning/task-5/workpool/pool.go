package workpool

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ==================== Worker Pool 核心实现 ====================

// Job 表示一个工作任务
type Job struct {
	ID      int
	Payload string // 任务数据（实际场景中可以是任意类型）
}

// Result 表示任务的执行结果
type Result struct {
	JobID    int
	WorkerID int
	Output   string
	Err      error
	Duration time.Duration
}

// Pool 是 Worker Pool 的核心结构
type Pool struct {
	workerCount int
	jobs        chan Job    // 任务输入 channel
	results     chan Result // 结果输出 channel
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// New 创建一个新的 Worker Pool
// numWorkers: worker 数量
// jobQueueSize: 任务队列缓冲区大小
func New(numWorkers int, jobQueueSize int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		workerCount: numWorkers,
		jobs:        make(chan Job, jobQueueSize),
		results:     make(chan Result, jobQueueSize),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// worker 是每个 worker goroutine 的执行逻辑
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			// Pool 被关闭，退出
			fmt.Printf("  [Worker %d] 收到退出信号，停止工作\n", id)
			return
		case job, ok := <-p.jobs:
			if !ok {
				// jobs channel 已关闭
				fmt.Printf("  [Worker %d] jobs channel 关闭，退出\n", id)
				return
			}

			// 执行任务
			start := time.Now()
			fmt.Printf("  [Worker %d] 开始处理 Job #%d: %s\n", id, job.ID, job.Payload)

			// 模拟实际工作
			output, err := p.processJob(job)

			duration := time.Since(start)
			fmt.Printf("  [Worker %d] 完成 Job #%d (耗时 %v)\n", id, job.ID, duration)

			// 发送结果（带 context 保护，防止发送阻塞时无法退出）
			select {
			case p.results <- Result{
				JobID:    job.ID,
				WorkerID: id,
				Output:   output,
				Err:      err,
				Duration: duration,
			}:
			case <-p.ctx.Done():
				return
			}
		}

	}
}

// processJob 模拟实际的任务处理逻辑
func (p *Pool) processJob(job Job) (string, error) {
	// 模拟耗时操作
	time.Sleep(time.Duration(500+job.ID%500) * time.Millisecond)

	// 模拟：Payload 为 "error" 时返回错误
	if job.Payload == "error" {
		return "", fmt.Errorf("任务 #%d 执行失败: payload is 'error'", job.ID)
	}
	return fmt.Sprintf("已处理: %s", job.Payload), nil
}

// Start 启动所有 worker
func (p *Pool) Start() {
	fmt.Printf("[Pool] 启动 %d 个 worker\n", p.workerCount)
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Submit 提交一个任务到队列
// 返回 false 表示 Pool 已关闭，任务被拒绝
func (p *Pool) Submit(job Job) bool {
	select {
	case <-p.ctx.Done():
		return false //  Pool 已关闭
	case p.jobs <- job:
		return true
	}
}

// SubmitBatch 批量提交任务
func (p *Pool) SubmitBatch(jobs []Job) (accepted int) {
	for _, job := range jobs {
		if p.Submit(job) {
			accepted++
		}
	}
	return
}

// Results 返回结果只读 channel
func (p *Pool) Results() <-chan Result {
	return p.results
}

// CollectResults 收集所有结果（阻塞直到 results channel 关闭）
// 调用前需要先 Close() 等待 workers 完成
func (p *Pool) CollectResults() []Result {
	var results []Result
	for r := range p.results {
		results = append(results, r)
	}
	return results
}

// Shutdown 优雅关闭 Pool
// 流程: 关闭 jobs channel → 等待所有 worker 完成 → 关闭 results channel
func (p *Pool) Shutdown() {
	fmt.Println("[Pool] 开始优雅关闭...")
	close(p.jobs) // 不再接受新任务，worker 处理完剩余任务后退出
	p.wg.Wait()
	close(p.results)
	fmt.Println("[Pool] 关闭完成")
}

// ShutdownNow 立即取消所有 worker
func (p *Pool) ShutdownNow() {
	fmt.Println("[Pool] 立即取消所有 worker!")
	p.cancel()    // 取消 context
	close(p.jobs) // 关闭输入
	p.wg.Wait()   // 等待 worker 退出
	close(p.results)
	fmt.Println("[Pool] 强制关闭完成")
}

// ==================== 使用示例 ====================

func Demo() {
	fmt.Println("=== Worker Pool 演示 ===")

	// 创建 Pool: 3 个 worker, 队列大小 10
	pool := New(3, 10)
	pool.Start()

	// 提交 10 个任务
	jobs := make([]Job, 10)
	for i := 0; i < 10; i++ {
		jobs[i] = Job{
			ID:      i + 1,
			Payload: fmt.Sprintf("task-%d", i+1),
		}
	}
	// 添加一个会出错的任务
	jobs = append(jobs, Job{ID: 11, Payload: "error"})

	// 关键：先启动 goroutine 消费 results，再提交任务
	// 否则无缓冲 results channel 会导致 worker 发送结果时阻塞 → 死锁
	var wg sync.WaitGroup
	var results []Result
	wg.Add(1)
	go func() {
		defer wg.Done()
		for r := range pool.Results() {
			results = append(results, r)
		}
	}()

	accepted := pool.SubmitBatch(jobs)
	fmt.Printf("\n提交了 %d 个任务\n\n", accepted)

	// 优雅关闭（关闭 jobs → 等 worker 完成 → 关闭 results）
	pool.Shutdown()
	wg.Wait() // 等结果收集 goroutine 退出
	fmt.Println("\n=== 执行结果 ===")
	successCount, failCount := 0, 0
	for _, r := range results {
		if r.Err != nil {
			failCount++
			fmt.Printf("  ❌ Job #%d (Worker %d): %v\n", r.JobID, r.WorkerID, r.Err)
		} else {
			successCount++
			fmt.Printf("  ✅ Job #%d (Worker %d): %s (耗时 %v)\n", r.JobID, r.WorkerID, r.Output, r.Duration)
		}
	}
	fmt.Printf("\n成功: %d, 失败: %d\n", successCount, failCount)
}
