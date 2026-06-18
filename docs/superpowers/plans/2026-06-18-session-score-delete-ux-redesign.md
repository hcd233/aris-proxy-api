# Session 评分与删除交互体验重设计 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重设计 session 列表/详情页的评分与删除交互，消除内联确认气泡导致的布局挤压，统一两页 UI 组件。

**Architecture:** 新建统一 `ScoreDots` 组件（固定容器宽度 + `×` 绝对定位，圆点零位移），替换列表页和详情页的评分逻辑；详情页删除确认从 absolute 小气泡改为 AlertDialog，与列表页统一。前端无测试框架，验证用 `npm run lint && npm run build`。

**Tech Stack:** Next.js 16 + React 19 + TypeScript + Tailwind v4 + shadcn/ui (@base-ui/react) + lucide-react + sonner

**Spec:** `docs/superpowers/specs/2026-06-18-session-score-delete-ux-redesign.md`

---

## 文件结构

| 文件 | 责任 | 操作 |
|------|------|------|
| `web/src/components/session-detail/score-dots.tsx` | 统一评分组件：5 圆点 + hover 预览 + 点击即提交 + `×` 清除（固定宽度零位移） | **新建** |
| `web/src/app/(dashboard)/sessions/page.tsx` | 列表页：移除 `scoreConfirm` state 和 Yes/No 气泡 JSX，改用 `ScoreDots`；Score 列宽改 `w-[160px]` | 修改 |
| `web/src/components/session-detail/session-detail-client.tsx` | 详情页：改用 `ScoreDots`；移除 absolute 删除浮层，改用 `AlertDialog` | 修改 |
| `web/src/components/session-detail/score-dot-input.tsx` | 旧评分输入组件 | **删除** |
| `web/src/components/session-detail/score-stars.tsx` | 旧评分展示组件 | **删除** |

---

### Task 1: 新建 ScoreDots 统一评分组件

**Files:**
- Create: `web/src/components/session-detail/score-dots.tsx`

- [ ] **Step 1: 创建 ScoreDots 组件**

创建 `web/src/components/session-detail/score-dots.tsx`：

```tsx
"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";

interface ScoreDotsProps {
  score: number | undefined;
  scoring: boolean;
  onScore: (value: number) => void;
  onClear: () => void;
  size?: number;
}

export function ScoreDots({
  score,
  scoring,
  onScore,
  onClear,
  size = 16,
}: ScoreDotsProps) {
  const [hover, setHover] = useState<number | null>(null);

  const gap = Math.round(size * 0.5);
  const clearBtn = Math.round(size * 1.25);
  const reserved = clearBtn + gap + 4;
  const dotsWidth = 5 * size + 4 * gap;
  const containerWidth = dotsWidth + reserved;

  const displayScore = hover != null ? hover : score;
  const isRated = score != null;

  return (
    <span
      role="group"
      aria-label={isRated ? `Rated ${score} out of 5` : "Session rating"}
      className="relative inline-flex items-center"
      style={{ width: `${containerWidth}px` }}
      onMouseLeave={() => setHover(null)}
    >
      <span className="inline-flex items-center" style={{ gap: `${gap}px` }}>
        {[1, 2, 3, 4, 5].map((v) => (
          <button
            key={v}
            type="button"
            disabled={scoring}
            onClick={(e) => {
              e.stopPropagation();
              onScore(v);
            }}
            onMouseEnter={() => setHover(v)}
            aria-label={`Rate ${v}`}
            className={cn(
              "rounded-full transition-colors duration-150 disabled:opacity-30",
              displayScore != null && v <= displayScore
                ? "bg-primary"
                : "bg-muted-foreground/30 hover:bg-primary",
            )}
            style={{
              width: `${size}px`,
              height: `${size}px`,
              padding: `${Math.round(size * 0.25)}px`,
              backgroundClip: "content-box",
            }}
          />
        ))}
      </span>

      {isRated && (
        <button
          type="button"
          onClick={onClear}
          disabled={scoring}
          aria-label="Remove rating"
          className="absolute rounded text-muted-foreground/40 transition-colors hover:text-destructive disabled:opacity-30"
          style={{
            right: "4px",
            top: "50%",
            transform: "translateY(-50%)",
            width: `${clearBtn}px`,
            height: `${clearBtn}px`,
            fontSize: `${Math.round(size * 0.9)}px`,
            lineHeight: 1,
            display: "inline-flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          ×
        </button>
      )}
    </span>
  );
}
```

> **布局说明**: 容器 `position: relative` 固定宽度 = 圆点组宽度 + 右侧预留空间。圆点组在文档流中居左，`×` 用 `absolute right: 4px` 浮在预留空间。未评分时不渲染 `×`，但容器宽度不变 → 圆点零位移。圆点用 `padding` + `backgroundClip: content-box` 扩大命中区，实际视觉尺寸仍是 `size`。

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npx eslint src/components/session-detail/score-dots.tsx`
Expected: 无错误

- [ ] **Step 3: 验证类型检查通过**

Run: `cd web && npx tsc --noEmit`
Expected: 无错误

- [ ] **Step 4: Commit**

```bash
git add web/src/components/session-detail/score-dots.tsx
git commit -m "feat(web): add unified ScoreDots component with fixed-width layout"
```

---

### Task 2: 列表页集成 ScoreDots + 移除内联确认气泡

**Files:**
- Modify: `web/src/app/(dashboard)/sessions/page.tsx`

- [ ] **Step 1: 更新 import**

在 `web/src/app/(dashboard)/sessions/page.tsx` 中，将 `ScoreDotInput` 的 import 替换为 `ScoreDots`。

找到（约第 31 行）：
```tsx
import { ScoreDotInput } from "@/components/session-detail/score-dot-input";
```

替换为：
```tsx
import { ScoreDots } from "@/components/session-detail/score-dots";
```

- [ ] **Step 2: 移除 scoreConfirm state**

找到（约第 88 行）：
```tsx
  const [scoreConfirm, setScoreConfirm] = useState<{ id: number; value: number } | null>(null);
```

删除该行。

- [ ] **Step 3: 简化 handleScoreSession**

找到 `handleScoreSession` 函数（约第 269-285 行）：
```tsx
  const handleScoreSession = async (e: React.MouseEvent, sessionId: number, score: number) => {
    e.stopPropagation();
    if (scoring !== null) return;
    setScoring(sessionId);
    setScoreConfirm(null);
    try {
      await api.scoreSession({ sessionId, score });
      setSessions((prev) =>
        prev.map((s) => (s.id === sessionId ? { ...s, score } : s)),
      );
      toast.success("Scored");
    } catch {
      toast.error("Failed to score");
    } finally {
      setScoring(null);
    }
  };
```

替换为（移除 `e` 参数和 `setScoreConfirm(null)`）：
```tsx
  const handleScoreSession = async (sessionId: number, score: number) => {
    if (scoring !== null) return;
    setScoring(sessionId);
    try {
      await api.scoreSession({ sessionId, score });
      setSessions((prev) =>
        prev.map((s) => (s.id === sessionId ? { ...s, score } : s)),
      );
      toast.success("Scored");
    } catch {
      toast.error("Failed to score");
    } finally {
      setScoring(null);
    }
  };
```

- [ ] **Step 4: 替换移动端卡片的评分 JSX**

找到移动端卡片中的评分渲染块（约第 451-497 行），从：
```tsx
                          {s.score != null ? (
                            <div className="flex items-center gap-1">
                              <div className="flex items-center gap-0.5">
                                {[1, 2, 3, 4, 5].map((v) => (
                                  <span
                                    key={v}
                                    className={`inline-block size-2 rounded-full ${v <= (s.score ?? 0) ? "bg-primary" : "bg-muted-foreground/30"}`}
                                    aria-hidden
                                  />
                                ))}
                              </div>
                              <button
                                type="button"
                                disabled={scoring === s.id}
                                onClick={(e) => handleDeleteScore(e, s.id)}
                                className="text-xs text-muted-foreground/30 hover:text-destructive disabled:opacity-30"
                                aria-label="Remove score"
                              >
                                ×
                              </button>
                            </div>
                          ) : scoreConfirm?.id === s.id ? (
                            <div className="flex items-center gap-1 rounded border border-border bg-secondary/50 px-1.5 py-0.5">
                              <span className="text-xs text-muted-foreground">{scoreConfirm.value}?</span>
                              <button
                                type="button"
                                onClick={(e) => handleScoreSession(e, s.id, scoreConfirm.value)}
                                disabled={scoring === s.id}
                                className="rounded px-1 text-xs font-medium text-foreground hover:text-green-600 disabled:opacity-50"
                              >
                                Yes
                              </button>
                              <button
                                type="button"
                                onClick={(e) => { e.stopPropagation(); setScoreConfirm(null); }}
                                className="rounded px-1 text-xs text-muted-foreground hover:text-foreground"
                              >
                                No
                              </button>
                            </div>
                          ) : (
                            <ScoreDotInput
                              onPick={(v) => setScoreConfirm({ id: s.id, value: v })}
                              disabled={scoring === s.id}
                              size={8}
                            />
                          )}
```

替换为：
```tsx
                          <ScoreDots
                            score={s.score}
                            scoring={scoring === s.id}
                            onScore={(v) => handleScoreSession(s.id, v)}
                            onClear={() => handleDeleteScore({ stopPropagation: () => {} } as React.MouseEvent, s.id)}
                            size={isMobile ? 20 : 16}
                          />
```

- [ ] **Step 5: 更新 handleDeleteScore 签名**

找到 `handleDeleteScore` 函数（约第 287-302 行）：
```tsx
  const handleDeleteScore = async (e: React.MouseEvent, sessionId: number) => {
    e.stopPropagation();
    if (scoring !== null) return;
    setScoring(sessionId);
    try {
      await api.deleteScoreSession(sessionId);
      setSessions((prev) =>
        prev.map((s) => (s.id === sessionId ? { ...s, score: undefined } : s)),
      );
      toast.success("Score removed");
    } catch {
      toast.error("Failed to remove score");
    } finally {
      setScoring(null);
    }
  };
```

替换为（移除 `e` 参数，ScoreDots 的 onClear 不传 event）：
```tsx
  const handleDeleteScore = async (sessionId: number) => {
    if (scoring !== null) return;
    setScoring(sessionId);
    try {
      await api.deleteScoreSession(sessionId);
      setSessions((prev) =>
        prev.map((s) => (s.id === sessionId ? { ...s, score: undefined } : s)),
      );
      toast.success("Score removed");
    } catch {
      toast.error("Failed to remove score");
    } finally {
      setScoring(null);
    }
  };
```

然后修正 Step 4 中的 `onClear` 调用，从：
```tsx
                            onClear={() => handleDeleteScore({ stopPropagation: () => {} } as React.MouseEvent, s.id)}
```
改为：
```tsx
                            onClear={() => handleDeleteScore(s.id)}
```

- [ ] **Step 6: 替换桌面端表格的评分 JSX**

找到桌面端表格 Score 单元格（约第 608-657 行），从：
```tsx
                        <TableCell>
                          <div className="flex justify-center">
                            {s.score != null ? (
                              <div className="flex items-center gap-1">
                                <div className="flex items-center gap-0.5">
                                  {[1, 2, 3, 4, 5].map((v) => (
                                    <span
                                      key={v}
                                      className={`inline-block size-2 rounded-full ${v <= (s.score ?? 0) ? "bg-primary" : "bg-muted-foreground/30"}`}
                                      aria-hidden
                                    />
                                  ))}
                                </div>
                                <button
                                  type="button"
                                  disabled={scoring === s.id}
                                  onClick={(e) => handleDeleteScore(e, s.id)}
                                  className="text-xs text-muted-foreground/20 hover:text-destructive disabled:opacity-30"
                                  aria-label="Remove score"
                                >
                                  ×
                                </button>
                              </div>
                            ) : scoreConfirm?.id === s.id ? (
                              <div className="flex items-center gap-1 rounded border border-border bg-secondary/50 px-1.5 py-0.5">
                                <span className="text-xs text-muted-foreground">{scoreConfirm.value}?</span>
                                <button
                                  type="button"
                                  onClick={(e) => handleScoreSession(e, s.id, scoreConfirm.value)}
                                  disabled={scoring === s.id}
                                  className="rounded px-1 text-xs font-medium text-foreground hover:text-green-600 disabled:opacity-50"
                                >
                                  Yes
                                </button>
                                <button
                                  type="button"
                                  onClick={(e) => { e.stopPropagation(); setScoreConfirm(null); }}
                                  className="rounded px-1 text-xs text-muted-foreground hover:text-foreground"
                                >
                                  No
                                </button>
                              </div>
                            ) : (
                              <ScoreDotInput
                                onPick={(v) => setScoreConfirm({ id: s.id, value: v })}
                                disabled={scoring === s.id}
                                size={8}
                              />
                            )}
                          </div>
                        </TableCell>
```

替换为：
```tsx
                        <TableCell className="w-[160px]">
                          <div className="flex justify-center">
                            <ScoreDots
                              score={s.score}
                              scoring={scoring === s.id}
                              onScore={(v) => handleScoreSession(s.id, v)}
                              onClear={() => handleDeleteScore(s.id)}
                              size={16}
                            />
                          </div>
                        </TableCell>
```

- [ ] **Step 7: 更新表头 Score 列宽**

找到表头 Score 列（约第 555 行）：
```tsx
                    <TableHead className="w-[80px] text-center">Score</TableHead>
```

替换为：
```tsx
                    <TableHead className="w-[160px] text-center">Score</TableHead>
```

- [ ] **Step 8: 验证 lint 通过**

Run: `cd web && npx eslint src/app/\(dashboard\)/sessions/page.tsx`
Expected: 无错误

- [ ] **Step 9: 验证类型检查通过**

Run: `cd web && npx tsc --noEmit`
Expected: 无错误

- [ ] **Step 10: Commit**

```bash
git add web/src/app/\(dashboard\)/sessions/page.tsx
git commit -m "refactor(web): integrate ScoreDots into sessions list, remove inline confirm bubble"
```

---

### Task 3: 详情页集成 ScoreDots + 删除改用 AlertDialog

**Files:**
- Modify: `web/src/components/session-detail/session-detail-client.tsx`

- [ ] **Step 1: 更新 import**

在 `web/src/components/session-detail/session-detail-client.tsx` 中：

找到（约第 32 行）：
```tsx
import { ScoreStars } from "./score-stars";
```

替换为：
```tsx
import { ScoreDots } from "./score-dots";
```

在文件顶部 import 区域添加 AlertDialog 组件（紧跟其他 ui import 之后，约第 30 行 Sheet import 块之后）：
```tsx
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { AlertTriangle } from "lucide-react";
```

> 注意：`AlertTriangle` 需要从 `lucide-react` 导入，检查文件顶部已有的 lucide-react import 块（约第 6-12 行），将 `AlertTriangle` 加入其中，而非单独再写一行。最终 lucide-react import 应为：
> ```tsx
> import {
>   AlertTriangle,
>   ArrowLeft,
>   History,
>   MessagesSquare,
>   Share2,
>   Trash2,
>   Wrench,
> } from "lucide-react";
> ```

- [ ] **Step 2: 替换 header 中的 ScoreStars 为 ScoreDots**

找到 header 中的评分组件（约第 340-346 行）：
```tsx
      <ScoreStars
        score={score}
        scoring={scoring}
        onScore={handleScore}
        onClear={handleDeleteScore}
        size={isMobile ? 9 : 11}
      />
```

替换为：
```tsx
      <ScoreDots
        score={score}
        scoring={scoring}
        onScore={handleScore}
        onClear={handleDeleteScore}
        size={isMobile ? 20 : 16}
      />
```

- [ ] **Step 3: 移除 absolute 删除浮层，改为打开 AlertDialog**

找到 header 中的删除按钮及其 absolute 浮层（约第 364-396 行）：
```tsx
      <div className="relative">
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => setDeleteConfirmOpen(true)}
          className="size-10 text-foreground/70 hover:text-destructive"
          aria-label="Delete session"
          title="Delete session"
        >
          <Trash2 className="size-5" />
        </Button>
        {deleteConfirmOpen && (
          <div className="absolute right-0 top-full z-50 mt-1 inline-flex items-center gap-1 rounded-md border border-border bg-popover p-1 shadow-md">
            <span className="px-1 text-xs text-muted-foreground">Delete?</span>
            <button
              type="button"
              onClick={handleDelete}
              disabled={deleting}
              className="rounded px-1.5 py-0.5 text-xs font-medium text-destructive transition-colors hover:bg-destructive/10 disabled:opacity-50"
            >
              {deleting ? "..." : "Yes"}
            </button>
            <button
              type="button"
              onClick={() => setDeleteConfirmOpen(false)}
              disabled={deleting}
              className="rounded px-1.5 py-0.5 text-xs text-muted-foreground transition-colors hover:text-foreground disabled:opacity-50"
            >
              No
            </button>
          </div>
        )}
      </div>
```

替换为（仅保留按钮，浮层移除）：
```tsx
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={() => setDeleteConfirmOpen(true)}
        className="size-10 text-foreground/70 hover:text-destructive"
        aria-label="Delete session"
        title="Delete session"
      >
        <Trash2 className="size-5" />
      </Button>
```

- [ ] **Step 4: 在组件 return 的末尾添加 AlertDialog**

找到组件 return 末尾的 ShareDialog（约第 487-492 行）：
```tsx
      <ShareDialog
        sessionId={metadata.id}
        existingShareID={metadata.shareID}
        open={shareOpen}
        onOpenChange={setShareOpen}
      />

    </>
```

在 ShareDialog 之后、`</>` 之前插入 AlertDialog：
```tsx
      <ShareDialog
        sessionId={metadata.id}
        existingShareID={metadata.shareID}
        open={shareOpen}
        onOpenChange={setShareOpen}
      />

      <AlertDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="flex items-center gap-2">
              <AlertTriangle className="size-5 text-destructive" />
              Delete session?
            </AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete{" "}
              <strong>Session #{metadata.id}</strong> and all its messages. This
              action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

    </>
```

- [ ] **Step 5: 验证 lint 通过**

Run: `cd web && npx eslint src/components/session-detail/session-detail-client.tsx`
Expected: 无错误

- [ ] **Step 6: 验证类型检查通过**

Run: `cd web && npx tsc --noEmit`
Expected: 无错误

- [ ] **Step 7: Commit**

```bash
git add web/src/components/session-detail/session-detail-client.tsx
git commit -m "refactor(web): use ScoreDots and AlertDialog in session detail"
```

---

### Task 4: 删除旧的评分组件文件

**Files:**
- Delete: `web/src/components/session-detail/score-dot-input.tsx`
- Delete: `web/src/components/session-detail/score-stars.tsx`

- [ ] **Step 1: 确认无其他引用**

Run: `cd web && grep -r "score-dot-input\|score-stars\|ScoreDotInput\|ScoreStars" src/ --include="*.tsx" --include="*.ts"`
Expected: 无输出（Task 2 和 Task 3 已移除所有引用）

> 如果有输出，说明还有遗漏的引用，需先修复再继续。

- [ ] **Step 2: 删除文件**

```bash
rm web/src/components/session-detail/score-dot-input.tsx
rm web/src/components/session-detail/score-stars.tsx
```

- [ ] **Step 3: 验证类型检查通过**

Run: `cd web && npx tsc --noEmit`
Expected: 无错误

- [ ] **Step 4: Commit**

```bash
git add -A web/src/components/session-detail/
git commit -m "refactor(web): remove obsolete ScoreDotInput and ScoreStars components"
```

---

### Task 5: 全量验证 + 浏览器交互验证

- [ ] **Step 1: 全量 lint**

Run: `cd web && npm run lint`
Expected: 无错误

- [ ] **Step 2: 全量构建**

Run: `cd web && npm run build`
Expected: 构建成功，无类型错误

- [ ] **Step 3: 浏览器验证评分交互**

启动本地环境：
```bash
go run main.go server start --host localhost --port 8080
```
另开终端：
```bash
cd web && NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npm run dev
```
浏览器访问 `http://localhost:3000/web`，登录后进入 Sessions 页面，验证：

1. **列表页未评分**：点击圆点 → 立即提交，无确认气泡，圆点位置不跳动
2. **列表页已评分**：显示填充圆点 + `×`；点击新分值 → 覆盖成功；点 `×` → 清除成功；圆点位置在未评分/已评分切换时零位移
3. **列表页删除**：点垃圾桶 → AlertDialog 居中弹窗 → Cancel 关闭 / Delete 删除成功
4. **详情页评分**：header 中圆点交互同列表页
5. **详情页删除**：点垃圾桶 → AlertDialog 居中弹窗（非小气泡）→ Delete 删除后返回列表
6. **移动端**：缩小窗口到移动端尺寸，圆点 20px，触控目标达标，AlertDialog 居中可用

- [ ] **Step 4: Commit 验证通过标记**

```bash
git commit --allow-empty -m "chore(web): verify score and delete UX redesign"
```

---

## Self-Review

**1. Spec coverage:**
- 评分点击即提交无确认气泡 → Task 2 Step 3-6（移除 scoreConfirm + Yes/No 气泡，改用 ScoreDots）
- 已评分可覆盖/点 × 清除 → Task 1 ScoreDots 组件行为
- 圆点零位移 → Task 1 固定容器宽度 + × 绝对定位
- 删除两页统一 AlertDialog → Task 3 Step 3-4（详情页改 AlertDialog）
- 组件统一 → Task 1 新建 ScoreDots，Task 2-3 集成，Task 4 删除旧组件
- 无障碍 aria-label → Task 1 ScoreDots 实现
- 触控目标 → Task 1 padding + backgroundClip 扩大命中区
- 响应式圆点尺寸 → Task 2-3 size 参数（移动端 20 / 桌面 16）
- 表格 Score 列宽 160px → Task 2 Step 6-7

**2. Placeholder scan:** 无 TBD/TODO，所有步骤含完整代码。

**3. Type consistency:**
- `ScoreDotsProps` 在 Task 1 定义，Task 2-3 使用一致（score/scoring/onScore/onClear/size）
- `handleScoreSession` 签名 Task 2 Step 3 改为 `(sessionId, score)`，Step 4/6 调用一致
- `handleDeleteScore` 签名 Task 2 Step 5 改为 `(sessionId)`，Step 4/6 调用一致
- 详情页 `handleScore`/`handleDeleteScore` 签名未变（已是 `(value)` / `()`），Task 3 Step 2 传参一致
