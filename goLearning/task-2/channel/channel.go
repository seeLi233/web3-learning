package channel

import (
	"fmt"
	"sync"
)

// 1.题目 ：编写一个程序，使用通道实现两个协程之间的通信。一个协程生成从1到10的整数，并将这些整数发送到通道中，另一个协程从通道中接收这些整数并打印出来。
func channelOne() {
	channel := make(chan int)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()

		for i := 1; i <= 10; i++ {
			channel <- i
		}
		close(channel)
	}()

	go func() {
		defer wg.Done()

		for data := range channel {
			fmt.Printf("消费者: 接收 %d\n", data)
		}
	}()

	wg.Wait()
}

// 2.题目 ：实现一个带有缓冲的通道，生产者协程向通道中发送100个整数，消费者协程从通道中接收这些整数并打印。
func channelTwo() {
	channel := make(chan int, 100)
	for i := 1; i <= 100; i++ {
		channel <- i
	}
	close(channel)

	for data := range channel {
		fmt.Printf("消费者: 接收 %d (缓冲区: %d/%d)\n", data, len(channel), cap(channel))
	}
}

func ChannelTest() {
	// 测试题目1
	channelOne()
	// 测试题目2
	channelTwo()
}
