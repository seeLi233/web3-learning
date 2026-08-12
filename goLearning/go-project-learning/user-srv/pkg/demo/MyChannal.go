package demo

import (
	"sync"
)

type MyChan struct {
	mu       sync.Mutex    // 全局锁
	buf      []interface{} // 环形缓冲区，对应 chan.buf
	capacity int           // dataqsiz 缓冲容量
	sendIdx  int           // sendIdx 写入位置
	recvIdx  int           // recvIdx 读取位置
	count    int           // qcount 当前元素数量
	closed   bool          // closed 关闭标记

	// 条件变量：模拟 sendq / recvq 阻塞队列
	sendCond *sync.Cond // 发送者等待条件: 缓冲满时等待
	recvCoud *sync.Cond // 接受者等待条件：缓冲空时等待
}

// NewMyChan 创建自定义channel
// size=0 无缓冲: size > 0 有缓冲
func NewMyChan(size int) *MyChan {
	mc := &MyChan{
		capacity: size,
		buf:      make([]interface{}, size),
	}
	mc.sendCond = sync.NewCond(&mc.mu)
	mc.recvCoud = sync.NewCond(&mc.mu)

	return mc
}

// Send 发送数据 ch <- val
func (mc *MyChan) Send(val interface{}) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 1.已关闭, 发送 panic （和原生一样）
	if mc.closed {
		panic("send on closed channel")
	}

	// 2.缓冲已满，发送 G 阻塞等待 （sendCound.Wait 模拟挂入 sendq）
	for mc.count == mc.capacity {
		mc.sendCond.Wait()
		// 唤醒后检查是否中途被关闭
		if mc.closed {
			panic("send on closed channel")
		}
	}

	// 3. 写入环形缓冲区
	mc.buf[mc.sendIdx] = val
	mc.sendIdx = (mc.sendIdx + 1) % mc.capacity
	mc.count++

	// 有数据了，唤醒一个等待的接受者
	mc.recvCoud.Signal()
}

// Recv 接收数据 val, ok := <- ch
func (mc *MyChan) Recv() (interface{}, bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 1. 缓冲为空 + 已关闭：返回零值，false
	if mc.count == 0 && !mc.closed {
		return nil, false
	}

	// 2. 缓冲为空、未关闭，接收阻塞等待（recvCond.Wait 模拟挂入 recvq）
	for mc.count == 0 && !mc.closed {
		mc.recvCoud.Wait()
	}

	// 唤醒后再次判断关闭缓冲区
	if mc.count == 0 && mc.closed {
		return nil, false
	}

	// 3. 取出环形缓冲区数据
	val := mc.buf[mc.recvIdx]
	mc.recvIdx = (mc.recvIdx + 1) % mc.capacity
	mc.count--

	// 腾出位置, 唤醒一个等待的发送者
	mc.sendCond.Signal()
	return val, true
}

func (mc *MyChan) Close() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 重复 Close
	if mc.closed {
		panic("close of closed channel")
	}
	mc.closed = true

	// 广播所有阻塞的收发G
	// 等待 send 的 G 醒来会触发 panic；等待 recv 的 G 醒来返回 nil, false
	mc.sendCond.Broadcast()
	mc.recvCoud.Broadcast()
}

// func main() {
// 	// 测试1：有缓冲通道
// 	fmt.Println("===== 有缓冲通道测试 容量2 =====")
// 	chBuf := NewMyChan(2)
// 	chBuf.Send(10)
// 	chBuf.Send(20)
// 	v1, ok1 := chBuf.Recv()
// 	fmt.Printf("recv: %v, ok:%t\n", v1, ok1)
// 	chBuf.Send(30)
// 	v2, ok2 := chBuf.Recv()
// 	v3, ok3 := chBuf.Recv()
// 	fmt.Printf("recv: %v, ok:%t\n", v2, ok2)
// 	fmt.Printf("recv: %v, ok:%t\n", v3, ok3)

// 	// 测试2：并发收发
// 	fmt.Println("\n===== 并发无缓冲通道测试 =====")
// 	chNoBuf := NewMyChan(0)
// 	var wg sync.WaitGroup
// 	wg.Add(2)

// 	go func() {
// 		defer wg.Done()
// 		chNoBuf.Send(999)
// 		fmt.Println("发送完成 999")
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		val, ok := chNoBuf.Recv()
// 		fmt.Printf("协程接收：%v ok:%t\n", val, ok)
// 	}()
// 	wg.Wait()

// 	// 测试3：关闭通道逻辑
// 	fmt.Println("\n===== Close 关闭测试 =====")
// 	closeCh := NewMyChan(1)
// 	closeCh.Send(666)
// 	closeCh.Close()
// 	r1, o1 := closeCh.Recv()
// 	r2, o2 := closeCh.Recv()
// 	fmt.Printf("关闭后第一次recv：%v ok:%t\n", r1, o1)
// 	fmt.Printf("关闭后第二次recv：%v ok:%t\n", r2, o2)
// }
