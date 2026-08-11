# Blocked Words 页面列名与交互重设计 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重设计管理后台 Blocked Words 页面的列名与交互:列名术语统一、点击徽章切换动作、行内快速添加、支持批量删除(后端 `DELETE /api/v1/block?ids=`)。

**Architecture:** 前端 `web/src/app/(dashboard)/blocked/page.tsx` 单页改造 + 三语 locale 文案更新;后端 `internal/dto/blocked.go` / `internal/handler/blocked.go` / `internal/application/blocked/{port,command}` / `internal/infrastructure/repository/blocked_repository.go` 将单删改为支持逗号分隔 ids 批量删除,复用既有 `parseCommaSeparatedIDs`(internal/handler/session.go,同包)。批量删除前端交互完全对齐 sessions 页面的 `role="checkbox"` 自绘勾选模式。

**Tech Stack:** Go (huma v2 + GORM + samber/lo) / Next.js 16 App Router + React 19 + Tailwind v4 + shadcn/ui (base-nova) + lucide-react + sonner。

## Global Constraints

- 中文回复;superpowers 文档用中文撰写。
- 后端批量删除返回 `DeletedCount`(对齐 session 的 `DeleteSessionRsp`);失败列表 `Failures` 不需要(blocked 删除无权限/所有权分支)。
- 前端所有 HTTP 调用必须走 `web/src/lib/api-client.ts` 的 `api.*`,禁止直接 fetch。
- 前端 DTO 类型改动必须同步 `web/src/lib/types.ts`。
- 图标统一 `lucide-react`;Toast 用 `sonner`;禁止 `alert/confirm`。
- i18n 布局稳定性契约:表格 `<th>` 保持 `whitespace-nowrap`;列宽 `min-w`/`w-*` 按新文案核定;`zh`/`ja` 的 CJK 字号缩放已由 `globals.css` 处理,不改。
- 翻页时清空 `selected`(对齐 sessions 行为)。
- 命令统一前缀 `rtk`。

---

### Task 1: 后端批量删除 —— port 与 command(含单测)

**Files:**
- Modify: `internal/application/blocked/port/handler.go`(`DeleteBlockedCommand`)
- Modify: `internal/application/blocked/command/delete_blocked.go`
- Test: `test/unit/blocked_command/delete_blocked_test.go`(新建)

**Interfaces:**
- Consumes: `blocked.BlockedRepository`(domain 接口,Task 2 加 `DeleteBatch` 后满足)
- Produces: `port.DeleteBlockedCommand{BlockedIDs []uint}`;`NewDeleteBlockedHandler(repo blocked.BlockedRepository, rebuildNotify func(ctx context.Context)) port.DeleteBlockedHandler`(签名不变)

- [ ] **Step 1: 写失败测试**(仿 `test/unit/blocked_command/update_blocked_test.go` 的 fake repo 模式)

创建 `test/unit/blocked_command/delete_blocked_test.go`:

```go
package blocked_command_test

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/command"
	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	blockeddomain "github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
)

type deleteFakeRepo struct {
	deletedIDs []uint
}

func (f *deleteFakeRepo) FindByID(ctx context.Context, id uint) (*aggregate.Blocked, error) {
	return nil, nil
}

func (f *deleteFakeRepo) Create(ctx context.Context, word *aggregate.Blocked) (uint, error) {
	return 0, nil
}

func (f *deleteFakeRepo) Delete(ctx context.Context, id uint) error {
	return nil
}

func (f *deleteFakeRepo) DeleteBatch(ctx context.Context, ids []uint) error {
	f.deletedIDs = ids
	return nil
}

func (f *deleteFakeRepo) UpdateAction(ctx context.Context, id uint, action string) error {
	return nil
}

func (f *deleteFakeRepo) Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Blocked, *model.PageInfo, error) {
	return nil, nil, nil
}

func (f *deleteFakeRepo) ListAll(ctx context.Context) ([]*aggregate.Blocked, error) {
	return nil, nil
}

func (f *deleteFakeRepo) BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error {
	return nil
}

var _ blockeddomain.BlockedRepository = (*deleteFakeRepo)(nil)

func TestDeleteBlockedHandler_Batch(t *testing.T) {
	t.Parallel()
	repo := &deleteFakeRepo{}
	rebuildCalled := false
	h := command.NewDeleteBlockedHandler(repo, func(ctx context.Context) { rebuildCalled = true })

	err := h.Handle(context.Background(), port.DeleteBlockedCommand{BlockedIDs: []uint{1, 2, 3}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.deletedIDs) != 3 || repo.deletedIDs[0] != 1 || repo.deletedIDs[2] != 3 {
		t.Fatalf("expected DeleteBatch with [1 2 3], got %v", repo.deletedIDs)
	}
	if !rebuildCalled {
		t.Fatal("expected rebuildNotify to be called")
	}
}

func TestDeleteBlockedHandler_EmptyIDs(t *testing.T) {
	t.Parallel()
	repo := &deleteFakeRepo{}
	h := command.NewDeleteBlockedHandler(repo, func(ctx context.Context) {})

	err := h.Handle(context.Background(), port.DeleteBlockedCommand{BlockedIDs: []uint{}})
	if err != nil {
		t.Fatalf("empty ids should be a no-op success, got error: %v", err)
	}
	if repo.deletedIDs != nil {
		t.Fatalf("expected no DeleteBatch call, got %v", repo.deletedIDs)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk go test -count=1 -run TestDeleteBlockedHandler ./test/unit/blocked_command/
```

Expected: 编译失败 —— `DeleteBatch` 未定义(fake repo 的 `DeleteBatch` 与接口不匹配、`port.DeleteBlockedCommand` 无 `BlockedIDs` 字段)。

- [ ] **Step 3: 修改 port 与 command**

`internal/application/blocked/port/handler.go` 中:

```go
type DeleteBlockedCommand struct {
	BlockedIDs []uint
}
```

`internal/application/blocked/command/delete_blocked.go` 全文替换为:

```go
package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
)

type deleteBlockedHandler struct {
	repo          blocked.BlockedRepository
	rebuildNotify func(ctx context.Context)
}

func NewDeleteBlockedHandler(repo blocked.BlockedRepository, rebuildNotify func(ctx context.Context)) port.DeleteBlockedHandler {
	return &deleteBlockedHandler{repo: repo, rebuildNotify: rebuildNotify}
}

func (h *deleteBlockedHandler) Handle(ctx context.Context, cmd port.DeleteBlockedCommand) error {
	// 空列表视为无操作（防御，调用侧已校验）
	if len(cmd.BlockedIDs) == 0 {
		return nil
	}
	err := h.repo.DeleteBatch(ctx, cmd.BlockedIDs)
	if err != nil {
		return err
	}
	h.rebuildNotify(ctx)
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk go test -count=1 -run TestDeleteBlockedHandler ./test/unit/blocked_command/
```

Expected: 仍编译失败 —— 接口 `blocked.BlockedRepository` 还没有 `DeleteBatch`。**这是预期中的失败,进入 Task 2 补齐接口后转绿。**

- [ ] **Step 5: Commit**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk git add internal/application/blocked/port/handler.go internal/application/blocked/command/delete_blocked.go test/unit/blocked_command/delete_blocked_test.go && rtk git commit -m "feat(blocked): delete command 支持批量 ids"
```

---

### Task 2: 后端批量删除 —— domain 接口与 repository 实现

**Files:**
- Modify: `internal/domain/blocked/repository.go`(接口加 `DeleteBatch`)
- Modify: `internal/infrastructure/repository/blocked_repository.go`(实现 `DeleteBatch`)

**Interfaces:**
- Consumes: `DeleteBlockedCommand{BlockedIDs []uint}`(Task 1)
- Produces: `BlockedRepository.DeleteBatch(ctx context.Context, ids []uint) error`

- [ ] **Step 1: 领域接口加方法**

`internal/domain/blocked/repository.go` 的 `BlockedRepository` 接口中,`Delete(ctx, id uint) error` 之后加:

```go
	DeleteBatch(ctx context.Context, ids []uint) error
```

- [ ] **Step 2: repository 实现**

`internal/infrastructure/repository/blocked_repository.go` 的 `Delete` 方法之后加(复用 DAO 现成的 `BatchDeleteByField` 批量软删,与 `user_repository.go:216` / `endpoint_repository.go:294` 同款):

```go
// DeleteBatch 批量软删敏感词（单条 UPDATE deleted_at，原子）
func (r *blockedRepository) DeleteBatch(ctx context.Context, ids []uint) error {
	db := r.db.WithContext(ctx)
	return r.dao.BatchDeleteByField(db, constant.FieldID, ids)
}
```

(`constant.FieldID` 已存在,值为 `"id"`;`baseDAO.BatchDeleteByField(db, whereField string, values any)` 在 `internal/infrastructure/database/dao/base.go:94` 定义,`values == nil` 时直接返回 nil,无需防御。)`blocked_repository.go` 需确保已 import `constant`。

- [ ] **Step 3: 运行 Task 1 测试 + 全量编译**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk go test -count=1 -run TestDeleteBlockedHandler ./test/unit/blocked_command/ && rtk go build ./...
```

Expected: 测试 PASS,`go build ./...` 无错误。

- [ ] **Step 4: Commit**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk git add internal/domain/blocked/repository.go internal/infrastructure/repository/blocked_repository.go && rtk git commit -m "feat(blocked): repository 支持批量删除"
```

---

### Task 3: 后端批量删除 —— DTO 与 handler

**Files:**
- Modify: `internal/dto/blocked.go`(`DeleteBlockedReq`、新增 `DeleteBlockedRsp`)
- Modify: `internal/handler/blocked.go`(`HandleDeleteBlocked`)

**Interfaces:**
- Consumes: `DeleteBlockedCommand{BlockedIDs []uint}`(Task 1);`parseCommaSeparatedIDs(s string) ([]uint, error)`(internal/handler/session.go,同包直接复用)
- Produces: `DELETE /api/v1/block?ids=1,2,3` → `{"deletedCount": N}`;`BlockedHandler.HandleDeleteBlocked` 返回 `*dto.HTTPResponse[*dto.DeleteBlockedRsp]`

- [ ] **Step 1: 改 DTO**

`internal/dto/blocked.go`:

```go
// DeleteBlockedReq 删除请求（支持逗号分隔批量）
type DeleteBlockedReq struct {
	IDs string `query:"ids" required:"true" minLength:"1" doc:"Blocked ID 列表，逗号分隔，如 1 或 1,2,3"`
}

// DeleteBlockedRsp 删除响应
type DeleteBlockedRsp struct {
	CommonRsp
	DeletedCount int `json:"deletedCount,omitempty" doc:"成功删除数量"`
}
```

删除旧的 `DeleteBlockedReq{ID uint}` 定义。

- [ ] **Step 2: 改 handler**

`internal/handler/blocked.go`:

- `BlockedHandler` 接口签名改为:

```go
	HandleDeleteBlocked(ctx context.Context, req *dto.DeleteBlockedReq) (*dto.HTTPResponse[*dto.DeleteBlockedRsp], error)
```

- `HandleDeleteBlocked` 实现替换为(对齐 `HandleDeleteSession` 风格):

```go
func (h *blockedHandler) HandleDeleteBlocked(ctx context.Context, req *dto.DeleteBlockedReq) (*dto.HTTPResponse[*dto.DeleteBlockedRsp], error) {
	rsp := &dto.DeleteBlockedRsp{}

	ids, parseErr := parseCommaSeparatedIDs(req.IDs)
	if parseErr != nil {
		return nil, apiutil.NewHumaBizError(ctx, parseErr, ierr.ErrValidation.BizError())
	}

	err := h.delete.Handle(ctx, port.DeleteBlockedCommand{BlockedIDs: ids})
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHandler] Delete blocked word failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	rsp.DeletedCount = len(ids)
	logger.WithCtx(ctx).Info("[BlockedHandler] Blocked word(s) deleted", zap.Int("total", len(ids)))

	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 3: 编译 + 跑既有单测**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk go build ./... && rtk go test -count=1 ./internal/... ./test/unit/...
```

Expected: 全绿(blocked e2e 离线自动 skip,不影响)。

- [ ] **Step 4: Commit**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk git add internal/dto/blocked.go internal/handler/blocked.go && rtk git commit -m "feat(blocked): DELETE /api/v1/block 支持 ids 批量删除"
```

---

### Task 4: 后端 e2e 更新 + 批量删除用例

**Files:**
- Modify: `test/e2e/blocked/blocked_test.go`

**Interfaces:**
- Consumes: `DELETE /api/v1/block?ids=`(Task 3)
- Produces: 批量删除 e2e 用例

- [ ] **Step 1: 更新 deleteBlockedWord helper**

`test/e2e/blocked/blocked_test.go` 中 helper 的 URL 从 `?id=%d` 改为 `?ids=%d`:

```go
	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, fmt.Sprintf("%s/api/v1/block?ids=%d", baseURL, id), http.NoBody)
```

- [ ] **Step 2: 新增批量删除 e2e 用例**(追加到文件末尾)

```go
// TestBlocked_BatchDelete 验证 DELETE /api/v1/block?ids=1,2,3 批量删除及 deletedCount 返回。
func TestBlocked_BatchDelete(t *testing.T) {
	baseURL, _, adminToken := mustBlockedE2EEnv(t)

	w1 := uniqueWord("batchdel")
	id1 := createBlockedWord(t, baseURL, adminToken, w1, "deny")
	id2 := createBlockedWord(t, baseURL, adminToken, uniqueWord("batchdel"), "allow")

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete,
		fmt.Sprintf("%s/api/v1/block?ids=%d,%d", baseURL, id1, id2), http.NoBody)
	if err != nil {
		t.Fatalf("failed to create batch delete request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send batch delete request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("batch delete status = %d, body: %s", resp.StatusCode, string(respBody))
	}

	var rsp struct {
		Data struct {
			DeletedCount int `json:"deletedCount"`
		} `json:"data"`
	}
	if err := sonic.Unmarshal(respBody, &rsp); err != nil {
		t.Fatalf("batch delete returned unexpected response: %s", string(respBody))
	}
	if rsp.Data.DeletedCount != 2 {
		t.Fatalf("expected deletedCount=2, got %d", rsp.Data.DeletedCount)
	}

	// 删除后不应再出现在列表中
	if _, ok := findBlockedID(t, baseURL, adminToken, w1); ok {
		t.Fatal("blocked word still exists after batch delete")
	}
}
```

(若文件中原有 `findBlockedID` 的语义为"存在返回 true",按现状使用;若返回 `(id, ok)` 则 `ok` 语义为找到,断言 `ok == false`。)

- [ ] **Step 3: 本地编译验证**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk go vet ./test/e2e/blocked/
```

Expected: 无错误(E2E 离线 skip,不实际执行)。

- [ ] **Step 4: Commit**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk git add test/e2e/blocked/blocked_test.go && rtk git commit -m "test(blocked): e2e 适配 ids 批量删除"
```

---

### Task 5: 前端 API 与类型

**Files:**
- Modify: `web/src/lib/api-client.ts`(`deleteBlocked` / `batchDeleteBlocked`)
- Modify: `web/src/lib/types.ts`(`DeleteBlockedRsp`)

**Interfaces:**
- Consumes: 后端 `DELETE /api/v1/block?ids=` 返回 `{"deletedCount": N}`(Task 3)
- Produces: `api.deleteBlocked(id: number): Promise<CommonRsp>`、`api.batchDeleteBlocked(ids: number[]): Promise<DeleteBlockedRsp>`

- [ ] **Step 1: types.ts 新增响应类型**

`web/src/lib/types.ts` 的 Blocked Words 段,`BlockedItem` 之后加:

```ts
export interface DeleteBlockedRsp extends CommonRsp {
  deletedCount?: number;
}
```

- [ ] **Step 2: api-client.ts 更新方法**

`web/src/lib/api-client.ts` Blocked Words 段,替换 `deleteBlocked` 并新增 `batchDeleteBlocked`:

```ts
  async deleteBlocked(id: number): Promise<DeleteBlockedRsp> {
    return this.request<DeleteBlockedRsp>(`/api/v1/block?ids=${id}`, { method: "DELETE" });
  }

  async batchDeleteBlocked(ids: number[]): Promise<DeleteBlockedRsp> {
    return this.request<DeleteBlockedRsp>(`/api/v1/block?ids=${ids.join(",")}`, {
      method: "DELETE",
    });
  }
```

- [ ] **Step 3: 类型检查**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12/web && rtk npx tsc --noEmit
```

Expected: 无类型错误。

- [ ] **Step 4: Commit**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk git add web/src/lib/api-client.ts web/src/lib/types.ts && rtk git commit -m "feat(web): blocked API 支持批量删除"
```

---

### Task 6: 前端页面重做 —— blocked/page.tsx

**Files:**
- Modify: `web/src/app/(dashboard)/blocked/page.tsx`(整页改造)

**Interfaces:**
- Consumes: `api.listBlocked` / `api.createBlocked` / `api.updateBlocked` / `api.deleteBlocked` / `api.batchDeleteBlocked`(Task 5);`DeleteConfirmDialog` / `useDeleteConfirm` / `SearchInput` / `PaginationBar` / `ListEmptyState` / `TableSkeleton` / `PageHeader`(既有组件,不变)
- Produces: 新页面交互(行内添加、点击徽章切换、勾选批量删除)

改造要点(全部落在本文件):

- [ ] **Step 1: 移除新增 Dialog 相关 state/代码**

删除: `dialogOpen`、`form`、`saving`、`handleCreate`、整个 `<Dialog>` 块、`RadioGroup` / `Label` / `Dialog*` imports、`emptyForm`、`actionOptions`、`Plus` 图标(PageHeader actions 不再放"添加"按钮)。

- [ ] **Step 2: ActionBadge 改为可点击按钮**

```tsx
function ActionBadge({
  action,
  t,
  onClick,
  disabled,
}: {
  action: BlockedAction;
  t: (key: string) => string;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={t("blocked.action_switch_hint")}
      className={cn(
        "inline-flex cursor-pointer items-center rounded-full border border-transparent px-2 py-0.5 text-[11px] font-medium transition-colors",
        "hover:ring-2 hover:ring-current/20 disabled:opacity-60 disabled:cursor-not-allowed",
        action === "deny"
          ? "bg-destructive/10 text-destructive hover:bg-destructive/20"
          : "bg-emerald-500/10 text-emerald-600 hover:bg-emerald-500/20 dark:text-emerald-400",
      )}
    >
      {action === "deny" ? t("blocked.action_deny") : t("blocked.action_allow")}
    </button>
  );
}
```

页面内新增 `togglingId` state 与 `handleToggleAction`(替换旧版,加禁用防抖):

```tsx
const [togglingId, setTogglingId] = useState<number | null>(null);

const handleToggleAction = useCallback(
  async (item: BlockedItem) => {
    if (togglingId !== null) return;
    const next: BlockedAction = item.action === "deny" ? "allow" : "deny";
    setTogglingId(item.id);
    try {
      await api.updateBlocked(item.id, { action: next });
      toast.success(t("blocked.action_updated"));
      fetchItems(persistedPage, persistedPageSize);
    } catch (err) {
      showErrorToast(err, { title: t("blocked.action_update_error") });
    } finally {
      setTogglingId(null);
    }
  },
  [togglingId, fetchItems, persistedPage, persistedPageSize, t],
);
```

表格/卡片中调用 `onClick={() => handleToggleAction(item)}` / `disabled={togglingId !== null}`。**移除 `RefreshCw` 按钮**(切换入口改为徽章本身)。

- [ ] **Step 3: 行内快速添加**

在 `<SearchInput>` 所在容器改为两输入框并排(`flex flex-col gap-2 sm:flex-row sm:items-center`):

```tsx
const [inlineWord, setInlineWord] = useState("");
const [adding, setAdding] = useState(false);

const handleInlineAdd = useCallback(async () => {
  const word = inlineWord.trim();
  if (!word || adding) return;
  setAdding(true);
  try {
    await api.createBlocked({ word, action: "deny" });
    toast.success(t("blocked.created_success"));
    setInlineWord("");
    fetchItems(persistedPage, persistedPageSize);
  } catch (err) {
    showErrorToast(err, { title: t("blocked.create_error") });
  } finally {
    setAdding(false);
  }
}, [inlineWord, adding, fetchItems, persistedPage, persistedPageSize, t]);
```

JSX(桌面并排、移动堆叠):

```tsx
<div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center">
  <SearchInput
    className="sm:max-w-xs"
    placeholder={t("blocked.search_placeholder")}
    value={searchQuery}
    onChange={setSearchQuery}
    onSearch={handleSearch}
  />
  <Input
    className="sm:max-w-xs"
    placeholder={t("blocked.inline_add_placeholder")}
    value={inlineWord}
    onChange={(e) => setInlineWord(e.target.value)}
    onKeyDown={(e) => {
      if (e.key === "Enter") handleInlineAdd();
    }}
    disabled={adding}
  />
</div>
```

(若 `SearchInput` 不接受 `className`,用外层 `div` 包 `max-w`。)

- [ ] **Step 4: 批量删除(对齐 sessions)**

新增 state 与 handler:

```tsx
const [selected, setSelected] = useState<Set<number>>(new Set());
const [batchDeleting, setBatchDeleting] = useState(false);
const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false);

const toggleSelect = (id: number, e: React.MouseEvent) => {
  e.stopPropagation();
  setSelected((prev) => {
    const next = new Set(prev);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    return next;
  });
};

const toggleSelectAll = () => {
  if (selected.size === items.length) setSelected(new Set());
  else setSelected(new Set(items.map((i) => i.id)));
};

const handleBatchDelete = async () => {
  if (selected.size === 0) return;
  setBatchDeleting(true);
  try {
    const ids = Array.from(selected);
    const rsp = await api.batchDeleteBlocked(ids);
    toast.success(
      t("blocked.batch_delete_success").replace("{count}", String(rsp.deletedCount ?? ids.length)),
    );
    setSelected(new Set());
    fetchItems(persistedPage, persistedPageSize);
  } catch (err) {
    showErrorToast(err, { title: t("blocked.batch_delete_error") });
  } finally {
    setBatchDeleting(false);
    setBatchDeleteConfirmOpen(false);
  }
};
```

`fetchItems` 成功后清空选中(翻页/搜索/刷新时):

```tsx
setSelected(new Set());
```

表格结构改动(桌面):

- 表头首列加全选框(复制 sessions 的 `role="checkbox"` div 模式,`onClick={toggleSelectAll}`,勾选态 `selected.size === items.length`);
- 表格首列加行勾选框(复制 sessions 行勾选 div,`onClick={(e) => toggleSelect(id, e)}`);
- 工具栏(搜索/添加输入框所在行右侧)加批量按钮,选中 >0 时显示:

```tsx
{selected.size > 0 && (
  <Button
    variant="destructive"
    size="sm"
    onClick={() => setBatchDeleteConfirmOpen(true)}
    className="gap-1.5"
  >
    <Trash2 className="size-3.5" />
    {t("common.delete")} {selected.size}
  </Button>
)}
```

移动端卡片:卡片首行左侧加行勾选框(sessions 移动端模式:`mt-0.5 flex size-4 shrink-0 ...`)。

页面末尾追加批量确认对话框(复用 `DeleteConfirmDialog`,与单删并列;props 已核对:`open/onOpenChange/title/description/confirmLabel/loading/onConfirm`):

```tsx
<DeleteConfirmDialog
  open={batchDeleteConfirmOpen}
  onOpenChange={setBatchDeleteConfirmOpen}
  title={t("common.are_you_sure")}
  description={t("blocked.batch_delete_confirm").replace("{count}", String(selected.size))}
  confirmLabel={t("common.delete")}
  loading={batchDeleting}
  onConfirm={handleBatchDelete}
/>
```

imports 新增: `Check`、`Trash2`(lucide);移除 `Plus`、`RefreshCw`、`RadioGroup`、`Label`、`Dialog*`。

- [ ] **Step 5: 运行 lint 与 build**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12/web && rtk npm run lint && rtk npm run build
```

Expected: lint 无 error;build 成功产出 `out/`。

- [ ] **Step 6: Commit**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk git add web/src/app/\(dashboard\)/blocked/page.tsx && rtk git commit -m "feat(web): blocked 页面重做列名与交互"
```

---

### Task 7: 三语 locale 文案

**Files:**
- Modify: `web/src/locales/zh.json`
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/ja.json`

**Interfaces:**
- Consumes: Task 6 中新增的 key: `blocked.inline_add_placeholder`、`blocked.action_switch_hint`、`blocked.batch_delete_confirm`、`blocked.batch_delete_success`、`blocked.batch_delete_error`
- Produces: 无(文案终态)

- [ ] **Step 1: zh.json**

`blocked.*` 段:

- `"blocked.word"`: `"词汇"` → `"敏感词"`
- `"blocked.action"`: `"动作"` → `"处理动作"`
- 删除不再使用的 key:`"blocked.create"`、`"blocked.create_placeholder"`(确认页面无引用后删)
- 新增:

```json
  "blocked.inline_add_placeholder": "输入敏感词，回车添加（默认拦截）",
  "blocked.action_switch_hint": "点击切换为拦截/放行",
  "blocked.batch_delete_confirm": "确定要删除选中的 {count} 个拦截词吗？",
  "blocked.batch_delete_success": "已删除 {count} 个拦截词",
  "blocked.batch_delete_error": "批量删除失败"
```

- [ ] **Step 2: en.json**

- `"blocked.word"`: `"Word"` → `"Sensitive Word"`
- `"blocked.action"`: `"Action"` → `"Handle Action"`
- 删除 `"blocked.create"`、`"blocked.create_placeholder"`
- 新增:

```json
  "blocked.inline_add_placeholder": "Enter a sensitive word, press Enter to add (blocks by default)",
  "blocked.action_switch_hint": "Click to switch between block/allow",
  "blocked.batch_delete_confirm": "Delete the selected {count} blocked words?",
  "blocked.batch_delete_success": "Deleted {count} blocked words",
  "blocked.batch_delete_error": "Failed to batch delete"
```

- [ ] **Step 3: ja.json**

- `"blocked.word"`: `"ワード"` → `"機密ワード"`
- `"blocked.action"`: `"動作"` → `"処理動作"`
- 删除 `"blocked.create"`、`"blocked.create_placeholder"`
- 新增:

```json
  "blocked.inline_add_placeholder": "機密ワードを入力して Enter で追加（デフォルトはブロック）",
  "blocked.action_switch_hint": "クリックでブロック/許可を切り替え",
  "blocked.batch_delete_confirm": "選択した {count} 件のブロックワードを削除しますか？",
  "blocked.batch_delete_success": "{count} 件のブロックワードを削除しました",
  "blocked.batch_delete_error": "一括削除に失敗しました"
```

- [ ] **Step 4: 校验 i18n 键完整性 + build**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12/web && rtk npm run build
```

Expected: 构建成功(若项目有 locale key 完整性检查脚本则一并运行;无则 build 通过即可)。

- [ ] **Step 5: Commit**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk git add web/src/locales/zh.json web/src/locales/en.json web/src/locales/ja.json && rtk git commit -m "feat(web): blocked 三语文案更新"
```

---

### Task 8: 整体验证

- [ ] **Step 1: 后端全量 lint + 测试**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk make lint && rtk go test -count=1 ./internal/... ./test/unit/...
```

Expected: lint 通过,测试全绿。

- [ ] **Step 2: 前端 lint + build**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12/web && rtk npm run lint && rtk npm run build
```

Expected: 通过。

- [ ] **Step 3: Chrome MCP 浏览器验证**

本地起后端(`go run ./cmd/server server start --host localhost --port 8080`)与前端(`npm run dev`, `NEXT_PUBLIC_API_BASE_URL=http://localhost:8080`),用 Chrome MCP 打开 `http://localhost:3000/web` 登录后进入 Blocked Words 页,逐项验证:

1. 列名为「敏感词 / 处理动作 / 命中次数 / 创建时间 / 操作」;
2. 行内输入框输入词 + 回车 → toast 成功,列表新增 deny 徽章行;
3. 点击 deny 徽章 → 变为 allow;再点回 deny;
4. 勾选两行 → 「删除 2」按钮出现 → 确认 → 列表刷新、选中清空;
5. 单行删除按钮 + 确认框仍可用;
6. 移动端视口(375px)下:添加输入框、勾选框、徽章切换可用。

- [ ] **Step 4: 提交收尾**

```bash
cd .worktrees/feature-blocked-words-redesign-2026-08-12 && rtk git status && rtk git log --oneline -8
```

Expected: 工作区干净,8 个左右 feat/test commit 按序排列。

---

## Self-Review 记录

- **Spec 覆盖**:列名(词汇→敏感词、动作→处理动作,创建时间保留)→ Task 6/7;点击徽章切换 → Task 6 Step 2;行内添加默认 deny → Task 6 Step 3;单删保留 → Task 6 Step 4(deleteConfirm 不变);批量删除 sessions 样式 → Task 6 Step 4;后端 ids 批量 → Task 1/2/3;e2e → Task 4;前端 API → Task 5;三语文案 → Task 7;验证 → Task 8。全量覆盖,无缺口。
- **占位符扫描**:无 TBD/TODO;所有代码步骤含完整代码。
- **类型一致性**:`DeleteBlockedCommand{BlockedIDs []uint}`、`DeleteBlockedRsp{DeletedCount int}`、`api.batchDeleteBlocked(ids: number[]): Promise<DeleteBlockedRsp>`、locale key `blocked.inline_add_placeholder` / `action_switch_hint` / `batch_delete_*` 在 Task 5→6→7 间一致;`parseCommaSeparatedIDs` 为既有函数,签名 `(s string) ([]uint, error)`。
