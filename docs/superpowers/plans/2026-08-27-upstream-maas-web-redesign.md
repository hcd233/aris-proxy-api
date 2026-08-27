# Upstream MaaS 单页重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 endpoints/models 合并为单一 `/upstream` 页面（表格内分组行），后端收敛为单一读接口 `GET /api/v1/upstream/list`（嵌套 user 对象、按 endpoint 组分页），并删除两条旧 list 接口、更名客户端分发路由。

**Architecture:** 后端新增 application 层 upstream query handler（复用既有 endpoint/model/user 仓储，新增两个仓储读方法），HTTP 契约只有新分组列表；CRUD 路由全部不动。前端删除两页合并为 `/upstream` 单页，dashboard 统计一次调用取双数。

**Tech Stack:** Go + huma + gorm + uber-fx（后端）；Next.js 16 App Router + Tailwind v4 + shadcn/ui + base-ui（前端）。

**Spec:** [2026-08-27-upstream-maas-web-redesign-design.md](../specs/2026-08-27-upstream-maas-web-redesign-design.md)

## Global Constraints

- 测试只放 `test/unit/<topic>/` 与 `test/e2e/<topic>/`；只用标准库 `testing`；禁止 testify/gomck/time.Sleep 同步。
- JSON 序列化统一 `github.com/bytedance/sonic`；禁止 `encoding/json`/`json.RawMessage`/`any`/`interface{}`。
- 业务错误走 `internal/common/ierr`；禁止 `fmt.Errorf`/`errors.New`。业务包禁建本地 const 块与 `common.go`。
- HTTP 状态码用 `fiber.StatusXxx`；DTO 时间字段 `time.Time`；Context 契约见 docs/agents/go-backend.md。
- 命令一律加 `rtk` 前缀（如 `rtk go test`、`rtk git commit`）。
- 开发在 `.worktrees/` 下分支 `feature/upstream-maas-web-redesign-2026-08-27` 进行（serena 不认 worktree 路径：worktree 内批量编辑用 perl/Sed 替代 serena 工具）。
- 前端：Tailwind v4 + `cn()`；图标仅 lucide-react；Toast 仅 sonner；调用后端只走 `src/lib/api-client.ts`；`truncate` 必须包 TooltipTrigger（lint 强制）；不改 basePath。
- 分支/提交信息使用规范前缀（feat/refactor/test/docs/chore）。每个任务独立提交。

## 已确认风险（执行者须知）

- 客户端路由改名 `/api/v1/client/list` → `/api/v1/model/list` 会使**已分发的旧版 aris CLI** 失效。用户已明确决策接受。
- 生产 `demo_configs.modules` 若存量含 `"endpoints"`/`"models"` 字符串，枚举更名后需一次性 SQL 归并（Task 2 末尾给出语句）。

---

### Task 1: upstream application 层（port + query handler + 单测）

**Files:**
- Create: `internal/application/upstream/port/handler.go`
- Create: `internal/application/upstream/query/list_upstream.go`
- Modify: `internal/domain/llmproxy/repository.go`（两个接口各加一个方法）
- Modify: `internal/infrastructure/repository/endpoint_repository.go`、`internal/infrastructure/repository/model_repository.go`（实现）
- Test: `test/unit/upstream_query/list_upstream_test.go`

**Interfaces:**
- Consumes: `llmproxy.EndpointRepository.BatchFindByIDs(ctx, ids) (map[uint]*aggregate.Endpoint, error)`、`identity.UserRepository.BatchFindByIDs(ctx, ids) (map[uint]*identityaggregate.User, error)`（均已存在）
- Produces:
  ```go
  // port 包
  type UpstreamUserView struct{ ID uint; Name string; Avatar string }
  type UpstreamEndpointView struct {
      ID uint; User *UpstreamUserView; Name, OpenaiBaseURL, AnthropicBaseURL, MaskedAPIKey string
      SupportOpenAIChatCompletion, SupportOpenAIResponse, SupportAnthropicMessage bool
      CreatedAt, UpdatedAt time.Time
  }
  type UpstreamModelView struct {
      ID uint; User *UpstreamUserView; Alias, ModelID, UpstreamModel string
      Enabled bool; ContextLength, MaxOutputTokens int; Capabilities []enum.InputModality
      CreatedAt, UpdatedAt time.Time
  }
  type UpstreamGroupView struct {
      Endpoint *UpstreamEndpointView; Models []*UpstreamModelView
      ModelCount int; Truncated bool
  }
  type ListUpstreamQuery struct {
      model.CommonParam; IsDemo bool; ScopeUserID uint; Username string
  }
  type ListUpstreamHandler interface {
      Handle(ctx context.Context, q ListUpstreamQuery) ([]*UpstreamGroupView, int64 /*modelTotal*/, *model.PageInfo, error)
  }
  func NewListUpstreamHandler(endpointRepo llmproxy.EndpointRepository, modelRepo llmproxy.ModelRepository, userRepo identity.UserRepository) upstreamport.ListUpstreamHandler
  ```

- [ ] **Step 1: 新增仓储读方法（先接口后实现）**

`internal/domain/llmproxy/repository.go` 在 `EndpointRepository` 接口追加：

```go
// FindIDsByScope 按租户范围返回全部可见 endpoint ID 列表（id 升序）；
// scopeUserID==0（admin 视角）不过滤。用于 upstream 分组视图的分页基数。
FindIDsByScope(ctx context.Context, scopeUserID uint) ([]uint, error)
```

`ModelRepository` 接口追加：

```go
// ListByEndpointIDs 批量拉取一组 endpoint 名下的 model 聚合（不做二次 scope 过滤，
// 调用方传入的 endpointIDs 必须已经过 scope 解析）。未命中的 endpointID 自然返回空集合。
ListByEndpointIDs(ctx context.Context, endpointIDs []uint) ([]*aggregate.Model, error)
```

实现放各自 repository 文件。表名参考同文件既有查询；`users` 表投影字段写法照抄 `user_repository.go` 的 `BatchFindByIDs`：

```go
// endpoint_repository.go 内
func (r *endpointRepository) FindIDsByScope(ctx context.Context, scopeUserID uint) ([]uint, error) {
    db := r.db.WithContext(ctx)
    q := db.Model(&database.Endpoint{})
    if scopeUserID > 0 {
        q = q.Where(constant.FieldUserID, scopeUserID) // 字段常量若不存在则到 internal/common/constant/database.go 补 FieldUserID = "user_id"
    }
    var ids []uint
    if err := q.Order(constant.FieldID).Pluck(constant.FieldID, &ids).Error; err != nil {
        return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find endpoint ids by scope")
    }
    return ids, nil
}

// model_repository.go 内
func (r *modelRepository) ListByEndpointIDs(ctx context.Context, endpointIDs []uint) ([]*aggregate.Model, error) {
    ids := lo.Uniq(lo.Filter(endpointIDs, func(id uint, _ int) bool { return id != 0 }))
    out := make([]*aggregate.Model, 0, len(ids))
    if len(ids) == 0 {
        return out, nil
    }
    db := r.db.WithContext(ctx)
    records, err := r.dao.GetByFields(db, constant.FieldEndpointID, ids) // 用该文件内既有 dao 批查能力；若无等价方法，参照 BatchGetByField 用法补一条 IN 查询
    ...
    for _, rec := range records { out = append(out, toModelAggregate(rec)) }
    return out, nil
}
```

> 注：`dao` 具体批查函数名以 `internal/infrastructure/repository/` 中现有用法为准（先用 Serena/Grep 看同文件内 `BatchGetByField` 的调用方式再落笔；聚合转换器 `toModelAggregate`/`toEndpointAggregate` 已存在直接复用）。

- [ ] **Step 2: 写失败的单元测试**

新建 `test/unit/upstream_query/fake_repo.go` + `list_upstream_test.go`。fake 参照 `test/unit/endpoint_query/list_endpoints_demo_test.go` 的手写 stub 风格（实现完整接口，多出的两个新方法一并实现为内存 map/slice 操作）。核心用例：

```go
func TestListUpstream_GroupPaginationAndBucketing(t *testing.T) {
    // 3 个 endpoint(ep1~ep3)、4 个 model(ep1x2, ep2x1, ep3x1)，pageSize=2
    // 断言：第 1 页返回 ep1,ep2 两组；groups[0].ModelCount==2；
    //       pageInfo.Total==3；modelTotal==4；Truncated==false
}
func TestListUpstream_KeywordAggregatesWholeGroup(t *testing.T) {
    // keyword="sonnet" 只命中 ep1 下的模型 → 结果只含 ep1 一组且整组模型返回，pageInfo.Total==1
}
func TestListUpstream_TruncationAt200(t *testing.T) {
    // ep1 下放 205 个模型 → Models 截断 200 条，Truncated==true，ModelCount==200（展示计数），外部 modelTotal 不受截断影响
}
func TestListUpstream_NestedUserFilledAndMissing(t *testing.T) {
    // ep 归属 user 存在 → User.Name/Avatar 正确回填；归属 user 缺失 → 整个 User 为 nil
}
func TestListUpstream_ScopeIsolation(t *testing.T) {
    // ScopeUserID=u2 时结果不含 u1 的 endpoint
}
func TestListUpstream_UsernameResolveToEmpty(t *testing.T) {
    // admin 视角传不存在的 username → 返回空组与非 nil PageInfo，不报错
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `rtk go build ./... && rtk go test -count=1 ./test/unit/upstream_query/`
Expected: 编译失败（`NewListUpstreamHandler` 未定义）——这是 TDD 预期红灯。

- [ ] **Step 4: 实现 port 与 query handler**

`internal/application/upstream/port/handler.go` 按 Interfaces 声明原样落地。

`internal/application/upstream/query/list_upstream.go` 核心实现（组装顺序即 spec 数据流）：

```go
const groupModelLimit = 200 // ponytail: 单组上限写死 200；需要真分页时引入组内游标

func (h *listUpstreamHandler) Handle(ctx context.Context, q upstreamport.ListUpstreamQuery) ([]*upstreamport.UpstreamGroupView, int64, *model.PageInfo, error) {
    log := logger.WithCtx(ctx)

    scope := q.ScopeUserID
    if scope == 0 && q.Username != "" {
        u, err := h.userRepo.FindByName(ctx, q.Username)
        if err != nil {
            log.Error("[UpstreamQuery] Find user by name failed", zap.Error(err))
            return nil, 0, nil, err
        }
        if u == nil {
            return []*upstreamport.UpstreamGroupView{}, 0,
                &model.PageInfo{Page: q.Page, PageSize: q.PageSize}, nil
        }
        scope = u.AggregateID()
    }

    allIDs, err := h.endpointRepo.FindIDsByScope(ctx, scope)
    if err != nil { log.Error(...); return nil, 0, nil, err }

    epsByID, err := h.endpointRepo.BatchFindByIDs(ctx, allIDs)
    if err != nil { ... fail-fast 同风格 ... }

    models, err := h.modelRepo.ListByEndpointIDs(ctx, allIDs)
    if err != nil { ... }

    // 按 endpoint 分桶
    modelsByEp := make(map[uint][]*aggregate.Model, len(allIDs))
    for _, m := range models { modelsByEp[m.EndpointID()] = append(modelsByEp[m.EndpointID()], m) }

    // keyword 过滤：命中 endpoint 名称或其下任一模型字段 → 保留整组
    matchedIDs := allIDs
    if q.Query != "" {
        kw := strings.ToLower(q.Query)
        matchedIDs = lo.Filter(allIDs, func(id uint, _ int) bool {
            if ep := epsByID[id]; ep != nil && strings.Contains(strings.ToLower(ep.Name()), kw) { return true }
            return lo.SomeBy(modelsByEp[id], func(m *aggregate.Model) bool {
                return strings.Contains(strings.ToLower(m.Alias().String()), kw) ||
                    strings.Contains(strings.ToLower(m.ModelID()), kw) ||
                    strings.Contains(strings.ToLower(m.UpstreamModel()), kw)
            })
        })
    }

    // 内存分页：total = 过滤后的组数
    start := (q.Page - 1) * q.PageSize
    end := min(start+q.PageSize, len(matchedIDs))
    pageIDs := matchedIDs[min(start, len(matchedIDs)):end]

    usersByID := h.loadUsers(ctx, epsByID, modelsByEp)

    groups := make([]*upstreamport.UpstreamGroupView, 0, len(pageIDs))
    var modelTotal int64
    for _, id := range pageIDs { modelTotal += int64(len(modelsByEp[id])) } // modelTotal 口径跟随当前筛选（含非当前页组）

    for _, id := range pageIDs {
        ep := epsByID[id]
        mvs := lo.Map(modelsByEp[id], func(m *aggregate.Model, _ int) *upstreamport.UpstreamModelView {
            return toModelView(m, usersByID, q.IsDemo)
        })
        truncated := false
        if len(mvs) > groupModelLimit { mvs = mvs[:groupModelLimit]; truncated = true }
        groups = append(groups, &upstreamport.UpstreamGroupView{
            Endpoint:   toEndpointView(ep, usersByID, q.IsDemo),
            Models:     mvs,
            ModelCount: len(mvs),
            Truncated:  truncated,
        })
    }

    pageInfo := &model.PageInfo{Page: q.Page, PageSize: q.PageSize, Total: int64(len(matchedIDs))}
    log.Info("[UpstreamQuery] List upstream", zap.Int("groups", len(groups)), zap.Int64("models", modelTotal))
    return groups, modelTotal, pageInfo, nil
}
```

`toEndpointView`/`toModelView`/`toUserView`/`loadUsers` 写法对照删除前的 `list_endpoints.go`/`list_models.go`（demo 脱敏规则一致：demo 时 baseURL/upstreamModel 掩码；MaskedAPIKey 恒掩码）。`min` 用 Go 1.21+ 内建。

- [ ] **Step 5: 全量测试 + lint 通过**

Run: `rtk go test -count=1 ./test/unit/upstream_query/ ./test/unit/endpoint_resolver/ ./test/unit/llmproxy_usecase/`
Expected: PASS（新增两个接口方法导致其它 fake 编译失败时，在本步同步给 `test/unit/**` 所有实现这两个接口的 fake 补上空实现——用 `rtk grep -rn "llmproxy.EndpointRepository\|llmproxy.ModelRepository" test/unit` 找全）。

Run: `make lint` — Expected: 无新增告警。

- [ ] **Step 6: Commit**

```bash
rtk git add -A && rtk git commit -m "feat(upstream): application 层分组列表查询与仓储读方法"
```

---

### Task 2: upstream HTTP 契约 + 路由挂接 + 删除旧 list

**Files:**
- Create: `internal/dto/upstream.go`、`internal/handler/upstream.go`、`internal/router/upstream.go`
- Modify: `internal/bootstrap/modules/application.go`、`internal/router/router.go`、`internal/common/constant/string.go`（Tag）、`internal/common/enum/demo_module.go`
- Delete: `internal/application/endpoint/query/list_endpoints.go`、`internal/application/model/query/list_models.go`、旧 list 相关 port/dto/handler 代码
- Modify: `internal/router/endpoint.go`、`internal/router/model.go`（移除 list 注册）、`internal/handler/endpoint.go`、`internal/handler/model.go`（移除 Handle 方法）、`internal/application/endpoint/port/handler.go`、`internal/application/model/port/handler.go`（移除 View/Query/Handler 定义中 list 部分）
- Test: 更新受影响单测的编译

**Interfaces:**
- Consumes: Task 1 的 `upstreamport.ListUpstreamHandler` 及各 View
- Produces: `GET /api/v1/upstream/list`（JWT）；`handler.NewUpstreamHandler(deps UpstreamDependencies)`；fx 提供者 `NewListUpstreamHandler`

- [ ] **Step 1: 新增 DTO（`internal/dto/upstream.go`）**

```go
// Package dto Upstream DTO
type ListUpstreamReq struct {
    model.CommonParam
    Username string `query:"username,omitempty" doc:"按归属用户名过滤(仅管理员生效)"`
}

type ListUpstreamRsp struct {
    CommonRsp
    Groups    []*UpstreamGroupItem `json:"groups,omitempty" doc:"Endpoint 分组列表"`
    PageInfo  *model.PageInfo      `json:"pageInfo,omitempty" doc:"端点组分页信息(total=端点数)"`
    ModelTotal int64               `json:"modelTotal" doc:"当前筛选范围内模型总数"`
}

type UpstreamGroupItem struct {
    Endpoint   *UpstreamEndpointItem `json:"endpoint" required:"true" doc:"端点详情"`
    Models     []*UpstreamModelItem  `json:"models" doc:"端点下模型（组内不分页，上限 200）"`
    ModelCount int                   `json:"modelCount" doc:"模型数量(截断后口径)"`
    Truncated  bool                  `json:"truncated,omitempty" doc:"组内模型是否被截断"`
}

type UpstreamUserItem struct {
    ID     uint   `json:"id" doc:"用户 ID"`
    Name   string `json:"name" doc:"用户名"`
    Avatar string `json:"avatar" doc:"头像 URL"`
}

type UpstreamEndpointItem struct { /* 字段与原 EndpointItem 相同，但 Username string 替换为 */ }
//  User *UpstreamUserItem `json:"user,omitempty" doc:"归属用户"`

type UpstreamModelItem struct { /* 字段与原 ModelItem 相同但去掉 Endpoint 字段，Username 替换为 */ }
//  User *UpstreamUserItem `json:"user,omitempty" doc:"归属用户"`
```

> 按 huma-dto-conventions 补全每字段的 `doc` 注释与 author/update 头；老 `dto.EndpointItem`/`dto.ModelItem`/`ListEndpointsRsp`/`ListModelsRsp` 删除前先用 `serena_find_referencing_symbols` 确认仅剩 handler 映射引用。

- [ ] **Step 2: 新增 HTTP handler（`internal/handler/upstream.go`）**

```go
type UpstreamHandler interface {
    HandleListUpstream(ctx context.Context, req *dto.ListUpstreamReq) (*dto.HTTPResponse[*dto.ListUpstreamRsp], error)
}
type UpstreamDependencies struct{ List upstreamport.ListUpstreamHandler }
func NewUpstreamHandler(deps UpstreamDependencies) UpstreamHandler { ... }

func (h *upstreamHandler) HandleListUpstream(...) {
    // scope/isDemo/scopeFor 逻辑与 endpoint.go 现实现完全一致：
    // isGlobalScope := perm == enum.PermissionAdmin;
    // ScopeUserID: lo.Ternary(isGlobalScope, 0, util.CtxValueUint(ctx, constant.CtxKeyUserID))
    // 映射 view→item（User 非 nil 才填充，对齐 apikeys handler 的嵌套映射写法）
}
```

（`scopeFor` 是 `package handler` 内已有未导出函数，直接复用。）

- [ ] **Step 3: 新增路由（`internal/router/upstream.go` + 注册）**

```go
func initUpstreamRouter(upstreamGroup huma.API, upstreamHandler handler.UpstreamHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner, demoAccessor demoport.DemoModuleAccessor, auditSubmitter demoport.DemoSubmitter) {
    upstreamGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))
    upstreamGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(cache, "demoAccess", "", constant.PeriodDemoAccess, constant.LimitDemoAccess, middleware.WithPermissionFilter(enum.PermissionDemo)))
    upstreamGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(cache, "upstreamManage", constant.CtxKeyUserID, constant.PeriodManageAPIKey, constant.LimitManageAPIKey))

    huma.Register(upstreamGroup, huma.Operation{
        OperationID: "listUpstream", Method: http.MethodGet, Path: constant.RoutePathList,
        Summary: "ListUpstream", Description: "List endpoint-grouped upstream configurations",
        Tags: []string{constant.TagUpstream},
        Security: []map[string][]string{{constant.SecuritySchemeJWT: {}}},
        Middlewares: huma.Middlewares{
            middleware.LimitUserPermissionWithDemoMiddleware("listUpstream", enum.PermissionUser, enum.DemoModuleUpstream, demoAccessor, auditSubmitter),
        },
    }, upstreamHandler.HandleListUpstream)
}
```

配套改动：
- `internal/common/constant/string.go` 增加 `TagUpstream = "Upstream"`（放在 TagEndpoint 旁，命名随既有 Tag 常量风格）。
- `internal/router/router.go`：deps 结构加 `UpstreamHandler handler.UpstreamHandler`；`v1` 组挂载处仿照 `initClientRouter` 增加 `initUpstreamRouter(huma.NewGroup(v1Group, "/upstream"), ...)`（v1Group 前缀以文件内 endpoint/model 组的同款写法为准）。
- `internal/bootstrap/modules/application.go`：增加

```go
func NewListUpstreamHandler(endpointRepo llmproxy.EndpointRepository, modelRepo llmproxy.ModelRepository, userRepo identity.UserRepository) upstreamport.ListUpstreamHandler {
    return upstreamquery.NewListUpstreamHandler(endpointRepo, modelRepo, userRepo)
}
```

并把原来喂给 NewListEndpointsHandler/NewListModelsHandler 的 fx 提供项替换为它（handler 组装处 `handler.UpstreamHandler` 由 fx Invoke/Provide 结构体接线，位置对齐现有 EndpointHandler/ModelHandler 注册行）。

- [ ] **Step 4: 删除旧 list 契约**

顺序执行：
1. `internal/router/endpoint.go`：删掉 `OperationID: "listEndpoints"` 的整段 `huma.Register`（Create/Update/Delete 保留）。
2. `internal/router/model.go`：同上删 `listModels` 注册。
3. `internal/handler/endpoint.go`：删 `HandleListEndpoints` 方法及接口行、deps/struct 的 `List` 字段与赋值；映射代码随之消失。
4. `internal/handler/model.go`：删 `HandleListModels` 同款。
5. `internal/application/endpoint/port/handler.go`：删 `EndpointView`、`ListEndpointsQuery`、`ListEndpointsHandler`。
6. `internal/application/model/port/handler.go`：删 `EndpointView`、`ModelView`、`ListModelsQuery`、`ListModelsHandler`。
7. 删文件 `internal/application/endpoint/query/list_endpoints.go`、`internal/application/model/query/list_models.go`（其中 demo 掩码逻辑已在 Task 1 迁移进 upstream handler）。
8. `internal/dto/endpoint.go`、`internal/dto/model.go`：删 `EndpointItem`、`ListEndpointsReq/Rsp`、`ModelItem`、`ListModelsReq/Rsp`（CRUD req/rsp 保留）。删之前逐个 `serena_find_referencing_symbols` 处理残余引用（web hooks 测试/apikey 设计文档提及不阻断编译的不算）。
9. `internal/bootstrap/modules/application.go`：删 `NewListEndpointsHandler`、`NewListModelsHandler`。

- [ ] **Step 5: demo 模块枚举归并（`internal/common/enum/demo_module.go`）**

```go
DemoModuleUpstream DemoModule = "upstream" // 上游端点与模型配置（原 endpoints+models 合并）
```

删除 `DemoModuleEndpoints`/`DemoModuleModels` 两个常量，`DemoModules` 切片用 `DemoModuleUpstream` 占据原 endpoints 的位置。全局 `rtk grep -rn "DemoModuleEndpoints\|DemoModuleModels" --include="*.go"` 把残余引用（路由中间件参数、demo 页面枚举转换、e2e）改指 `DemoModuleUpstream`。

- [ ] **Step 6: 全量构建 + 受影响测试**

Run: `rtk go build ./... && make test && make lint`
Expected: PASS。失败点集中在：实现了被删 port 接口的旧 fake、引用旧 DTO 的旧单测——一并修正为走新 upstream 契约或直接删除对应用例（断言迁移在 Task 4 系统性重写）。

- [ ] **Step 7: Commit**

```bash
rtk git add -A && rtk git commit -m "feat(upstream): GET /api/v1/upstream/list 契约上线，删除 endpoint/model list 旧接口"
```

---

### Task 3: 客户端分发路由改名 `/client/list` → `/model/list`

**Files:**
- Modify: `internal/router/client.go:26`、`internal/common/constant/traceclient.go:36`、`test/e2e/client_models/client_models_test.go`

**Interfaces:**
- Consumes: 无变化（鉴权 API Key、响应结构不变、OperationID 仍为 `listClientModels`）
- Produces: 对外路径 `GET /api/v1/model/list`

- [ ] **Step 1: 改注册路径**

`internal/router/client.go` 的 `huma.Register` 里 `Path: constant.RoutePathList` 改为 `Path: "/model/list"`（资源式显式路径，不再借用通用 /list 常量）。

- [ ] **Step 2: 改 SDK 常量**

`internal/common/constant/traceclient.go:36`：

```go
ClientModelsListPath = "/api/v1/model/list"
```

唯一消费方 `internal/client/api/client.go:70` 走常量自动生效。

- [ ] **Step 3: 更新 e2e 三处硬编码**

`test/e2e/client_models/client_models_test.go:51,55,103,106,130` 的 `/api/v1/client/list` 全部替换为 `/api/v1/model/list`。

- [ ] **Step 4: 构建与聚焦验证**

Run: `rtk go build ./... && rtk go test -count=1 ./test/e2e/client_models/`
Expected: PASS（确认新路径下 API Key 鉴权链路完好）。

- [ ] **Step 5: Commit**

```bash
rtk git add -A && rtk git commit -m "refactor(client): 模型分发路由更名为 /api/v1/model/list"
```

---

### Task 4: E2E——新增 upstream_list 用例 + 迁移旧断言

**Files:**
- Create: `test/e2e/upstream_list/upstream_list_test.go`
- Modify: `test/e2e/user_scope_config/*`、`test/e2e/model_capabilities/*`、`test/e2e/model_id/*`、`test/e2e/client_models/*`（如 Task 3 未覆盖完）

**Interfaces:**
- Consumes: `GET /api/v1/upstream/list`、CRUD `POST/PATCH/DELETE /api/v1/{endpoint,model}`
- Produces: 可回归的上游配置隔离与分组分页用例集

- [ ] **Step 1: 读样板**

先读 `test/e2e/user_scope_config/` 的整套 harness（路由自注册方式、JWT 铸造、fixtures 组织），本任务的骨架完全复制它的做法；请求体 fixtures 按 `test/e2e/<topic>/fixtures/requests/*.json` 规范放置。

- [ ] **Step 2: 写主用例（每个函数一个断言主题）**

```go
func TestUpstreamList_PaginationCountsByGroups(t *testing.T)          // 3 端点 4 模型 pageSize=2：groups 数==2；pageInfo.total==3；rsp.ModelTotal==4
func TestUpstreamList_NoCrossPageGroupSplit(t *testing.T)             // 第 1/2 页取并集无重复 endpoint.id，每组 Models 非空
func TestUpstreamList_KeywordReturnsWholeGroup(t *testing.T)          // query="alias关键字"：命中组的 Models 长度==该组全部模型数
func TestUpstreamList_NestedUserObject(t *testing.T)                  // user{name,avatar} 与创建者一致；软删创建者后列表该项 user 缺省(json 无 user 键)
func TestUpstreamList_UserScopeIsolation(t *testing.T)                // 用户 B 的 token 看不到 A 的端点组；username 参数被忽略
func TestUpstreamList_AdminFilterByUsername(t *testing.T)             // admin token username=A → 只有 A 的组
func TestUpstreamGroup_CRUDFromContext(t *testing.T)                  // POST /model 带 endpointID=A 创建成功后 GET 列表中组 modelCount+1；PATCH /model 变更 endpointID 仍被允许（属性行为保持，UI 不暴露）
func TestEndpointDelete_CascadesModels(t *testing.T)                  // DELETE /endpoint/{id} 后 upstream/list 该组消失且 modelTotal 相应减少（固化级联行为）
```

（fixture 用户名/头像 URL 等值放 `fixtures/*.json`，不内联大段 JSON。）

- [ ] **Step 3: 先跑红灯再转绿**

Run: `rtk go test -count=1 ./test/e2e/upstream_list/` — 若 Step 2 有笔误先修复；随后跑受影响旧套件：

Run: `rtk go test -count=1 ./test/e2e/user_scope_config/ ./test/e2e/model_capabilities/ ./test/e2e/model_id/`
Expected: PASS。旧套件里所有通过已删除的 `GET /endpoint/list`、`GET /model/list` 取数的步骤改为调 `GET /api/v1/upstream/list` 并从 `groups[].models` 取数据（断言目标不变，仅取数路径变更）。

- [ ] **Step 4: 全量回归 + Commit**

Run: `make test`
Expected: 全绿。

```bash
rtk git add -A && rtk git commit -m "test(e2e): upstream 分组分页/检索/隔离用例与旧列表断言迁移"
```

---

### Task 5: 前端类型与 API client 扩展

**Files:**
- Modify: `web/src/lib/types.ts`、`web/src/lib/api-client.ts`

**Interfaces:**
- Produces:
  ```ts
  // types.ts
  export interface UpstreamUser { id: number; name: string; avatar: string; }
  export interface UpstreamEndpointItem extends 既有 endpoint 字段形状（含 maskedAPIKey 等） { user?: UpstreamUser; }
  export interface UpstreamModelItem { id:number; alias:string; modelId:string; upstreamModel:string; enabled:boolean; contextLength:number; maxOutputTokens:number; capabilities:string[]; user?: UpstreamUser; createdAt:string; updatedAt:string; }
  export interface UpstreamGroupItem { endpoint: UpstreamEndpointItem; models: UpstreamModelItem[]; modelCount: number; truncated?: boolean; }
  export interface ListUpstreamRsp extends CommonRsp { groups?: UpstreamGroupItem[]; pageInfo?: PageInfo; modelTotal?: number; }
  // api-client.ts
  async listUpstream(page: number, pageSize: number, query?: string, username?: string): Promise<ListUpstreamRsp>
  ```

- [ ] **Step 1: 加类型与方法（旧类型暂留，Task 7 再删）**

在 `types.ts` 的 apikeys 类型区附近加上述四个 interface；`api-client.ts` 在 Sessions 区后新增：

```ts
async listUpstream(
  page: number = 1,
  pageSize: number = 10,
  query?: string,
  username?: string,
): Promise<ListUpstreamRsp> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
  if (query) params.set("query", query);
  if (username) params.set("username", username);
  return this.request<ListUpstreamRsp>(`/api/v1/upstream/list?${params}`);
}
```

- [ ] **Step 2: 验证构建**

Run: `cd web && npm run lint && npm run build`（注意 rtk next build 有假构建坑——核对 `out/index.html` mtime 已更新）

- [ ] **Step 3: Commit**

```bash
rtk git add -A && rtk git commit -m "feat(web): upstream 分组列表类型与 API client"
```

---

### Task 6: `/upstream` 页面实现（形态 A 分组表 + 双 Dialog + 移动卡）

**Files:**
- Create: `web/src/app/(dashboard)/upstream/page.tsx`
- Modify: 无（导出弹窗组件原样 import 复用）

**Interfaces:**
- Consumes: Task 5 的 `api.listUpstream` / 类型；既有组件 `PageHeader`、`FilterBar/useFilterBar`、`PaginationBar`、`TableSkeleton`、`ListEmptyState`、`DeleteButton/DeleteConfirmDialog/useDeleteConfirm`、`Avatar(size="sm")`、`TooltipRoot 系列`、`ProviderIcon`、`TraceInstallPopover`、`ExportDialog/ExportClaudecodeDialog/ExportCodexDialog/ExportPiDialog`、`useOptimisticUpdate`、`copyTextToClipboard`、`api.createEndpoint/updateEndpoint/deleteEndpoint/createModel/updateModel/deleteModel/listUsers`
- Produces: 路由 `/upstream/` 页面默认导出

页面结构要点（照此精确实现）：

- [ ] **Step 1: 数据层骨架**

```tsx
const ENDPOINT_FETCH_LIMIT = 100;           // 不再需要（endpoint 选择器取消），勿携带
const VALID_PAGE_SIZES = [10, 20, 50];
// state: groups: UpstreamGroupItem[]、pageInfo、modelTotal、loading
// facets（仅 admin）：key "username"、label t("upstream.filter_by_username")、target "param"、single:true
// fetchUpstream(page, pageSize, query?, username?) → api.listUpstream(...)；setGroups/setPageInfo/setModelTotal
// useEffect [queryParams] 回第 1 页查询（沿用 endpoints 页现版 eslint-disable 注释模式）
```

- [ ] **Step 2: 导出数据派生**

```tsx
export const exportModels = useMemo(() => {
  const seen = new Set<string>();
  return groups.flatMap((g) => g.models).filter((m) => {
    if (!m.enabled || seen.has(m.alias)) return false;
    seen.add(m.alias);
    return true;
  });
}, [groups]);
```

四个导出弹窗与 `<TraceInstallPopover />` 挂 PageHeader actions 区（布局照旧 models 页现状）。

- [ ] **Step 3: 桌面分组表渲染**

外层仍是 `<Table>`；每个 group 渲染一个 `<TableRow>` 作为组头（`className="bg-muted/60 hover:bg-muted/60"`，全部 `<TableCell colSpan={9}>` 包一行 flex），随后该组每个 model 渲染常规数据行：

- 组头内容顺序：`<Avatar size="sm">`（avatar 缺省时 fallback 首字母大写；`g.user` 缺省整体显示 `<span className="text-muted-foreground">—</span>`）→ 用户名（`max-w-[14ch] truncate` 且包 TooltipTrigger，名字与 endpoint 名一样可点击跳 `/endpoints`？否——旧路由已死，纯文本即可）→ endpoint 名（font-medium）→ 协议 badges（`supportOpenAIChatCompletion/OpenAIResponse/AnthropicMessage` + ProviderIcon，复刻现 endpoints 页样式）→ `· N` 计数（`text-xs text-muted-foreground`，N=modelCount）→ 截断提示（`g.truncated` 时 Badge variant outline 文案 `t("upstream.truncated")`）→ 右侧操作：编辑铅笔（`openEditEndpoint(g.endpoint)`）、DeleteButton（locked=isDemo()，onClick `deleteEndpointConfirm.openDelete(g.endpoint)`）、`[+ 模型]` Button size sm variant outline（`openCreateModel(g.endpoint)`，aria-label `t("upstream.add_model")`）。
- 组头行占位符策略：恒定文本 `—` 不需要 tooltip；用户名与 endpoint 名都 truncate，均须处于 TooltipTrigger 子树（lint error 级强制）。
- 模型行 8 列（比旧 models 表少 Endpoint 列）：alias（ProviderIcon + 点击复制 + Tooltip "click_to_copy"，沿用 handleCopyAlias 现实现）、modelId（12ch truncate 或 `—`）、upstream（20ch truncate）、limits（context/maxOutput 两枚 badge + Tooltip 全数值）、capabilities（CapabilityBadges 组件原样搬入本文件）、enabled Switch（`useOptimisticUpdate` 作用于扁平化 models——以 `(groupId, modelId)` 定位回填后 flatten 再 setState）、created、操作 ✎/🗑。
- 表头列顺序与模型行一致；`TableHead` 全部保留 `whitespace-nowrap` 默认（table.tsx 已内置）。

- [ ] **Step 4: 端点 Dialog（新建含 admin 代建）**

沿用 endpoints 页现有 EndpointForm 字段与校验（name/openaiBaseURL/anthropicBaseURL/apiKey/support×3），DialogDescription 加 `min-h-[2.5rem]`；**新增**：`isAdmin()` 时渲染「归属用户」Select：

```tsx
// state: userOptions: {id:number;name:string}[] ，isAdmin() 时 openCreate 前 ensure UsersLoaded()
const rsp = await api.listUsers(1, 500);
setUserOptions((rsp.users?.items ?? []).map(u => ({ id: u.id, name: u.name })));
// form 增加可选字段 ownerUserID?: number；提交时 ...(form.ownerUserID ? { ownerUserID: form.ownerUserID } : {})
```

（`api.listUsers`、`CreateEndpointReqBody.ownerUserID` 均已存在，签名以 `api-client.ts:319`、`types.ts:399` 为准。）

- [ ] **Step 5: 模型 Dialog（无 endpoint 选择器）**

从 models 页现行表单裁剪：alias/modelId 跟随逻辑、upstream/context(maxOutput presets popover)/capability switches 全部保留；**删除** endpoint Select 区块。state 增加 `targetEndpointID: number`，由 `openCreateModel(ep)/openEditModel(model, group.endpoint.id)` 注入；create 提交体带 `endpointID: targetEndpointID`；update 提交体不带 endpointID。

- [ ] **Step 6: 删除确认双通道**

```tsx
const deleteEndpointConfirm = useDeleteConfirm<UpstreamEndpointItem>({
  onConfirm: async (ep) => { await api.deleteEndpoint(ep.id); toast.success(t("endpoints.deleted_success")); refresh(); },
  onError: ...,
});
// DeleteConfirmDialog description: t("upstream.delete_endpoint_desc").replace("{name}", ep.name).replace("{count}", String(groupModelCount(ep.id)))
// 组计数辅助: groups.find(g => g.endpoint.id === ep.id)?.modelCount ?? 0
const deleteModelConfirm = useDeleteConfirm<UpstreamModelItem>({ ...现有 models 版同构 });
```

- [ ] **Step 7: Mobile 卡片形态**

每组一张卡（rounded-lg border bg-card p-4）：头行为 Avatar+用户名+endpoint 名+badges+操作钮；下方 `divide-y` 列出模型摘要（alias/upstream/limits 徽标/enabled Switch/编辑删除）。空态与加载态沿用 `ListEmptyState`(icon Server) / `TableSkeleton`。由 `<PermissionGuard module="upstream">` 包裹、`isMobile` 分流结构复制 endpoints/models 页现状。

- [ ] **Step 8: 自验 + Commit**

Run: `cd web && npm run lint && npm run build`
Expected: 0 error（重点盯 truncate-requires-tooltip、exhaustive-deps 手动 disable 是否齐全）。

```bash
rtk git add -A && rtk git commit -m "feat(web): /upstream 分组页（表格内分组行+CRUD 弹窗+移动卡片）"
```

---

### Task 7: 导航切换、dashboard 收敛、旧资产清理、locale

**Files:**
- Modify: `web/src/app/(dashboard)/layout.tsx`、`web/src/app/(dashboard)/page.tsx`、`web/src/lib/types.ts`、`web/src/lib/api-client.ts`、`web/src/components/permission-guard.tsx`（仅 DemoModule 类型来源处）、`web/src/locales/en.json|ja.json|zh.json`
- Delete: `web/src/app/(dashboard)/endpoints/`、`web/src/app/(dashboard)/models/`

**Interfaces:**
- Consumes: Task 6 的 `/upstream` 页
- Produces: 单一入口；旧页面与旧 API 方法清零

- [ ] **Step 1: 导航替换（layout.tsx gateway 组）**

```tsx
// 删除 nav.endpoints(/endpoints/, icon Server, demoModule "endpoints")
//   与 nav.models(/models/, icon Cpu, demoModule "models") 两项，原位合入：
{
  labelKey: "nav.upstream",
  href: "/upstream/",
  icon: <Layers className="size-4" />,   // lucide Layers 图标（Server/Cpu 语义二合一；如 imports 需调整同步处理）
  demoModule: "upstream",
},
```

NavItem 的 `demoModule` 类型是 TS 联合（`types.ts:26` 起），把联合里 `"endpoints" | "models"` 替换为 `"upstream"`。

- [ ] **Step 2: dashboard 统计收敛（page.tsx）**

```tsx
const canListUpstream = isAdmin() || isModuleOpen("upstream");
const upstreamRsp = canListUpstream ? await api.listUpstream(1, 1).catch(() => null) : null;
// stats.endpoints = upstreamRsp?.pageInfo?.total ?? 0
// stats.models    = upstreamRsp?.modelTotal ?? 0
```

删除 `canListEndpoints/canListModels/endpointsRsp/modelsRsp` 四个旧变量与两次旧调用。

- [ ] **Step 3: 删除旧页面目录与旧 API 面**

```bash
rtk git rm -r "web/src/app/(dashboard)/endpoints" "web/src/app/(dashboard)/models"
```

`api-client.ts` 删除 `listEndpoints`；`types.ts` 删除 `EndpointItem`、`ListEndpointsRsp`、`ModelItem`、`ListModelsRsp` 等仅这两页使用的类型（每删一个先全局搜索确认零引用——`rtk grep -rn "<TypeName>" web/src`）。`createEndpoint/updateEndpoint/deleteEndpoint`/model CRUD 方法保留（upstream 页在用）。

- [ ] **Step 4: locale 三语键增删**

`en.json` 示例（zh/ja 语义对应，简繁/汉字准确翻译）：

```jsonc
"nav": { "upstream": "Upstream MaaS" },                    // zh: "上游服务" ja: "上流サービス"
"upstream": {
  "title": "Upstream MaaS",
  "subtitle": "Manage upstream endpoints and their models",
  "filter_by_username": "Filter by owner",
  "add_model": "Add model",
  "truncated": "Truncated",
  "delete_endpoint_desc": "This will delete endpoint \"{name}\" and affect {count} model(s) under it.",
  "owner": "Owner",
  "load_error": "Failed to load upstream configuration"
  // 组头/模型行操作文案尽量复用现存 endpoints.* / models.* 键；删除两页后无人引用的键顺带清理：
  // endpoints.search_endpoints/empty/all_endpoints/base_url/... 、models.all_models/no_models/export*/...
}
```

清理顺序：先全局搜 `t("endpoints.` 与 `t("models.` 在剩余代码中的引用清单，JSON 里只删无引用键，防误删共用键（`common.created` 等别动）。

- [ ] **Step 5: 全站校验**

Run: `cd web && npm run lint && npm run format:check && npm run build`
Expected: 0 error 0 warn。再人工核对 `web/out/` 产物时间戳（rtk next build 假构建陷阱）。

- [ ] **Step 6: Commit**

```bash
rtk git add -A && rtk git commit -m "refactor(web): endpoints/models 合并为 /upstream 入口，旧页面与旧接口面下线"
```

---

### Task 8: 端到端联调验证（本地 + chrome mcp）

**Files:** 无代码改动（发现问题则回到对应任务修）

- [ ] **Step 1: 起后端**

Run: `go run ./cmd/server server start --host localhost --port 8080`

- [ ] **Step 2: 起前端并浏览器验证**

`NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 cd web && npm run dev` 后用 chrome-devtools mcp 打开 `http://localhost:3000/web/`：
1. 登录后侧边栏出现 Upstream MaaS、原 Endpoints/Models 消失；
2. 分组表：组头头像/用户名/badges/计数正确，翻页 10 档位文案为「每页 N 个端点」语义（PaginationBar totalLabel 传 `t("pagination.upstreams")`——若_pagination 空间没有该键，三语补齐_）；
3. 关键词搜组内模型别名 → 整组出现；admin username facet 生效；
4. 四类弹窗闭环：新建端点(admin 代建下拉) → 组内新建模型 → 编辑模型(enabled 开关乐观更新) → 删除端点提示影响 N 个模型；
5. 截断场景（造 >200 模型的组或临时把常量调成 3 本地验证后还原）Badge 出现；
6. mobile 视口（devtools resize）卡片形态正常；
7. Dashboard 统计卡的 Endpoints/Models 数字来自一次 upstream 调用。

发现的问题就地修复后再走一遍第 5 步 lint/build。

- [ ] **Step 3: 沉淀经验 + 最终提交**

用 `serena_write_memory` 记录本次可复用结论（分组视图分页语义、demo 枚举归并的存量数据注意事项、client 路由改名风险窗口）。

```bash
rtk git add -A && rtk git commit -m "chore: upstream 重构收尾" ; rtk git push origin feature/upstream-maas-web-redesign-2026-08-27
```

推送后在 GitHub 开 PR 到 master（CI 仅跑 lint，合并即自动部署发布）。

---

## Self-Review 结论

- **Spec 覆盖**：嵌套 user（Task 1/2/5）、组分页+modelTotal+truncated（Task 1/2/4/6）、keyword 整组聚合（Task 1/4/6）、admin 代建+username 过滤（Task 2/6/8）、demo 枚举归并与存量 SQL 提示（Task 2 风险注记）、client 改名（Task 3）、旧 list 删除与消费方迁移（Task 2/4/7）、前端契约与 locale（Task 5/7）、dashboard 双数合一（Task 7）。无缺口。
- **占位符扫描**：Task 1 Step 1 的 dao 批查函数名标注为“以现文件用法为准”，这是防错指引而非待办缺口，其余均为真实代码。
- **类型一致性**：`ListUpstreamHandler.Handle` 四返回值在各任务间一致；TS `listUpstream` 签名与 Task 6 调用一致；`module="upstream"`/`demoModule:"upstream"` 前后端拼写一致。
