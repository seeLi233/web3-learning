package application

import (
	"context"
	"fmt"

	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/common/pkg/otel"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/go-project-learning/project/user-srv/internal/repository"
	"github.com/go-project-learning/project/user-srv/internal/repository/cache"
	"github.com/go-project-learning/project/user-srv/pkg/bcrypt"
	"github.com/go-project-learning/project/user-srv/pkg/code"
	"github.com/go-project-learning/project/user-srv/pkg/jwt"
	"github.com/go-project-learning/project/user-srv/pkg/sms"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// TODO Register、CodeLogin、PwdLogin、UpdateUser、AddressCRUD、BindCoupon、AddBlack
type UserApp struct {
	smsSender sms.Sender
	userRepo  repository.UserRepository // 注入接口，不是具体类型
	memberApp *MemberApp                // 用户注册时初始化会员信息
}

// NewUserApp 创建 UserApp 实例
//
// 为什么现在多了一个 userRepo 参数？
// → 以前 UserApp 直接 import db 包调用包级函数——依赖是硬编码的。
//
//	现在通过构造函数从外部传入——依赖变成可替换的。
//
// 这种模式的好处（面试要能说）：
// → 测试时传 Mock：NewUserApp(smsSender, mockRepo, memberApp)
// → 切库时传新实现：NewUserApp(smsSender, pgRepo, memberApp)
// → 代码不改，行为可替换——开闭原则（对扩展开放，对修改关闭）
func NewUserApp(smsSender sms.Sender, userRepo repository.UserRepository, memberApp *MemberApp) *UserApp {
	return &UserApp{
		smsSender: smsSender,
		userRepo:  userRepo,
		memberApp: memberApp,
	}
}

// LoginResult 登录结果
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
	User         *entity.User
}

// SendCode 发送短信验证码
func (a *UserApp) SendCode(ctx context.Context, phone string) error {
	tracer := otel.GetTracer("user-app")
	ctx, span := tracer.Start(ctx, "App-SendCode")
	defer span.End()
	span.SetAttributes(attribute.String("phone", phone))

	// 1.检查频率限制
	if cache.CheckLimit(ctx, phone) {
		return fmt.Errorf("验证码发送过于频繁，请60秒后重试")
	}

	// 2.生成验证码
	code := code.GenerateCode()

	// 3.存入 Redis
	if err := cache.SetCode(ctx, phone, code); err != nil {
		logger.Error("存储验证码失败", zap.Error(err))
		return fmt.Errorf("系统错误")
	}

	// 4.设置频繁限制
	if err := cache.SetLimit(ctx, phone); err != nil {
		logger.Error("设置频率限制失败", zap.Error(err))
	}

	// 5.发送短信
	if err := a.smsSender.Send(phone, code); err != nil {
		logger.Error("发送短信失败", zap.String("phone", phone), zap.Error(err))
		return fmt.Errorf("短信发送失败")
	}

	return nil
}

// PhoneLogin 手机验证码登录 （不存在则自动注册）
func (a *UserApp) PhoneLogin(ctx context.Context, phone, verifyCode string) (*LoginResult, error) {
	tracer := otel.GetTracer("user-app")
	ctx, span := tracer.Start(ctx, "App-PhoneLogin")
	defer span.End()

	// 校验验证码
	savedCode, err := cache.GetCode(ctx, phone)
	if err != nil || savedCode != verifyCode {
		return nil, fmt.Errorf("验证码错误或已过期")
	}

	// 2. 删除已使用的验证码
	cache.DeleteCode(ctx, phone)

	// 3. 查询用户
	user, err := a.userRepo.GetByPhone(ctx, phone)
	if err != nil {
		// 用户不存在，自动注册
		span.AddEvent("用户不存在，自动注册")
		user = &entity.User{
			Phone:    phone,
			Username: "user_" + phone, // 默认用户名
			Status:   1,
		}
		if err := a.userRepo.Create(ctx, user); err != nil {
			logger.Error("自动注册失败", zap.Error(err))
			return nil, fmt.Errorf("注册失败")
		}

		// 注册成功后，初始化会员信息（默认普通会员）
		if err := a.memberApp.InitMember(ctx, user.ID); err != nil {
			// 会员初始化失败不影响登录，记录日志即可
			logger.Error(fmt.Sprintf("会员初始化失败, userId: %d", user.ID), zap.Error(err))
		}
	}

	// 4. 签发 JWT
	accessToken, refreshToken, expiresAt, err := jwt.GenerateToken(user.ID, user.Username, user.Phone)
	if err != nil {
		logger.Error("签发 JWT 失败", zap.Error(err))
		return nil, fmt.Errorf("系统错误")
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         user,
	}, nil
}

// EmailLogin 邮箱密码登录
func (a *UserApp) EmailLogin(ctx context.Context, email, password string) (*LoginResult, error) {
	tracer := otel.GetTracer("user-app")
	ctx, span := tracer.Start(ctx, "App-EmailLogin")
	defer span.End()

	// 1. 查询用户
	user, err := a.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

	// 2. 校验密码
	if !bcrypt.CheckPassword(user.Password, password) {
		return nil, fmt.Errorf("密码错误")
	}

	// 3. 签发 JWT
	accessToken, refreshToken, expiresAt, err := jwt.GenerateToken(
		user.ID, user.Username, user.Phone,
	)
	if err != nil {
		logger.Error("签发 JWT 失败", zap.Error(err))
		return nil, fmt.Errorf("系统错误")
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         user,
	}, nil
}

// PwdLogin 通用账号密码登录（支持手机/邮箱/用户名）
func (a *UserApp) PwdLogin(ctx context.Context, account, password string) (*LoginResult, error) {
	tracer := otel.GetTracer("user-app")
	ctx, span := tracer.Start(ctx, "App-PwdLogin")
	defer span.End()

	var user *entity.User
	var err error

	// 1. 判断 account 类型并查询
	user, err = a.userRepo.GetByUsername(ctx, account)
	if err != nil {
		user, err = a.userRepo.GetByEmail(ctx, account)
		if err != nil {
			user, err = a.userRepo.GetByPhone(ctx, account)
			if err != nil {
				return nil, fmt.Errorf("用户不存在")
			}
		}
	}

	// 2. 校验密码
	if !bcrypt.CheckPassword(user.Password, password) {
		return nil, fmt.Errorf("密码错误")
	}

	// 3. 签发 JWT
	accessToken, refreshToken, expiresAt, err := jwt.GenerateToken(
		user.ID, user.Username, user.Phone,
	)
	if err != nil {
		return nil, fmt.Errorf("系统错误")
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         user,
	}, nil
}

func (a *UserApp) GetUserByID(ctx context.Context, userID uint) (*entity.User, error) {
	return a.userRepo.GetByID(ctx, userID)
}

func (a *UserApp) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	return a.userRepo.GetByUsername(ctx, username)
}

func (a *UserApp) GetByPhone(ctx context.Context, phone string) (*entity.User, error) {
	return a.userRepo.GetByPhone(ctx, phone)
}

func (a *UserApp) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	return a.userRepo.GetByEmail(ctx, email)
}

// ============ 新增用户资料 ============
func (a *UserApp) ListUsers(ctx context.Context, username, phone, email string, page, pageSize int) ([]*entity.User, int64, error) {
	return a.userRepo.ListUsers(ctx, username, phone, email, page, pageSize)
}

func (a *UserApp) Create(ctx context.Context, user *entity.User) error {
	// 3. 保存到数据库
	if err := a.userRepo.Create(ctx, user); err != nil {
		return fmt.Errorf("新增失败")
	}

	return nil
}

// ============ 更新用户资料 ============

// UpdateUser 更新用户资料（用户名/手机/邮箱可选更新）
//
// 业务规则：
//  1. 只更新非空字段（传入空字符串 = 不修改该字段）
//  2. 更新前检查唯一性（用户名/手机/邮箱不能和别人重复）
//  3. 检查时排除自己（自己的用户名/手机/邮箱不算冲突）
func (a *UserApp) UpdateUser(ctx context.Context, userID uint, username, phone, email string) (*entity.User, error) {
	// 先查询用户是否存在
	user, err := a.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

	// 更新字段, 只更新非空字段
	if username != "" {
		// 检查用户名是否已经被占用
		if exists, _ := a.userRepo.ExistsByUsername(ctx, username, userID); exists {
			return nil, fmt.Errorf("用户名已被占用")
		}
		user.Username = username
	}

	if phone != "" {
		// 检查手机号是否已经被占用
		if exists, _ := a.userRepo.ExistsByPhone(ctx, phone, userID); exists {
			return nil, fmt.Errorf("手机号已被占用")
		}
		user.Phone = phone
	}

	if email != "" {
		// 检查邮箱是否已经被占用
		if exists, _ := a.userRepo.ExistsByEmail(ctx, email, userID); exists {
			return nil, fmt.Errorf("邮箱已被占用")
		}
		user.Email = email
	}

	// 3. 保存到数据库
	if err := a.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("更新失败")
	}

	return user, nil
}
