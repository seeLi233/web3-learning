package main

import "sort"

// 136. 只出现一次的数字
func singleNumber(nums []int) int {
	var number int

	// 通过与或运算进行排除
	for _, value := range nums {
		number ^= value
	}
	return number
}

// 回文数
func isPalindrome(x int) bool {
	// 1. 负数不能为回文数
	// 2. 最后一位为 0 的回文数只能是 0
	if x < 0 || (x%10 == 0 && x != 0) {
		return false
	}

	number := 0
	// 数字倒转
	for x > number {
		number = (number * 10) + (x % 10)
		x /= 10
	}
	// 相等返回 true, 不相等返回 false
	return x == number || x == number/10
}

// 有效的括号
func isValid(s string) bool {
	// 长度不为偶数则不能有效
	length := len(s)
	if length%2 == 1 {
		return false
	}
	// 定义闭口映射
	pairs := map[byte]byte{
		')': '(',
		']': '[',
		'}': '{',
	}
	// 创建切片
	// 循环字符串，没有遇到闭口则存入切片
	// 遇到闭口则判断是否为第一个字符, 如果不是，从映射中找出对应开口判断是否等于切片最后一个元素
	// 如果不是，则不是有效的括号，如果是，则删除切片最后一个元素，继续遍历
	// 最后判断切片是否清空，清空则整个字符串的括号都为有效括号
	stack := []byte{}
	for i := 0; i < length; i++ {
		if pairs[s[i]] > 0 {
			if len(stack) == 0 || stack[len(stack)-1] != pairs[s[i]] {
				return false
			} else {
				stack = stack[:len(stack)-1]
			}
		} else {
			stack = append(stack, s[i])
		}
	}

	return len(stack) == 0
}

// 最长公共前缀
func longestCommonPrefix(strs []string) string {
	// 空字符串返回空字符串
	if len(strs) == 0 {
		return ""
	}

	prefix := strs[0]
	count := len(strs)
	// 字符串数组中遍历
	for i := 1; i < count; i++ {
		// 获取最长公共前缀
		prefix = lcp(prefix, strs[i])
		if len(prefix) == 0 {
			break
		}
	}
	return prefix
}

func lcp(str1, str2 string) string {
	length := min(len(str1), len(str2))
	index := 0
	// 判断前缀是否一致，一致则继续，不一致则停止
	for index < length && str1[index] == str2[index] {
		index++
	}
	// 返回对应前缀内容
	return str1[:index]
}

func min(x, y int) int {
	if x < y {
		return x
	}

	return y
}

// 加一
func plusOne(digits []int) []int {
	length := len(digits)
	// 从后往前判断
	// 如果遇到不为9的数字，则加一，将后面所有为9的数字设置为0
	for i := length - 1; i >= 0; i-- {
		if digits[i] != 9 {
			digits[i]++
			for j := i + 1; j < length; j++ {
				digits[j] = 0
			}
			return digits
		}
	}

	digits = make([]int, length+1)
	digits[0] = 1
	return digits
}

//  删除有序数组中的重复项
func removeDuplicates(nums []int) int {
	length := len(nums)
	if length == 0 {
		return 0
	}
	// 定义快慢指标
	slow := 1
	for fast := 1; fast < length; fast++ {
		// 快指标判断不在重复之后将不重复的数字定在慢指标位置
		if nums[fast] != nums[fast-1] {
			nums[slow] = nums[fast]
			slow++
		}
	}
	return slow
}

// 56. 合并区间
func merge(intervals [][]int) [][]int {
	if len(intervals) == 0 {
		return intervals
	}
	// 排序切片，按照起始位置排序
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i][0] < intervals[j][0]
	})
	// 创建用于存储合并后的区间
	merged := make([][]int, 0)
	// 当前第一个区间
	current := intervals[0]
	// 判断当前区间与下一个区间是否重合
	for _, interval := range intervals[1:] {
		if current[1] >= interval[0] && current[1] < interval[1] {
			// 存在重合, 将其合并
			current[1] = interval[1]
		} else {
			// 不存在重合，假如区间
			merged = append(merged, current)
			current = interval
		}
	}
	// 合并最后一个区间
	merged = append(merged, current)
	return merged
}

// 两数之和
func twoSum(nums []int, target int) []int {
	mappingData := map[int]int{}
	for i, x := range nums {
		if p, ok := mappingData[target-x]; ok {
			return []int{p, i}
		}
		mappingData[x] = i
	}
	return nil
}

func main() {

}
