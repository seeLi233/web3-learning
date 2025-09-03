package main

import (
	"web3/task2/channel"
	"web3/task2/goroutine"
	"web3/task2/lock"
	"web3/task2/objectoriented"
	"web3/task2/pointer"
)

func main() {
	// 指针测试
	pointer.PointerTest()
	// goroutine 测试
	goroutine.GoroutineTest()
	// 面向对象测试
	objectoriented.ObjectOrientedTest()
	// channel 测试
	channel.ChannelTest()
	// 锁测试
	lock.LockTest()

}
