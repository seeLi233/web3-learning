package enhanced

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

// ==================== Worker ====================

type Worker struct {
	id          int
	taskChan    <-chan Task   // 只读
	resultChan  chan<- Result // 只写
	quit        chan struct{}
	completed   atomic.Int64 // 原子计数：处理了多少任务
	failedCount atomic.Int64 // 原子计数：失败了多少
}

func NewWorker(id int, taskChan <-chan Task, resultChan chan<- Result) *Worker {
	return &Worker{
		id:         id,
		taskChan:   taskChan,
		resultChan: resultChan,
		quit:       make(chan struct{}),
	}
}

// ==================== Run（defer 教学重点！）⭐⭐⭐ ====================

func (w *Worker) Run(ctx context.Context) {
	// ⭐ defer 用法 1: 退出时打印统计（LIFO — 最后注册，但第二个执行）
	defer func() {
		fmt.Printf("[Worker-%d] 👋 退出，共处理 %d 个任务，失败 %d\n", w.id, w.completed.Load(), w.failedCount.Load())
	}()

	// ⭐ defer 用法 2: 全局 panic 兜底（LIFO — 第一个注册，最后执行）
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Worker-%d] 🔥 自身 PANIC: %v", w.id, r)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.quit:
			return
		case task, ok := <-w.taskChan:
			if !ok {
				return
			}
			result := w.process(task)
			w.completed.Add(1)
			if result.Error != nil {
				w.failedCount.Add(1)
			}

			// 发送结果（非阻塞，满了就丢弃）
			select {
			case w.resultChan <- result:
			default:
				log.Printf("[Worker-%d] ⚠️ 结果队列满，丢弃 %s", w.id, task.ID())
			}
		}
	}
}

// ==================== process（单个任务，defer 三层包裹）⭐⭐⭐ ====================

func (w *Worker) process(task Task) (r Result) {
	start := time.Now()
	r = Result{
		TaskID:    task.ID(),
		TaskName:  task.Name(),
		WorkerID:  w.id,
		StartTime: start,
	}

	// ⭐ defer 1: 记录耗时（LIFO — 第 3 个注册，第 1 个执行）
	defer func() {
		r.EndTime = time.Now()
		r.Duration = r.EndTime.Sub(r.StartTime)
		fmt.Printf("[Worker-%d] ⏱️  %s 耗时 %v\n", w.id, task.ID(), r.Duration)
	}()

	// ⭐ defer 2: 单个任务的 panic 恢复（LIFO — 第 2 个注册，第 2 个执行）
	defer func() {
		if rec := recover(); rec != nil {
			r.Error = fmt.Errorf("任务 panic: %v", rec)
			// ⚠️ 注意：这里没有直接访问 r，因为 r 是命名返回值，defer 可以修改它！
		}
	}()

	// ⭐ defer 3: 演示参数快照（LIFO — 第 1 个注册，第 3 个执行）
	// 关键：taskName 在 defer 注册时就"快照"了，即使后面 task 变了也不会变
	taskName := task.Name()
	defer func() {
		fmt.Printf("[Worker-%d] 📸 参数快照验证: 任务名确定是 %q（不是闭包引用）\n", w.id, taskName)
	}()

	// 实际执行
	output, err := task.Execute(context.Background())
	r.Output = output
	r.Error = err
	return
	// ⚠️ 注意：命名返回值 + defer 的组合：
	// 1. return 语句先把 output/err 赋给 r.Output/r.Error
	// 2. 然后按 LIFO 顺序执行所有 defer（可以修改 r！）
	// 3. 最后函数返回最终的 r
}

func (w *Worker) Stop() {
	close(w.quit)
}

// Stats 返回 worker 统计
func (w *Worker) Stats() (completed, failed int64) {
	return w.completed.Load(), w.failedCount.Load()
}
