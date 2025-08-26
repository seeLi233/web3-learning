package pointer

import "fmt"

// 指针
// 1.题目 ：编写一个Go程序，定义一个函数，该函数接收一个整数指针作为参数，在函数内部将该指针指向的值增加10，然后在主函数中调用该函数并输出修改后的值。

func questionOne(number *int) {
	*number += 10
}

// 2.题目 ：实现一个函数，接收一个整数切片的指针，将切片中的每个元素乘以2。

func questionTwo(sliceData *[]int) {
	slices := *sliceData

	for index := range slices {
		slices[index] *= 2
	}
}

func PointerTest() {
	// 测试指针题目1
	number := 10
	questionOne(&number)

	fmt.Println("原始值: 10, 输出值：", number)

	// 测试指针题目2
	sliceData := []int{1, 2, 3, 4, 5}
	questionTwo(&sliceData)

	fmt.Println("原始值：1,2,3,4,5， 输出值：", sliceData)
}
