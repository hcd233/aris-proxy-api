package agent_harness

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	sessionquery "github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

func captureSessionListSQL(t *testing.T, db *gorm.DB) *[]string {
	t.Helper()

	sqls := make([]string, 0, 4)
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_demo_filter_sql", func(tx *gorm.DB) {
		sqls = append(sqls, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	return &sqls
}

func sessionListParam() model.CommonParam {
	return model.CommonParam{
		PageParam: model.PageParam{Page: 1, PageSize: 20},
		SortParam: model.SortParam{Sort: enum.SortDesc, SortField: "created_at"},
	}
}

// TestListSessionsByIDsAppliesFilters 钉住 demo 视角的筛选不再被忽略：
// 抽样 ID scope 之上必须叠加时间范围与 harness 条件。
func TestListSessionsByIDsAppliesFilters(t *testing.T) {
	t.Parallel()

	db := newRuleDryRunDB(t)
	sqls := captureSessionListSQL(t, db)
	repo := repository.NewSessionReadRepository(db, nil)

	criteria, err := sessionquery.ParseSessionFilterCriteria("harness:ZCode")
	if err != nil {
		t.Fatalf("ParseSessionFilterCriteria: %v", err)
	}
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)

	if _, _, err := repo.ListSessionsByIDs(context.Background(), []uint{1, 2, 3}, sessionListParam(), start, end, "", criteria); err != nil {
		t.Fatalf("ListSessionsByIDs: %v", err)
	}

	if len(*sqls) == 0 {
		t.Fatal("ListSessionsByIDs produced no SQL")
	}
	pageSQL := (*sqls)[len(*sqls)-1]
	if !strings.Contains(pageSQL, "id IN (") {
		t.Errorf("demo scope must stay restricted to sampled ids: %s", pageSQL)
	}
	if !strings.Contains(pageSQL, "created_at") {
		t.Errorf("demo scope must apply the requested time range: %s", pageSQL)
	}
	if !strings.Contains(pageSQL, "agent_harness_rules") {
		t.Errorf("demo scope must apply the harness filter: %s", pageSQL)
	}
}

// TestListSessionsByOwnerIDsKeepsFilterShape 回归：非 demo 路径的筛选形态不变。
func TestListSessionsByOwnerIDsKeepsFilterShape(t *testing.T) {
	t.Parallel()

	db := newRuleDryRunDB(t)
	sqls := captureSessionListSQL(t, db)
	repo := repository.NewSessionReadRepository(db, nil)

	criteria, err := sessionquery.ParseSessionFilterCriteria("harness:Other")
	if err != nil {
		t.Fatalf("ParseSessionFilterCriteria: %v", err)
	}
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)

	if _, _, err := repo.ListSessionsByOwnerIDs(context.Background(), []uint{7}, sessionListParam(), start, end, "", criteria); err != nil {
		t.Fatalf("ListSessionsByOwnerIDs: %v", err)
	}

	if len(*sqls) == 0 {
		t.Fatal("ListSessionsByOwnerIDs produced no SQL")
	}
	pageSQL := (*sqls)[len(*sqls)-1]
	if !strings.Contains(pageSQL, "api_key_id IN (") {
		t.Errorf("owner scope must stay restricted to the user's api keys: %s", pageSQL)
	}
	if !strings.Contains(pageSQL, "IS NULL") {
		t.Errorf("harness:Other must compile to IS NULL: %s", pageSQL)
	}
}
