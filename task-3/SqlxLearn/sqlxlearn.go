package sqlxlearn

import (
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// 题目1：使用SQL扩展库进行查询
// 假设你已经使用Sqlx连接到一个数据库，并且有一个 employees 表，包含字段 id 、 name 、 department 、 salary 。
// 要求 ：
//     编写Go代码，使用Sqlx查询 employees 表中所有部门为 "技术部" 的员工信息，并将结果映射到一个自定义的 Employee 结构体切片中。
//     写Go代码，使用Sqlx查询 employees 表中工资最高的员工信息，并将结果映射到一个 Employee 结构体中。

func SqlxCompanyTest() {
	db, err := sqlx.Connect("mysql", "root:26783492Michael0$@tcp(127.0.0.1:3306)/company?charset=utf8mb4&parseTime=True")
	if err != nil {
		log.Fatal(err)
	}
	// 初始化数据库
	initDataBase(db)
	defer db.Close()

	// 插入数据
	insertEmployee(db, "张三", "技术部", 20000.05)
	insertEmployee(db, "李四", "技术部", 30000.05)
	insertEmployee(db, "王五", "销售部", 7008.15)
	insertEmployee(db, "赵六", "销售部", 40000.45)

	// 查询部门为技术部的员工信息
	employees := queryEmployeeByDep(db, "技术部")
	fmt.Println("技术部员工=====================")
	for _, employee := range employees {
		fmt.Printf("姓名：%s，部门：%s，薪酬：%f \n", employee.Name, employee.Department, employee.Salary)
	}

	// 查询工资最高的员工信息
	employee := queryHighestSalarEmp(db)
	fmt.Println("最高工资员工=====================")
	fmt.Printf("姓名：%s，部门：%s，薪酬：%f \n", employee.Name, employee.Department, employee.Salary)

	_, err = db.Exec(`DELETE FROM employees`)
	if err != nil {
		log.Fatal(err)
	}
}

type Employee struct {
	Id         int     `db:"id"`
	Name       string  `db:"name"`
	Department string  `db:"department"`
	Salary     float64 `db:"salary"`
}

func initDataBase(db *sqlx.DB) {

	employeesTable := `
	CREATE TABLE IF NOT EXISTS employees(
		id INT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(200) NOT NULL,
		department VARCHAR(200) NOT NULL,
		salary DECIMAL(10, 2)
	);
	`

	_, err := db.Exec(employeesTable)
	if err != nil {
		log.Fatal(err)
	}
}

func insertEmployee(db *sqlx.DB, name string, department string, salary float64) {
	_, err := db.Exec(`INSERT INTO employees (name, department, salary) VALUES (?, ?, ?)`, name, department, salary)
	if err != nil {
		log.Fatal(err)
	}
}

func queryEmployeeByDep(db *sqlx.DB, department string) []Employee {
	var employees []Employee
	err := db.Select(&employees, `SELECT id, name, department, salary FROM employees WHERE department = ?`, department)

	if err != nil {
		log.Fatal(err)
	}
	return employees
}

func queryHighestSalarEmp(db *sqlx.DB) Employee {
	var employee Employee
	err := db.Get(&employee, `SELECT id, name, department, salary FROM employees ORDER BY salary DESC LIMIT 1`)

	if err != nil {
		log.Fatal(err)
	}
	return employee
}

// 题目2：实现类型安全映射
// 假设有一个 books 表，包含字段 id 、 title 、 author 、 price 。
// 要求 ：
//     定义一个 Book 结构体，包含与 books 表对应的字段。
//     编写Go代码，使用Sqlx执行一个复杂的查询，例如查询价格大于 50 元的书籍，并将结果映射到 Book 结构体切片中，确保类型安全。

type Book struct {
	Id     int     `db:"id"`
	Title  string  `db:"title"`
	Author string  `db:"author"`
	Price  float64 `db:"price"`
}

func SqlxLibraryTest() {
	db, err := sqlx.Connect("mysql", "root:26783492Michael0$@tcp(127.0.0.1:3306)/library?charset=utf8mb4&parseTime=True")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 初始化数据
	initLibraryDatabase(db)

	// 插入一些数据
	insertBooks(db, "恶意", "东野圭吾", 46)
	insertBooks(db, "凡人修仙传", "忘语", 265)
	insertBooks(db, "漫长的告别", "雷蒙德", 49)
	insertBooks(db, "推理群星闪耀时", "江户川乱步", 98)

	// 查询所有书籍
	allBooks := queryAllBooks(db)
	for _, book := range allBooks {
		fmt.Printf("书籍：%s, 作者：%s, 价格：%f \n", book.Title, book.Author, book.Price)
	}

	// 查询大于某个价格的书本
	books := queryBooksByPri(db, 50)
	fmt.Println("价格大于50的书================================")
	for _, book := range books {
		fmt.Printf("书籍：%s, 作者：%s, 价格：%f \n", book.Title, book.Author, book.Price)
	}
}

func initLibraryDatabase(db *sqlx.DB) {
	booksTable := `
	CREATE TABLE IF NOT EXISTS books(
		id INT PRIMARY KEY AUTO_INCREMENT,
		title VARCHAR(200) NOT NULL,
		author VARCHAR(200) NOT NULL,
		price DECIMAL(10, 2)
	);
	`

	_, err := db.Exec(booksTable)
	if err != nil {
		log.Fatal(err)
	}
}

func insertBooks(db *sqlx.DB, title string, author string, price float64) {
	_, err := db.Exec(`INSERT INTO books (title, author, price) VALUES (?, ?, ?)`, title, author, price)
	if err != nil {
		log.Fatal(err)
	}
}

func queryBooksByPri(db *sqlx.DB, price float64) []Book {
	var books []Book

	err := db.Select(&books, `SELECT id, title, author, price FROM books WHERE price > ?`, price)
	if err != nil {
		log.Fatal(err)
	}

	return books
}

func queryAllBooks(db *sqlx.DB) []Book {
	var books []Book

	err := db.Select(&books, `SELECT id, title, author, price FROM books`)
	if err != nil {
		log.Fatal(err)
	}

	return books
}
