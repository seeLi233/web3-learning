package demo

import (
	"sync/atomic"
	"time"
)

const (
	mutexLocked      = 1 << iota // 0bit: 上锁标记 1
	mutexWaking                  // 1bit: 唤醒中标记 2
	mutexStarving                // 2bit: 饥饿标记 4
	mutexWaiterShift = iota      // 3bit开始等待者数量
)

type MyMutex struct {
	state int32
	sema  chan struct{} // 简易信号量, 代替 runtime_Semacquire/Semrelease
}

func NewMyMutex() *MyMutex {
	// 缓冲chan模拟信号量队列
	return &MyMutex{
		sema: make(chan struct{}, 1000),
	}
}

func (m *MyMutex) Lock() {
	// 快速路径： 没人占用， 直接 cas 上锁
	if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
		return
	}

	iter := 0
	// 慢速排队逻辑
	for {
		old := atomic.LoadInt32(&m.state)
		// 如果锁空闲, 尝试抢锁
		if old&mutexLocked == 0 {
			if atomic.CompareAndSwapInt32(&m.state, old, old|mutexLocked) {
				return
			}
			continue
		}

		// 最多自旋 4 次
		if iter < 4 {
			iter++
			// CPU 空转模拟 runtime_doSpin
			for k := 0; k < 1e4; k++ {
			}
			continue
		}

		// 锁被占用：等待计数 +1
		new := old + (1 << mutexWaiterShift)
		// 更新状态
		if atomic.CompareAndSwapInt32(&m.state, old, new) {
			// 阻塞休眠，等待唤醒
			<-m.sema
		}

		// 被唤醒后循环尝试抢锁
	}
}

func (m *MyMutex) Unlock() {
	// 原子清楚 locked 位
	newState := atomic.AddInt32(&m.state, -mutexLocked)
	if newState == 0 {
		// 无等待着，直接结束
		return
	}

	// 还有等待goroutine，唤醒一个
	// 等待着数量 -1
	atomic.AddInt32(&m.state, (-1 << mutexWaiterShift))
	// 发信号唤醒队首
	m.sema <- struct{}{}
}

func main() {
	mu := NewMyMutex()
	cnt := 0

	for i := 0; i < 5; i++ {
		go func(idx int) {
			for j := 0; j < 1000; j++ {
				mu.Lock()
				cnt++
				mu.Unlock()
			}
			println("goroutine", idx, "done")
		}(i)
	}

	time.Sleep(time.Second)
	println("final cnt:", cnt)
}
