package handler

import (
	"github/seeli/task-6/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// 模拟数据库（实际项目用 GORM/PostgreSQL）
var usersDB = []map[string]interface{}{
	{"id": 1, "name": "Alice", "email": "alice@example.com"},
	{"id": 2, "name": "Bob", "email": "bob@example.com"},
}

// ListUsers 获取用户列表
// GET /api/v1/users?page=1&pageSize=20
func ListUser(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	model.SuccessPage(c, usersDB, int64(len(usersDB)), page, pageSize)
}

// GetUser 获取单个用户
// GET /api/v1/users/:id
func GetUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		model.BadRequest(c, "用户ID格式错误")
		return
	}

	for _, u := range usersDB {
		if u["id"] == id {
			model.Success(c, u)
			return
		}
	}
	model.NotFound(c, "用户不存在")
}

// CreateUserReq 创建用户请求体
type CreateUserReq struct {
	Name  string `json:"name" binding:"required,min=2,max=50"`
	Email string `json:"email" binding:"required,email"`
}

// CreateUser 创建用户
// POST /api/v1/users
func CreateUser(c *gin.Context) {
	var req CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		model.BadRequest(c, "参数校验失败:"+err.Error())
		return
	}

	newUser := map[string]interface{}{
		"id":    len(usersDB) + 1,
		"name":  req.Name,
		"email": req.Email,
	}
	usersDB = append(usersDB, newUser)

	model.Created(c, newUser)
}

// UpdateUserReq 更新用户请求体
type UpdateUserReq struct {
	Name  string `json:"name" binding:"omitempty,min=2,max=50"`
	Email string `json:"email" binding:"omitempty,email"`
}

// UpdateUser 部分更新用户（PATCH）
// PATCH /api/v1/users/:id
func UpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		model.BadRequest(c, "用户ID格式错误")
		return
	}

	var req UpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		model.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}

	for i, u := range usersDB {
		if u["id"] == id {
			if req.Name != "" {
				usersDB[i]["name"] = req.Name
			}
			if req.Email != "" {
				usersDB[i]["email"] = req.Email
			}
			model.Success(c, usersDB[i])
			return
		}
	}
	model.NotFound(c, "用户不存在")
}

// DeleteUser 删除用户
// DELETE /api/v1/users/:id
func DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		model.BadRequest(c, "用户ID格式错误")
		return
	}

	for i, u := range usersDB {
		if u["id"] == id {
			usersDB = append(usersDB[:i], usersDB[i+1:]...)
			c.JSON(http.StatusOK, model.Response{
				Code:    model.CodeSuccess,
				Message: "已删除",
			})
			return
		}
	}
	model.NotFound(c, "用户不存在")
}
