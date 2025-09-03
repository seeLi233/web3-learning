package goroutine

import (
	"fmt"
	"sync"
	"time"
)

// Goroutine
// 1.题目 ：编写一个程序，使用 go 关键字启动两个协程，一个协程打印从1到10的奇数，另一个协程打印从2到10的偶数。
// 打印奇数
func odd() {
	for i := 1; i < 10; i += 2 {
		fmt.Println("输出奇数：", i)
		// 短暂睡眠，这样证明两个协程同时在运作
		time.Sleep(100 * time.Millisecond)
	}
}

// 打印偶数
func even() {
	for i := 2; i <= 10; i += 2 {
		fmt.Println("输出偶数：", i)
		// 短暂睡眠，这样证明两个协程同时在运作
		time.Sleep(100 * time.Millisecond)
	}
}

// 2.题目 ：设计一个任务调度器，接收一组任务（可以用函数表示），并使用协程并发执行这些任务，同时统计每个任务的执行时间。
// 定义任务类型
type Task func()

// 存储任务执行结果
type TaskResult struct {
	taskId       int
	startTime    time.Time
	endTime      time.Time
	durationTime time.Duration
	success      bool
}

// 任务调度器
type Scheduler struct {
	tasks       []Task
	taskResults []TaskResult
	wg          sync.WaitGroup
	mutex       sync.Mutex
	taskCount   int
}

// 创建任务调度器
func newScheduler() *Scheduler {
	return &Scheduler{
		tasks:       make([]Task, 0),
		taskResults: make([]TaskResult, 0),
	}
}

// 将任务添加到任务调度器
func (s *Scheduler) addTaskToScheduler(task Task) {
	s.tasks = append(s.tasks, task)
}

func (s *Scheduler) excuteScheduler(id int, task Task) {
	defer s.wg.Done()
	// 记录开始时间
	startTime := time.Now()

	success := true

	func() {
		defer func() {
			if r := recover(); r != nil {
				success = false
				fmt.Println("执行任务出错")
			}
		}()
		task()
	}()

	endTime := time.Now()
	durationTime := endTime.Sub(startTime)

	s.mutex.Lock()
	s.taskResults[id] = TaskResult{
		taskId:       id,
		startTime:    startTime,
		endTime:      endTime,
		durationTime: durationTime,
		success:      success,
	}
	s.mutex.Unlock()
}

func (s *Scheduler) printSchedulerResult() {

	var totalDurationTime time.Duration
	for index, result := range s.taskResults {
		fmt.Printf("任务 %d, 执行时间： %v, 开始时间： %v,结束时间：%v \n", index, result.durationTime, result.startTime.Format("15:04:05.000"), result.endTime.Format("15:04:05.000"))
		totalDurationTime += result.durationTime
	}

	fmt.Printf("任务执行总持续时间：%v \n", totalDurationTime)
}

func (s *Scheduler) run() {
	s.taskCount = len(s.tasks)
	s.taskResults = make([]TaskResult, s.taskCount)

	s.wg.Add(s.taskCount)

	for index, task := range s.tasks {
		go s.excuteScheduler(index, task)
	}

	s.wg.Wait()
}

func GoroutineTest() {
	// 测试 Goroutine 题目1
	// 为了看到结果，使用 sync 进行等待接收
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		odd()
	}()

	go func() {
		defer wg.Done()
		even()
	}()
	wg.Wait()

	// 测试 Goroutine 题目2
	scheduler := newScheduler()

	// 添加一些示例任务
	scheduler.addTaskToScheduler(func() {
		fmt.Println("任务 0: 开始执行")
		time.Sleep(2 * time.Second) // 模拟耗时操作
		fmt.Println("任务 0: 执行完成")
	})

	scheduler.addTaskToScheduler(func() {
		fmt.Println("任务 1: 开始执行")
		time.Sleep(1 * time.Second) // 模拟耗时操作
		fmt.Println("任务 1: 执行完成")
	})

	scheduler.addTaskToScheduler(func() {
		fmt.Println("任务 2: 开始执行")
		time.Sleep(3 * time.Second) // 模拟耗时操作
		fmt.Println("任务 2: 执行完成")
	})

	scheduler.addTaskToScheduler(func() {
		fmt.Println("任务 3: 开始执行")
		time.Sleep(500 * time.Millisecond) // 模拟耗时操作
		fmt.Println("任务 3: 执行完成")
	})

	// 添加一个会失败的任务
	scheduler.addTaskToScheduler(func() {
		fmt.Println("任务 4: 开始执行")
		time.Sleep(1 * time.Second)
		panic("模拟任务执行失败") // 故意引发panic
	})
	// 执行调度任务
	scheduler.run()
	// 打印结果
	scheduler.printSchedulerResult()
}
