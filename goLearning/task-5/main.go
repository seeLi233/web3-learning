package main

import (
	"fmt"
	"github/seeli/task5/channel"
	"github/seeli/task5/downloader"
	"github/seeli/task5/goroutine"
	"github/seeli/task5/workpool"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("  Day 33: Go 并发编程深度解析")
	fmt.Println("  goroutine + channel + Worker Pool")
	fmt.Println("========================================")
	fmt.Println()

	// Part A: goroutine 底层原理
	goroutine.GoroutineDeepDive()

	// Part B: channel 底层行为
	channel.ChannelDeepDive()

	// Part C: Worker Pool
	workpool.Demo()

	// Part D: 并发下载器（生产者-消费者）
	downloader.Demo()

	fmt.Println("\n=== Day 33 学习完成! ===")
}
