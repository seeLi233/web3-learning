//go:build e2e

// 为什么用 build tag 而不是塞进普通测试？
// → 让 E2E 和单元测试解耦：普通 go test 自动跳过（不碰 Docker），
//    E2E 只在 go test -tags=e2e 时显式运行。这是 Go 生态隔离慢测试的标准做法

package e2e

import (
	"testing"

	"github.com/go-project-learning/project/common/pkg/database"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"gorm.io/gorm"
)

// hardDeleteAll 硬删除所有测试涉及的表，返回错误（供前后清理复用）
//
// 为什么用 Unscoped()？
// → entity.User / MemberInfo 内嵌 gorm.Model，带 DeletedAt（软删除）。
//
//	普通 Delete 只标记 deleted_at，记录还在、唯一索引还被占，
//	下次重跑会撞 "Duplicate entry"。Unscoped() 做硬删除，真正释放唯一索引
func hardDeleteAll() error {
	// 为什么这个顺序？（先 MemberInfo 后 User）
	// → member_infos.user_id 外键引用 users.id，先删子表再删父表，避免外键约束冲突
	tables := []interface{}{&entity.MemberInfo{}, &entity.User{}}
	for _, table := range tables {
		err := database.DB.Unscoped().
			Session(&gorm.Session{AllowGlobalUpdate: true}). // 允许无 WHERE 的全表删除
			Delete(table).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// cleanDatabase 测试「前清」：失败直接终止测试（前置条件不满足，后面没意义）
func cleanDatabase(t *testing.T) {
	t.Helper()
	if err := hardDeleteAll(); err != nil {
		t.Fatalf("清空表失败: %v", err)
	}
}

// setupTest 测试前置：前清 + 后清，双向隔离
//
// 为什么前后都清？
// → "前清"保证本测试从干净状态开始（不依赖上个测试的残留）
//
//	"后清"保证本测试的脏数据不污染下个测试（谁都不依赖谁）
//	双向隔离是"稳定不 flaky"的第 1 条军规
func setupTest(t *testing.T) {
	t.Helper()
	cleanDatabase(t) // 前清
	t.Cleanup(func() {
		// cleanup 里不能用 Fatalf（会中断），失败只 Errorf 记录
		if err := hardDeleteAll(); err != nil {
			t.Errorf("清理残留数据失败: %v", err)
		}
	})
}
