import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { TINY_TEXT_CLASSES } from "../styles/tiny-text";

/**
 * 手机字号契约（spec §5.3）：手机上正文 ≥14px、次要文字 ≥12px。
 * - CJK 的 :lang() 缩放把 text-xs/text-sm 压到 11px/13px 左右，手机上恢复成 Tailwind 默认值；
 * - 源码里写死的 <12px 任意字号类必须登记在 TINY_TEXT_CLASSES，并在手机上被统一提升到 12px。
 */

const SRC = fileURLToPath(new URL("..", import.meta.url));
const css = readFileSync(join(SRC, "app/globals.css"), "utf8");

const walk = (dir: string): string[] =>
  readdirSync(dir).flatMap((name) => {
    const p = join(dir, name);
    return statSync(p).isDirectory() ? walk(p) : p.endsWith(".tsx") ? [p] : [];
  });

const toPx = (n: number, unit: string) => (unit === "rem" ? n * 16 : n);

/** 取 `@media (width < 48rem) {` 起始的整块文本 */
function mobileBlocks(src: string): Array<{ start: number; body: string }> {
  const out: Array<{ start: number; body: string }> = [];
  const head = "@media (width < 48rem) {";
  let i = src.indexOf(head);
  while (i !== -1) {
    let depth = 0;
    let j = i + head.length - 1;
    for (; j < src.length; j++) {
      if (src[j] === "{") depth++;
      else if (src[j] === "}" && --depth === 0) break;
    }
    out.push({ start: i, body: src.slice(i, j + 1) });
    i = src.indexOf(head, j);
  }
  return out;
}

describe("手机字号", () => {
  const blocks = mobileBlocks(css);

  it("max-md 下 zh/ja 恢复 text-xs=0.75rem、text-sm=0.875rem", () => {
    const lastLang = css.indexOf("--text-xs: 0.66rem"); // 原 :lang(ja) 缩放块
    const block = blocks.find(
      (b) => b.body.includes(":lang(zh)") && b.body.includes(":lang(ja)") && b.start > lastLang,
    );
    expect(block, "原 :lang() 块之后需要有手机覆盖块").toBeDefined();
    for (const decl of ["--text-xs: 0.75rem", "--text-sm: 0.875rem"]) {
      expect(block!.body).toContain(decl);
    }
  });

  it("源码中出现的每个 <12px 任意字号类都在 TINY_TEXT_CLASSES 里", () => {
    const found = new Set<string>();
    for (const file of walk(SRC)) {
      const src = readFileSync(file, "utf8");
      for (const m of src.matchAll(/(?<![\w:-])text-\[(\d+(?:\.\d+)?)(px|rem)\]/g)) {
        if (toPx(Number(m[1]), m[2]) < 12) found.add(m[0]);
      }
    }
    expect(found.size).toBeGreaterThan(0);
    for (const cls of found) expect(TINY_TEXT_CLASSES, cls).toContain(cls);
  });

  it("每个 TINY_TEXT_CLASSES 都在 max-md 规则里被提升到 0.75rem", () => {
    const block = blocks.find((b) => b.body.includes('[class~="text-['));
    expect(block).toBeDefined();
    for (const cls of TINY_TEXT_CLASSES) {
      expect(block!.body, cls).toContain(`[class~="${cls}"]`);
    }
    expect(block!.body).toContain("font-size: 0.75rem");
  });
});
