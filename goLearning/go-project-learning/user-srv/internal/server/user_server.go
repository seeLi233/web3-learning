package server

import (
	"context"
	"fmt"

	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/common/pkg/otel"
	"github.com/go-project-learning/project/user-srv/api/pb"
	"github.com/go-project-learning/project/user-srv/internal/application"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/go-project-learning/project/user-srv/pkg/bcrypt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

type UserServiceServer struct {
	pb.UnimplementedUserServiceServer
	userApp *application.UserApp
}

func NewUserServiceServer(userApp *application.UserApp) *UserServiceServer {
	return &UserServiceServer{userApp: userApp}
}

// ==================================注册用户操作==================================

// func (s *UserServiceServer) RegisterUserRPC(host, port string) {
// 	// 1. 监听端口
// 	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%s", host, port))
// 	if err != nil {
// 		log.Fatalf("端口监听失败: %v", err)
// 		// logger.Fatel("端口监听失败", zap.Error(err))
// 	}

// 	// 2. 创建gRPC服务器
// 	grpcServer := grpc.NewServer(
// 		grpc.StatsHandler(otelgrpc.NewServerHandler()),
// 		grpc.UnaryInterceptor(userLogInterceptor),
// 	)

// 	// 3. 注册服务（从 internal/service 引入）
// 	userService := NewUserServiceServer()
// 	pb.RegisterUserServiceServer(grpcServer, userService)

// 	log.Println("✅ 用户服务启动成功 :" + port)
// 	//logger.Info("用户服务启动成功 :50051")

// 	// 4. 启动服务
// 	if err := grpcServer.Serve(lis); err != nil {
// 		log.Fatalf("服务启动失败: %v", err)
// 		// logger.Fatel("服务启动失败", zap.Error(err))
// 	}
// }

// func  userLogInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
// 	// 1. 请求进入：前置日志
// 	start := time.Now()
// 	logger.Info(fmt.Sprintf("[全局日志] 开始请求 | 接口: %s | 请求参数: %v", info.FullMethod, req))

// 	// 2. 执行真正的业务入口
// 	resp, err = handler(ctx, req)

// 	// 3. 请求结束：后置日志
// 	cost := time.Since(start)
// 	logger.Info(fmt.Sprintf("[全局日志] 结束请求 | 接口: %s | 耗时: %v | 错误: %v", info.FullMethod, cost, err))

// 	return resp, err
// }

// ==================================添加用户服务==================================

func (s *UserServiceServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	logger.Info("开始创建用户....")

	tracer := otel.GetTracer("user-service")
	ctx, span := tracer.Start(ctx, "Service-RegisterUser")
	defer span.End()

	span.SetAttributes(attribute.String("user.username", req.Name))

	// 子 Span: 检查用户名
	_, checkNameSpan := tracer.Start(ctx, "CheckUsernameExists")
	// 校验用户名是否已存在
	_, err := s.userApp.GetByUsername(ctx, req.Name)
	checkNameSpan.End()
	if err == nil {
		span.AddEvent("用户名已存在")
		// return bizerror.NewBizError(bizerror.UserExists, "用户名已存在")
		logger.Error(fmt.Sprintf("用户名已存在[%s]", req.Name))
		return &pb.CreateUserResponse{Code: 10003, Msg: fmt.Sprintf("用户名已存在[%s]", req.Name)}, nil
	}

	// 2. 查手机号（需要在 repository 里加一个 GetUserByPhone 方法）
	_, checkPhoneSpan := tracer.Start(ctx, "CheckPhoneExists")
	_, err = s.userApp.GetByPhone(ctx, req.Phone)
	checkPhoneSpan.End()
	if err == nil {
		return &pb.CreateUserResponse{Code: 10005, Msg: fmt.Sprintf("手机号已注册[%s]", req.Phone)}, nil
	}

	// 3. 查邮箱
	_, checkEmailSpan := tracer.Start(ctx, "CheckEmailExists")
	_, err = s.userApp.GetByEmail(ctx, req.Email)
	checkEmailSpan.End()
	if err == nil {
		return &pb.CreateUserResponse{Code: 10006, Msg: fmt.Sprintf("邮箱已注册[%s]", req.Email)}, nil
	}
	// 密码加密（在这里做，不在 gateway）
	hashedPwd, err := bcrypt.HashPassword(req.Password)
	if err != nil {
		return &pb.CreateUserResponse{Code: 50001, Msg: "密码加密失败"}, nil
	}

	user := &entity.User{
		Username: req.Name,
		Password: hashedPwd,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   1, // 默认状态为活跃
	}

	_, createSpan := tracer.Start(ctx, "CreateUserInDB")
	err = s.userApp.Create(ctx, user)
	if err != nil {
		createSpan.RecordError(err)
		return &pb.CreateUserResponse{
			Code: 10002,
			Msg:  "用户创建失败",
		}, err
	}
	createSpan.End()

	return &pb.CreateUserResponse{
		Code: 0,
		Msg:  "创建成功",
		Data: &pb.User{
			// Id:    1001,
			Name:  user.Username,
			Phone: user.Phone,
			Email: user.Email,
		},
	}, nil
}

// ==================================更改用户服务==================================
func (s *UserServiceServer) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	logger.Info(fmt.Sprintf("更新用户信息 ID: %d", req.Id))

	tracer := otel.GetTracer("userService-UpdateUser")
	ctx, span := tracer.Start(ctx, "Service-UpdateUser")
	defer span.End()

	span.SetAttributes(attribute.Int64("user.id", req.Id))

	user, err := s.userApp.UpdateUser(ctx, uint(req.Id), req.Name, req.Phone, req.Email)

	if err != nil {
		logger.Error(fmt.Sprintf("用户数据更新失败, 用户ID:[%d]", req.Id), zap.Error(err))
		return &pb.UpdateUserResponse{Code: 10008, Msg: fmt.Sprintf("用户数据更新失败, 用户ID:[%d]", req.Id)}, err
	}

	return &pb.UpdateUserResponse{
		Code: 0,
		Msg:  fmt.Sprintf("用户更新成功, 用户ID: [%d]", req.Id),
		Data: &pb.User{
			Id:    int64(user.ID),
			Name:  user.Username,
			Phone: user.Phone,
			Email: user.Email,
		},
	}, nil
}

// ==================================查询用户服务==================================
func (s *UserServiceServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	logger.Info(fmt.Sprintf("获取用户 ID: %d", req.Id))
	// 埋点
	tracer := otel.GetTracer("userService-getUser")
	ctx, span := tracer.Start(ctx, "Service-GetUser")
	defer span.End()

	span.SetAttributes(attribute.Int64("user.id", req.Id))

	user, err := s.userApp.GetUserByID(ctx, uint(req.Id))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.Error(fmt.Sprintf("获取用户信息失败%d", req.Id), zap.Error(err))
		return &pb.GetUserResponse{
			Code: 10002,
			Msg:  "用户不存在",
		}, err
	}

	return &pb.GetUserResponse{
		Code: 0,
		Msg:  "success",
		Data: &pb.User{
			Id:    int64(user.ID),
			Name:  user.Username,
			Phone: user.Phone,
			Email: user.Email,
		},
	}, nil
}

func (s *UserServiceServer) ListUser(ctx context.Context, req *pb.ListUserRequest) (*pb.ListUserResponse, error) {
	// 埋点
	tracer := otel.GetTracer("userService-ListUser")
	ctx, span := tracer.Start(ctx, "Service-ListUser")
	defer span.End()

	user := entity.User{}
	_, dbSpan := tracer.Start(ctx, "DB-GetUsers")
	users, _, err := s.userApp.ListUsers(ctx, user.Username, user.Phone, user.Email, int(req.Page), int(req.Size))
	if err != nil {
		return &pb.ListUserResponse{Code: 50001, Msg: "查询失败"}, nil
	}
	dbSpan.End()

	var resDatas []*pb.User = make([]*pb.User, len(users))
	for index, user := range users {
		resDatas[index] = &pb.User{
			Id:    int64(user.ID),
			Name:  user.Username,
			Phone: user.Phone,
			Email: user.Email,
		}
	}

	return &pb.ListUserResponse{
		Code: 0,
		Msg:  "success",
		Data: resDatas,
	}, nil
}

// ==================================发送验证码==================================

func (s *UserServiceServer) SendCode(ctx context.Context, req *pb.SendCodeRequest) (*pb.SendCodeResponse, error) {
	tracer := otel.GetTracer("user-service")
	ctx, span := tracer.Start(ctx, "Service-SendCode")
	defer span.End()

	err := s.userApp.SendCode(ctx, req.Phone)
	if err != nil {
		span.RecordError(err)
		return &pb.SendCodeResponse{Code: 30001, Msg: err.Error()}, nil
	}

	return &pb.SendCodeResponse{Code: 0, Msg: "验证码已发送"}, nil
}

// ==================================手机验证码登录==================================
func (s *UserServiceServer) PhoneLogin(ctx context.Context, req *pb.PhoneLoginRequest) (*pb.LoginResponse, error) {
	tracer := otel.GetTracer("user-service")
	ctx, span := tracer.Start(ctx, "Service-PhoneLogin")
	defer span.End()

	result, err := s.userApp.PhoneLogin(ctx, req.Phone, req.Code)
	if err != nil {
		span.RecordError(err)
		return &pb.LoginResponse{Code: 10007, Msg: err.Error()}, nil
	}

	return &pb.LoginResponse{
		Code:         0,
		Msg:          "登录成功",
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
		User: &pb.User{
			Id:    int64(result.User.ID),
			Name:  result.User.Username,
			Phone: result.User.Phone,
			Email: result.User.Email,
		},
	}, nil
}

// ==================================邮箱密码登录==================================

func (s *UserServiceServer) EmailLogin(ctx context.Context, req *pb.EmailLoginRequest) (*pb.LoginResponse, error) {
	tracer := otel.GetTracer("user-service")
	ctx, span := tracer.Start(ctx, "Service-EmailLogin")
	defer span.End()

	result, err := s.userApp.EmailLogin(ctx, req.Email, req.Password)
	if err != nil {
		span.RecordError(err)
		return &pb.LoginResponse{Code: 10007, Msg: err.Error()}, nil
	}

	return &pb.LoginResponse{
		Code:         0,
		Msg:          "登录成功",
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
		User: &pb.User{
			Id:    int64(result.User.ID),
			Name:  result.User.Username,
			Phone: result.User.Phone,
			Email: result.User.Email,
		},
	}, nil
}

// ==================================通用账号密码登录==================================

func (s *UserServiceServer) PwdLogin(ctx context.Context, req *pb.PwdLoginRequest) (*pb.LoginResponse, error) {
	tracer := otel.GetTracer("user-service")
	ctx, span := tracer.Start(ctx, "Service-PwdLogin")
	defer span.End()

	result, err := s.userApp.PwdLogin(ctx, req.Account, req.Password)
	if err != nil {
		span.RecordError(err)
		return &pb.LoginResponse{Code: 10007, Msg: err.Error()}, nil
	}

	return &pb.LoginResponse{
		Code:         0,
		Msg:          "登录成功",
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
		User: &pb.User{
			Id:    int64(result.User.ID),
			Name:  result.User.Username,
			Phone: result.User.Phone,
			Email: result.User.Email,
		},
	}, nil
}
