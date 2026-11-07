import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

/**
 * 首屏字体替换不位移：Maple Mono CN 以 font-display: swap 分片加载，加载前用
 * styles/font-fallback.css 里按度量对齐的本地字体顶替。这里守护三件事：
 * ① globals.css 引入了回退字体并在 --font-sans / --font-mono 中紧跟 Maple；
 * ② 不再混入比例字体（Avenir Next 等），否则字体到位时整页宽度回流；
 * ③ 每个回退 face 的 ascent/descent-override 与 size-adjust 一致（Maple 1.02 / 0.30em）。
 */

const globals = readFileSync(new URL("../app/globals.css", import.meta.url), "utf8");
const fallback = readFileSync(new URL("../styles/font-fallback.css", import.meta.url), "utf8");

const FALLBACK_FAMILIES = [
  "Maple Mono CN Fallback Menlo",
  "Maple Mono CN Fallback Liberation",
  "Maple Mono CN Fallback Consolas",
  "Maple Mono CN Fallback CJK",
];

const stack = (name: string) => {
  const m = globals.match(new RegExp(`${name}:\\s*([^;]+);`));
  if (!m) throw new Error(`${name} not found in globals.css`);
  return m[1].split(",").map((s) => s.trim().replace(/^"|"$/g, ""));
};

const faces = [...fallback.matchAll(/@font-face\s*\{([^}]*)\}/g)].map((m) => m[1]);
const descriptor = (face: string, name: string) => {
  const m = face.match(new RegExp(`${name}:\\s*([\\d.]+)%`));
  if (!m) throw new Error(`${name} missing in @font-face`);
  return Number(m[1]);
};

describe("Maple Mono CN metric-matched fallback", () => {
  it("is imported by globals.css", () => {
    expect(globals).toContain('@import "../styles/font-fallback.css";');
  });

  it.each(["--font-sans", "--font-mono"])("%s puts the fallbacks right after Maple", (name) => {
    const families = stack(name);
    expect(families.slice(0, 1 + FALLBACK_FAMILIES.length)).toEqual([
      "Maple Mono CN",
      ...FALLBACK_FAMILIES,
    ]);
    expect(families).not.toContain("Avenir Next");
    expect(families).not.toContain("sans-serif");
  });

  it("declares every fallback family at 400 and 700", () => {
    for (const family of FALLBACK_FAMILIES) {
      const weights = faces
        .filter((f) => f.includes(`font-family: "${family}";`))
        .map((f) => f.match(/font-weight:\s*(\d+)/)?.[1]);
      expect(weights.sort(), family).toEqual(["400", "700"]);
    }
  });

  it("pins vertical metrics to Maple after size-adjust", () => {
    expect(faces).toHaveLength(FALLBACK_FAMILIES.length * 2);
    for (const face of faces) {
      const sa = descriptor(face, "size-adjust") / 100;
      expect(descriptor(face, "ascent-override")).toBeCloseTo(102 / sa, 1);
      expect(descriptor(face, "descent-override")).toBeCloseTo(30 / sa, 1);
      expect(descriptor(face, "line-gap-override")).toBe(0);
    }
  });
});
