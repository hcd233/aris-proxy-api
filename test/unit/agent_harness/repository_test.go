package agent_harness

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/agentharness"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

// newRuleDryRunDB 打开 postgres DryRun 会话：只构建 SQL 不执行（不建连接）。
func newRuleDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.Open("host=127.0.0.1 user=guard dbname=guard port=5432 sslmode=disable"), &gorm.Config{
		Logger:               gormlogger.Default.LogMode(gormlogger.Silent),
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("failed to open dryrun pg db: %v", err)
	}
	return db.Session(&gorm.Session{DryRun: true})
}

// captureCreateDest 收集 Create 回调拿到的目标行，用于断言规范化后的落库值。
func captureCreateDest(t *testing.T, db *gorm.DB) *[]*dbmodel.AgentHarnessRule {
	t.Helper()

	dests := make([]*dbmodel.AgentHarnessRule, 0, 1)
	if err := db.Callback().Create().After("gorm:create").Register("test:capture_rule_create", func(tx *gorm.DB) {
		if row, ok := tx.Statement.Dest.(*dbmodel.AgentHarnessRule); ok {
			dests = append(dests, row)
		}
	}); err != nil {
		t.Fatalf("register create callback: %v", err)
	}
	return &dests
}

func TestRuleRepositoryListOrdersByLongestPattern(t *testing.T) {
	t.Parallel()

	db := newRuleDryRunDB(t)
	sqls := make([]string, 0, 1)
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_rule_list", func(tx *gorm.DB) {
		sqls = append(sqls, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	repo := repository.NewAgentHarnessRuleRepository(db)
	rules, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	_ = rules

	if len(sqls) != 1 {
		t.Fatalf("List produced %d queries, want 1: %v", len(sqls), sqls)
	}
	if !strings.Contains(sqls[0], `FROM "agent_harness_rules"`) {
		t.Errorf("List SQL does not scan agent_harness_rules: %s", sqls[0])
	}
	if !strings.Contains(strings.ToLower(sqls[0]), "order by length(pattern) desc, pattern") {
		t.Errorf("List must order by longest pattern first: %s", sqls[0])
	}
}

func TestBuildAgentHarnessRuleRowNormalizesPattern(t *testing.T) {
	t.Parallel()

	row, err := repository.BuildAgentHarnessRuleRow(agentharness.Rule{Pattern: "  ZCode/ ", Name: "ZCode"})
	if err != nil {
		t.Fatalf("BuildAgentHarnessRuleRow returned error: %v", err)
	}
	if got := row.Pattern; got != "zcode/" {
		t.Errorf("persisted pattern = %q, want %q", got, "zcode/")
	}
}

func TestRuleRepositoryCreateRejectsWildcardPattern(t *testing.T) {
	t.Parallel()

	db := newRuleDryRunDB(t)
	dests := captureCreateDest(t, db)
	repo := repository.NewAgentHarnessRuleRepository(db)

	_, err := repo.Create(context.Background(), agentharness.Rule{Pattern: "a%b", Name: "Broken"})
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("Create err = %v, want ErrValidation", err)
	}
	if len(*dests) != 0 {
		t.Errorf("Create must not reach the DB for an invalid pattern, got %d inserts", len(*dests))
	}
}

func TestRuleRepositoryFingerprintSQL(t *testing.T) {
	t.Parallel()

	db := newRuleDryRunDB(t)
	sqls := make([]string, 0, 1)
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_rule_fingerprint", func(tx *gorm.DB) {
		sqls = append(sqls, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	repo := repository.NewAgentHarnessRuleRepository(db)
	if _, err := repo.Fingerprint(context.Background()); err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}

	if len(sqls) != 1 {
		t.Fatalf("Fingerprint produced %d queries, want 1: %v", len(sqls), sqls)
	}
	lowered := strings.ToLower(sqls[0])
	if !strings.Contains(lowered, "count(*)") || !strings.Contains(lowered, "max(updated_at)") {
		t.Errorf("Fingerprint must aggregate count(*) and max(updated_at): %s", sqls[0])
	}
}
