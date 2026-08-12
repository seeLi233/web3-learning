package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-project-learning/project/common/pkg/config"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(cfg config.MysqlConfig, isDev bool) {
	// MySQL 数据库连接配置
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Dbname,
		cfg.Charset,
	)
	// dsn := "root:password@tcp(localhost:3306)/go_project?charset=utf8mb4&parseTime=True&loc=Local"

	var err error
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // 设置日志级别为 Info
	})

	if err != nil {
		log.Fatal("连接数据库失败", zap.Error(err))
	}

	db.Logger = newLoggerConfig(isDev)

	// 连接池配置
	sqlDb, _ := db.DB()
	sqlDb.SetMaxIdleConns(100)
	sqlDb.SetMaxOpenConns(20)

	DB = db
	log.Println("数据库连接成功")

	// 自动迁移数据库表结构
	// err = db.AutoMigrate(&model.User{}, &model.Order{}, &model.OrderItem{}, &model.Product{})
	// if err != nil {
	// 	log.Fatal("自动迁移数据库表失败", zap.Error(err))
	// }
}

// 区分开发环境与别的环境，开发环境开启全量日志，其他环境开启慢日志
func newLoggerConfig(isDev bool) logger.Interface {
	if isDev {
		log.Println("打开全量日志")
		return logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				LogLevel: logger.Info,
				Colorful: true,
			},
		)
	} else {
		log.Println("打开慢SQL日志")
		return logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold: 200 * time.Millisecond, // 超过 200 ms 即为慢 SQL
				LogLevel:      logger.Warn,            // 只打印警告、错误、慢 SQL
			},
		)
	}
}
