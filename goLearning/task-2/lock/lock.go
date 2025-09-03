package lock

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// 1.题目 ：编写一个程序，使用 sync.Mutex 来保护一个共享的计数器。启动10个协程，每个协程对计数器进行1000次递增操作，最后输出计数器的值。
type Counter struct {
	mu    sync.Mutex
	value int
}

func (counter *Counter) increment() {
	counter.mu.Lock()
	defer counter.mu.Unlock()
	counter.value++
}

func lockOne() {
	counter := Counter{}

	var wg sync.WaitGroup

	// 启动10个线程
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < 1000; j++ {
				counter.increment()
			}

		}()
	}
	wg.Wait()
	fmt.Printf("计数器最终输出结果：%d \n", counter.value)
}

// 2.题目 ：使用原子操作（ sync/atomic 包）实现一个无锁的计数器。启动10个协程，每个协程对计数器进行1000次递增操作，最后输出计数器的值。
func lockTwo() {
	var counter int64

	var wg sync.WaitGroup

	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < 1000; j++ {
				atomic.AddInt64(&counter, 1)
			}
		}()
	}

	wg.Wait()
	finalCounter := atomic.LoadInt64(&counter)
	fmt.Printf("原子计数器最终输出结果：%d \n", finalCounter)
}

func LockTest() {
	// 测试题目1
	lockOne()
	// 测试题目2
	lockTwo()
}
