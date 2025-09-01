package crud

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

// 题目1：基本CRUD操作
// 假设有一个名为 students 的表，包含字段 id （主键，自增）、 name （学生姓名，字符串类型）、 age （学生年龄，整数类型）、 grade （学生年级，字符串类型）。
// 要求 ：
//     编写SQL语句向 students 表中插入一条新记录，学生姓名为 "张三"，年龄为 20，年级为 "三年级"。
//     编写SQL语句查询 students 表中所有年龄大于 18 岁的学生信息。
//     编写SQL语句将 students 表中姓名为 "张三" 的学生年级更新为 "四年级"。
//     编写SQL语句删除 students 表中年龄小于 15 岁的学生记录。

func CrudTest() {
	initDataBase()

	insertStudent("张三", 20, "三年级")
	insertStudent("李四", 19, "二年级")
	insertStudent("王五", 18, "二年级")
	insertStudent("赵六", 14, "一年级")

	fmt.Print("查询所有年龄大于 18 岁的学生信息========================= \n")

	students := queryStudentsByAge(18)
	for _, student := range students {
		fmt.Printf("姓名：%s, 年龄：%d, 年级：%s \n", student.Name, student.Age, student.Grade)
	}

	fmt.Print("更新前学生信息========================= \n")
	students = queryAllStudent()

	for _, student := range students {
		fmt.Printf("姓名：%s, 年龄：%d, 年级：%s \n", student.Name, student.Age, student.Grade)
	}

	updateStudentGrade("张三", "四年级")

	fmt.Print("更新年级后学生信息========================= \n")
	queryAllStudent()

	students = queryAllStudent()

	for _, student := range students {
		fmt.Printf("姓名：%s, 年龄：%d, 年级：%s \n", student.Name, student.Age, student.Grade)
	}

	deleteStudentByAge(15)

	fmt.Print("删除后学生信息========================= \n")

	students = queryAllStudent()

	for _, student := range students {
		fmt.Printf("姓名：%s, 年龄：%d, 年级：%s \n", student.Name, student.Age, student.Grade)
	}

	stmt, err := db.Prepare("DROP TABLE students")
	if err != nil {
		log.Fatal(err)
	}

	defer stmt.Close()

	stmt.Exec()
}

// Student 结构体表示学生信息
type Student struct {
	ID    int
	Name  string
	Age   int
	Grade string
}

var db *sql.DB

func initDataBase() {
	var err error

	dsn := "root:26783492Michael0$@tcp(127.0.0.1:3306)/school?charset=utf8mb4&parseTime=True"

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}

	// 检查数据库连接
	err = db.Ping()
	if err != nil {
		log.Fatal("数据库连接测试失败:", err)
	}

	// 创建students表
	sqlTable := `
	CREATE TABLE IF NOT EXISTS students(
		id INT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(200) NOT NULL,
		age INT NOT NULL,
		grade VARCHAR(50) NOT NULL
	);
	`

	_, err = db.Exec(sqlTable)
	if err != nil {
		log.Fatal(err)
	}
}

// 插入新学生
func insertStudent(name string, age int, grade string) {
	stmt, err := db.Prepare("INSERT INTO students(name, age, grade) VALUES(?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(name, age, grade)
	if err != nil {
		log.Fatal(err)
	}
}

// 查询所有年龄大于某个数的学生
func queryStudentsByAge(minAge int) []Student {
	rows, err := db.Query("SELECT id, name, age, grade FROM students WHERE age > ?", minAge)

	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var students []Student
	for rows.Next() {
		var student Student
		err := rows.Scan(&student.ID, &student.Name, &student.Age, &student.Grade)
		if err != nil {
			log.Fatal(err)
		}
		students = append(students, student)
	}
	return students
}

// 更新学生的年级
func updateStudentGrade(name, newGrade string) {
	stmt, err := db.Prepare("UPDATE students SET grade = ? WHERE name = ?")

	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(newGrade, name)
	if err != nil {
		log.Fatal(err)
	}
}

// 删除年龄小于某个数字的学生
func deleteStudentByAge(maxAge int) {
	stmt, err := db.Prepare("DELETE FROM students WHERE age < ?")

	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(maxAge)
	if err != nil {
		log.Fatal(err)
	}
}

func queryAllStudent() []Student {
	var students []Student
	rows, err := db.Query("SELECT id, name, age, grade FROM students")

	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var student Student
		err := rows.Scan(&student.ID, &student.Name, &student.Age, &student.Grade)
		if err != nil {
			log.Fatal(err)
		}
		students = append(students, student)
	}

	return students
}
