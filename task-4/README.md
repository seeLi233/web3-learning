基于 Go、Gin 和 GORM 开发的个人博客系统后端，提供完整的文章管理、用户认证和评论功能。

功能特性
1.用户注册与登录（JWT 认证）

2.文章的创建、读取、更新和删除（CRUD）

3.文章的评论功能

4.数据验证和错误处理

5.数据库关系和外键约束

6.请求日志记录

技术栈
编程语言: Go 1.21+

Web 框架: Gin

ORM: GORM

数据库: MySQL/PostgreSQL

认证: JWT (JSON Web Tokens)

密码加密: bcrypt

项目结构
text
blog-backend/
├── config/           # 配置文件
├── controllers/      # 控制器层
├── database/         # 数据库连接
├── middleware/       # 中间件
├── models/           # 数据模型
├── utils/            # 工具函数
├── go.mod           # Go 模块定义
├── go.sum           # 依赖校验和
├── main.go          # 应用入口
└── .env.example     # 环境变量示例
环境要求
Go 1.21 或更高版本

MySQL 5.7+

Git

安装步骤
1. 克隆项目
git clone https://github.com/seeLi233/web3-learning/tree/main/task-4
cd blog-backend
2. 配置环境变量
复制环境变量示例文件：

cp example.env config.env
编辑 .env 文件，设置你的配置：
# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_NAME=blog
DB_USER=root
DB_PASSWORD=your_password

# JWT 配置
JWT_SECRET=your-super-secret-jwt-key-with-at-least-32-characters

# 服务器配置
SERVER_PORT=8080

3. 安装依赖
go mod download

4. 创建数据库
登录到你的数据库管理系统，创建数据库：CREATE DATABASE blog;

5. 运行应用
go run main.go
服务器将在 http://localhost:8080 启动（或你在 .env 中设置的端口）。

API 文档
---------------------------------------------------------------------------
认证端点
===========================================================================
注册用户
URL: POST /api/auth/register

Body:

json
{
  "username": "testuser",
  "password": "password123",
  "email": "test@example.com"
}
===========================================================================
用户登录
URL: POST /api/auth/login

Body:

json
{
  "username": "testuser",
  "password": "password123"
}
响应:

json
{
  "token": "jwt-token-here"
}
---------------------------------------------------------------------------
文章端点
===========================================================================
获取所有文章
URL: GET /api/posts

认证: 可选
===========================================================================
获取单篇文章
URL: GET /api/posts/:id

认证: 可选
===========================================================================
创建文章
URL: POST /api/posts

认证: 需要 (Bearer Token)

Body:

json
{
  "title": "My First Post",
  "content": "This is the content of my first post."
}
===========================================================================
更新文章
URL: PUT /api/posts/:id

认证: 需要 (Bearer Token)

Body:

json
{
  "title": "Updated Title",
  "content": "Updated content."
}
===========================================================================
删除文章
URL: DELETE /api/posts/:id

认证: 需要 (Bearer Token)
---------------------------------------------------------------------------
评论端点
===========================================================================
获取文章评论
URL: GET /api/posts/:id/comments

认证: 可选
===========================================================================
创建评论
URL: POST /api/posts/:id/comments

认证: 需要 (Bearer Token)

Body:

json
{
  "content": "This is a comment on the post."
}
===========================================================================
使用 Postman 测试
导入 Postman 集合（可从项目导出）

按照以下顺序测试：

测试顺序
注册用户: POST /api/auth/register

用户登录: POST /api/auth/login (保存 token)

创建文章: POST /api/posts (使用 token)

获取所有文章: GET /api/posts

创建评论: POST /api/posts/1/comments (使用 token)

获取评论: GET /api/posts/1/comments

更新文章: PUT /api/posts/1 (使用 token)

删除文章: DELETE /api/posts/1 (使用 token)
===========================================================================

测试用例已导出到 web3learning.postman_collection.json 文件中，在 postman 中导入即可
