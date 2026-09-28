// 串行约束：顶层用例各自装配独立内存库，但 TestCrossTenant... 的子测试共享
// 同一 sqlite 内存库（cache=shared）与同一份 fixture 数据，并发写会触发
// `database table is locked`；且 delete 子测试有顺序依赖（放最后）。
// nolint 作用域是文件级，故此处与 fixture_test.go 各写一次。
//
//nolint:paralleltest // 共享 sqlite 内存库 + 子测试顺序依赖，必须串行
package cross_tenant_session

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// 覆盖范围说明：
//
// 本包覆盖 sqlite 可执行的 5 条归属路径（元数据 / 详情 / 评分 / 分享 / 删除）。
// 另外 2 条路径的 SQL 含 PostgreSQL 专属语法，sqlite 无法解析，按仓库既有
// 惯例（见 test/unit/session_export_repository 的包注释）下沉到仓储层测试：
//
//   - 列表 ListSessionsByOwnerIDs：SessionSummarySelect 含
//     jsonb_array_length(message_ids::jsonb)
//   - 导出预览 PreviewExport：SELECT 含 jsonb_array_elements_text
//
// 其中最高危的导出路径（泄露完整会话内容）由 ListSessionsForExport 在
// test/unit/session_export_repository 真实执行断言，该方法 sqlite 兼容。

// TestCrossTenantSameKeyNameCannotReachOthersSession 跨用户同名 API Key 不得触达他人会话。
//
// 用户 B 持有与用户 A 同名的 API Key（"shared-name"）。按名称归属的实现会把
// A 的会话判成 B 的——改按 api_key_id 判定后本用例必须全绿。
//
// 子测试串行（不 t.Parallel）：共享同一 sqlite 内存库，并发写会触发表锁。
func TestCrossTenantSameKeyNameCannotReachOthersSession(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	sid := f.sessionA.ID

	t.Run("metadata rejected", func(t *testing.T) {
		status, data := f.do(t, http.MethodGet,
			fmt.Sprintf("%s/session/metadata?id=%d", constant.WebAPIPrefix, sid), f.tokenB, "")
		requireRejected(t, status, data, "B must not read A's session metadata")
	})

	t.Run("detail rejected", func(t *testing.T) {
		status, data := f.do(t, http.MethodGet,
			fmt.Sprintf("%s/session?id=%d", constant.WebAPIPrefix, sid), f.tokenB, "")
		requireRejected(t, status, data, "B must not read A's session detail")
	})

	t.Run("score rejected and has no effect", func(t *testing.T) {
		body := fmt.Sprintf(`{"sessionId":%d,"score":1}`, sid)
		status, data := f.do(t, http.MethodPost,
			constant.WebAPIPrefix+"/session/score", f.tokenB, body)
		requireRejected(t, status, data, "B must not score A's session")

		// 评分必须真的没发生：重新读库确认分数未被改写
		// （fixture 内存对象不经请求链路更新，读它会得到恒真断言）
		var row dbmodel.Session
		if err := f.db.First(&row, sid).Error; err != nil {
			t.Fatalf("reload session: %v", err)
		}
		if row.Score == nil || *row.Score != 5 {
			t.Fatalf("A's session score must stay 5, got %v", row.Score)
		}
	})

	t.Run("share rejected", func(t *testing.T) {
		body := fmt.Sprintf(`{"sessionId":%d,"expiresIn":"1d"}`, sid)
		status, data := f.do(t, http.MethodPost,
			constant.WebAPIPrefix+"/session/share", f.tokenB, body)
		requireRejected(t, status, data, "B must not share A's session")
	})

	// delete 放最后：它是破坏性操作，前面的用例依赖会话存活
	t.Run("delete rejected and session survives", func(t *testing.T) {
		status, data := f.do(t, http.MethodDelete,
			fmt.Sprintf("%s/session?ids=%d", constant.WebAPIPrefix, sid), f.tokenB, "")
		if status != http.StatusOK {
			t.Fatalf("expect HTTP 200 envelope, got status=%d body=%s", status, data)
		}
		// 批量删除接口以 failures 列表表达单条失败，不返回顶层 error；
		// deletedCount 必须为 0（有删除成功即越权）
		if strings.Contains(string(data), `"deletedCount":1`) {
			t.Fatalf("delete must not succeed for foreign session, body=%s", data)
		}
		if !strings.Contains(string(data), `"failures"`) {
			t.Fatalf("delete must report failure for foreign session, body=%s", data)
		}

		// 删除必须真的没发生：会话仍存活（未被软删）
		var row dbmodel.Session
		if err := f.db.First(&row, sid).Error; err != nil {
			t.Fatalf("A's session must survive B's delete attempt: %v", err)
		}
		if row.DeletedAt != 0 {
			t.Fatalf("A's session must not be soft-deleted, deleted_at=%d", row.DeletedAt)
		}
	})
}

// TestOwnSessionStillReachable 正向守护：A 必须仍能访问自己的会话。
//
// 防止修复走向另一极端（过度收紧把合法访问也拒了）。
func TestOwnSessionStillReachable(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	sid := f.sessionA.ID

	status, data := f.do(t, http.MethodGet,
		fmt.Sprintf("%s/session/metadata?id=%d", constant.WebAPIPrefix, sid), f.tokenA, "")
	if status != http.StatusOK || bizErrorCode(t, data) != 0 {
		t.Fatalf("A must reach own session metadata, status=%d body=%s", status, data)
	}
	if !strings.Contains(string(data), `"id":1`) {
		t.Fatalf("A's own session must be returned, body=%s", data)
	}
}

// TestOrphanSessionNotReachableByNormalUser 空归属会话对普通用户不可达。
//
// 约 2.6k 条存量会话的 api_key_name 为空（改造后 api_key_id 为 0），归属信息
// 已永久丢失、不做推断。它们不得因「空值匹配」泄露给任意普通用户，
// 但 admin 必须仍可访问（运维需要）。
func TestOrphanSessionNotReachableByNormalUser(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	orphanID := f.sessionOrphan.ID
	path := fmt.Sprintf("%s/session/metadata?id=%d", constant.WebAPIPrefix, orphanID)

	for name, token := range map[string]string{"tenantA": f.tokenA, "tenantB": f.tokenB} {
		status, data := f.do(t, http.MethodGet, path, token, "")
		requireRejected(t, status, data, name+" must not reach orphan session")
	}

	status, data := f.do(t, http.MethodGet, path, f.tokenAdmin, "")
	if status != http.StatusOK || bizErrorCode(t, data) != 0 {
		t.Fatalf("admin must reach orphan session, status=%d body=%s", status, data)
	}
}
