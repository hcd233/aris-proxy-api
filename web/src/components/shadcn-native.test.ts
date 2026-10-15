import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * 源码契约：业务代码不再手写 shadcn 已提供的组件；生成的 shadcn 组件已按项目规范化
 * （cn 走 @/lib/utils、无 tw-animate 进出场类，动效交给 styles/motion-*.css 语法层）。
 */

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

const TW_ANIMATE =
  /\b(animate-in|animate-out|fade-in-0|fade-out-0|zoom-in-95|zoom-out-95|slide-in-from-|slide-out-to-)/;

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

  it("业务代码不再手写加载圈", () => {
    expect(offenders(/animate-spin/)).toEqual([]);
  });

  it("业务代码不再用原生 table / hr", () => {
    expect(offenders(/<table[\s>]/)).toEqual([]);
    expect(offenders(/<hr[\s/>]/)).toEqual([]);
  });

  it("错误提示块改用 Alert", () => {
    expect(offenders(/border-destructive\/\d+ bg-destructive\//)).toEqual([]);
  });

  it("dataset 进度条改用 Progress", () => {
    const src = read("app/(dashboard)/dataset/page.tsx");
    expect(src).toContain("<Progress");
    expect(src).not.toMatch(/h-full rounded-full bg-primary/);
  });
});

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
  it.each(mobileLists)("%s 移动端列表用 Item", (p) => {
    expect(read(p)).toContain("<ItemGroup");
  });

  it.each([
    "components/session-detail/session-history-list.tsx",
    "components/playground/session-sidebar.tsx",
  ])("%s 会话列表用 Item 与 ScrollArea", (p) => {
    expect(read(p)).toMatch(/<Item[\s>]/);
    expect(read(p)).toContain("<ScrollArea");
  });

  it("代码 / JSON 预览与图表卡片表格用 ScrollArea", () => {
    expect(read("components/trace-detail/trace-detail-client.tsx")).not.toMatch(/overflow-auto/);
    expect(read("app/(dashboard)/dataset/page.tsx")).not.toMatch(/overflow-auto/);
    for (const p of [
      "components/charts/model-cost-bar-chart.tsx",
      "components/charts/model-token-bar-chart.tsx",
    ]) {
      expect(read(p)).toContain("<ScrollArea");
    }
  });
});

describe("③ 组合修正", () => {
  it("share-dialog 不再按钮嵌套", () => {
    expect(read("components/share/share-dialog.tsx")).not.toMatch(/<PopoverTrigger>\s*<button/);
  });

  it.each(["components/pagination-bar.tsx", "components/language-switcher.tsx"])(
    "%s 用 DropdownMenuRadioGroup",
    (p) => {
      expect(read(p)).toContain("DropdownMenuRadioGroup");
    },
  );

  it("权限拦截页用 Empty", () => {
    expect(read("components/permission-guard.tsx")).toContain("<Empty");
  });

  it("侧栏分组标题用 SidebarGroupLabel", () => {
    expect(read("app/(dashboard)/layout.tsx")).toContain("<SidebarGroupLabel");
  });

  it.each([
    "components/chat/markdown-lite.tsx",
    "app/(dashboard)/upstream/grouped-view.tsx",
    "components/trace-install-popover.tsx",
  ])("%s 复用 CopyButton", (p) => {
    expect(read(p)).toContain("<CopyButton");
    expect(read(p)).not.toContain("copyTextToClipboard");
  });

  it("CopyButton 基于 Button", () => {
    expect(read("components/ui/copy-button.tsx")).toContain("<Button");
  });

  it("排序表头、分组折叠、工具栏关闭按钮用 Button", () => {
    expect(read("app/(dashboard)/upstream/flat-view.tsx")).not.toMatch(/<button[\s>]/);
    expect(read("app/(dashboard)/upstream/grouped-view.tsx")).not.toMatch(/<button[\s>]/);
    expect(read("components/shared/reading-layout.tsx")).not.toMatch(/<button[\s>]/);
  });

  it("比例条浮层用 Tooltip", () => {
    const src = read("components/charts/stacked-ratio-bar.tsx");
    expect(src).not.toContain("createPortal");
    expect(src).toContain("TooltipContent");
  });
});
