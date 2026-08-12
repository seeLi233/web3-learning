package db

import (
	"context"

	"github.com/go-project-learning/project/common/pkg/otel"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/go-project-learning/project/user-srv/internal/repository"
	"github.com/go-project-learning/project/user-srv/internal/repository/cache"
	"gorm.io/gorm"
)

// ============ 结构体 + 构造函数 ============

// UserRepo GORM 实现的用户数据访问层
//
// 为什么 db 字段是小写（私有）？
// → 外部不应该直接访问 r.db——如果暴露了，调用方可能绕过 Repository 直接写 SQL，
//
//	这就破坏了"数据访问统一走 Repository"的约定。
//	小写字段 + 构造函数 = 强制通过方法来操作数据。
type UserRepo struct {
	db *gorm.DB
}

// NewUserRepo 创建 UserRepo 实例（构造函数注入）
//
// 为什么用构造函数注入而不是直接用全局变量？
// → 1. 测试时可以注入测试数据库或 Mock
// → 2. 一个进程连接多个数据库时（主库/从库），可以创建不同的 UserRepo 实例
// → 3. 依赖关系在创建时确定，编译器帮你检查——忘记注入会编译失败
func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

// ============ 编译时接口验证 ============

// var _ repository.UserRepository = (*UserRepo)(nil) 会在编译时强制检查
// *UserRepo 是否实现了 UserRepository 接口——如果方法签名不匹配，编译直接失败。
var _ repository.UserRepository = (*UserRepo)(nil)

// ----------------------------------------1. 创建用户----------------------------------------

// Create 创建用户
func (r *UserRepo) Create(ctx context.Context, user *entity.User) error {
	tracer := otel.GetTracer("mysql-user")
	ctx, span := tracer.Start(ctx, "MySQL-CreateUser")
	defer span.End()

	return r.db.WithContext(ctx).Create(user).Error
}

// BatchInsertUsers 批量插入用户（接口外的扩展方法）
//
// 为什么这个方法不在 UserRepository 接口里？
// → 接口只定义核心 CRUD。批量操作是管理后台或数据迁移用的，业务层一般不调。
//
//	遵循 YAGNI 原则：接口里不放"可能用到"的方法，只放"一定会用到"的。
func (r *UserRepo) BatchInsertUsers(users []entity.User) error {
	return r.db.CreateInBatches(users, 20).Error
}

// ----------------------------------------2. 根据ID查询用户----------------------------------------

// GetByID 根据主键 ID 查询用户（带缓存策略）
func (r *UserRepo) GetByID(ctx context.Context, id uint) (*entity.User, error) {
	tracer := otel.GetTracer("mysql-user")
	ctx, span := tracer.Start(ctx, "MySQL-GetByID")
	defer span.End()

	// 1. 先查缓存
	if user, ok := cache.GetUserCache(ctx, int(id)); ok {
		span.AddEvent("命中 Redis 缓存")
		return user, nil
	}

	// 2. 查 DB
	var user entity.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		return nil, err
	}

	// 3. 写缓存
	cache.SetUserCache(ctx, &user)

	return &user, nil
}

// GetByPhone 根据手机号查询用户
func (r *UserRepo) GetByPhone(ctx context.Context, phone string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("phone = ?", phone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByEmail 根据邮箱查询用户
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ----------------------------------------3. 根据用户名查询用户----------------------------------------

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	tracer := otel.GetTracer("mysql-user")
	ctx, span := tracer.Start(ctx, "MySQL-GetByUsername")
	defer span.End()

	var user entity.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) ExistsByUsername(ctx context.Context, username string, excludeID uint) (bool, error) {
	tracer := otel.GetTracer("mysql-user")
	ctx, span := tracer.Start(ctx, "MySQL-ExistsByUsername")
	defer span.End()
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.User{}).Where("username = ? and id <> ?", username, excludeID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepo) ExistsByPhone(ctx context.Context, phone string, excludeID uint) (bool, error) {
	tracer := otel.GetTracer("mysql-user")
	ctx, span := tracer.Start(ctx, "MySQL-ExistsByPhone")
	defer span.End()
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.User{}).Where("phone = ? and id <> ?", phone, excludeID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepo) ExistsByEmail(ctx context.Context, email string, excludeID uint) (bool, error) {
	tracer := otel.GetTracer("mysql-user")
	ctx, span := tracer.Start(ctx, "MySQL-ExistsByEmail")
	defer span.End()
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.User{}).Where("email = ? and id <> ?", email, excludeID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ----------------------------------------4. 更新用户信息----------------------------------------

// Update 更新用户信息（全量更新 Save）
func (r *UserRepo) Update(ctx context.Context, user *entity.User) error {
	err := r.db.WithContext(ctx).Save(user).Error
	if err == nil {
		cache.DeleteUserCache(ctx, int(user.ID))
	}
	return err
}

// ----------------------------------------5. 删除用户----------------------------------------

// Delete 根据 ID 删除用户（软删除——GORM 默认行为）
func (r *UserRepo) Delete(ctx context.Context, id uint) error {
	err := r.db.WithContext(ctx).Delete(&entity.User{}, id).Error
	if err == nil {
		cache.DeleteUserCache(ctx, int(id))
	}
	return err
}

// ----------------------------------------6. 分页获取用户信息----------------------------------------

// ListUsers 分页查询用户列表（管理后台用）
func (r *UserRepo) ListUsers(ctx context.Context, username, phone, email string, page, pageSize int) ([]*entity.User, int64, error) {
	var users []entity.User
	var total int64

	db := r.db.WithContext(ctx).Model(&entity.User{})

	if username != "" {
		db = db.Where("username LIKE ?", "%"+username+"%")
	}
	if phone != "" {
		db = db.Where("phone LIKE ?", "%"+phone+"%")
	}
	if email != "" {
		db = db.Where("email LIKE ?", "%"+email+"%")
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	err := db.
		Order("ID DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	// 转换为指针切片，与接口其余方法保持一致
	result := make([]*entity.User, len(users))
	for i := range users {
		result[i] = &users[i]
	}

	return result, total, nil
}

// ----------------------------------------7. 查询用户订单信息----------------------------------------

// GetUserWithOrdersByID 查询用户及其订单（Preload 预加载关联数据）
func (r *UserRepo) GetUserWithOrdersByID(uid uint) (*entity.User, error) {
	var user entity.User
	err := r.db.Preload("Orders").Preload("Orders.Items").Preload("Orders.Items.Product").First(&user, uid).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
