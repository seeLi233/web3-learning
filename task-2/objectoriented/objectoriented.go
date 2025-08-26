package objectoriented

import (
	"fmt"
	"math"
)

// 1.题目 ：定义一个 Shape 接口，包含 Area() 和 Perimeter() 两个方法。然后创建 Rectangle 和 Circle 结构体，实现 Shape 接口。在主函数中，创建这两个结构体的实例，并调用它们的 Area() 和 Perimeter() 方法。
type Shape interface {
	Area() float64
	Perimeter() float64
}

type Rectangle struct {
	width  float64
	height float64
}

type Circle struct {
	Radius float64
}

func (rectangle Rectangle) Area() float64 {
	return rectangle.width * rectangle.height
}

func (circle Circle) Area() float64 {
	return circle.Radius * circle.Radius * math.Pi
}

func (rectangle Rectangle) Perimeter() float64 {
	return 2 * (rectangle.width + rectangle.height)
}

func (circle Circle) Perimeter() float64 {
	return 2 * math.Pi * circle.Radius
}

func printShapeInfo(s Shape) {
	fmt.Printf("面积: %.2f\n", s.Area())
	fmt.Printf("周长: %.2f\n", s.Perimeter())
}

// 2.题目 ：使用组合的方式创建一个 Person 结构体，包含 Name 和 Age 字段，再创建一个 Employee 结构体，组合 Person 结构体并添加 EmployeeID 字段。为 Employee 结构体实现一个 PrintInfo() 方法，输出员工的信息。

type Person struct {
	Name string
	Age  int
}

type Employee struct {
	id     int
	person Person
}

func (employee Employee) PrintInfo() {
	fmt.Printf("姓名： %s, 年龄：%d \n", employee.person.Name, employee.person.Age)
}

func ObjectOrientedTest() {
	// 创建 Rectangle 实例
	rect := Rectangle{width: 5, height: 3}
	fmt.Println("矩形 (宽: 5, 高: 3):")
	printShapeInfo(rect)

	// 创建 Circle 实例
	circle := Circle{Radius: 4}
	fmt.Println("圆形 (半径: 4):")
	printShapeInfo(circle)

	employee := Employee{id: 000001, person: Person{Name: "张三", Age: 26}}
	employee.PrintInfo()
}
