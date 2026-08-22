# Demo 访问事件审计 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 记录 demo 账户的登录事件（成功/被拒）与模块级 API 访问（放行/被拒），admin 在 Audit 下新增子页查看。

**Architecture:** 新表 `demo_access_audits` + 协程池异步写入（照抄 `SubmitModelCallAuditTask` 模式）；模块访问埋点内嵌在 `limitUserPermission` 的 demo 分支（管理 API 错误一律 HTTP 200，无法凭状态码分类）；登录埋点在 demo login 用例；查询接口挂现有 audit 路由组（admin only）；前端 `audit/demo` 子页照抄 cron 审计页骨架。

**Tech Stack:** Go 1.25 + Fiber v3 + Huma + GORM + fx、React/Next.js + TypeScript。

**Spec:** `docs/superpowers/specs/2026-08-22-demo-access-audit-design.md`

## Global Constraints

- 所有回复、注释、文档使用中文；代码标识符英文。
- 测试只用标准库 `testing`，禁止 testify/gomock、禁止 `time.Sleep` 同步；`*_test.go` 只能放 `test/unit/<topic>/` 或 `test/e2e/<topic>/`。
- 编写 Go 代码必须加载 `golang-samber-lo` 和 `golang-samber-mo` skill。
- 实现阶段激活 `ponytail`（full）：不建投机抽象；刻意简化处用 `// ponytail: <ceiling>, <upgrade path>` 注释标记。
- 常量一律进 `internal/common/constant/`，枚举进 `internal/common/enum/`，禁止魔法值。
- Git：worktree 开发，分支名 `feature/demo-access-audit-2026-08-23`；每任务一提交。
- 提交信息格式参照仓库历史（如 `feat(audit): ...`）。

---

### Task 1: 数据模型与常量

**Files:**
- Create: `internal/common/enum/demo_access_action.go`
- Create: `internal/common/constant/sql.go`（追加常量）
- Create: `internal/infrastructure/database/model/demo_access_audit.go`
- Modify: `internal/infrastructure/database/model/base.go:29-32`（AutoMigrate 注册）

**Interfaces:**
- Produces:
  - 枚举：`DemoAccessActionLogin = "login"`、`DemoAccessActionLoginDenied = "login_denied"`、`DemoAccessActionModuleAccess = "module_access"`、`DemoAccessActionModuleDenied = "module_denied"`（type `DemoAccessAction = string`）
  - 拒绝原因常量：`DemoAccessReasonLoginDisabled = "login_disabled"`、`DemoAccessReasonNoDemoUser = "no_demo_user"`、`DemoAccessReasonModuleClosed = "module_closed"`
  - 表名常量 `FieldTableDemoAccessAudit = "demo_access_audits"`、字段常量 `FieldAction/FieldModule/FieldPath/FieldIP(FieldIP 已存在则复用)/FieldUserAgent(已存在 FieldUserAgent)/FieldReason`
  - GORM model `dbmodel.DemoAccessAudit{BaseModel; Action, Module, Path, IP, UserAgent, Reason string}`，`TableName()` 返回 `demo_access_audits`

- [ ] **Step 1: 写 model 与 enum**

`internal/common/enum/demo_access_action.go`：

```go
package enum

// DemoAccessAction Demo 访问审计动作类型
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type DemoAccessAction = string

const (
	DemoAccessActionLogin        DemoAccessAction = "login"         // demo 登录成功
	DemoAccessActionLoginDenied  DemoAccessAction = "login_denied"  // demo 登录被拒
	DemoAccessActionModuleAccess DemoAccessAction = "module_access" // 模块访问放行
	DemoAccessActionModuleDenied DemoAccessAction = "module_denied" // 模块访问被白名单拒绝
)
```

拒绝原因常量追加到 `internal/common/constant/string.go`（或紧邻 demo 相关常量的文件）：

```go
// ── Demo access audit reason ──
DemoAccessReasonLoginDisabled = "login_disabled"
DemoAccessReasonNoDemoUser    = "no_demo_user"
DemoAccessReasonModuleClosed  = "module_closed"
```

`internal/infrastructure/database/model/demo_access_audit.go`（字段 tag 风格对齐 `cron.go:37-48` 的 CronCallAudit）：

```go
package model

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// DemoAccessAudit Demo 访问审计记录
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type DemoAccessAudit struct {
	BaseModel
	Action    string `json:"action" gorm:"column:action;not null;index;comment:动作:login/login_denied/module_access/module_denied"`
	Module    string `json:"module" gorm:"column:module;not null;default:'';comment:demo 模块名;index"`
	Path      string `json:"path" gorm:"column:path;not null;default:'';comment:请求路径"`
	IP        string `json:"ip" gorm:"column:ip;not null;default:'';comment:客户端 IP"`
	UserAgent string `json:"user_agent" gorm:"column:user_agent;not null;default:'';comment:User-Agent"`
	Reason    string `json:"reason" gorm:"column:reason;not null;default:'';comment:拒绝原因:login_disabled/no_demo_user/module_closed"`
}

// TableName 返回表名
func (DemoAccessAudit) TableName() string {
	return constant.FieldTableDemoAccessAudit
}
```

注：若 `model` 包此文件不需要 `common/model` import 则去掉。表名/字段常量加入 `internal/common/constant/sql.go`（跟随既有命名风格，参考 `sql.go:275` 的 `FieldTableCronCallAudit`）。AutoMigrate 列表加 `&DemoAccessAudit{}`。

- [ ] **Step 2: 编译验证**

Run: `rtk go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
rtk git add internal/common/ internal/infrastructure/database/model/
rtk git commit -m "feat(demo-access-audit): add DemoAccessAudit model and enums"
```

---

### Task 2: DTO 任务结构体 + 仓储 + 协程池提交

**Files:**
- Modify: `internal/dto/asynctask.go`（末尾追加任务结构体）
- Create: `internal/application/demoaccessaudit/port/handler.go`
- Create: `internal/infrastructure/repository/demo_access_audit_repository.go`
- Modify: `internal/infrastructure/pool/store_pool.go`（追加 Submit 方法）
- Modify: `internal/bootstrap/modules/repository.go`（注册仓储提供者）

**Interfaces:**
- Consumes: Task 1 的 `dbmodel.DemoAccessAudit`、枚举常量。
- Produces:
  - `dto.DemoAccessAuditTask{Ctx context.Context; Action, Module, Path, IP, UserAgent, Reason string}`（`internal/dto/asynctask.go`）
  - 端口包 `application/demoaccessaudit/port`：
    - `DemoAccessAuditView{ID uint; Action, Module, Path, IP, UserAgent, Reason string; CreatedAt time.Time}`
    - `ListDemoAccessAuditsHandler interface { Handle(ctx context.Context, param commonmodel.CommonParam, startTime, endTime time.Time, filterExp string) ([]*DemoAccessAuditView, *commonmodel.PageInfo, error) }`
    - `ListDemoAccessAuditOptionsHandler interface { Handle(ctx context.Context, field, keyword string, startTime, endTime time.Time) ([]string, error) }`
    - `DemoAccessAuditRepository interface { Save(ctx context.Context, view *DemoAccessAuditView) error; List(...同上); ListDistinctActions(...); ListDistinctModules(...) }`（签名对齐 `cronaudit/port/handler.go:44-53`）
  - `repository.NewDemoAccessAuditRepository(db *gorm.DB) port.DemoAccessAuditRepository`
  - `PoolManager.SubmitDemoAccessAuditTask(task *dto.DemoAccessAuditTask) error`
  - fx：`modules.NewDemoAccessAuditRepository` 注册进 RepositoryModule

- [ ] **Step 1: 写 DTO 任务结构体**

`internal/dto/asynctask.go` 末尾追加（对齐 `ModelCallAuditTask` 风格）：

```go
// DemoAccessAuditTask Demo 访问审计异步落库任务
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type DemoAccessAuditTask struct {
	Ctx       context.Context
	Action    string // login / login_denied / module_access / module_denied
	Module    string // demo 模块名；login 类为空串
	Path      string // 请求路径
	IP        string // 客户端 IP
	UserAgent string // User-Agent
	Reason    string // 拒绝原因；成功时为空串
}
```

- [ ] **Step 2: 写端口定义**

创建 `internal/application/demoaccessaudit/port/handler.go`，内容完全对照 `internal/application/cronaudit/port/handler.go` 的结构（View / ListHandler / ListOptionsHandler / Repository 四个接口），字段替换为 View 六字段 + CreatedAt。

- [ ] **Step 3: 写仓储实现**

创建 `internal/infrastructure/repository/demo_access_audit_repository.go`：Save 直接 `Create(&dbmodel.DemoAccessAudit{...})`；List 对照 `cron_audit_repository.go:75-128`（分页 + startTime/endTime + filter + 排序），filter 字段配置用 Task 1 定义的 `action`/`module` 两列；`ListDistinctActions`/`ListDistinctModules` 对照 `cron_audit_repository.go:139-190`。

filter 常量（加到 `internal/common/constant/sql.go` 或 cron 常量旁）：

```go
DemoAccessAuditFilterFieldAction  = "action"
DemoAccessAuditFilterFieldModule  = "module"
DemoAccessAuditFilterActionSQLColumn  = "action"
DemoAccessAuditFilterModuleSQLColumn  = "module"
```

- [ ] **Step 4: 写协程池提交方法**

`internal/infrastructure/pool/store_pool.go` 追加（对照 `SubmitModelCallAuditTask` :187-209）：

```go
// SubmitDemoAccessAuditTask 提交 Demo 访问审计任务到协程池（best-effort，失败仅打日志）
func (pm *PoolManager) SubmitDemoAccessAuditTask(task *dto.DemoAccessAuditTask) error {
	return pm.storePool.Go(func() {
		l := logger.WithCtx(task.Ctx)
		view := &demoauditport.DemoAccessAuditView{
			Action:    task.Action,
			Module:    task.Module,
			Path:      task.Path,
			IP:        task.IP,
			UserAgent: task.UserAgent,
			Reason:    task.Reason,
		}
		if err := pm.demoAccessAuditRepo.Save(task.Ctx, view); err != nil {
			l.Error("[StorePool] Failed to store demo access audit", zap.Error(err))
			return
		}
	})
}
```

配套修改：
- `PoolManager` struct（`pool.go:27`）加字段 `demoAccessAuditRepo demoauditport.DemoAccessAuditRepository`；
- `NewPoolManager`（`pool.go:41`）加参数并赋值；
- `NewPoolManager` 的调用点（fx infra module，`rtk grep -rn "NewPoolManager" internal/bootstrap/` 定位）同步传参。

- [ ] **Step 5: 注册 DI**

`internal/bootstrap/modules/repository.go`：
- `fx.Provide` 列表加 `NewDemoAccessAuditRepository`（:37 附近）；
- 加包装函数 `func NewDemoAccessAuditRepository(db *gorm.DB) demoauditport.DemoAccessAuditRepository { return repository.NewDemoAccessAuditRepository(db) }`（对照 :98-100 的 NewDemoConfigRepository）。

- [ ] **Step 6: 编译 + 全量测试**

Run: `rtk go build ./... && rtk go test ./test/unit/bootstrap/ -count=1`
Expected: 编译通过，bootstrap 单测通过（fx 容器可解析）

- [ ] **Step 7: Commit**

```bash
rtk git add internal/
rtk git commit -m "feat(demo-access-audit): async write path via pool submitter"
```

---

### Task 3: 登录埋点（demo login 用例）

**Files:**
- Modify: `internal/application/demo/port/handler.go:38-51`（DemoLoginCommand 扩展）
- Modify: `internal/application/demo/command/login.go`（注入 submitter + 三处埋点）
- Modify: `internal/handler/demo.go:84-94`（handler 取 IP/UA 放入 command）
- Modify: `internal/bootstrap/modules/application.go:271-282`（DI 参数扩展）
- Test: `test/unit/demo_access_audit/login_audit_test.go`

**Interfaces:**
- Consumes: Task 2 的 `dto.DemoAccessAuditTask`、`PoolManager.SubmitDemoAccessAuditTask`。
- Produces:
  - `demoport.DemoSubmitter interface { SubmitDemoAccessAuditTask(task *dto.DemoAccessAuditTask) error }`（窄接口，定义在 demo port 包）
  - `demoport.DemoLoginCommand` 扩展两字段：`ClientIP string`、`UserAgent string`
  - `democommand.NewDemoLoginHandler(configRepo, userRepo, accessSigner, refreshSigner, submitter demoport.DemoSubmitter)` 五参构造

- [ ] **Step 1: 写失败单测**

`test/unit/demo_access_audit/login_audit_test.go`：用 fake configRepo/userRepo/signers/submitter（全部手写 stub，标准库 testing），覆盖三分支断言 submitter 收到的 task：

1. 开关关闭 → action=`login_denied`, reason=`login_disabled`，且 IP/UA 透传。
2. 无 demo 用户 → action=`login_denied`, reason=`no_demo_user`。
3. 成功 → action=`login`，reason 为空串，module/path 为空串。

测试骨架（stub 结构对照 `test/unit/demo_config/update_config_test.go` 的手写 fake 风格）：

```go
package demo_access_audit_test

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/command"
	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

type fakeConfigRepo struct {
	port.DemoConfigRepository
	loginEnabled bool
	userExists   bool
}

func (f *fakeConfigRepo) Get(context.Context) (*port.DemoConfigEntity, error) {
	return &port.DemoConfigEntity{LoginEnabled: f.loginEnabled}, nil
}

// fakeUserRepo / fakeSigner / fakeSubmitter 同理按接口最小实现……
// 断言核心：submitter.tasks[0].Action / .Reason / .IP / .UserAgent 与预期一致，
// 且成功分支 err == nil、拒绝分支 errors.Is(err, ierr.ErrNoPermission / ErrDataNotExists)。
```

（执行时补全 fakeUserRepo 实现 `FindByPermission`/`TouchLastLogin`；fakeSigner 实现 `EncodeToken`/`DecodeToken`——以 `identityservice.TokenSigner` 实际签名为准；fakeSubmitter 收集 tasks 切片。）

- [ ] **Step 2: 运行确认失败**

Run: `go test -count=1 ./test/unit/demo_access_audit/`
Expected: FAIL（构造函数参数不匹配或未实现）

- [ ] **Step 3: 实现 port/command/handler/DI 改动**

1. `internal/application/demo/port/handler.go`：`DemoLoginCommand` 加 `ClientIP string`、`UserAgent string`；新增 `DemoSubmitter` 窄接口。
2. `internal/application/demo/command/login.go`：`demoLoginHandler` 加 `submitter` 字段；三个埋点（在现有 log.Info/Error 之后）统一走 helper：

```go
func (h *demoLoginHandler) submitAudit(ctx context.Context, action enum.DemoAccessAction, reason string, cmd port.DemoLoginCommand) {
	task := &dto.DemoAccessAuditTask{
		Ctx:       util.CopyContextValues(ctx),
		Action:    action,
		IP:        cmd.ClientIP,
		UserAgent: cmd.UserAgent,
		Reason:    reason,
	}
	if err := h.submitter.SubmitDemoAccessAuditTask(task); err != nil {
		logger.WithCtx(ctx).Warn("[DemoCommand] Submit demo access audit failed", zap.Error(err))
	}
}
```

调用点：
- `!config.LoginEnabled` 分支 return 前 → `submitAudit(ctx, enum.DemoAccessActionLoginDenied, constant.DemoAccessReasonLoginDisabled, cmd)`
- `user == nil` 分支 return 前 → `submitAudit(ctx, enum.DemoAccessActionLoginDenied, constant.DemoAccessReasonNoDemoUser, cmd)`
- 成功 return 前 → `submitAudit(ctx, enum.DemoAccessActionLogin, "", cmd)`

3. `internal/handler/demo.go` HandleLogin：IP/UA 无法从 request context 直接取得（JWT 中间件不注入），因此新增轻量中间件在路由层注入。新建 `internal/middleware/request_meta.go`：

```go
// internal/middleware/request_meta.go 新增
func InjectRequestMetaMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		fCtx := humafiber.Unwrap(ctx)
		ctx = huma.WithValue(ctx, constant.CtxKeyClientIP, fCtx.IP())
		ctx = huma.WithValue(ctx, constant.CtxKeyClientUA, fCtx.Get(constant.HTTPHeaderUserAgent))
		next(ctx)
	}
}
```

配套：`internal/common/constant/ctx.go` 加 `CtxKeyClientIP enum.CtxKey = "clientIP"`、`CtxKeyClientUA enum.CtxKey = "clientUA"`；`util.CtxValueString` 可直接读取。
`router/demo.go` demoLogin 的 Middlewares 加 `middleware.InjectRequestMetaMiddleware()`（放在限流之后）；`handler.HandleLogin` 用 `util.CtxValueString(ctx, constant.CtxKeyClientIP)` / `...ClientUA` 填充 command。

4. `internal/bootstrap/modules/application.go` `demoLoginParams` 加 `Submitter demoport.DemoSubmitter`；`NewDemoLoginHandler` 传五参。

`DemoSubmitter` 的 fx 绑定：在 application.go 加

```go
fx.Annotate(
	func(pm *pool.PoolManager) demoport.DemoSubmitter { return pm },
	fx.As(new(demoport.DemoSubmitter)),
),
```

（`*pool.PoolManager` 因 Task 2 已有该方法天然满足接口；如 fx 对指针方法集绑定报错，则退化为显式 adapter struct。）

- [ ] **Step 4: 运行单测确认通过**

Run: `go test -count=1 ./test/unit/demo_access_audit/ && rtk go build ./...`
Expected: PASS

- [ ] **Step 5: 全量回归**

Run: `make test`
Expected: 全部通过

- [ ] **Step 6: Commit**

```bash
rtk git add internal/ test/unit/demo_access_audit/
rtk git commit -m "feat(demo-access-audit): audit demo login success and denial"
```

---

### Task 4: 模块访问埋点（权限中间件 demo 分支）

**Files:**
- Modify: `internal/middleware/permission.go`（demo 分支加埋点）
- Modify: `internal/common/constant/ctx.go`（如需 ctx key）
- Test: `test/unit/demo_access_audit/module_access_audit_test.go`

**Interfaces:**
- Consumes: Task 2 的 submitter；Task 3 的 `InjectRequestMetaMiddleware`（IP/UA 已在 ctx）。
- Produces:
  - `LimitUserPermissionWithDemoMiddleware(serviceName string, requiredPermission enum.Permission, demoModule enum.DemoModule, demoAccessor demoport.DemoModuleAccessor, auditSubmitter demoport.DemoSubmitter)` —— **五参新签名**
  - `LimitUserPermissionMiddleware` 签名不变（不产生审计）

- [ ] **Step 1: 写失败单测**

`test/unit/demo_access_audit/module_access_audit_test.go`：直接测 `limitUserPermission` 返回的闭包（同包内可测私有函数需放 `middleware` 包内？不行——测试必须在 `test/unit/`，因此导出测试入口：通过公开的 `LimitUserPermissionMiddleware` / `LimitUserPermissionWithDemoMiddleware` 构造闭包测试）。构造 huma ctx 较重，采用轻量方式：将「分类+组装 task」逻辑抽为中间件包内纯函数 `classifyDemoAccess(permission enum.Permission, demoModule enum.DemoModule, open bool) (action enum.DemoAccessAction, reason string, ok bool)` 并导出为 `ClassifyDemoAccess`（仅测试可见性需要，文档注明供审计用），单测直接测该函数：

| permission | demoModule | open | 期望 |
|-----------|-----------|------|------|
| demo | sessions | true | (`module_access`, "", true) |
| demo | sessions | false | (`module_denied`, "module_closed", true) |
| user | sessions | true | ("", "", false) —— 不产生审计 |
| admin | — | — | ("", "", false) |

```go
package demo_access_audit_test

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func TestClassifyDemoAccess(t *testing.T) {
	cases := []struct {
		name        string
		permission  enum.Permission
		open        bool
		wantAction  enum.DemoAccessAction
		wantReason  string
		wantAudited bool
	}{
		{"demo allowed", enum.PermissionDemo, true, enum.DemoAccessActionModuleAccess, "", true},
		{"demo denied", enum.PermissionDemo, false, enum.DemoAccessActionModuleDenied, constant.DemoAccessReasonModuleClosed, true},
		{"user not audited", enum.PermissionUser, true, "", "", false},
		{"admin not audited", enum.PermissionAdmin, false, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			action, reason, ok := middleware.ClassifyDemoAccess(tc.permission, tc.open)
			if action != tc.wantAction || reason != tc.wantReason || ok != tc.wantAudited {
				t.Fatalf("got (%q,%q,%v), want (%q,%q,%v)", action, reason, ok, tc.wantAction, tc.wantReason, tc.wantAudited)
			}
		})
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test -count=1 ./test/unit/demo_access_audit/`
Expected: FAIL（`ClassifyDemoAccess` 未定义）

- [ ] **Step 3: 实现**

`internal/middleware/permission.go`：

```go
// ClassifyDemoAccess 判定一次 demo 模块访问是否需要审计及动作分类。
// 非 demo 身份返回 ok=false（admin/user 正常使用路由不产生审计记录）。
//
//	@param permission enum.Permission 当前请求身份
//	@param open bool 目标模块是否对 demo 开放
//	@return enum.DemoAccessAction
//	@return string 拒绝原因
//	@return bool 是否产生审计
func ClassifyDemoAccess(permission enum.Permission, open bool) (enum.DemoAccessAction, string, bool) {
	switch {
	case permission != enum.PermissionDemo:
		return "", "", false
	case open:
		return enum.DemoAccessActionModuleAccess, "", true
	default:
		return enum.DemoAccessActionModuleDenied, constant.DemoAccessReasonModuleClosed, true
	}
}
```

`limitUserPermission` demo 分支改造（permission==demo 时，无论放行/拒绝都先分类并提交审计；auditSubmitter 为 nil 时跳过）：

```go
if permission == enum.PermissionDemo {
	open := demoModule != "" && demoAccessor != nil && demoAccessor.IsModuleOpen(ctx.Context(), demoModule)
	if action, reason, audited := ClassifyDemoAccess(permission, open); audited && auditSubmitter != nil {
		fCtx := humafiber.Unwrap(ctx)
		_ = auditSubmitter.SubmitDemoAccessAuditTask(&dto.DemoAccessAuditTask{
			Ctx:       util.CopyContextValues(ctx.Context()),
			Action:    action,
			Module:    string(demoModule),
			Path:      fCtx.Path(),
			IP:        util.CtxValueString(ctx.Context(), constant.CtxKeyClientIP),
			UserAgent: util.CtxValueString(ctx.Context(), constant.CtxKeyClientUA),
			Reason:    reason,
		})
	}
	if !open {
		// 原 logger + WriteErrorResponse 拒绝逻辑保持不变
		...
		return
	}
	next(ctx)
	return
}
```

注意：原实现中 `IsModuleOpen` 只在拒绝路径调用，改后放行路径也会调用一次——`get_config.go` 的 accessor 是读缓存的查询，开销可接受；保持 fail-closed 语义不变（accessor nil 或 IsModuleOpen panic 安全性维持现状）。

所有调用点升级为五参（grep 定位）：`router/audit.go`(9 处)、`router/session.go`(6 处)、`router/model.go`、`router/endpoint.go`、`router/trigger.go`、`router/cron.go`、`router/metrics.go`。这些 init 函数链上加 `auditSubmitter demoport.DemoSubmitter` 参数透传（`router/router.go` APIRouterDependencies 加字段、`bootstrap/router.go` routeParams 加字段并从 fx 注入）。

- [ ] **Step 4: 运行单测确认通过**

Run: `go test -count=1 ./test/unit/demo_access_audit/ && rtk go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add internal/
rtk git commit -m "feat(demo-access-audit): audit demo module access in permission middleware"
```

---

### Task 5: 查询接口（DTO + 应用服务 + handler + 路由）

**Files:**
- Create: `internal/dto/demo_access_audit.go`
- Create: `internal/application/demoaccessaudit/query/list_demo_access_audits.go`
- Create: `internal/application/demoaccessaudit/query/option_list.go`
- Modify: `internal/handler/cron.go`（CronHandler 加两个方法）
- Modify: `internal/router/audit.go`（注册两个接口）
- Modify: `internal/bootstrap/modules/{application.go,handler.go}`（DI）

**Interfaces:**
- Consumes: Task 2 的仓储 List/ListDistinctActions/ListDistinctModules。
- Produces:
  - `dto.ListDemoAccessAuditsReq{Page, PageSize, Query, Sort, SortField, StartTime, EndTime, Filter}`（结构对照 `dto.ListCronCallAuditsReq` :92-101）
  - `dto.ListDemoAccessAuditsRsp{CommonRsp; Logs []*dto.DemoAccessAuditItem; PageInfo *model.PageInfo}`
  - `dto.DemoAccessAuditItem{ID uint; Action, Module, Path, IP, UserAgent, Reason string; CreatedAt time.Time}`（json: id/action/module/path/ip/userAgent/reason/createdAt）
  - `dto.DemoAccessAuditOptionListReq{Field enum:"action,module", Keyword, StartTime, EndTime}` + Rsp
  - 路由：`GET /api/v1/audit/demo/log/list`（OperationID `listDemoAccessAudits`）、`GET /api/v1/audit/demo/option/list`（OperationID `listDemoAccessAuditOptions`），均 `Middlewares: LimitUserPermissionMiddleware(name, enum.PermissionAdmin)`（无 demo 白名单），Tag `constant.TagAudit`

- [ ] **Step 1: DTO**

新建 `internal/dto/demo_access_audit.go`，四个结构体完全对照 `internal/dto/cron.go:88-149` 的 CronCallAudit 系列（字段替换为六业务字段）。

- [ ] **Step 2: 应用服务**

完全对照 cronaudit 的 query 层模式（`internal/application/cronaudit/query/list_cron_call_audits.go`）：

`list_demo_access_audits.go`：

```go
package query

type listDemoAccessAuditsHandler struct{ repo port.DemoAccessAuditRepository }

func NewListDemoAccessAuditsHandler(repo port.DemoAccessAuditRepository) port.ListDemoAccessAuditsHandler {
	return &listDemoAccessAuditsHandler{repo: repo}
}

func (h *listDemoAccessAuditsHandler) Handle(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, filterStr string) ([]*port.DemoAccessAuditView, *model.PageInfo, error) {
	param.QueryFields = []string{constant.FieldPath, constant.FieldIP}
	return h.repo.List(ctx, param, startTime, endTime, filterStr)
}
```

`option_list.go` 同理封装 ListDistinctActions / ListDistinctModules（按 field 参数分发，对照 cronaudit/query/option_list.go）。

fx 注册（`internal/bootstrap/modules/application.go`，对照 :460-466 的 NewListCronCallAuditsHandler）：

```go
NewListDemoAccessAuditsHandler,
NewListDemoAccessAuditOptionsHandler,

func NewListDemoAccessAuditsHandler(repo demoauditport.DemoAccessAuditRepository) demoauditport.ListDemoAccessAuditsHandler {
	return demoauditquery.NewListDemoAccessAuditsHandler(repo)
}
```

- [ ] **Step 3: handler 方法**

在 `internal/handler/cron.go` 的 CronHandler 接口加 `HandleListDemoAccessAudits(ctx, req *dto.ListDemoAccessAuditsReq)` / `HandleListDemoAccessAuditOptions(ctx, req *dto.DemoAccessAuditOptionListReq)`；`CronDependencies` 加 `ListDemoAccessAudits demoauditport.ListDemoAccessAuditsHandler`、`ListDemoAccessAuditOptions demoauditport.ListDemoAccessAuditOptionsHandler` 两字段；实现对照 `HandleListCronCallAudits` :120-152。`modules/handler.go` NewCronDependencies 同步加参数。

- [ ] **Step 4: 路由注册**

`internal/router/audit.go` 末尾注册两个 Operation（Path `/demo/log/list`、`/demo/option/list`），权限 `LimitUserPermissionMiddleware("listDemoAccessAudits", enum.PermissionAdmin)`——注意此处用**不带 Demo** 的版本，demo 无法访问。

- [ ] **Step 5: 编译 + bootstrap 单测**

Run: `rtk go build ./... && go test -count=1 ./test/unit/bootstrap/`
Expected: 通过

- [ ] **Step 6: Commit**

```bash
rtk git add internal/
rtk git commit -m "feat(demo-access-audit): admin query api for demo access audits"
```

---

### Task 6: 前端子页

**Files:**
- Modify: `web/src/lib/types.ts`（DemoAccessAuditItem 类型）
- Modify: `web/src/lib/api-client.ts`（listDemoAccessAudits / listDemoAccessAuditOptions）
- Create: `web/src/app/(dashboard)/audit/demo/page.tsx`
- Modify: `web/src/app/(dashboard)/layout.tsx:124-150`（ops 组加导航项）
- Modify: `web/src/locales/{zh,en,ja}.json`

**Interfaces:**
- Consumes: Task 5 的两个 GET 接口。
- Produces: 页面 `/audit/demo/`（adminOnly）。

- [ ] **Step 1: types + api-client**

`web/src/lib/types.ts` 加（对照 CronCallAuditItem）：

```ts
export interface DemoAccessAuditItem {
  id: number;
  action: string;
  module: string;
  path: string;
  ip: string;
  userAgent: string;
  reason: string;
  createdAt: string;
}
```

`web/src/lib/api-client.ts` 加（对照 listCronCallAudits :594-679 内的实现模式）：

```ts
async listDemoAccessAudits(params: {
  page: number; pageSize: number; sort?: string; sortField?: string;
  startTime?: string; endTime?: string; filter?: string;
}): Promise<{ logs?: DemoAccessAuditItem[]; pageInfo?: PageInfo; error?: ApiError }> {
  const qs = new URLSearchParams(/* 同 listCronCallAudits 的拼装方式 */);
  return this.request(`/api/v1/audit/demo/log/list?${qs}`);
}
```

（返回类型与错误处理完全对照现有 `listCronCallAudits`。）

- [ ] **Step 2: 页面组件**

创建 `web/src/app/(dashboard)/audit/demo/page.tsx`：整页复制 `audit/cron/page.tsx` 骨架后做以下替换：

- `PermissionGuard adminOnly`（**不带** module prop——demo 用户不可见）。
- 表格列：时间 / 动作徽章（四态着色：`login`=secondary、`module_access`=default、`login_denied`/`module_denied`=destructive）/ 模块 / 路径 / IP / UA（truncate）/ 原因（仅 denied 有值）。
- facets：action、module 两个（fetchOptionsFor 对应 option 接口），无 trace 列。
- persistKey：`dashboard.demoAccessAudit`。
- i18n key 前缀 `demo_access_audit.*`。

- [ ] **Step 3: 导航项**

`layout.tsx` ops 组（:125 ops group items）在 cron_audit 后加：

```tsx
{
  labelKey: "nav.demo_access_audit",
  href: "/audit/demo/",
  icon: <Footprints className="size-4" />,
  adminOnly: true,
},
```

（icon 从 lucide-react 选一个未用的，如 `Footprints`；不加 demoModule——demo 导航渲染时无 key 的 adminOnly 项对 demo 隐藏，符合「demo 不可见」。执行时核对 layout 中 nav 过滤逻辑确认 adminOnly 项对 demo 的行为一致。）

- [ ] **Step 4: i18n**

三个 locale 文件加 `nav.demo_access_audit` 与 `demo_access_audit.*`（page_title/page_subtitle/logs_title/time/action/module/path/ip/user_agent/reason/filter_action/filter_module/search_placeholder/no_logs），zh/en 必填，ja 对照翻译。

- [ ] **Step 5: lint + build**

Run: `cd web && rtk lint && rtk tsc && npm run build`
Expected: 全部通过

- [ ] **Step 6: 本地浏览器验证**

本地起 dev server，admin 登录查看 `/audit/demo/` 渲染与筛选交互（chrome MCP）。

- [ ] **Step 7: Commit**

```bash
rtk git add web/
rtk git commit -m "feat(web): demo access audit page under Audit module"
```

---

### Task 7: E2E 用例 + 全量验证

**Files:**
- Create: `test/e2e/demo_access_audit/demo_access_audit_test.go`

**Interfaces:**
- Consumes: 全部前序任务的接口。
- Produces: e2e 回归用例。

- [ ] **Step 1: 写 e2e**

骨架对照 `test/e2e/demo/demo_account_test.go`（mustE2EEnv/doJSON 复制或抽公共；环境变量 BASE_URL/ADMIN_TOKEN/USER_TOKEN，缺省 Skip）。流程：

1. demo 登录（POST /api/v1/demo/login，无鉴权）→ 期望 200，存 demo token。
2. 用 demo token 访问一个开放模块列表（如 GET /api/v1/model/list，若该模块未开放则先经 admin PATCH /demo/config 打开 modules 含 models）→ 期望 200。
3. 用 demo token 探测锁定模块（选 modules 未开放的任一模块接口，如 GET /api/v1/trigger/list；若全开放则先 PATCH 关掉一个）→ 期望业务码 no_permission。
4. admin GET /api/v1/audit/demo/log/list?page=1&pageSize=50&sort=desc&sortField=created_at → 解析 logs，断言最近记录中包含 action=login、action=module_access、action=module_denied 各至少一条。
5. demo token 访问同一查询接口 → 期望业务码 no_permission（demo 不可见）。

注意步骤 3 的探测会真实写一条 module_denied 记录、步骤 2 会改 demo 配置——测试结束恢复原 modules 配置（先 GET 存快照，defer PATCH 还原）。

- [ ] **Step 2: 本地起服跑通**

本地 docker-compose 起 Postgres/Redis + `make run-dev`（或按 docs/agents/commands.md 的本地启动方式），设置环境变量运行：

Run: `BASE_URL=http://localhost:8080 ADMIN_TOKEN=xxx USER_TOKEN=xxx go test -v -count=1 ./test/e2e/demo_access_audit/`
Expected: PASS（含三类审计记录断言）

- [ ] **Step 3: 全量测试 + ponytail-review**

Run: `make test && make lint`
Expected: 全绿。

然后激活 `ponytail-review` skill 审查本次 diff 的过度工程。

- [ ] **Step 4: Commit**

```bash
rtk git add test/e2e/demo_access_audit/
rtk git commit -m "test: add demo access audit e2e coverage"
```

---

## 收尾（不在本计划内自动执行）

- 部署：推送 master / 合并 PR 自动触发 CI（需用户明确指示）。
- 部署后：跑 `test/e2e/demo_access_audit/` + `test/e2e/demo/` 回归；浏览器验证生产页面。
- Serena 沉淀工程经验后再提交。
