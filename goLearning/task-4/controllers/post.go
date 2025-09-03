package controllers

import (
	"net/http"
	"strconv"
	"web3/task4/database"
	"web3/task4/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CreatePostInput struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

func CreatePost(c *gin.Context) {
	userId := c.MustGet("user_id").(uint)

	var input CreatePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	post := models.Post{
		Title:   input.Title,
		Content: input.Content,
		UserId:  userId,
	}

	if err := database.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create post"})
		return
	}

	c.JSON(http.StatusCreated, post)
}

func GetPosts(c *gin.Context) {
	var posts []models.Post
	if err := database.DB.Preload("User").Preload("Comments").Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch posts"})
		return
	}
	c.JSON(http.StatusOK, posts)
}

func GetPost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post Id"})
		return
	}

	var post models.Post
	if err := database.DB.Preload("User").Preload("Comments").First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	c.JSON(http.StatusOK, post)
}

func UpdatePost(c *gin.Context) {
	userId := c.MustGet("user_id").(uint)
	id, err := strconv.Atoi(c.Param("id"))

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post Id"})
		return
	}

	var post models.Post
	if err := database.DB.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	if post.UserId != userId {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only update your own posts"})
		return
	}

	var input CreatePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := database.DB.Model(&post).Updates(models.Post{
		Title:   input.Title,
		Content: input.Content,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update post"})
		return
	}

	c.JSON(http.StatusOK, post)
}

func DeletePost(c *gin.Context) {
	userId := c.MustGet("user_id").(uint)
	id, err := strconv.Atoi(c.Param("id"))

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post Id"})
		return
	}

	var post models.Post
	if err := database.DB.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	if post.UserId != userId {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete your own posts"})
		return
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {

		if err := tx.Where("post_id = ?", id).Delete(&models.Comment{}).Error; err != nil {
			return err
		}

		if err := tx.Delete(&post).Error; err != nil {
			return err
		}

		return nil
	})

	//if err := database.DB.Delete(&post).Error; err != nil {
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete post"})
	//	return
	//}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}
