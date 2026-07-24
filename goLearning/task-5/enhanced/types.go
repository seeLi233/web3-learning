package enhanced

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"
)

// ==================== Task 接口（隐式实现）⭐ ====================
// 注意：没有任何类型需要写 "implements Task"
// 只要实现了 Execute / ID / Name 三个方法，就自动是 Task

type Task interface {
	Execute(ctx context.Context) (interface{}, error) // 执行任务
	ID() string                                       // 唯一标识
	Name() string                                     // 任务名
}

// ==================== Result ====================

type Result struct {
	TaskID    string
	TaskName  string
	WorkerID  int
	Output    interface{}
	Error     error
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
}

func (r Result) String() string {
	if r.Error != nil {
		return fmt.Sprintf("[Worker-%d] %s(%s) ❌ 失败: %v (耗时 %v)", r.WorkerID, r.TaskName, r.TaskID, r.Error, r.Duration)
	}
	return fmt.Sprintf("[Worker-%d] %s(%s) ✅ 完成: %v (耗时 %v)", r.WorkerID, r.TaskName, r.TaskID, r.Output, r.Duration)
}

// ==================== 任务类型 1: ComputeTask（正常） ====================

type ComputeTask struct {
	TaskID   string
	TaskName string
	N        int // 计算 fib(N)
}

func (t *ComputeTask) ID() string   { return t.TaskID }
func (t *ComputeTask) Name() string { return t.TaskName }

func (t *ComputeTask) Execute(ctx context.Context) (interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(100+rand.IntN(400)) * time.Millisecond):
		// 模拟计算
		result := fib(t.N)
		return fmt.Sprintf("fib(%d)=%d", t.N, result), nil
	}
}

func fib(n int) int {
	if n <= 1 {
		return n
	}
	a, b := 0, 1
	for i := 2; i <= n; i++ {
		a, b = a, a+b
	}
	return b
}

// ==================== 任务类型 2: FailingTask（error 返回）⭐ ====================

type FailingTask struct {
	TaskID      string
	TaskName    string
	FailMessage string
}

func (t *FailingTask) ID() string   { return t.TaskID }
func (t *FailingTask) Name() string { return t.TaskName }

func (t *FailingTask) Execute(ctx context.Context) (interface{}, error) {
	// ⭐ 业务失败 → 返回 error，不 panic
	time.Sleep(100 * time.Millisecond)
	return nil, fmt.Errorf("失败: %s", t.FailMessage)
}

// ==================== 任务类型 3: PanicTask（defer recover 捕获）⭐⭐ ====================

type PanicTask struct {
	TaskID   string
	TaskName string
}

func (t *PanicTask) ID() string   { return t.TaskID }
func (t *PanicTask) Name() string { return t.TaskName }

func (t *PanicTask) Execute(ctx context.Context) (interface{}, error) {
	time.Sleep(50 * time.Millisecond)
	panic("💥 PanicTask 故意 panic！") // worker 的 defer recover 会兜底
	return nil, nil
}

// ==================== 任务类型 4: SlowTask（响应 context 取消）⭐ ====================

type SlowTask struct {
	TaskID   string
	TaskName string
	SleepFor time.Duration
}

func (t *SlowTask) ID() string   { return t.TaskID }
func (t *SlowTask) Name() string { return t.TaskName }

func (t *SlowTask) Execute(ctx context.Context) (interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("任务被取消: %w", ctx.Err()) // ⭐ %w 包装错误链
	case <-time.After(t.SleepFor):
		return fmt.Sprintf("睡 %v 后醒来", t.SleepFor), nil
	}
}
