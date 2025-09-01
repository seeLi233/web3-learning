package gormlearn

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const dsn string = "root:26783492Michael0$@tcp(127.0.0.1:3306)/postcast?charset=utf8mb4&parseTime=True&loc=Local"

// 题目1：模型定义
// 假设你要开发一个博客系统，有以下几个实体： User （用户）、 Post （文章）、 Comment （评论）。
// 要求 ：
//     使用Gorm定义 User 、 Post 和 Comment 模型，其中 User 与 Post 是一对多关系（一个用户可以发布多篇文章）， Post 与 Comment 也是一对多关系（一篇文章可以有多个评论）。
//     编写Go代码，使用Gorm创建这些模型对应的数据库表。

type User struct {
	Id         uint      `gorm:"primaryKey"`
	Username   string    `gorm:"size:50;not null;uniqueIndex"`
	Email      string    `gorm:"size:100;not null;uniqueIndex"`
	Password   string    `gorm:"size:255;not null"`
	PostsCount int       `gorm:"default:0"` // 题目三新增，用户文章数量统计字段
	CreatTime  time.Time `gorm:"autoCreateTime"`
	UpdateTime time.Time `gorm:"autoUpdateTime"`
	Posts      []Post    `gorm:"foreignKey:UserId"` // 一对多关系：用户可以发布多篇文章
}

type Post struct {
	Id            uint      `gorm:"primaryKey"`
	Title         string    `gorm:"size:200;not null"`
	Content       string    `gorm:"type:text;not null"`
	CommentsCount int       `gorm:"default:0"`            // 题目三新增，文章评论数量统计字段
	CommentStatus string    `gorm:"size20;default:'无评论'"` // 题目三新增，评论状态字段
	UserId        uint      `gorm:"not null;index"`       // 外键，关联用户
	CreatTime     time.Time `gorm:"autoCreateTime"`
	UpdateTime    time.Time `gorm:"autoUpdateTime"`
	Comments      []Comment `gorm:"foreignKey:PostId"` // 一对多关系：文章拥有多个评论
}

type Comment struct {
	Id         uint      `gorm:"primaryKey"`
	Content    string    `gorm:"type:text;not null"`
	UserId     uint      `gorm:"not null;index"` // 外键，关联用户
	PostId     uint      `gorm:"not null;index"` // 外键，关联文章
	CreatTime  time.Time `gorm:"autoCreateTime"`
	UpdateTime time.Time `gorm:"autoUpdateTime"`
	Post       Post      `gorm:"foreignKey:PostId"`
	User       User      `gorm:"foreignKey:UserId"`
}

func PostcastTest() {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold: time.Second,
				LogLevel:      logger.Info,
				Colorful:      true,
			},
		),
	})

	if err != nil {
		panic("fail to connect database" + err.Error())
	}

	err = db.AutoMigrate(&User{}, &Post{}, &Comment{})
	if err != nil {
		panic("failed to migrate database:" + err.Error())
	}
}

// 题目2：关联查询
// 基于上述博客系统的模型定义。
// 要求 ：
//     编写Go代码，使用Gorm查询某个用户发布的所有文章及其对应的评论信息。
//     编写Go代码，使用Gorm查询评论数量最多的文章信息。

func QueryPostcastTest() {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database:" + err.Error())
	}

	//err = insertTestData(db)
	//if err != nil {
	//	log.Printf("插入数据失败：%v", err)
	//}

	err = queryUserPostsWithComments(db, 1)
	if err != nil {
		log.Printf("查询用户文章失败：%v", err)
	}

	err = queryMostCommentedPost(db)
	if err != nil {
		log.Panicf("查询最多评论文章失败：%v", err)
	}
}

func insertTestData(db *gorm.DB) error {

	// 创建用户
	users := []User{
		{Username: "张三", Email: "zhangsan@qq.com", Password: "zhangsan_hash_1"},
		{Username: "李四", Email: "lisi@qq.com", Password: "lisi_hash_2"},
		{Username: "王五", Email: "wangwu@qq.com", Password: "wangwu_hash_3"},
	}

	for i := range users {
		result := db.Create(&users[i])
		if result.Error != nil {
			return result.Error
		}
	}

	posts := []Post{
		{Title: "Go语言入门指南", Content: "Go语言是Google开发的一种...", UserId: users[0].Id},
		{Title: "GORM使用教程", Content: "GORM是Go语言的一个ORM库，提供了...", UserId: users[0].Id},
		{Title: "Web开发最佳实战", Content: "在现代Web开发中...", UserId: users[1].Id},
		{Title: "数据库设计原则", Content: "良好的数据库设计是...", UserId: users[1].Id},
		{Title: "RESTful API设计指南", Content: "RESTful API是一种设计风格...", UserId: users[1].Id},
		{Title: "微服务架构解析", Content: "微服务架构是一种将...", UserId: users[2].Id},
	}

	for i := range posts {
		result := db.Create(&posts[i])
		if result.Error != nil {
			return result.Error
		}
	}

	comments := []Comment{
		{Content: "非常好的入门指南", UserId: users[1].Id, PostId: posts[0].Id},
		{Content: "期待更多关于Go语言的高级主题", UserId: users[2].Id, PostId: posts[0].Id},
		{Content: "GORM确实很方便", UserId: users[2].Id, PostId: posts[1].Id},
		{Content: "这些最佳实践在项目中很有用", UserId: users[0].Id, PostId: posts[2].Id},
		{Content: "数据库设计确实很重要", UserId: users[2].Id, PostId: posts[3].Id},
		{Content: "RESTful API是最近正在研究的内容", UserId: users[0].Id, PostId: posts[4].Id},
	}

	for i := range comments {
		result := db.Create(&comments[i])
		if result.Error != nil {
			return result.Error
		}
	}

	return nil
}

func queryUserPostsWithComments(db *gorm.DB, userId uint) error {
	var user User

	result := db.Preload("Posts.Comments").First(&user, userId)
	if result.Error != nil {
		return result.Error
	}

	fmt.Printf("用户 %s 的文章列表:\n", user.Username)
	for _, post := range user.Posts {
		fmt.Printf(" - 文章标题： %s \n", post.Title)
		fmt.Printf("   评论数量： %d \n", len(post.Comments))
		for _, comment := range post.Comments {
			fmt.Printf("    *评论：%s \n", comment.Content)
		}
	}

	return nil
}

func queryMostCommentedPost(db *gorm.DB) error {
	var post Post
	result := db.Select("posts.*, COUNT(comments.id) as comment_count").
		Joins("LEFT JOIN comments ON comments.post_id = posts.id").
		Group("posts.id").
		Order("comment_count DESC").
		First(&post)

	if result.Error != nil {
		return result.Error
	}

	var commentCount int64
	db.Model(&Comment{}).Where("post_id = ?", post.Id).Count(&commentCount)

	fmt.Printf("\n评论最多的文章：\n")
	fmt.Printf("标题：%s\n", post.Title)
	fmt.Printf("评论数量： %d\n", commentCount)

	return nil
}

// 题目3：钩子函数
// 继续使用博客系统的模型。
// 要求 ：
//     为 Post 模型添加一个钩子函数，在文章创建时自动更新用户的文章数量统计字段。
//     为 Comment 模型添加一个钩子函数，在评论删除时检查文章的评论数量，如果评论数量为 0，则更新文章的评论状态为 "无评论"。

func HookTest() {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database:" + err.Error())
	}

	err = db.AutoMigrate(&User{}, &Post{}, &Comment{})

	if err != nil {
		panic("failed to migrate database:" + err.Error())
	}

	user := User{
		Username: "testuser", Email: "test@qq.com", Password: "testpassword",
	}
	db.Create(&user)

	post := Post{
		Title: "测试文章", Content: "这是一篇测试文章内容", UserId: user.Id,
	}
	db.Create(&post)

	// 查询用户以查看文章数量是否更新
	var updateUser User
	db.First(&updateUser, user.Id)
	fmt.Printf("用户 %s 的文章数量：%d \n", updateUser.Username, updateUser.PostsCount)

	// 创建测试评论
	comment := Comment{
		Id: 12, Content: "这是一条测试评论", UserId: user.Id, PostId: post.Id,
	}
	db.Create(&comment)

	fmt.Printf("创建评论：%s （ID: %d）\n", comment.Content, comment.Id)

	// 更新文章的评论数量
	var commentCount int64
	db.Model(&Comment{}).Where("post_id = ?", post.Id).Count(&commentCount)
	db.Model(&Post{}).Where("id = ?", post.Id).Update("comments_count", commentCount)

	// 查询文章以查看评论数量
	var updatedPost Post
	db.First(&updatedPost, post.Id)
	fmt.Printf("文章 %s 的评论数量：%d， 评论状态：%s \n", updatedPost.Title, updatedPost.CommentsCount, updatedPost.CommentStatus)

	// 删除评论 - 这将触发 AfterDelete 钩子
	db.Delete(&comment)
	fmt.Printf("已删除评论 ID: %d \n", comment.Id)

	// 再次查询文章以查看评论状态是否更新
	db.First(&updatedPost, post.Id)
	fmt.Printf("文章 %s 的评论数量：%d， 评论状态：%s \n", updatedPost.Title, updatedPost.CommentsCount, updatedPost.CommentStatus)

	// 清理测试数据
	db.Delete(&post)
	db.Delete(&user)
}

func (p *Post) AfterCreate(tx *gorm.DB) error {
	result := tx.Model(&User{}).Where("id = ?", p.UserId).Update("posts_count", gorm.Expr("posts_count + 1"))

	if result.Error != nil {
		return result.Error
	}

	result = tx.Model(&Post{}).Where("id = ?", p.Id).Update("comment_status", "")
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (c *Comment) AfterDelete(tx *gorm.DB) error {
	var count int64
	result := tx.Model(&Comment{}).Where("post_id = ?", c.PostId).Count(&count)
	if result.Error != nil {
		return result.Error
	}

	if count == 0 {
		// 评论数量为 0
		result = tx.Model(&Post{}).Where("id = ?", c.PostId).Update("comment_status", "无评论")
		if result.Error != nil {
			return result.Error
		}
	} else {
		// 评论数量为 0
		result = tx.Model(&Post{}).Where("id = ?", c.PostId).Update("comment_status", "")
		if result.Error != nil {
			return result.Error
		}
	}

	result = tx.Model(&Post{}).Where("id = ?", c.PostId).Update("comments_count", count)
	if result.Error != nil {
		return result.Error
	}

	return nil
}
