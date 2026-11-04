// Package session_tool_search 会话工具按名称搜索：仓储 SQL 形态与查询分流。
//
// 搜索谓词依赖 PostgreSQL 专有的 ::jsonb 强转（tool 为 text 列 + serializer:json），
// sqlite 无法执行，仓储层用 DryRun 钉住 SQL 形态（同 session_list_split 先例）。
package session_tool_search

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/samber/lo"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	sessionport "github.com/hcd233/aris-mass/internal/application/session/port"
	"github.com/hcd233/aris-mass/internal/application/session/query"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/domain/session"
	"github.com/hcd233/aris-mass/internal/infrastructure/repository"
)

type capturedSQL struct {
	sql  string
	vars []any
}

func newDryRunDB(t *testing.T) (*gorm.DB, *[]capturedSQL) {
	t.Helper()
	db, err := gorm.Open(postgres.Open("host=127.0.0.1 user=guard dbname=guard port=5432 sslmode=disable"), &gorm.Config{
		Logger:               gormlogger.Default.LogMode(gormlogger.Silent),
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("failed to open dryrun pg db: %v", err)
	}
	dry := db.Session(&gorm.Session{DryRun: true})
	captured := make([]capturedSQL, 0, 2)
	if err := dry.Callback().Query().After("gorm:query").Register("test:capture_tool_search_sql", func(tx *gorm.DB) {
		captured = append(captured, capturedSQL{sql: tx.Statement.SQL.String(), vars: slices.Clone(tx.Statement.Vars)})
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	return dry, &captured
}

// TestSearchToolsByName_SQLShape COUNT 与分页都必须在数据库按会话范围 + 名称过滤；
// 分页保持会话 tool_ids 原序并带 LIMIT/OFFSET；关键词中的 LIKE 通配符按字面转义。
func TestSearchToolsByName_SQLShape(t *testing.T) {
	t.Parallel()
	dry, captured := newDryRunDB(t)
	repo := repository.NewSessionReadRepository(dry, nil)

	if _, _, err := repo.SearchToolsByName(context.Background(), []uint{3, 1, 2}, "a_b%", 2, 20); err != nil {
		t.Fatalf("SearchToolsByName failed: %v", err)
	}
	if len(*captured) != 2 {
		t.Fatalf("want 2 SQL statements (COUNT + page), got %d: %v", len(*captured), *captured)
	}

	for _, c := range *captured {
		for _, want := range []string{"tools", "tool::jsonb->>'name' ILIKE", "ESCAPE", "id = ANY(", "deleted_at = 0"} {
			if !strings.Contains(c.sql, want) {
				t.Errorf("SQL missing %q, got: %s", want, c.sql)
			}
		}
		if !slices.Contains(c.vars, any(`%a\_b\%%`)) {
			t.Errorf("keyword must be LIKE-escaped, vars: %v", c.vars)
		}
		if !slices.Contains(c.vars, any("{3,1,2}")) {
			t.Errorf("id scope must be passed as bigint[] literal, vars: %v", c.vars)
		}
	}

	countSQL, page := (*captured)[0].sql, (*captured)[1]
	if !strings.Contains(strings.ToLower(countSQL), "count(*)") {
		t.Errorf("first SQL must be COUNT, got: %s", countSQL)
	}
	for _, want := range []string{"ORDER BY array_position(", "LIMIT ", "OFFSET "} {
		if !strings.Contains(page.sql, want) {
			t.Errorf("page SQL missing %q, got: %s", want, page.sql)
		}
	}
	// page=2, pageSize=20 → LIMIT 20 OFFSET 20（占位符参数）
	if n := len(page.vars); n < 2 || page.vars[n-2] != any(20) || page.vars[n-1] != any(20) {
		t.Errorf("want LIMIT 20 OFFSET 20, vars: %v", page.vars)
	}
}

// TestSearchToolsByName_EmptyScopeSkipsDB 会话无工具时不发 SQL
func TestSearchToolsByName_EmptyScopeSkipsDB(t *testing.T) {
	t.Parallel()
	dry, captured := newDryRunDB(t)
	repo := repository.NewSessionReadRepository(dry, nil)

	tools, total, err := repo.SearchToolsByName(context.Background(), nil, "x", 1, 20)
	if err != nil || total != 0 || len(tools) != 0 || len(*captured) != 0 {
		t.Fatalf("want empty result without SQL, got tools=%v total=%d err=%v sql=%v", tools, total, err, *captured)
	}
}

type fakeMetaQuery struct{ toolIDs []uint }

func (f *fakeMetaQuery) Handle(context.Context, sessionport.GetSessionMetaByUserQuery) (*sessionport.SessionMetaView, error) {
	return &sessionport.SessionMetaView{ToolIDs: f.toolIDs}, nil
}

type fakeReadRepo struct {
	session.SessionReadRepository
	searchIDs     []uint
	searchKeyword string
	searchPage    int
	searchSize    int
	searchCalls   int
}

func (r *fakeReadRepo) SearchToolsByName(_ context.Context, ids []uint, keyword string, page, pageSize int) ([]*session.ToolDetailProjection, int64, error) {
	r.searchCalls++
	r.searchIDs, r.searchKeyword, r.searchPage, r.searchSize = ids, keyword, page, pageSize
	return []*session.ToolDetailProjection{{ID: 2, Tool: &vo.UnifiedTool{Name: "Bash"}}}, 7, nil
}

type fakeCache struct {
	sessionport.SessionDetailCache
	getToolsCalls int
}

func (c *fakeCache) GetTools(_ context.Context, ids []uint) (hits map[uint]*sessionport.ToolCacheRecord, missing []uint, err error) {
	c.getToolsCalls++
	return lo.SliceToMap(ids, func(id uint) (uint, *sessionport.ToolCacheRecord) {
		return id, &sessionport.ToolCacheRecord{ID: id, Tool: &vo.UnifiedTool{Name: "t"}}
	}), nil, nil
}

func (c *fakeCache) SetTools(context.Context, []*sessionport.ToolCacheRecord) error { return nil }

// TestListSessionTools_KeywordGoesToRepository 带关键词时整段交给数据库搜索（不读工具缓存），
// 总数取匹配数；关键词首尾空白被去除。
func TestListSessionTools_KeywordGoesToRepository(t *testing.T) {
	t.Parallel()
	repo := &fakeReadRepo{}
	detailCache := &fakeCache{}
	h := query.NewListSessionToolsHandler(repo, &fakeMetaQuery{toolIDs: []uint{5, 2, 9}}, detailCache)

	res, err := h.Handle(context.Background(), sessionport.ListSessionToolsQuery{SessionID: 1, IsAdmin: true, Page: 2, PageSize: 10, Keyword: "  bash "})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if repo.searchCalls != 1 || repo.searchKeyword != "bash" || repo.searchPage != 2 || repo.searchSize != 10 || !slices.Equal(repo.searchIDs, []uint{5, 2, 9}) {
		t.Fatalf("unexpected search call: %+v", repo)
	}
	if detailCache.getToolsCalls != 0 {
		t.Errorf("keyword search must not read tool cache")
	}
	if res.Total != 7 || len(res.Tools) != 1 || res.Tools[0].Tool.Name != "Bash" {
		t.Errorf("unexpected result: total=%d tools=%v", res.Total, res.Tools)
	}
}

// TestListSessionTools_BlankKeywordKeepsCachedPath 空白关键词走原分页 + 缓存路径
func TestListSessionTools_BlankKeywordKeepsCachedPath(t *testing.T) {
	t.Parallel()
	repo := &fakeReadRepo{}
	detailCache := &fakeCache{}
	h := query.NewListSessionToolsHandler(repo, &fakeMetaQuery{toolIDs: []uint{5, 2, 9}}, detailCache)

	res, err := h.Handle(context.Background(), sessionport.ListSessionToolsQuery{SessionID: 1, IsAdmin: true, Page: 1, PageSize: 2, Keyword: "   "})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if repo.searchCalls != 0 || detailCache.getToolsCalls != 1 {
		t.Fatalf("blank keyword must use cached path: search=%d getTools=%d", repo.searchCalls, detailCache.getToolsCalls)
	}
	if res.Total != 3 || len(res.Tools) != 2 || res.Tools[0].ID != 5 || res.Tools[1].ID != 2 {
		t.Errorf("unexpected result: total=%d tools=%v", res.Total, res.Tools)
	}
}
