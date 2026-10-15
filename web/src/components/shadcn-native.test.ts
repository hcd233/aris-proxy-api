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
