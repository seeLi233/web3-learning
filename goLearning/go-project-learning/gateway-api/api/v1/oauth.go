package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/internal/handler"
	"github.com/go-project-learning/project/gateway-api/internal/middleware"
)

func RegisterOAuthRoutes(r *gin.RouterGroup) {
	// 创建处理器
	oauthHandler := handler.NewOAuthHandler()
	githubHandler := handler.NewGitHubHandler()

	oauth := r.Group("/oauth")
	{
		// ============ OAuth2.0 核心接口 ============

		// 授权页面（GET 显示页面，POST 处理授权）
		oauth.GET("/authorize", oauthHandler.AuthorizePage)
		oauth.POST("/authorize", oauthHandler.Authorize)

		// 用授权码换取 token（无需鉴权）
		oauth.POST("/token", oauthHandler.Token)

		// 获取用户信息（需要 access_token）
		oauth.GET("/userinfo", oauthHandler.UserInfo)

		// ============ GitHub 登录 ============

		// GitHub 登录（重定向到 GitHub）
		oauth.GET("/github/login", githubHandler.GitHubLogin)

		// GitHub 回调
		oauth.GET("/github/callback", githubHandler.GitHubCallback)

		// 绑定 GitHub 账号（需要先登录）
		oauth.POST("/github/bind", middleware.JWTAuth(), githubHandler.BindGitHub)

		// 注册并绑定 GitHub
		oauth.POST("/github/register", githubHandler.RegisterAndBind)
	}
}
