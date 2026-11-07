import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

/**
 * Maple Mono CN 的 woff2 托管在 Cloudflare R2（static.lvlvko.top），不再进构建
 * 产物与 Go 二进制。这里守护三件事：
 * ① 根布局引用生成好的远程 CSS，而不是 npm 包里的相对路径版本（后者会让 Next
 *    把 456 个 woff2 复制回 _next/static/media）；
 * ② 生成物里没有残留相对 url()，且所有地址指向同一个 R2 前缀；
 * ③ 400/700 各 228 个 unicode-range 分片、local() 优先、swap 策略都没丢。
 */
const css = readFileSync(new URL("../styles/maple-mono-cn.css", import.meta.url), "utf8");
const layout = readFileSync(new URL("../app/layout.tsx", import.meta.url), "utf8");

const faces = [...css.matchAll(/@font-face\s*\{([^}]*)\}/g)].map((m) => m[1]);
const urls = [...css.matchAll(/url\("([^"]+)"\)/g)].map((m) => m[1]);

const REMOTE_PREFIX = "https://static.lvlvko.top/fonts/maple-mono-cn/";

describe("Maple Mono CN hosted on R2", () => {
  it("is imported by the root layout instead of the npm package css", () => {
    expect(layout).toContain('"../styles/maple-mono-cn.css"');
    expect(layout).not.toContain("@chinese-fonts/maple-mono-cn/dist");
  });

  it("points every font url at the R2 prefix", () => {
    expect(urls).toHaveLength(456);
    for (const url of urls) {
      expect(url.startsWith(REMOTE_PREFIX)).toBe(true);
    }
    expect(css).not.toMatch(/url\("(?:\.\/|\/)/);
  });

  it("keeps both weights split by unicode-range", () => {
    const weight = (w: number) => faces.filter((f) => f.includes(`font-weight:${w}`));
    expect(weight(400)).toHaveLength(228);
    expect(weight(700)).toHaveLength(228);
    for (const face of faces) {
      expect(face).toContain("unicode-range:");
      expect(face).toContain("font-display:swap");
    }
  });

  it("still prefers a locally installed Maple before downloading", () => {
    for (const face of faces) {
      expect(face).toContain('local("Maple Mono CN")');
    }
  });
});
