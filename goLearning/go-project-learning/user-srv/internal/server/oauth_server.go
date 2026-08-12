package server

import (
	"context"

	"github.com/go-project-learning/project/user-srv/api/pb"
	"github.com/go-project-learning/project/user-srv/internal/application"
)

type OAuthServer struct {
	pb.UnimplementedOAuthServiceServer
	oauthApp *application.OAuthApp
	userApp  *application.UserApp
}

func NewOAuthServer(oauthApp *application.OAuthApp, userApp *application.UserApp) *OAuthServer {
	return &OAuthServer{
		oauthApp: oauthApp,
		userApp:  userApp,
	}
}

// CreateAuthorizationCode 创建授权码
func (s *OAuthServer) CreateAuthorizationCode(ctx context.Context, req *pb.CreateCodeRequest) (*pb.CreateCodeResponse, error) {
	code, err := s.oauthApp.CreateAuthorizationCode(
		ctx,
		req.ClientId,
		req.RedirectUri,
		req.Scope,
		uint(req.UserId),
	)

	if err != nil {
		return &pb.CreateCodeResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	return &pb.CreateCodeResponse{
		Code:              0,
		Msg:               "success",
		AuthorizationCode: code,
	}, nil
}

// ValidateAuthorizationCode 验证授权码
func (s *OAuthServer) ValidateAuthorizationCode(ctx context.Context, req *pb.ValidateCodeRequest) (*pb.ValidateCodeResponse, error) {
	userID, scope, err := s.oauthApp.ValidateAndConsumeCode(
		ctx,
		req.Code,
		req.ClientId,
		req.RedirectUri,
	)

	if err != nil {
		return &pb.ValidateCodeResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	return &pb.ValidateCodeResponse{
		Code:   0,
		Msg:    "success",
		UserId: uint64(userID),
		Scope:  scope,
	}, nil
}

// CreateAccessToken 创建 Access Token
func (s *OAuthServer) CreateAccessToken(ctx context.Context, req *pb.CreateTokenRequest) (*pb.CreateTokenResponse, error) {
	accessToken, refreshToken, expiresIn, err := s.oauthApp.CreateToken(
		ctx,
		req.ClientId,
		uint(req.UserId),
		req.Scope,
	)

	if err != nil {
		return &pb.CreateTokenResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	return &pb.CreateTokenResponse{
		Code:         0,
		Msg:          "success",
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

// ValidateAccessToken 验证 Access Token
func (s *OAuthServer) ValidateAccessToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	userID, clientID, scope, err := s.oauthApp.ValidateAccessToken(ctx, req.AccessToken)

	if err != nil {
		return &pb.ValidateTokenResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	return &pb.ValidateTokenResponse{
		Code:     0,
		Msg:      "success",
		UserId:   uint64(userID),
		ClientId: clientID,
		Scope:    scope,
	}, nil
}

// RefreshAccessToken 刷新 Token
func (s *OAuthServer) RefreshAccessToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	accessToken, refreshToken, expiresIn, err := s.oauthApp.RefreshToken(
		ctx,
		req.RefreshToken,
		req.ClientId,
	)

	if err != nil {
		return &pb.RefreshTokenResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	return &pb.RefreshTokenResponse{
		Code:         0,
		Msg:          "success",
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

// GetUserInfoByToken 获取用户信息
func (s *OAuthServer) GetUserInfoByToken(ctx context.Context, req *pb.GetUserInfoRequest) (*pb.GetUserInfoResponse, error) {
	// 这里需要调用 user-srv 的用户服务获取用户信息
	// 简化实现，直接返回 token 中的信息
	userID, _, scope, err := s.oauthApp.ValidateAccessToken(ctx, req.AccessToken)

	if err != nil {
		return &pb.GetUserInfoResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	// 查询用户信息
	user, err := s.userApp.GetUserByID(ctx, uint(userID))
	if err != nil {
		return &pb.GetUserInfoResponse{Code: 50002, Msg: "用户不存在"}, nil
	}

	// TODO: 调用 UserApp 获取用户详细信息
	return &pb.GetUserInfoResponse{
		Code:     0,
		Msg:      "success",
		UserId:   uint64(userID),
		Username: user.Username,
		Phone:    user.Phone,
		Email:    user.Email,
		Scope:    scope,
	}, nil
}

// ValidateClient 验证客户端
func (s *OAuthServer) ValidateClient(ctx context.Context, req *pb.ValidateClientRequest) (*pb.ValidateClientResponse, error) {
	client, err := s.oauthApp.ValidateClient(ctx, req.ClientId, req.ClientSecret)
	if err != nil {
		return &pb.ValidateClientResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	return &pb.ValidateClientResponse{
		Code:         0,
		Msg:          "success",
		Name:         client.Name,
		RedirectUris: client.RedirectURIs,
		Scope:        client.Scope,
	}, nil
}

// GitHubLogin GitHub 登录
func (s *OAuthServer) GitHubLogin(ctx context.Context, req *pb.GitHubLoginRequest) (*pb.GitHubLoginResponse, error) {
	// TODO: 实现 GitHub 登录
	return &pb.GitHubLoginResponse{
		Code: 0,
		Msg:  "success",
	}, nil
}

// BindGitHubAccount 绑定 GitHub 账号
func (s *OAuthServer) BindGitHubAccount(ctx context.Context, req *pb.BindGitHubRequest) (*pb.BindGitHubResponse, error) {
	err := s.oauthApp.BindGitHubAccount(
		ctx,
		req.GithubId,
		req.GithubUsername,
		req.GithubEmail,
		uint(req.UserId),
	)

	if err != nil {
		return &pb.BindGitHubResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	return &pb.BindGitHubResponse{
		Code: 0,
		Msg:  "success",
	}, nil
}
