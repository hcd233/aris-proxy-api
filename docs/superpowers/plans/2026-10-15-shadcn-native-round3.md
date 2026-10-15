# Web 端 shadcn 原生组件替换（第三轮）实施计划

> **执行方式**：按用户规则不使用子 Agent，用 superpowers:executing-plans 在当前会话内逐任务执行。步骤用 `- [ ]` 跟踪。

**目标**：把 `web/src` 里手写的反馈、表单、列表、浮层、筛选、底部弹层和聊天 UI 换成 shadcn 原生组件，外观跟随 shadcn 默认。

**做法**：
- 新组件从 `shadcn add --view` 的输出提取源码落盘，不让 CLI 自动装依赖。
- 每个任务先在源码契约测试里加断言（先失败），再替换，最后跑 lint / test / format / build 并单独提交。
- 动效按 `data-slot` 写进两份语法层 CSS。

**技术栈**：Next.js 16（静态导出）、React 19、Tailwind v4、shadcn `base-nova`、`@base-ui/react` 1.9.0、vitest。

设计依据：`docs/superpowers/specs/2026-10-15-shadcn-native-round3-design.md`。

## 全局约束

- 工作目录：`/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/shadcn-native-round3`。下文命令中的 `web/` 均相对此目录。
- 不覆盖 `components/ui/button.tsx`、`separator.tsx`、`label.tsx`。
- 新组件的 `import { cn } from "cn"` 一律改为 `import { cn } from "@/lib/utils"`；不引入 npm 包 `cn`。
- 唯一新增运行时依赖：`@shadcn/react`，精确版本 `0.3.1`。
- 新组件删除 tw-animate 进出场类：`animate-in`、`animate-out`、`fade-in-0`、`fade-out-0`、`zoom-in-95`、`zoom-out-95`、`slide-in-from-*`、`slide-out-to-*`。
- 外观只用组件自带变体。className 只允许两类：
  - 布局（宽度、间距、栅格、截断）；
  - 语义色 Badge（trigger 的 deny / omit / capture、upstream 告警）。
- `truncate` / `line-clamp-1` 必须在 `TooltipTrigger` 渲染子树内（eslint `truncate-requires-tooltip` 为 error 级）。
- i18n 预留不变：Button 的 `min-w-*`；对话框横向工具行加 `flex-wrap`。
- 业务逻辑零改动：API 调用、筛选 DSL、权限判定、playground 会话逻辑不改。
- 每个任务的验收命令（下文称「标准验收」）：

  ```sh
  cd web && npm run lint && npm run test && npm run format:check && npx next build --webpack
  ```

  - worktree 软链了 node_modules，Turbopack 会拒绝，所以必须加 `--webpack`；
  - 构建完删除 `web/.next` 与 `web/out`（用 `git clean -fdX web/.next web/out`；若触发删除审批，留到收尾统一清理）。
- 提交：pre-commit hook 会对暂存的 web 文件自动跑 prettier 并重新暂存。本轮不改 Go 文件，hook 会跳过 Go 检查。每个任务只 `git add` 本任务文件。

## 文件结构

| 文件 | 职责 | 任务 |
|---|---|---|
| `web/src/components/shadcn-native.test.ts`（新） | 源码契约测试：断言手写模式已清除、新组件已规范化 | 1 起逐任务追加 |
| `web/src/components/ui/{spinner,progress,alert,field}.tsx`（新） | 反馈与表单原语 | 1 |
| `web/src/components/ui/{item,scroll-area}.tsx`（新） | 列表行、滚动区；scroll-area 增加横向条与 `viewportRef` / `onViewportScroll` | 2 |
| `web/src/components/ui/drawer.tsx`（新） | 底部弹层 | 5 |
| `web/src/components/ui/{message,bubble,attachment,marker,message-scroller}.tsx`（新） | 聊天原语 | 6 |
| `web/src/styles/motion-{anthropic,moonshot}.css` | 新 `data-slot` 的动效规则 | 1、2、5、6 |
| `web/package.json`、`web/package-lock.json` | `@shadcn/react@0.3.1` | 6 |
| 业务组件（见各任务） | 替换落点 | 1–6 |

---

### 任务 0：提交 spec 与计划

- [ ] 提交两份文档：

```bash
git add docs/superpowers/specs/2026-10-15-shadcn-native-round3-design.md docs/superpowers/plans/2026-10-15-shadcn-native-round3.md
git commit -m "docs(web): shadcn 原生组件替换第三轮设计与实施计划"
```

### 任务 1：反馈类（Spinner / Progress / Alert / Badge / Separator / Table）

**文件**：
- 新建：`web/src/components/shadcn-native.test.ts`
- 新建：`web/src/components/ui/{spinner,progress,alert,field}.tsx`
- 修改：
  - `app/callback/page.tsx`
  - `app/(dashboard)/dataset/page.tsx`
  - `app/(dashboard)/upstream/{model-dialog,pricing-dialog,shared}.tsx`
  - `components/share/share-dialog.tsx`
  - `app/login/page.tsx`
  - `components/chat/{assistant-message,multimodal-parts,markdown-lite}.tsx`
  - `components/playground/chat-turn.tsx`
  - `app/(dashboard)/audit/model/page.tsx`
  - `app/(dashboard)/trigger/page.tsx`
  - `components/session-detail/session-detail-client.tsx`
  - `app/share/page.tsx`
  - `components/charts/{model-cost-bar-chart,model-token-bar-chart}.tsx`
  - `styles/motion-{anthropic,moonshot}.css`

**接口**：
- 产出 `Spinner`（`ComponentProps<"svg">`）。
- 产出 `Progress`（Base UI Progress Root，`value: number`）及 `ProgressTrack` / `ProgressIndicator` / `ProgressLabel` / `ProgressValue`。
- 产出 `Alert` / `AlertTitle` / `AlertDescription` / `AlertAction`（`variant: "default" | "destructive"`）。
- 产出 `Field*` 全家（任务 2 使用）。

- [ ] **步骤 1：写契约测试骨架与 ① 断言**（先失败）

```ts
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const SRC = fileURLToPath(new URL("..", import.meta.url));
const walk = (dir: string): string[] =>
  readdirSync(dir).flatMap((name) => {
    const p = join(dir, name);
    return statSync(p).isDirectory() ? walk(p) : p.endsWith(".tsx") ? [p] : [];
  });
const tsx = walk(SRC);
const rel = (p: string) => relative(SRC, p);
const read = (p: string) => readFileSync(join(SRC, p), "utf8");
const business = tsx.filter((p) => !rel(p).startsWith("components/ui/"));
const offenders = (re: RegExp, pool = business) =>
  pool.filter((p) => re.test(readFileSync(p, "utf8"))).map(rel);

const TW_ANIMATE = /\b(animate-in|animate-out|fade-in-0|fade-out-0|zoom-in-95|zoom-out-95|slide-in-from-|slide-out-to-)/;

function expectGenerated(names: string[]) {
  for (const name of names) {
    const path = `components/ui/${name}.tsx`;
    it(`${path} 已生成且规范化`, () => {
      expect(existsSync(join(SRC, path))).toBe(true);
      const src = read(path);
      expect(src).not.toMatch(/from "cn"/);
      expect(src).not.toMatch(TW_ANIMATE);
    });
  }
}

describe("① 反馈类", () => {
  expectGenerated(["spinner", "progress", "alert", "field"]);
  it("业务代码不再手写加载圈", () => expect(offenders(/animate-spin/)).toEqual([]));
  it("业务代码不再用原生 table / hr", () => {
    expect(offenders(/<table[\s>]/)).toEqual([]);
    expect(offenders(/<hr[\s/>]/)).toEqual([]);
  });
  it("错误块改用 Alert", () =>
    expect(offenders(/border-destructive\/\d+ bg-destructive\//)).toEqual([]));
  it("dataset 进度条改用 Progress", () => {
    const src = read("app/(dashboard)/dataset/page.tsx");
    expect(src).toContain("<Progress");
    expect(src).not.toMatch(/h-full rounded-full bg-primary/);
  });
});
```

- [ ] **步骤 2：确认失败**

运行：`cd web && npx vitest run src/components/shadcn-native.test.ts`
预期：FAIL（文件不存在，命中 `animate-spin` 等）。

- [ ] **步骤 3：生成组件源码**

```sh
cd web
for c in spinner progress alert field; do
  ./node_modules/.bin/shadcn add $c --dry-run --view src/components/ui/$c.tsx > /tmp/shadcn-view/$c.txt
done
python3 - <<'PY'
import re,sys
for c in ["spinner","progress","alert","field"]:
    lines=open(f"/tmp/shadcn-view/{c}.txt").read().split("\n")
    s=next(i for i,l in enumerate(lines) if l.startswith("│ ┌"))
    e=next(i for i,l in enumerate(lines) if l.startswith("│ └"))
    body="\n".join(l[4:] if l.startswith("│ │ ") else l[3:] for l in lines[s+1:e])
    body=body.replace('from "cn"','from "@/lib/utils"')
    open(f"src/components/ui/{c}.tsx","w").write(body.rstrip()+"\n")
PY
npx prettier --write src/components/ui/{spinner,progress,alert,field}.tsx
grep -n -E 'animate-in|animate-out|fade-in-0|zoom-in-95|slide-in-from' src/components/ui/{spinner,progress,alert,field}.tsx
```

grep 命中的 tw-animate 类逐个删掉。

- [ ] **步骤 4：按位置替换**

1. **Spinner**：`Loader2 ... animate-spin` 换成 `<Spinner />`；在 Button 内写成 `<Spinner data-icon="inline-start" />`。同时删除 `Loader2` 导入。
   - 位置：`callback/page.tsx` L79（页面级，用 `<Spinner className="mx-auto mb-4 size-6" />`）、`dataset/page.tsx` L654、`model-dialog.tsx` L248、`pricing-dialog.tsx` L93、`share-dialog.tsx` L237。
   - filter-bar 的加载圈留到任务 4，契约测试先在本任务里同步替换为 `<Spinner />`，保证 ① 断言通过。
2. **Progress**：`dataset/page.tsx` 的两处进度条。

   ```tsx
   <Progress value={Math.max(6, (item.value / max) * 100)} />
   <Progress value={exportProgress} />
   ```

   - 删除外层 `div.h-2.5` 与内层 `style={{ width }}`；
   - 第二处原 `transition-[width]` 由组件自带。
3. **Alert**：替换以下错误块。

   ```tsx
   <Alert variant="destructive">
     <AlertTriangle />
     <AlertTitle>{标题}</AlertTitle>
     <AlertDescription>{正文}</AlertDescription>
   </Alert>
   ```

   - `login/page.tsx` L173：无标题，只用 `AlertDescription`，保留外层 `animate-fade mt-6`。
   - `assistant-message.tsx` / `multimodal-parts.tsx` 的 `PartRefusal`：图标 `ShieldAlert`，正文 `AlertDescription className="whitespace-pre-wrap break-words"`。
   - `chat-turn.tsx` L285：正文为 status + message；重试按钮放 `<AlertAction>`。
   - `markdown-lite.tsx` L119 mermaid 错误：`AlertTitle` 写 `mermaid · render error`，`AlertDescription` 内放两个 `<pre>`（错误、源码）。
   - `audit/model/page.tsx` L363：`Alert variant="destructive" className="mb-3"`。
4. **Badge**：
   - `upstream/shared.tsx` L68 告警：`<Badge variant="outline" className="border-amber-500/30 bg-amber-500/15 font-mono text-amber-600 dark:text-amber-400">`，语义色保留。
   - `upstream/shared.tsx` L124/156/169：`<Badge variant="secondary" className="font-mono tabular-nums">`。
   - `trigger/page.tsx` `ActionBadge`：`<Badge render={<button type="button" onClick={onClick} disabled={disabled} />} variant={action === "deny" ? "destructive" : "secondary"} className={语义色}>`。capture 的蓝、omit 的绿沿用原 className 中的 `bg-*/text-*/moonshot:*` 片段。
   - `session-detail-client.tsx` L417、`share/page.tsx` L299 工具数角标：`<Badge className="absolute -top-1.5 -right-1.5 h-4 min-w-4 px-1 tabular-nums">`。
   - `chat-turn.tsx` L249 角色标签：`<Badge variant="secondary" className="font-mono">`。
5. **分隔**：
   - `login/page.tsx` L201-205：整段换成 `<FieldSeparator>{t("login.demo_or")}</FieldSeparator>`。
   - `markdown-lite.tsx` 的 `hr`：`hr: () => <Separator className="my-4" />`。
6. **Table**：
   - `model-cost-bar-chart.tsx` / `model-token-bar-chart.tsx` 的 `<table>/<thead>/<tr>/<th>/<td>` 换成 `Table` / `TableHeader` / `TableRow` / `TableHead` / `TableBody` / `TableCell`，保留 `tabular-nums` 与列宽 className。
   - `markdown-lite.tsx` 的 GFM 映射 `table` / `thead` / `tbody` / `tr` / `th` / `td` 同样换成 `Table` 系列。

- [ ] **步骤 5：动效语法层**

在 `motion-anthropic.css` 末尾（Focus 段之前）追加：

```css
/* ---- Alert「墨迹落纸」: the notice settles in as the ink dries. ---- */
[data-theme="anthropic"] [data-slot="alert"] {
  animation: atelier-ink-set 420ms var(--ease-settle) backwards;
}
/* ---- Progress: the bar is drawn like a pen stroke, never snapping. ---- */
[data-theme="anthropic"] [data-slot="progress-indicator"] {
  transition: width 480ms var(--ease-settle);
}
```

在 `motion-moonshot.css` 追加同 slot 规则。`lunar-*` 关键帧以文件内实际名称为准，执行前先 `grep -n '@keyframes' motion-moonshot.css` 选取 rise / beam 类。

```css
[data-theme="moonshot"] [data-slot="alert"] {
  animation: lunar-rise 420ms var(--ease-orbit) backwards;
}
[data-theme="moonshot"] [data-slot="progress-indicator"] {
  transition: width 480ms var(--ease-orbit);
}
```

- [ ] **步骤 6：契约测试通过**：`npx vitest run src/components/shadcn-native.test.ts`，预期 PASS。
- [ ] **步骤 7：标准验收**，全部 exit 0。
- [ ] **步骤 8：提交**

```bash
git add web/src/components/shadcn-native.test.ts web/src/components/ui/{spinner,progress,alert,field}.tsx web/src/styles <本任务业务文件>
git commit -m "refactor(web): 加载/进度/错误提示/标签/分隔/表格换成 shadcn 原生组件"
```

### 任务 2：表单与列表（Field / Item / ScrollArea）

**文件**：
- 新建：`web/src/components/ui/{item,scroll-area}.tsx`
- 修改：
  - `upstream/{endpoint-dialog,model-dialog,pricing-editor}.tsx`
  - `components/cron/schedule-editor.tsx`
  - `components/demo-config-card.tsx`
  - `app/(dashboard)/profile/page.tsx`
  - `components/share/share-dialog.tsx`
  - 六个页面的 `isMobile` 分支：apikeys、sessions、trace、trigger、users、audit/model
  - `session-detail/session-history-list.tsx`
  - `playground/session-sidebar.tsx`
  - `trace-detail/trace-detail-client.tsx`
  - `dataset/page.tsx`
  - `shared/reading-layout.tsx`（桌面工具栏滚动区）
  - `charts/model-{cost,token}-bar-chart.tsx`
  - `styles/motion-*.css`

**接口**：
- 产出 `ScrollArea`，在生成版之上扩展两个属性：

  ```ts
  ScrollAreaPrimitive.Root.Props & {
    viewportRef?: React.Ref<HTMLDivElement>;
    onViewportScroll?: React.UIEventHandler<HTMLDivElement>;
  }
  ```

  Root 内同时渲染 `<ScrollBar />` 与 `<ScrollBar orientation="horizontal" />`。
- 产出 `ItemGroup` / `Item`（`variant: default | outline | muted`，`size`，`render`）/ `ItemMedia` / `ItemContent` / `ItemTitle` / `ItemDescription` / `ItemActions` / `ItemHeader` / `ItemFooter` / `ItemSeparator`。

- [ ] **步骤 1：追加 ② 断言**（先失败）

```ts
describe("② 表单与列表", () => {
  expectGenerated(["item", "scroll-area"]);
  const forms = [
    "app/(dashboard)/upstream/endpoint-dialog.tsx",
    "app/(dashboard)/upstream/model-dialog.tsx",
    "app/(dashboard)/upstream/pricing-editor.tsx",
    "components/cron/schedule-editor.tsx",
    "components/demo-config-card.tsx",
    "app/(dashboard)/profile/page.tsx",
    "components/share/share-dialog.tsx",
  ];
  it.each(forms)("%s 用 Field 布局", (p) => {
    expect(read(p)).toMatch(/<Field[\s>]/);
    expect(read(p)).not.toMatch(/<label[\s>]/);
  });
  const mobileLists = ["apikeys", "sessions", "trace", "trigger", "users", "audit/model"].map(
    (p) => `app/(dashboard)/${p}/page.tsx`,
  );
  it.each(mobileLists)("%s 移动端列表用 Item", (p) => expect(read(p)).toContain("<ItemGroup"));
  it.each([
    "components/session-detail/session-history-list.tsx",
    "components/playground/session-sidebar.tsx",
  ])("%s 会话列表用 Item", (p) => expect(read(p)).toContain("<Item"));
  it("trace 详情的代码块用 ScrollArea", () =>
    expect(read("components/trace-detail/trace-detail-client.tsx")).not.toMatch(/overflow-auto/));
});
```

- [ ] **步骤 2：确认失败**（同任务 1 的命令）。
- [ ] **步骤 3：生成 `item`、`scroll-area`**：用任务 1 步骤 3 的脚本，组件名换成 `item scroll-area`。随后给 `scroll-area.tsx` 加上扩展：

```tsx
function ScrollArea({
  className,
  children,
  viewportRef,
  onViewportScroll,
  ...props
}: ScrollAreaPrimitive.Root.Props & {
  viewportRef?: React.Ref<HTMLDivElement>;
  onViewportScroll?: React.UIEventHandler<HTMLDivElement>;
}) {
  return (
    <ScrollAreaPrimitive.Root data-slot="scroll-area" className={cn("relative", className)} {...props}>
      <ScrollAreaPrimitive.Viewport
        ref={viewportRef}
        onScroll={onViewportScroll}
        data-slot="scroll-area-viewport"
        className="size-full rounded-[inherit] transition-[color,box-shadow] outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-1"
      >
        {children}
      </ScrollAreaPrimitive.Viewport>
      <ScrollBar />
      <ScrollBar orientation="horizontal" />
      <ScrollAreaPrimitive.Corner />
    </ScrollAreaPrimitive.Root>
  );
}
```

- [ ] **步骤 4：Field 替换**

1. 规则：
   - `div.space-y-2 > Label + 控件` → `<Field><FieldLabel htmlFor>..</FieldLabel>{控件}<FieldDescription/></Field>`；
   - 外层 `div.space-y-4` → `<FieldGroup>`；
   - `<label className="flex items-center gap-2"><Checkbox/>文本</label>` → `<Field orientation="horizontal"><Checkbox id/><FieldLabel htmlFor>文本</FieldLabel></Field>`；
   - 一组勾选项 → `<FieldSet><FieldLegend variant="label">..</FieldLegend><FieldGroup>..</FieldGroup></FieldSet>`。
2. `endpoint-dialog.tsx`：四个协议勾选（L126-160）组成一个 `FieldSet`。
3. `pricing-editor.tsx` L176/187：Switch 行换成 `Field orientation="horizontal"`。
4. `demo-config-card.tsx` L102 模块卡片：

   ```tsx
   <FieldLabel htmlFor={id}>
     <Field orientation="horizontal">
       <Checkbox id={id} />
       <FieldContent>
         <FieldTitle>..</FieldTitle>
       </FieldContent>
     </Field>
   </FieldLabel>
   ```

5. `schedule-editor.tsx`、`model-dialog.tsx`、`profile/page.tsx`、`share-dialog.tsx` 按规则 1 逐处替换。share-dialog 的 RadioGroup 选项行写成 `<Field orientation="horizontal"><RadioGroupItem/><FieldLabel/></Field>`。
6. 字段校验态若存在：`Field data-invalid` + 控件 `aria-invalid`，错误文本用 `FieldError`。

- [ ] **步骤 5：Item 替换**

移动端卡片模板（以 apikeys 为例，其余五页同构，保留各自字段与 Tooltip 截断）：

```tsx
<ItemGroup>
  {keys.map((key) => (
    <Item key={key.id} variant="outline" role="listitem">
      <ItemContent className="min-w-0">
        <ItemTitle className="w-full">{/* 原 TooltipRoot 包裹的 name，保留 truncate */}</ItemTitle>
        <ItemDescription>{/* 原副信息行 */}</ItemDescription>
      </ItemContent>
      <ItemActions>{/* 原右侧操作按钮 */}</ItemActions>
    </Item>
  ))}
</ItemGroup>
```

- 卡片可整体点击的页面（如 sessions）用 `Item render={<Link href=.. />}` 或 `render={<button type="button" />}`。原 onClick 和 stopPropagation 原样保留。
- `session-history-list.tsx`：`<Item render={<button type="button" onClick={..} />} variant={active ? "muted" : "default"} size="sm">`。
- `playground/session-sidebar.tsx` 同上：行内操作按钮放 `ItemActions`，重命名 Input 放 `ItemContent`。

- [ ] **步骤 6：ScrollArea 替换**

- `trace-detail-client.tsx` 的 6 个 `<pre className="... max-h-60 overflow-auto ...">` 改成：

  ```tsx
  <ScrollArea className="mt-2 max-h-60 rounded-md bg-(--code-bg)">
    <pre className="p-3 font-mono text-[11px] ...">..</pre>
  </ScrollArea>
  ```

  - `max-h-96` 那两处同理；
  - 原 `whitespace-pre-wrap break-words` 保留在 pre 上。
- `dataset/page.tsx` L577 同理。
- `reading-layout.tsx` 桌面工具栏 L159-165：`<ScrollArea className="min-h-0 flex-1" viewportRef={(n) => onToolsScrollRootChange?.(n)} onViewportScroll={onToolsScroll}><div className="flex flex-col gap-2 p-4">{toolsPanel}</div></ScrollArea>`。
- `session-history-list.tsx` L97、`playground/session-sidebar.tsx` L49、两个 bar-chart 的 `h-64 overflow-y-auto`：换成 `ScrollArea` 并保留高度 className。
- 不动 `(dashboard)/layout.tsx` L557 主内容区，也不动 ReadingLayout 的 `messagesScrollRootRef` 容器。

- [ ] **步骤 7：动效语法层**：两份 CSS 各追加 item 规则。anthropic 用 `transition: background-color 220ms var(--ease-ink), border-color 220ms var(--ease-ink);`，moonshot 用对应 ease token。
- [ ] **步骤 8：契约测试 PASS；标准验收通过；提交**

提交信息：`refactor(web): 表单布局/移动端列表/滚动区换成 shadcn Field/Item/ScrollArea`

### 任务 3：组合修正

**文件**：
- `components/share/share-dialog.tsx`
- `components/pagination-bar.tsx`
- `components/language-switcher.tsx`
- `components/permission-guard.tsx`
- `app/(dashboard)/layout.tsx`
- `components/ui/copy-button.tsx`
- `components/chat/markdown-lite.tsx`
- `app/(dashboard)/upstream/{grouped-view,flat-view}.tsx`
- `components/trace-install-popover.tsx`
- `components/shared/reading-layout.tsx`
- `components/charts/stacked-ratio-bar.tsx`

**接口**：
- `CopyButton` 的 props 不变：`{ value, ariaLabel?, variant?: "label" | "icon", className? }`。
- 内部改为 `<Button variant="ghost" size={variant === "icon" ? "icon-xs" : "xs"}>`；`min-w-0` 用于抵消 xs 的宽度预留。

- [ ] **步骤 1：追加 ③ 断言**（先失败）

```ts
describe("③ 组合修正", () => {
  it("share-dialog 不再按钮嵌套", () =>
    expect(read("components/share/share-dialog.tsx")).not.toMatch(/<PopoverTrigger>\s*<button/));
  it.each(["components/pagination-bar.tsx", "components/language-switcher.tsx"])(
    "%s 用 DropdownMenuRadioGroup",
    (p) => expect(read(p)).toContain("DropdownMenuRadioGroup"),
  );
  it("权限拦截页用 Empty", () =>
    expect(read("components/permission-guard.tsx")).toContain("<Empty"));
  it("侧栏分组标题用 SidebarGroupLabel", () =>
    expect(read("app/(dashboard)/layout.tsx")).toContain("<SidebarGroupLabel"));
  it.each([
    "components/chat/markdown-lite.tsx",
    "app/(dashboard)/upstream/grouped-view.tsx",
    "components/trace-install-popover.tsx",
  ])("%s 复用 CopyButton", (p) => {
    expect(read(p)).toContain("<CopyButton");
    expect(read(p)).not.toContain("copyTextToClipboard");
  });
  it("CopyButton 基于 Button", () => expect(read("components/ui/copy-button.tsx")).toContain("<Button"));
  it("比例条浮层用 Tooltip", () => {
    const src = read("components/charts/stacked-ratio-bar.tsx");
    expect(src).not.toContain("createPortal");
    expect(src).toContain("TooltipContent");
  });
});
```

- [ ] **步骤 2：确认失败。**
- [ ] **步骤 3：逐项替换**

1. **share-dialog**：

   ```tsx
   <PopoverTrigger
     render={
       <Button
         variant="outline"
         className={cn("w-full justify-start font-normal", !customDate && "text-muted-foreground")}
       />
     }
   >
     <CalendarIcon data-icon="inline-start" />
     {..}
   </PopoverTrigger>
   ```

2. **pagination-bar**：

   ```tsx
   <DropdownMenuContent align="start">
     <DropdownMenuGroup>
       <DropdownMenuRadioGroup
         value={String(pageInfo.pageSize)}
         onValueChange={(v) => onChange(1, Number(v))}
       >
         {PAGE_SIZES.map((s) => (
           <DropdownMenuRadioItem key={s} value={String(s)}>
             {s} {t("pagination.per_page")}
           </DropdownMenuRadioItem>
         ))}
       </DropdownMenuRadioGroup>
     </DropdownMenuGroup>
   </DropdownMenuContent>
   ```

   - 删掉 `Check` 导入与 `ml-6`；
   - 根 div 的 className 拼接改用 `cn`；
   - 上一页 / 下一页按钮内图标加 `data-icon`，去掉 size 类。
3. **language-switcher**：
   - 触发器 `DropdownMenuTrigger render={<Button variant="ghost" size="xs" className="min-w-0" />}`；
   - 内容用 `DropdownMenuRadioGroup value={locale} onValueChange={(v) => setLocale(v as Locale)}` + `DropdownMenuRadioItem`；
   - 删除手写 isActive 与模板字符串 class。
4. **permission-guard** 的 `GuardState`：

   ```tsx
   <div className="page-surface flex min-h-screen items-center justify-center bg-background px-4">
     <Empty>
       <EmptyHeader>
         <EmptyMedia variant="icon">
           <ShieldAlert />
         </EmptyMedia>
         <EmptyTitle>{title}</EmptyTitle>
         <EmptyDescription>{description}</EmptyDescription>
       </EmptyHeader>
     </Empty>
   </div>
   ```

5. **layout.tsx** L254：

   ```tsx
   <SidebarGroupLabel render={<CollapsibleTrigger />} className="cursor-pointer">
     {..原内容..}
   </SidebarGroupLabel>
   ```

   - 删除原手写样式 className；
   - 若 `SidebarGroupLabel` 未导入，从 `@/components/ui/sidebar` 补导入。
6. **copy-button**：按「接口」改写，保留 toast 失败提示与 1400ms 已复制态。
7. **markdown-lite** L180 代码块复制：`<CopyButton value={code} variant="label" className="text-(--code-text)/35 hover:text-(--code-text)" />`，删除本地 copied 状态与 `copyTextToClipboard` 导入。
8. **grouped-view** L63：
   - 现在是「整行可点复制」，换成 `<span className="font-mono ...">{value}</span><CopyButton value={value} variant="icon" />`；
   - 原 `copy()` 若无其他引用，一并删除。
9. **trace-install-popover** L94：`<CopyButton value={command} variant="label" />`；外观差异接受（默认外观）。
10. **flat-view** L92 排序表头：`<Button variant="ghost" size="xs" className="-ml-2 min-w-0 uppercase tracking-[0.08em]" aria-label=..>`。
11. **grouped-view** L397 折叠按钮：`<Button variant="ghost" size="icon-xs" aria-expanded=.. aria-label=..>`，图标改为单个 `ChevronRight` 加 `data-[expanded]` 时 `rotate-90`。
12. **reading-layout** L150 关闭按钮：`<Button variant="ghost" size="icon-sm" onClick=.. aria-label=..><XIcon /></Button>`。
13. **stacked-ratio-bar**：
    - 删除 `createPortal`、`useLayoutEffect`、`tooltipPos`、`tooltipRef`、`segmentRefs`；
    - 每段改为：

      ```tsx
      <TooltipRoot key={s.key}>
        <TooltipTrigger
          render={<div style={{ width: `${(s.value / sum) * 100}%`, backgroundColor: s.color }} />}
          onMouseEnter={() => setFocusedKey(s.key)}
          className={cn("cursor-default transition-opacity", focusedKey !== null && focusedKey !== s.key && "opacity-30")}
        />
        <TooltipContent className="min-w-52">{原明细列表}</TooltipContent>
      </TooltipRoot>
      ```

    - 外层用 `TooltipProvider` 包裹；
    - `style` 里的宽度和颜色属于数据驱动的动态值，保留。

- [ ] **步骤 4：契约测试 PASS；标准验收通过；提交**

提交信息：`refactor(web): 组合修正——按钮嵌套、单选菜单、Empty、SidebarGroupLabel、CopyButton 去重、比例条 Tooltip`

### 任务 4：filter-bar 改用 Combobox

**文件**：修改 `web/src/components/filter-bar/filter-bar.tsx`，`filter-dsl.ts`、`use-filter-bar.ts`、`types.ts` 不改。

**接口**：`FilterBarProps` 不变。

- [ ] **步骤 1：追加 ④ 断言**（先失败）

```ts
describe("④ filter-bar", () => {
  const src = () => read("components/filter-bar/filter-bar.tsx");
  it("用 Combobox 承载建议弹层", () => {
    expect(src()).toContain("<Combobox");
    expect(src()).toContain("<ComboboxItem");
  });
  it("不再手写 listbox 与点击外部监听", () => {
    expect(src()).not.toContain('role="option"');
    expect(src()).not.toContain('addEventListener("mousedown"');
  });
});
```

- [ ] **步骤 2：确认失败。**
- [ ] **步骤 3：先读 Base UI 本地文档**：`web/node_modules/@base-ui/react/docs/react/components/combobox.md`，确认以下语义后再实现：
  - `inputValue` / `onInputValueChange` 受控；
  - `filter={null}`；
  - `items`；
  - 单选 `value={null}` 时重复选择是否仍触发 `onValueChange`；
  - `onOpenChange` 的 `eventDetails.reason`（`"item-press"` 等）；
  - `onItemHighlighted`。
- [ ] **步骤 4：重写渲染层**

保留 `matchDraftFacet`、`rows` / `heading` 的 useMemo、`pick`、`tokenLabel`、值选项加载 effect。

1. **删除**：`rootRef` 与点击外部的 effect、`highlight` 状态与 `hl`、手写 `onKeyDown` 中方向键 / Enter / Esc 的分支。
2. **新增状态**：
   - `const anchorRef = useComboboxAnchor();`
   - `const highlighted = useRef<SuggestionRow | undefined>(undefined);`
   - `const keepOpen = useRef(false);`
3. **结构**：

   ```tsx
   <Combobox
     items={rows}
     filter={null}
     value={null}
     onValueChange={(row: SuggestionRow | null) => {
       if (!row) return;
       keepOpen.current = row.kind === "facet";
       pick(row);
     }}
     inputValue={input}
     onInputValueChange={(v, d) => {
       if (d.reason === "item-press") return;
       setInput(v);
       setOpen(true);
     }}
     open={open && (rows.length > 0 || loadingKey !== null)}
     onOpenChange={(next, d) => {
       if (!next && d.reason === "item-press" && keepOpen.current) {
         keepOpen.current = false;
         return;
       }
       setOpen(next);
     }}
     onItemHighlighted={(row) => {
       highlighted.current = row ?? undefined;
     }}
     itemToStringLabel={(row: SuggestionRow) => rowKey(row)}
     autoHighlight
   >
     <div ref={anchorRef} className="...原容器 class...">
       <Search />
       {tokens.map(..标签..)}
       <ComboboxPrimitive.Input
         placeholder=..
         autoComplete="off"
         spellCheck={false}
         className="min-w-32 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground/60"
         onKeyDown={onKeyDown}
       />
       {tokens.length > 0 && (
         <Button variant="ghost" size="xs" className="min-w-0" onClick=..>
           <X data-icon="inline-start" />
           {t("filter_bar.clear_all")}
         </Button>
       )}
     </div>
     <ComboboxContent anchor={anchorRef} className="w-72">
       <ComboboxLabel>{loading ? t("filter_bar.loading_options") : heading}</ComboboxLabel>
       {loading ? (
         <div className="flex items-center gap-2 px-2 py-2 text-xs text-muted-foreground">
           <Spinner />
           {t("filter_bar.loading_options")}
         </div>
       ) : (
         <ComboboxList>
           {(row: SuggestionRow) => (
             <ComboboxItem key={rowKey(row)} value={row}>
               {..原三种行内容..}
             </ComboboxItem>
           )}
         </ComboboxList>
       )}
       <ComboboxEmpty>{t("filter_bar.no_options")}</ComboboxEmpty>
     </ComboboxContent>
   </Combobox>
   ```

   - `rowKey` 抽成同文件函数，复用原 key 表达式：`f:${key}` / `v:${key}:${value}` / `kw:${text}`。
   - `ComboboxPrimitive` 从 `@base-ui/react` 导入（与 `ui/combobox.tsx` 一致）。若 `ComboboxInput` 的 InputGroup 外观可接受，优先用 `ComboboxInput`；它与 token 内联布局冲突时，才用 Primitive.Input。
4. **保留的 `onKeyDown`**：
   - `Backspace` 且输入为空：删最后一个 token；
   - `Tab` 且 `highlighted.current` 存在：`preventDefault` + `keepOpen.current = row.kind === "facet"` + `pick(highlighted.current)`；
   - `Enter` 且 `rows.length === 0 && input.trim()`：`preventDefault` + `pick({ kind: "keyword", text: input.trim() })`。

   其余按键交给 Base UI。
5. **标签**：

   ```tsx
   <Badge variant="outline" className={cn("h-7 gap-0 pr-0.5", label.free && "border-dashed")}>
     ..k..
     <Tooltip 包裹的 v/>
     <Button
       variant="ghost"
       size="icon-xs"
       aria-label=..
       onClick={(e) => { e.stopPropagation(); removeToken(i); }}
     >
       <X />
     </Button>
   </Badge>
   ```

- [ ] **步骤 5：运行时验证**（构建产物 + CDP）

1. `npx next build --webpack` 后，用 `python3 -m http.server` 托管 `web/out` 父目录：`out` 软链为 `web`（参考记忆：`ln -s . web`）。
2. 用 chrome-headless-shell 加 `--remote-debugging-port` 打开 `/web/sessions/`（未登录会被守卫重定向）。
3. 改用独立 harness：
   - 用 esbuild 打包仅渲染 `<FilterBar facets=[…静态 options…] {...useFilterBar 替身} />` 的入口，参考记忆中 esbuild + `--tsconfig` 解析 `@/` 别名的做法；
   - 放进 `out/__harness/` 托管；
   - CDP `Input.dispatchKeyEvent` 依次执行：聚焦 → `ArrowDown` → `Enter`（输入应变为 `key:`）→ `ArrowDown` → `Enter`（出现 1 个 token）→ `Backspace`（token 归零）→ 输入 `foo` + `Enter`（出现关键词 token）。
4. 每一步用 `Runtime.evaluate` 断言 DOM。
5. harness 文件不入库。

- [ ] **步骤 6：契约测试 PASS；标准验收通过；提交**

提交信息：`refactor(web): 筛选条建议弹层改用 shadcn Combobox，键盘与焦点交给 Base UI`

### 任务 5：底部弹层改用 Drawer

**文件**：
- 新建：`web/src/components/ui/drawer.tsx`
- 删除：`web/src/components/session-detail/swipe-dismiss-sheet-body.tsx`
- 修改：`components/shared/reading-layout.tsx`、`styles/motion-*.css`

- [ ] **步骤 1：追加 ⑤ 断言**（先失败）

```ts
describe("⑤ Drawer", () => {
  expectGenerated(["drawer"]);
  it("删除手写下滑关闭实现", () =>
    expect(existsSync(join(SRC, "components/session-detail/swipe-dismiss-sheet-body.tsx"))).toBe(false));
  it("阅读布局移动端用 Drawer", () => {
    const src = read("components/shared/reading-layout.tsx");
    expect(src).toContain("<Drawer");
    expect(src).not.toContain("SwipeDismissSheetBody");
  });
});
```

- [ ] **步骤 2：确认失败；生成 `drawer.tsx`**（同脚本）。
- [ ] **步骤 3：替换 reading-layout 移动端 L81-106**

```tsx
{toolsCount > 0 && (
  <Drawer open={toolsOpen} onOpenChange={onToolsOpenChange} showSwipeHandle>
    <DrawerContent>
      <DrawerHeader className="flex-row items-center gap-2">
        <Wrench className="size-4 text-muted-foreground" />
        <DrawerTitle>{t("reading_layout.available_tools")}</DrawerTitle>
        <Badge variant="secondary">{toolsCount}</Badge>
        <DrawerClose render={<Button variant="ghost" size="sm" className="ml-auto" />}>
          {t("common.done")}
        </DrawerClose>
      </DrawerHeader>
      <ScrollArea
        className="min-h-0 flex-1"
        viewportRef={(n) => onToolsScrollRootChange?.(n)}
        onViewportScroll={onToolsScroll}
      >
        <div className="flex flex-col gap-2 px-3 pt-3 pb-[calc(env(safe-area-inset-bottom)+0.75rem)]">
          {toolsPanel}
        </div>
      </ScrollArea>
    </DrawerContent>
  </Drawer>
)}
```

- 删除 `Sheet` / `SheetContent` / `SwipeDismissSheetBody` 导入；
- 删除 `swipe-dismiss-sheet-body.tsx`（用 delete_file 工具）；
- `grep -rn SwipeDismissSheetBody web/src` 应为空。

- [ ] **步骤 4：动效语法层**：两份 CSS 各给 `drawer-overlay` 补底色。anthropic 与 dialog-overlay 同色 `rgb(58 44 30 / 0.18)`，moonshot 与其 dialog-overlay 同值。**不覆盖** `drawer-popup` 的 transform / transition。
- [ ] **步骤 5：运行时验证**：
  - CDP 将视口设为 390×844，打开 harness 渲染的 `ReadingLayout`；
  - 断言点击触发后 `[data-slot=drawer-popup]` 存在，点「完成」后消失；
  - 触摸拖拽需真机复验，交付时说明。
- [ ] **步骤 6：契约测试 PASS；标准验收通过；提交**

提交信息：`refactor(web): 工具底部弹层改用 shadcn Drawer，删除手写下滑关闭实现`

### 任务 6：聊天区（Message / Bubble / Marker / Attachment / MessageScroller）

**文件**：
- 新建：`web/src/components/ui/{message,bubble,attachment,marker,message-scroller}.tsx`
- 修改：
  - `web/package.json`、`web/package-lock.json`
  - `components/chat/{user-message,assistant-message,system-message,multimodal-parts}.tsx`
  - `components/playground/{chat-turn,conversation-view}.tsx`
  - `app/(dashboard)/playground/page.tsx`（仅当需要补 `min-h-0`）
  - `styles/motion-*.css`

- [ ] **步骤 1：追加 ⑥ 断言**（先失败）

```ts
describe("⑥ 聊天区", () => {
  expectGenerated(["message", "bubble", "attachment", "marker", "message-scroller"]);
  it.each([
    "components/chat/user-message.tsx",
    "components/chat/assistant-message.tsx",
    "components/playground/chat-turn.tsx",
  ])("%s 用 Message + Bubble", (p) => {
    expect(read(p)).toContain("<Message");
    expect(read(p)).toContain("<Bubble");
  });
  it("系统消息头用 Marker", () => expect(read("components/chat/system-message.tsx")).toContain("<Marker"));
  it("多模态附件用 Attachment", () =>
    expect(read("components/chat/multimodal-parts.tsx")).toContain("<Attachment"));
  it("playground 对话区用 MessageScroller", () => {
    const src = read("components/playground/conversation-view.tsx");
    expect(src).toContain("<MessageScrollerProvider");
    expect(src).not.toContain("scrollTop");
  });
  it("chat-turn 保留流式签名动效钩子", () => {
    const src = read("components/playground/chat-turn.tsx");
    expect(src).toContain("stream-body");
    expect(src).toContain("data-streaming");
  });
});
```

- [ ] **步骤 2：确认失败；生成 5 个组件**（同脚本）。
- [ ] **步骤 3：引入 `@shadcn/react@0.3.1`，不触发整树重装**

```sh
cd web
npm install --package-lock-only --save-exact @shadcn/react@0.3.1
(cd /tmp && npm pack @shadcn/react@0.3.1)
mkdir -p node_modules/@shadcn/react
tar -xzf /tmp/shadcn-react-0.3.1.tgz -C node_modules/@shadcn/react --strip-components=1
npx --yes npm@10 ci --dry-run > /tmp/npm10-parity.txt 2>&1; echo exit=$?
git diff --stat package.json package-lock.json
```

- 预期：`exit=0`；package.json 只多一行 `"@shadcn/react": "0.3.1"`；lock 只新增该包条目。
- 检查 `shadcn/tailwind.css` 是否提供 `scroll-fade-b`、`scrollbar-thin`、`scrollbar-gutter-stable`、`shimmer`：`grep -c` 各 ≥1。缺失的 utility 从 message-scroller / attachment 的 className 里删掉。

- [ ] **步骤 4：chat 组件改写**

1. **`user-message.tsx`**：

   ```tsx
   <Message align="end" style={style} className="animate-in fade-in slide-in-from-bottom-1 duration-300">
     <MessageContent>
       <Bubble variant="secondary" align="end">
         <BubbleContent>
           {parts.length > 0 && <MultimodalParts parts={parts} />}
           {text && <MarkdownLite text={text} raw />}
           {!text && parts.length === 0 && <span className="text-muted-foreground">—</span>}
         </BubbleContent>
       </Bubble>
       {text && (
         <MessageFooter>
           <CopyButton value={text} variant="icon" ariaLabel={t("chat_message.copy_aria")} />
         </MessageFooter>
       )}
     </MessageContent>
   </Message>
   ```

2. **`assistant-message.tsx`**：
   - 外层 `<Message>`；
   - 头像 `<MessageAvatar>`（模型图标 / Sparkles / ShieldAlert），删除手写圆底 div；
   - `<MessageContent>` 内依次放：`{time && <MessageHeader>{time}</MessageHeader>}`、ReasoningBlock、`<Bubble variant="ghost"><BubbleContent>{正文}</BubbleContent></Bubble>`、复制按钮 `<MessageFooter>`、refusal（任务 1 已改 Alert）、ToolCallCard 列表。
3. **`system-message.tsx`**：
   - 头部换成 `<Marker variant="separator"><MarkerContent>{System · time}</MarkerContent></Marker>`，复制按钮放在 Marker 外右侧同一行（`flex items-center gap-2`）；
   - 正文 `<Bubble variant="muted" className="max-w-full"><BubbleContent render={<Collapsible open=.. onOpenChange=.. />}>..原内容..</BubbleContent></Bubble>`。若 `render` 合并有问题，改为 BubbleContent 内包 Collapsible。
4. **`multimodal-parts.tsx`**：
   - 图片：

     ```tsx
     <Attachment orientation="vertical" render={<a href={url} target="_blank" rel="noopener noreferrer" />}>
       <AttachmentMedia variant="image">
         <img src={url} alt="" />
       </AttachmentMedia>
     </Attachment>
     ```

     `Attachment` 是 div 组件、不支持 `render` 时，外包 `<a>`。
   - 音频 / 文件：

     ```tsx
     <Attachment>
       <AttachmentMedia>{icon}</AttachmentMedia>
       <AttachmentContent>
         <TooltipRoot>
           <TooltipTrigger render={<AttachmentTitle>{label}</AttachmentTitle>} />
           <TooltipContent className="max-w-xs break-all">{label}</TooltipContent>
         </TooltipRoot>
         {meta && <AttachmentDescription>{meta}</AttachmentDescription>}
       </AttachmentContent>
     </Attachment>
     ```

   - 外层 `<AttachmentGroup>`；
   - `isSafeUrl` 校验保留。
5. **`chat-turn.tsx`**：
   - 根换成 `<Message align={isUser ? "end" : "start"} className="group/turn turn-enter" style=..>` + `<MessageContent>`；
   - assistant 头部 `<MessageHeader>`（ProviderIcon + 模型名）；
   - 用户气泡 `<Bubble variant="secondary" align="end"><BubbleContent className="whitespace-pre-wrap">{message.content}</BubbleContent></Bubble>`；
   - assistant 正文 `<Bubble variant="ghost"><BubbleContent ref={bodyRef} className="stream-body" data-streaming=..>..</BubbleContent></Bubble>`。`BubbleContent` 用 `useRender`，ref 与 data 属性透传；不透传时在 BubbleContent 内包 `div.stream-body`；
   - system / 其他角色 `<Bubble variant="outline">`；
   - 页脚指标与操作条放 `<MessageFooter>`；编辑态 editBox 保持在 MessageContent 内。
6. **`conversation-view.tsx`**：

   ```tsx
   <MessageScrollerProvider autoScroll defaultScrollPosition="end">
     <MessageScroller className="flex-1">
       <MessageScrollerViewport>
         <MessageScrollerContent aria-busy={sending} className="px-1 py-6">
           {messages.map((m, i) => (
             <MessageScrollerItem key={m.id} messageId={m.id} scrollAnchor={m.role === "user"}>
               <ChatTurn .. />
             </MessageScrollerItem>
           ))}
         </MessageScrollerContent>
       </MessageScrollerViewport>
       <MessageScrollerButton />
     </MessageScroller>
   </MessageScrollerProvider>
   ```

   - `messages.length === 0` 时直接返回原 `<Empty className="h-full">`；
   - 删除 `scrollRef`、`followRef`、`useEffect` 与 `aria-live`（Content 自带 `role="log"`）；
   - 检查 `playground/page.tsx` 中 ConversationView 的父容器是否为有高度约束的 flex 列，必要时补 `min-h-0`。

- [ ] **步骤 5：动效语法层**：两份 CSS 各追加 `message-scroller-button` 的悬停过渡，以及 `bubble-content` 的背景色过渡。不给 `message-scroller-item` 加 transform 动画：回合入场已由 `.turn-enter` 负责。
- [ ] **步骤 6：运行时验证**：
  - harness 渲染 `ConversationView`，注入 30 条消息；
  - 断言 viewport `scrollTop + clientHeight ≈ scrollHeight`（在底部）；
  - CDP `Input.dispatchMouseEvent` 滚轮上滚后，断言 `[data-slot=message-scroller-button][data-active=true]` 存在；
  - 再追加一条消息，断言仍未跟随（已释放）；
  - 点按钮后回到底部。
- [ ] **步骤 7：契约测试 PASS；标准验收通过；提交**

提交信息：`refactor(web): 聊天区改用 shadcn Message/Bubble/Marker/Attachment，playground 对话区改用 MessageScroller`

### 任务 7：收尾

- [ ] **双主题动效命中抽查**：
  - 构建产物 + chrome-headless-shell，在 `data-theme="anthropic"` 与 `"moonshot"` 下分别渲染 harness 中的 Alert / Item / Drawer overlay；
  - `getComputedStyle` 断言 `animation-name` 或 `transition-property` 命中本轮新增规则。
- [ ] **全量回归**：
  - 标准验收；
  - `cd .. && go test ./test/unit/web_dashboard_layout/...`（守护 layout.tsx 契约）。
- [ ] **文档**：
  - `web/CONTEXT.md` 不新增领域术语（本轮无新概念），不改；
  - `docs/agents/web-frontend.md`「动效契约」补一条：Drawer 的 transform 由手势驱动，语法层只覆盖 overlay。
- [ ] **经验沉淀**：用 update_memory 记录本轮可复用的坑，例如 shadcn add 会自动装 `cn` 包、`--view` 提取法、Combobox 两步选择的受控写法。
- [ ] **清理**：
  - 删除 `web/.next`、`web/out`、`/tmp` 下的 harness；
  - 询问用户是否提 PR 或直接合并到 master，未经确认不推送。
