# 修复分数筛选不生效和审计接口404问题

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复两个生产问题：1) Session列表分数筛选不生效；2) 审计页面调用不存在的接口导致404。

**Architecture:** 
1. 分数筛选：后端`ListSessionsByUserReq`缺少`Filter`字段，需要添加并实现筛选逻辑。
2. 审计接口404：前端调用`/api/v1/audit/option/list`，但后端没有这个路由。需要检查前端代码，要么修复前端调用，要么添加后端路由。

**Tech Stack:** Go, Fiber, Huma, GORM, Next.js, TypeScript

---

## 问题分析

### 问题1：分数筛选不生效
- **TraceId:** `d3573467-1ca4-4c48-97d5-d04e8eb64673`
- **现象:** 前端发送`filter=score%3A5`参数，但后端返回所有记录，未按分数筛选
- **根因:** 后端`ListSessionsByUserReq`（`internal/dto/session.go:79`）没有`Filter`字段，`ListSessionsByUserQuery`（`internal/application/session/port/handler.go:70`）也没有`Filter`字段
- **影响:** 前端分数筛选功能完全无效

### 问题2：审计接口404
- **TraceId:** `eb061cd7-90c6-4c23-b611-49479a8cf124`
- **现象:** 前端请求`/api/v1/audit/option/list`，返回404 Not Found
- **根因:** 后端审计路由（`internal/router/audit.go`）没有`/option/list`路径
- **影响:** 审计页面某个功能无法使用

---

## 文件结构

### 问题1相关文件
- `internal/dto/session.go:79` - 添加`Filter`字段到`ListSessionsByUserReq`
- `internal/application/session/port/handler.go:70` - 添加`Filter`字段到`ListSessionsByUserQuery`
- `internal/handler/session.go:113` - 传递`Filter`参数
- `internal/application/session/query/jwt_session_queries.go:49` - 处理`Filter`参数
- `internal/domain/session/repository.go:111-113` - 修改仓储接口支持筛选
- `internal/infrastructure/repository/session_repository.go` - 实现筛选逻辑

### 问题2相关文件
- `web/src/lib/api-client.ts` - 检查是否有`/audit/option/list`调用
- `web/src/app/(dashboard)/audit/page.tsx` - 检查审计页面逻辑
- `internal/router/audit.go` - 可能需要添加`/option/list`路由（如果前端需要）

---

## 任务分解

### Task 1: 为Session列表添加分数筛选支持

**Files:**
- Modify: `internal/dto/session.go:79`
- Modify: `internal/application/session/port/handler.go:70`
- Modify: `internal/handler/session.go:113`
- Modify: `internal/application/session/query/jwt_session_queries.go:49`
- Modify: `internal/domain/session/repository.go:111-113`
- Modify: `internal/infrastructure/repository/session_repository.go`
- Test: `test/e2e/session_list_filter/session_list_filter_test.go`

- [ ] **Step 1: 在DTO中添加Filter字段**

```go
// internal/dto/session.go:79
type ListSessionsByUserReq struct {
	model.PageParam
	Sort      enum.Sort `query:"sort" enum:"asc,desc"`
	SortField string    `query:"sortField" maxLength:"50"`
	StartTime time.Time `query:"startTime"`
	EndTime   time.Time `query:"endTime"`
	Keyword   string    `query:"keyword" maxLength:"200" doc:"搜索关键词（在消息内容和推理内容中搜索）"`
	Filter    string    `query:"filter" doc:"筛选条件，格式：field:value，如 score:5"`
}
```

- [ ] **Step 2: 在Query结构体中添加Filter字段**

```go
// internal/application/session/port/handler.go:70
type ListSessionsByUserQuery struct {
	UserID    uint
	IsAdmin   bool
	Page      int
	PageSize  int
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
	Keyword   string
	Filter    string
}
```

- [ ] **Step 3: 修改Handler传递Filter参数**

```go
// internal/handler/session.go:113
views, pageInfo, err := h.listByUser.Handle(ctx, port.ListSessionsByUserQuery{
	UserID:    userID,
	IsAdmin:   isAdmin,
	Page:      req.Page,
	PageSize:  req.PageSize,
	Sort:      req.Sort,
	SortField: req.SortField,
	StartTime: req.StartTime,
	EndTime:   req.EndTime,
	Keyword:   req.Keyword,
	Filter:    req.Filter, // 添加这一行
})
```

- [ ] **Step 4: 修改Query处理Filter参数**

```go
// internal/application/session/query/jwt_session_queries.go:49
func (h *listSessionsByUserHandler) Handle(ctx context.Context, q sessionport.ListSessionsByUserQuery) ([]*sessionport.SessionSummaryView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	param, err := sanitizeSessionListParam(ctx, q)
	if err != nil {
		return nil, nil, err
	}

	// 解析Filter参数
	filterField, filterValue := parseFilter(q.Filter)

	var projections []*session.SessionSummaryProjection
	var pageInfo *model.PageInfo

	if q.IsAdmin {
		projections, pageInfo, err = h.readRepo.ListAllSessions(ctx, param, q.StartTime, q.EndTime, q.Keyword, filterField, filterValue)
	} else {
		ownerNames, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, q.UserID)
		if lookupErr != nil {
			log.Error("[SessionQuery] Failed to lookup owner names", zap.Error(lookupErr), zap.Uint("userID", q.UserID))
			return nil, nil, lookupErr
		}
		if len(ownerNames) == 0 {
			return []*sessionport.SessionSummaryView{}, &model.PageInfo{Page: q.Page, PageSize: q.PageSize, Total: 0}, nil
		}
		projections, pageInfo, err = h.readRepo.ListSessionsByOwnerNames(ctx, ownerNames, param, q.StartTime, q.EndTime, q.Keyword, filterField, filterValue)
	}
	// ... 其余代码保持不变
}

// 添加解析Filter的辅助函数
func parseFilter(filter string) (field, value string) {
	if filter == "" {
		return "", ""
	}
	parts := strings.SplitN(filter, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
```

- [ ] **Step 5: 修改仓储接口支持筛选**

```go
// internal/domain/session/repository.go:111-113
type SessionReadRepository interface {
	// ListAllSessions 分页查询所有 Session 列表投影（admin 用）
	ListAllSessions(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, keyword string, filterField, filterValue string) ([]*SessionSummaryProjection, *model.PageInfo, error)
	// ListSessionsByOwnerNames 按多个 API Key name 分页查询 Session 列表投影
	ListSessionsByOwnerNames(ctx context.Context, ownerNames []string, param model.CommonParam, startTime, endTime time.Time, keyword string, filterField, filterValue string) ([]*SessionSummaryProjection, *model.PageInfo, error)
	// ... 其他方法保持不变
}
```

- [ ] **Step 6: 实现仓储的筛选逻辑**

```go
// internal/infrastructure/repository/session_repository.go
func (r *sessionReadRepository) ListAllSessions(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, keyword string, filterField, filterValue string) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	// ... 现有代码
	
	// 添加筛选条件
	if filterField != "" && filterValue != "" {
		switch filterField {
		case "score":
			score, err := strconv.Atoi(filterValue)
			if err == nil {
				query = query.Where("score = ?", score)
			}
		}
	}
	
	// ... 其余代码保持不变
}

func (r *sessionReadRepository) ListSessionsByOwnerNames(ctx context.Context, ownerNames []string, param model.CommonParam, startTime, endTime time.Time, keyword string, filterField, filterValue string) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	// ... 现有代码
	
	// 添加筛选条件
	if filterField != "" && filterValue != "" {
		switch filterField {
		case "score":
			score, err := strconv.Atoi(filterValue)
			if err == nil {
				query = query.Where("score = ?", score)
			}
		}
	}
	
	// ... 其余代码保持不变
}
```

- [ ] **Step 7: 编写E2E测试**

```go
// test/e2e/session_list_filter/session_list_filter_test.go
package session_list_filter

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

type testCase struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Filter      string `json:"filter"`
	ExpectedCount int `json:"expectedCount"`
}

func loadCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	var cases []testCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	return cases
}

func TestSessionListFilter(t *testing.T) {
	// 测试分数筛选功能
	cases := loadCases(t)
	
	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := os.Getenv("API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			// 构建请求URL
			url := baseURL + "/api/v1/session/list?page=1&pageSize=50&sort=desc&sortField=created_at"
			if tc.Filter != "" {
				url += "&filter=" + tc.Filter
			}
			
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			
			// 添加JWT token（需要从环境变量获取）
			token := os.Getenv("JWT_TOKEN")
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("failed to send request: %v", err)
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}
			
			// 解析响应
			var result struct {
				Sessions []struct {
					Score *int `json:"score"`
				} `json:"sessions"`
				PageInfo struct {
					Total int `json:"total"`
				} `json:"pageInfo"`
			}
			
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			
			// 验证筛选结果
			if tc.ExpectedCount > 0 && result.PageInfo.Total != tc.ExpectedCount {
				t.Errorf("expected %d sessions, got %d", tc.ExpectedCount, result.PageInfo.Total)
			}
			
			// 验证所有返回的session都有指定的分数
			if tc.Filter == "score:5" {
				for _, session := range result.Sessions {
					if session.Score == nil || *session.Score != 5 {
						t.Errorf("expected score 5, got %v", session.Score)
					}
				}
			}
		})
	}
}
```

- [ ] **Step 8: 创建测试数据文件**

```json
// test/e2e/session_list_filter/fixtures/cases.json
[
	{
		"name": "filter_by_score_5",
		"description": "筛选分数为5的session",
		"filter": "score:5",
		"expectedCount": -1
	},
	{
		"name": "filter_by_score_1",
		"description": "筛选分数为1的session",
		"filter": "score:1",
		"expectedCount": -1
	},
	{
		"name": "no_filter",
		"description": "不筛选，返回所有session",
		"filter": "",
		"expectedCount": -1
	}
]
```

- [ ] **Step 9: 运行测试验证**

```bash
go test -v -count=1 -run TestSessionListFilter ./test/e2e/session_list_filter/
```

- [ ] **Step 10: 运行规范扫描**

```bash
make lint
```

- [ ] **Step 11: 运行全量测试**

```bash
make test
```

- [ ] **Step 12: 提交代码**

```bash
git add internal/dto/session.go internal/application/session/port/handler.go internal/handler/session.go internal/application/session/query/jwt_session_queries.go internal/domain/session/repository.go internal/infrastructure/repository/session_repository.go test/e2e/session_list_filter/
git commit -m "feat: add score filter support for session list"
```

---

### Task 2: 修复审计接口404问题

**Files:**
- Investigate: `web/src/app/(dashboard)/audit/page.tsx`
- Investigate: `web/src/lib/api-client.ts`
- Modify: `internal/router/audit.go` (如果需要)
- Test: `test/e2e/audit_option_list/audit_option_list_test.go` (如果需要添加路由)

- [ ] **Step 1: 检查前端审计页面代码**

```bash
grep -r "option/list" web/src/
```

- [ ] **Step 2: 检查前端api-client.ts**

```bash
grep -r "audit" web/src/lib/api-client.ts
```

- [ ] **Step 3: 分析前端调用逻辑**

如果前端确实需要`/api/v1/audit/option/list`接口，则需要添加后端路由。如果前端调用错误，则需要修复前端代码。

- [ ] **Step 4A: 如果需要添加后端路由**

```go
// internal/router/audit.go
huma.Register(auditGroup, huma.Operation{
	OperationID: "listAuditOptions",
	Method:      http.MethodGet,
	Path:        "/option/list",
	Summary:     "ListAuditOptions",
	Description: "List audit options for filtering",
	Tags:        []string{constant.TagAudit},
	Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
	Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listAuditOptions", enum.PermissionUser)},
}, auditHandler.HandleListAuditOptions)
```

- [ ] **Step 4B: 如果需要修复前端调用**

修改前端代码，将`/api/v1/audit/option/list`改为正确的接口路径。

- [ ] **Step 5: 编写E2E测试（如果添加了路由）**

```go
// test/e2e/audit_option_list/audit_option_list_test.go
package audit_option_list

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func TestAuditOptionList(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := os.Getenv("API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	
	url := baseURL + "/api/v1/audit/option/list?field=user"
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	
	// 添加JWT token
	token := os.Getenv("JWT_TOKEN")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 6: 运行测试验证**

```bash
go test -v -count=1 -run TestAuditOptionList ./test/e2e/audit_option_list/
```

- [ ] **Step 7: 运行规范扫描**

```bash
make lint
```

- [ ] **Step 8: 运行全量测试**

```bash
make test
```

- [ ] **Step 9: 提交代码**

```bash
git add internal/router/audit.go test/e2e/audit_option_list/  # 或者 git add web/src/
git commit -m "fix: resolve audit option list 404 issue"
```

---

## 验证清单

### 问题1验证
- [ ] 前端发送`filter=score:5`参数时，后端只返回分数为5的session
- [ ] 前端发送`filter=score:1`参数时，后端只返回分数为1的session
- [ ] 前端不发送`filter`参数时，后端返回所有session
- [ ] 分数筛选与分页、排序、时间范围筛选正常配合工作

### 问题2验证
- [ ] 前端请求`/api/v1/audit/option/list`不再返回404
- [ ] 审计页面相关功能正常工作
- [ ] 没有其他接口受到404影响

---

## 回滚计划

如果修复导致问题：
1. 分数筛选问题：回滚`internal/dto/session.go`、`internal/application/session/port/handler.go`、`internal/handler/session.go`、`internal/application/session/query/jwt_session_queries.go`、`internal/domain/session/repository.go`、`internal/infrastructure/repository/session_repository.go`的修改
2. 审计接口问题：回滚`internal/router/audit.go`的修改（如果添加了路由）或回滚前端修改

---

## 相关文档
- AGENTS.md - 项目开发规范
- CLAUDE.md - 代码契约
- huma-dto-conventions skill - DTO设计规范
