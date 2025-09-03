package database

import (
	"fmt"
	"log"
	"web3/task4/config"
	"web3/task4/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() {
	cfg := config.LoadConfig()

	//dsn := "root:26783492Michael0$@tcp(127.0.0.1:3306)/blog?charset=utf8mb4&parseTime=True&loc=Local"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto migrate models
	db.AutoMigrate(&models.User{}, &models.Post{}, &models.Comment{})
	db.Migrator().CreateConstraint(&models.Comment{}, "Post")

	DB = db
	log.Println("Database connected successfully")
}
