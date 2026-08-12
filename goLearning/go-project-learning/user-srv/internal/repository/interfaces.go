package repository

import (
	"context"

	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
)

// UserRepository 定义用户数据访问的抽象接口
//
// 为什么用 interface 而不是直接依赖 *db.UserRepo？
// → 1. 可测试性：测试时注入 Mock，不需要真实数据库
// → 2. 可替换性：以后换 ORM、换数据库（MySQL→PostgreSQL），只改实现不改 Service
// → 3. 依赖倒置：Service 依赖抽象（接口），不依赖具体（GORM）
//
// Go 的优势：不需要显式声明 implements，只要 struct 有这些方法就自动满足接口
type UserRepository interface {
	// Create 创建用户
	// 为什么第一个方法是 Create？
	// → CRUD 是最基本的数据操作，按 C-R-U-D 顺序排列，符合阅读习惯
	Create(ctx context.Context, user *entity.User) error

	// GetByID 根据主键 ID 查询用户
	// 为什么返回值是 (*entity.User, error) 而不是 (entity.User, error)？
	// → 指针允许返回 nil（用户不存在），值类型总有零值，无法区分"不存在的用户"和"空用户"
	GetByID(ctx context.Context, id uint) (*entity.User, error)

	// GetByPhone 根据手机号查询用户
	GetByPhone(ctx context.Context, phone string) (*entity.User, error)

	// GetByEmail 根据邮箱查询用户
	GetByEmail(ctx context.Context, email string) (*entity.User, error)

	// GetByUsername 根据用户名查询用户
	GetByUsername(ctx context.Context, username string) (*entity.User, error)

	// ExistsByUsername 检查用户名是否被占用（排除指定用户）
	// 为什么需要 userId 参数？
	// → 更新用户时，用户自己的用户名不变，不应判定为"被占用"
	//    传入当前用户 ID 排除自己：WHERE username = ? AND id <> ?
	ExistsByUsername(ctx context.Context, username string, excludeID uint) (bool, error)

	// ExistsByPhone 检查手机号是否被占用（排除指定用户）
	ExistsByPhone(ctx context.Context, phone string, excludeID uint) (bool, error)

	// ExistsByEmail 检查邮箱是否被占用（排除指定用户）
	ExistsByEmail(ctx context.Context, email string, excludeID uint) (bool, error)

	// Update 更新用户信息（部分更新）
	// 为什么叫 Update 而不是 Save？
	// → Save 在 GORM 里是"全量保存"（零值字段也会写进数据库），Update 更接近"部分更新"的语义
	Update(ctx context.Context, user *entity.User) error

	// ListUsers 分页查询用户列表（管理后台用）
	ListUsers(ctx context.Context, username, phone, email string, page, pageSize int) ([]*entity.User, int64, error)

	// Delete 根据 ID 删除用户
	Delete(ctx context.Context, id uint) error
}

// AddressRepository 定义地址数据访问的抽象接口
//
// 为什么单独定义 Address 的接口而不是放在 UserRepository 里？
// → 接口隔离原则（ISP）：客户端不应依赖它不需要的方法。
//
//	处理地址的 Service 不需要用户 CRUD 方法，处理用户的 Service 不需要地址方法。
//	分离后各自独立演化，改动一个不影响另一个。
type AddressRepository interface {
	// Create 创建地址
	Create(ctx context.Context, addr *entity.Address) (*entity.Address, error)

	// Update 更新地址
	Update(ctx context.Context, addr *entity.Address) (*entity.Address, error)

	// Delete 删除地址
	Delete(ctx context.Context, id uint) error

	// GetByID 根据 ID 查询地址
	GetByID(ctx context.Context, id uint) (*entity.Address, error)

	// ListByUserID 查询用户的所有地址
	ListByUserID(ctx context.Context, userID uint) ([]*entity.Address, error)
}
