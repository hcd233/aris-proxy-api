package oauth_credential

import (
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	llmvo "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

// TestUpdateQuotaKeepsTokens 额度/错误窄列更新不得触碰 token 列。
//
// 回归（CR C1）的持久化侧保障：维护任务在 token 刷新之后写额度，若走整行 upsert
// （Save）会把 DB 中刚轮换出的新 token 覆盖回内存快照里的旧值。
// 该用例同时覆盖 serializer:json 列在 Select 白名单更新下能否正确落库。
func TestUpdateQuotaKeepsTokens(t *testing.T) { //nolint:paralleltest // modifies global config
	// 不并行：config.OAuthCredentialKey 是包级全局变量
	ctx := t.Context()
	config.OAuthCredentialKey = make([]byte, constant.SecretBoxKeyLength)
	t.Cleanup(func() { config.OAuthCredentialKey = nil })

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.EndpointOAuthCredential{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := repository.NewOAuthCredentialRepository(db)
	cred, err := aggregate.CreateOAuthCredential(1, enum.OAuthProviderOpenAI, "at-old", "rt-old", time.Now().Add(time.Hour), "acc-1", "a@b.c", "plus")
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	if err := repo.Save(ctx, cred); err != nil {
		t.Fatalf("save: %v", err)
	}

	// 模拟 token provider 刷新：DB 中的 token 变成新的（旧 refresh token 已被上游轮换）
	refreshed, err := repo.FindByEndpointID(ctx, 1)
	if err != nil || refreshed == nil {
		t.Fatalf("find refreshed credential: %v", err)
	}
	refreshed.ApplyTokens("at-new", "rt-new", time.Now().Add(2*time.Hour))
	if err := repo.Save(ctx, refreshed); err != nil {
		t.Fatalf("save refreshed: %v", err)
	}

	// 额度探测落库：只更新额度列与错误列
	snapshot := llmvo.QuotaSnapshot{
		Windows:    []llmvo.QuotaWindow{{Key: constant.OAuthQuotaWindowPrimary, UsedPercent: 42, WindowMinutes: 300}},
		CapturedAt: time.Now(),
	}
	probedAt := time.Now()
	if err := repo.UpdateQuota(ctx, 1, snapshot, probedAt); err != nil {
		t.Fatalf("update quota: %v", err)
	}
	if err := repo.UpdateLastError(ctx, 1, "quota probe failed: boom"); err != nil {
		t.Fatalf("update last error: %v", err)
	}

	got, err := repo.FindByEndpointID(ctx, 1)
	if err != nil || got == nil {
		t.Fatalf("reload credential: %v", err)
	}
	if got.AccessToken() != "at-new" || got.RefreshToken() != "rt-new" {
		t.Fatalf("quota update must not touch tokens, got access=%q refresh=%q", got.AccessToken(), got.RefreshToken())
	}
	if len(got.Quota().Windows) != 1 || got.Quota().Windows[0].UsedPercent != 42 {
		t.Fatalf("quota snapshot not persisted: %+v", got.Quota())
	}
	if got.QuotaUpdatedAt().IsZero() {
		t.Fatal("quota updated time must be persisted")
	}
	if got.LastError() != "quota probe failed: boom" {
		t.Fatalf("last error not persisted: %q", got.LastError())
	}

	// 探测成功必须清掉上一次的错误摘要：否则一次临时失败会一直显示到下次 token 刷新
	if err := repo.UpdateQuota(ctx, 1, snapshot, time.Now()); err != nil {
		t.Fatalf("second update quota: %v", err)
	}
	got, err = repo.FindByEndpointID(ctx, 1)
	if err != nil || got == nil {
		t.Fatalf("reload credential after successful probe: %v", err)
	}
	if got.LastError() != "" {
		t.Fatalf("successful probe must clear last error, got %q", got.LastError())
	}
	if got.AccessToken() != "at-new" || got.RefreshToken() != "rt-new" {
		t.Fatalf("quota update must not touch tokens, got access=%q refresh=%q", got.AccessToken(), got.RefreshToken())
	}
}
