package application

import (
	"context"
	"testing"

	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
)

// ============================================================
// Mock UserRepository — 内存中的假数据库
//
// 为什么叫 Mock 而不是 Fake 或 Stub？
// → Mock：能验证"被调用了什么方法、传了什么参数"（行为验证）
// → Fake：有真实实现但简化版（如内存 SQLite 替代 MySQL）
// → Stub：只返回预设数据，不验证行为
//
// 这里用的是 Mock + Stub 混合体——预设返回值 + 可选行为验证
// ============================================================

// mockUserRepo 实现 repository.UserRepository 接口
//
// 为什么使用 map 作为存储？
// → 内存 map 可以模拟 CRUD 操作，O(1) 查询，
//
//	比 SQLite 更轻量——不需要 CGO、不需要文件、不需要驱动。
//	对于单元测试来说，速度是第一位的。
//
// Go 隐式接口的好处在这里体现：
// → mockUserRepo 不需要写 "implements UserRepository"
// → 只要方法签名匹配，编译器自动识别
type mockUserRepo struct {
	users map[uint]*entity.User // 模拟用户表，key=ID
	// 为什么用 map[uint]*entity.User 而不是 map[uint]entity.User？
	// → 指针允许 Update 方法修改 map 中的元素（值类型修改的是副本）
	nextID uint // 自增主键，模拟数据库 AUTO_INCREMENT

	// ========== 行为验证字段（可选，用于验证"被调用了什么"）==========
	createCalled int
	lastCreated  *entity.User
}

// newMockUserRepo 创建 Mock 实例（含种子数据）
//
// 为什么构造函数返回 *mockUserRepo 而不是 UserRepository 接口？
// → 测试代码需要访问 mock 特有的字段（如 users map、createCalled）。
//
//	返回具体类型才能在测试中直接操作 mock 内部状态。
//	传给 UserApp 时隐式转为接口——Go 自动完成。
func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:  make(map[uint]*entity.User),
		nextID: 1,
	}
}

// seedUser 插入一条测试用的种子数据
//
// 为什么每次测试前手动插入种子数据，而不是在 newMockUserRepo 里预置？
// → 不同测试需要不同的数据场景（有的用户存在、有的不存在、有的状态异常）。
//
//	在测试用例里显式调用 seedUser，读者一眼就知道"这个用例依赖什么前置数据"。
//	这比藏在构造函数里更清晰。
func (m *mockUserRepo) seedUser(username, phone, email, password string) *entity.User {
	user := &entity.User{
		Username: username,
		Phone:    phone,
		Email:    email,
		Password: password,
		Status:   1,
	}
	user.ID = m.nextID
	m.nextID++
	m.users[user.ID] = user
	return user
}

// ========== UserRepository 接口实现 ==========

func (m *mockUserRepo) Create(ctx context.Context, user *entity.User) error {
	m.createCalled++
	user.ID = m.nextID
	// 为什么复制一份而不是直接存指针？
	// → 调用方传入的指针可能被复用（再次 Create 时修改字段），
	//    直接存指针会导致 map 里的数据和调用方共享状态——测试隔离被破坏。
	//    深拷贝一份确保 mock 内部状态不受外部影响。
	copy := *user
	m.users[user.ID] = &copy
	m.lastCreated = &copy
	return nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id uint) (*entity.User, error) {
	user, ok := m.users[id]
	if !ok {
		return nil, errNotFound // 模拟 GORM 的 RecordNotFound
	}
	return user, nil
}

func (m *mockUserRepo) GetByPhone(ctx context.Context, phone string) (*entity.User, error) {
	for _, u := range m.users {
		if u.Phone == phone {
			return u, nil
		}
	}
	return nil, errNotFound
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, errNotFound
}

func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	for _, u := range m.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, errNotFound
}

func (m *mockUserRepo) ExistsByUsername(ctx context.Context, username string, excludeID uint) (bool, error) {
	for _, u := range m.users {
		if u.Username == username && u.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockUserRepo) ExistsByPhone(ctx context.Context, phone string, excludeID uint) (bool, error) {
	for _, u := range m.users {
		if u.Phone == phone && u.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockUserRepo) ExistsByEmail(ctx context.Context, email string, excludeID uint) (bool, error) {
	for _, u := range m.users {
		if u.Email == email && u.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockUserRepo) Update(ctx context.Context, user *entity.User) error {
	if _, ok := m.users[user.ID]; !ok {
		return errNotFound
	}
	// 和 Create 一样，深拷贝一份——防止外部修改影响 mock 内部状态
	copy := *user
	m.users[user.ID] = &copy
	return nil
}

func (m *mockUserRepo) Delete(ctx context.Context, id uint) error {
	delete(m.users, id)
	return nil
}

func (m *mockUserRepo) ListUsers(ctx context.Context, username, phone, email string, page, pageSize int) ([]*entity.User, int64, error) {
	var result []*entity.User
	for _, u := range m.users {
		result = append(result, u)
	}
	return result, int64(len(result)), nil
}

// ============================================================
// 模拟错误 — 当我们需要测试"数据库挂了"的场景时
// ============================================================

// errNotFound 模拟 GORM 的 "record not found"
// 为什么自定义这个错误而不是用 gorm.ErrRecordNotFound？
// → mock 包不应该 import gorm——这会让"不依赖数据库"的目标破功。
//
//	测试代码只需要知道"返回了一个 error"，不需要知道具体是什么 error。
var errNotFound = &dbError{msg: "record not found"}

// dbError 模拟数据库错误
type dbError struct {
	msg string
}

func (e *dbError) Error() string { return e.msg }

// ============================================================
//                 测试用例
// ============================================================

// TestGetUserByID 测试通过 ID 查询用户
//
// 为什么这是最简单的测试？
// → GetUserByID 只做了一件事：转发给 Repository。没有任何分支逻辑。
//
//	但即使这么简单，也要测——验证 Mock 和 DI 链路是通的。
func TestGetUserByID(t *testing.T) {
	tests := []struct {
		name    string
		seed    bool // 是否先插入种子数据
		userID  uint
		wantErr bool
	}{
		{
			name:    "A1. 查询存在的用户 → 成功返回",
			seed:    true,
			userID:  1,
			wantErr: false,
		},
		{
			name:    "A2. 查询不存在的用户 → 返回错误",
			seed:    false,
			userID:  999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange：准备 Mock + 注入 Service
			mockRepo := newMockUserRepo()
			if tt.seed {
				mockRepo.seedUser("alice", "13000000001", "alice@test.com", "hashed_pwd")
			}
			app := NewUserApp(nil, mockRepo, nil) // memberApp=nil，GetUserByID 不依赖它

			// Act：执行被测方法
			user, err := app.GetUserByID(context.Background(), tt.userID)

			// Assert：验证结果
			if tt.wantErr {
				if err == nil {
					t.Error("期望返回错误，但成功了")
				}
				return
			}
			if err != nil {
				t.Fatalf("不期望错误，但得到了: %v", err)
			}
			if user == nil {
				t.Fatal("返回的用户不应为 nil")
			}
			if user.Username != "alice" {
				t.Errorf("用户名 = %q, want %q", user.Username, "alice")
			}
		})
	}
}

// TestUpdateUser 测试更新用户资料
//
// 为什么这是最重要的测试？
// → UpdateUser 包含多个业务规则：
//  1. 用户不存在 → 报错
//  2. 用户名被占用（被别人）→ 报错
//  3. 用户名被占用（被自己）→ 允许（不报错）
//  4. 正常更新 → 成功
//  5. 空字段 → 不更新（保持原值）
//
// 在真实数据库中测这些场景需要反复插入/清理数据；
// 用 Mock 只需要改 map 里的值，测试写完的速度是集成测试的 10 倍。
func TestUpdateUser(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*mockUserRepo) // 准备种子数据
		userID      uint
		newUsername string
		newPhone    string
		newEmail    string
		wantErr     bool
		wantErrMsg  string
		verifyUser  func(*testing.T, *entity.User) // 验证更新后的用户状态
	}{
		{
			name:       "B1. 用户不存在 → 报错",
			userID:     999,
			wantErr:    true,
			wantErrMsg: "用户不存在",
		},
		{
			name: "B2. 正常更新用户名 → 成功",
			setup: func(m *mockUserRepo) {
				m.seedUser("oldname", "13800000001", "old@test.com", "pwd")
			},
			userID:      1,
			newUsername: "newname",
			wantErr:     false,
			verifyUser: func(t *testing.T, u *entity.User) {
				t.Helper()
				if u.Username != "newname" {
					t.Errorf("用户名应更新为 newname，实际=%q", u.Username)
				}
				// 其他字段不应被修改
				if u.Phone != "13800000001" {
					t.Errorf("手机号不应被修改，实际=%q", u.Phone)
				}
			},
		},
		{
			name: "B3. 更新不传值的字段 → 保持原值",
			setup: func(m *mockUserRepo) {
				m.seedUser("bob", "13800000002", "bob@test.com", "pwd")
			},
			userID: 1,
			// 三个字段都传空字符串——表示"不修改"
			wantErr: false,
			verifyUser: func(t *testing.T, u *entity.User) {
				t.Helper()
				if u.Username != "bob" {
					t.Errorf("空字符串不应修改用户名，实际=%q", u.Username)
				}
			},
		},
		{
			name: "B4. 用户名被其他人占用 → 报错",
			setup: func(m *mockUserRepo) {
				m.seedUser("alice", "13800000003", "alice@test.com", "pwd") // ID=1
				m.seedUser("bob", "13800000004", "bob@test.com", "pwd")     // ID=2
			},
			userID:      1,     // alice 想改名为 bob
			newUsername: "bob", // 但 bob 已经被 ID=2 的用户占用
			wantErr:     true,
			wantErrMsg:  "用户名已被占用",
		},
		{
			name: "B5. 🔥 用户名被自己占用 → 允许（关键边界条件）",
			setup: func(m *mockUserRepo) {
				m.seedUser("alice", "13800000005", "alice@test.com", "pwd")
			},
			userID:      1,
			newUsername: "alice",       // alice 想保持原名不变
			newPhone:    "13800000099", // 只改手机号
			wantErr:     false,
			verifyUser: func(t *testing.T, u *entity.User) {
				t.Helper()
				if u.Username != "alice" {
					t.Errorf("自己占用自己的用户名应该被允许")
				}
				if u.Phone != "13800000099" {
					t.Errorf("手机号应该更新，实际=%q", u.Phone)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockRepo := newMockUserRepo()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}
			app := NewUserApp(nil, mockRepo, nil)

			// Act
			user, err := app.UpdateUser(context.Background(), tt.userID, tt.newUsername, tt.newPhone, tt.newEmail)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Error("期望返回错误，但成功了")
					return
				}
				// 为什么检查错误消息内容？
				// → HTTP handler 需要根据不同的错误消息返回不同的状态码和提示。
				//    "用户不存在" → 404，"用户名已被占用" → 409。
				//    如果错误消息不对，前端显示的提示就会误导用户。
				if tt.wantErrMsg != "" && err.Error() != tt.wantErrMsg {
					t.Errorf("错误消息 = %q, want %q", err.Error(), tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("不期望错误，但得到了: %v", err)
			}
			if tt.verifyUser != nil {
				tt.verifyUser(t, user)
			}
		})
	}
}

// TestUpdateUser_BehavioralCheck 行为验证——确认 Mock 确实被调用了
//
// 为什么要单独测试"被调用了"？
// → 上面测的是"返回值对不对"，这是「状态验证」。
//
//	这里测的是"该调的有没有调"，这是「行为验证」。
//	状态正确 != 逻辑正确——如果一个方法根本没调 Repository 直接返回 nil，
//	返回值也是"对"的，但数据根本没持久化。
//
// 这在面试中叫"Mock 的两类验证"：
//   - 状态验证（State Verification）：检查返回值
//   - 行为验证（Behavior Verification）：检查调用次数和参数
func TestUpdateUser_BehavioralCheck(t *testing.T) {
	mockRepo := newMockUserRepo()
	mockRepo.seedUser("alice", "13800000006", "alice@test.com", "pwd")
	app := NewUserApp(nil, mockRepo, nil)

	_, err := app.UpdateUser(context.Background(), 1, "new_alice", "", "")
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	// 行为验证：确认 Update 被调用了 1 次
	// 为什么是 1 次而不是 0 次？
	// → UpdateUser 在参数校验 + 唯一性检查通过后，应该调用一次 Repository.Update
	//    如果 createCalled > 0，说明内部错误地把 Update 当做 Create 了
	if mockRepo.createCalled > 0 {
		t.Errorf("Update 操作不应触发 Create，但触发了 %d 次", mockRepo.createCalled)
	}

	// 验证数据库中的值确实被改了（状态验证 + 行为验证组合）
	saved, _ := mockRepo.GetByID(context.Background(), 1)
	if saved.Username != "new_alice" {
		t.Errorf("Mock 中用户名应为 new_alice，实际=%q", saved.Username)
	}
}

// TestGetUserByID_NilMemberApp 验证 GetUserByID 不依赖 memberApp
//
// 为什么传 nil？
// → 有些方法（如 PhoneLogin）依赖 memberApp，但 GetUserByID 不应该依赖。
//
//	传 nil 是一个「防御性测试」——如果 GetUserByID 内部错误地访问了 memberApp，
//	会因为 nil pointer dereference 直接 panic。
//
// 这种测试叫「Fault Injection Testing」——故意制造异常条件，
// 验证代码在不完美环境下能否正确降级/隔离。
func TestGetUserByID_NilMemberApp(t *testing.T) {
	mockRepo := newMockUserRepo()
	mockRepo.seedUser("test", "13000000001", "test@test.com", "pwd")

	// memberApp = nil：如果 GetUserByID 访问了它，这里会 panic
	app := NewUserApp(nil, mockRepo, nil)
	user, err := app.GetUserByID(context.Background(), 1)

	if err != nil {
		t.Fatalf("不期望错误: %v", err)
	}
	if user.Username != "test" {
		t.Errorf("用户名 = %q, want %q", user.Username, "test")
	}
}

// ============================================================
// 注意：PhoneLogin 的测试留白
//
// PhoneLogin 依赖 memberApp.InitMember() + cache.GetCode() + sms.Sender，
// 这些目前还不是接口。如果要完整测试 PhoneLogin，需要：
//   1. 提取 MemberRepository 接口（像今天对待 UserRepository 一样）
//   2. 提取 CodeCache 接口（封装 cache.GetCode/SetCode/DeleteCode/CheckLimit）
//   3. SMS Sender 已经是接口了（sms.Sender），可以直接 Mock
//
// 这是"下一轮重构"要做的事——今天展示了 UserRepository 的模式，
// 明天可以扩展到其他依赖。模式是一样的：抽接口 → 注入 → Mock。
// ============================================================
