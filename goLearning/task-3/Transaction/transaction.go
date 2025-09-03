package transaction

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

// 题目2：事务语句
// 假设有两个表： accounts 表（包含字段 id 主键， balance 账户余额）和 transactions 表（包含字段 id 主键， from_account_id 转出账户ID， to_account_id 转入账户ID， amount 转账金额）。
// 要求 ：
// 编写一个事务，实现从账户 A 向账户 B 转账 100 元的操作。在事务中，需要先检查账户 A 的余额是否足够，如果足够则从账户 A 扣除 100 元，向账户 B 增加 100 元，并在 transactions 表中记录该笔转账信息。如果余额不足，则回滚事务。

func TransactionTest() {
	// 创建表
	initDataBase()

	// 插入一个用户有 100.01 块的用户
	insertCustomer(100.01)
	// 插入一个没有钱的用户
	insertCustomer(0)
	// 查询用户1 和用户2 的余额
	fmt.Printf("用户1的余额：%f \n", queryBalance(1).Balance)
	fmt.Printf("用户2的余额：%f \n", queryBalance(2).Balance)
	// 用户1 向用户2 转 10 元
	transferMoney(1, 2, 10)
	// 查询用户1 和用户2 的余额
	fmt.Printf("用户1的余额：%f \n", queryBalance(1).Balance)
	fmt.Printf("用户2的余额：%f \n", queryBalance(2).Balance)
	// 用户2 再向用户1 转 20 元
	transferMoney(2, 1, 20)

	// 这里回滚了之后在查询一遍
	fmt.Printf("用户1的余额：%f \n", queryBalance(1).Balance)
	fmt.Printf("用户2的余额：%f \n", queryBalance(2).Balance)

	// 移除表
	dropTable()
}

var db *sql.DB

type Account struct {
	Id      int
	Balance float64
}

// 创建数据库表
func initDataBase() {
	var err error

	dsn := "root:26783492Michael0$@tcp(127.0.0.1:3306)/trasaction?charset=utf8mb4&parseTime=True"

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}

	// 检查数据库连接
	err = db.Ping()
	if err != nil {
		log.Fatal("数据库连接测试失败:", err)
	}

	// 创建 accounts 表
	accountsTable := `
	CREATE TABLE IF NOT EXISTS accounts(
		id INT PRIMARY KEY AUTO_INCREMENT,
		balance INT NOT NULL
	);
	`

	// 创建 transactions 表
	transactionsTable := `
	CREATE TABLE IF NOT EXISTS transactions(
		id INT PRIMARY KEY AUTO_INCREMENT,
		from_account_id INT NOT NULL,
		to_account_id INT NOT NULL,
		amount DECIMAL(10, 2)
	);
	`

	_, err = db.Exec(accountsTable)

	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(transactionsTable)

	if err != nil {
		log.Fatal(err)
	}
}

func insertCustomer(balance float64) {
	stmt, err := db.Prepare("INSERT INTO accounts (balance) VALUES (?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(balance)
	if err != nil {
		log.Fatal(err)
	}
}

// 实现事务转账
func transferMoney(fromAccountId int, toAccountId int, amount float64) error {
	// 开始事务
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	// 确保在函数返回时处理事务回滚或提交
	defer func() {
		if p := recover(); p != nil {
			// 发生 panic， 回滚事务
			tx.Rollback()
			// 重新抛出 panic
			panic(p)
		} else if err != nil {
			// 存在错误，回滚事务
			tx.Rollback()
		} else {
			// 提交事务
			err = tx.Commit()
			if err != nil {
				log.Fatal("提交任务失败：", err)
			}
		}
	}()

	// 查询转出账户余额是否足够
	var currentBalance float64
	err = tx.QueryRow(`SELECT balance FROM accounts WHERE id = ? FOR UPDATE`, fromAccountId).Scan(&currentBalance)

	if err != nil {
		return err
	}

	if currentBalance < amount {
		return errors.New("转出账户余额不足")
	}

	// 从转出账户扣款
	_, err = tx.Exec(`UPDATE accounts SET balance = balance - ?  WHERE id = ?`, amount, fromAccountId)
	if err != nil {
		return err
	}

	// 向转入账户增加金额
	_, err = tx.Exec(`UPDATE accounts SET balance = balance + ? WHERE id = ?`, amount, toAccountId)
	if err != nil {
		return err
	}

	// 记录转账记录
	_, err = tx.Exec(`INSERT INTO transactions (from_account_id, to_account_id, amount) VALUES (?, ?, ?)`, fromAccountId, toAccountId, amount)
	if err != nil {
		return err
	}

	return nil
}

func queryBalance(accountId int) Account {
	var account Account
	err := db.QueryRow("SELECT id, balance FROM accounts WHERE id = ?", accountId).Scan(&account.Id, &account.Balance)
	if err != nil {
		log.Fatal(err)
	}
	return account
}

func dropTable() {
	stmt, err := db.Prepare("DROP TABLE accounts")

	if err != nil {
		log.Fatal(err)
		stmt.Close()
	}

	stmt.Exec()
	if err != nil {
		log.Fatal(err)
		stmt.Close()
	}
	stmt, err = db.Prepare("DROP TABLE transactions")
	if err != nil {
		log.Fatal(err)
		stmt.Close()
	}

	stmt.Exec()
	if err != nil {
		log.Fatal(err)
		stmt.Close()
	}
}
