package controllers

import (
	"net/http"
	"strconv"
	"web3/task4/database"
	"web3/task4/models"

	"github.com/gin-gonic/gin"
)

type CreateCommentInput struct {
	Content string `json:"content" binding:"required"`
}

func CreateComment(c *gin.Context) {
	userId := c.MustGet("user_id").(uint)
	postId, err := strconv.Atoi(c.Param("id"))

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post Id"})
		return
	}

	var input CreateCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var post models.Post
	if err := database.DB.First(&post, postId).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	comment := models.Comment{
		Content: input.Content,
		UserId:  userId,
		PostId:  uint(postId),
	}

	if err := database.DB.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create comment"})
		return
	}

	c.JSON(http.StatusOK, comment)
}

func GetComments(c *gin.Context) {
	postId, err := strconv.Atoi(c.Param("id"))

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post Id"})
		return
	}

	var comments []models.Comment
	if err := database.DB.Preload("User").Where("post_id = ?", postId).Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch comments"})
		return
	}

	c.JSON(http.StatusOK, comments)
}
