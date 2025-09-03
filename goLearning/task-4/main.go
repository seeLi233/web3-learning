package main

import (
	"log"
	"os"
	"web3/task4/controllers"
	"web3/task4/database"
	"web3/task4/middleware"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {

	// 获取配置
	if err := godotenv.Load("config.env"); err != nil {
		log.Println("No .env file found")
	}

	// 连接数据库
	database.ConnectDB()

	router := gin.Default()

	router.Use(middleware.LoggerMiddleware())

	api := router.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", controllers.Register)
			auth.POST("/login", controllers.Login)
		}

		posts := api.Group("/posts")
		{
			posts.GET("/", controllers.GetPosts)
			posts.GET("/:id", controllers.GetPost)

			protected := posts.Group("/")
			protected.Use(middleware.AuthMiddleware())
			{
				protected.POST("/", controllers.CreatePost)
				protected.PUT("/:id", controllers.UpdatePost)
				protected.DELETE("/:id", controllers.DeletePost)

				protected.POST("/:id/comments", controllers.CreateComment)
			}

			posts.GET("/:id/comments", controllers.GetComments)
		}
	}

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start serve:", err)
	}
}
