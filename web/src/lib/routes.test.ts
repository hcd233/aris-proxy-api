import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import nextConfig from "../../next.config";
import { BASE_PATH, LOGIN_PATH, SESSION_DETAIL_PATH, SHARE_PATH, withBasePath } from "./routes";

describe("withBasePath", () => {
  it("prefixes route paths with the app basePath", () => {
    expect(withBasePath(LOGIN_PATH)).toBe("/web/login/");
    expect(withBasePath(SHARE_PATH)).toBe("/web/share/");
    expect(withBasePath(`${SESSION_DETAIL_PATH}?id=7`)).toBe("/web/sessions/detail/?id=7");
  });
});

describe("BASE_PATH", () => {
  it("stays in sync with next.config.ts", () => {
    expect(BASE_PATH).toBe(nextConfig.basePath);
  });
});

/**
 * 源码契约：站内路径不再手写 `/web` 前缀，统一经 withBasePath（见 lib/routes.ts）。
 * 只匹配「引号紧跟 /web」的字面量（`"/web/login/"`、`` `/web/x` ``），
 * 不会误伤 `/api/web/v1` 这类 API 前缀或注释。
 */
describe("hard-coded basePath guard", () => {
  const SRC = fileURLToPath(new URL("..", import.meta.url));
  const walk = (dir: string): string[] =>
    readdirSync(dir).flatMap((name) => {
      const p = join(dir, name);
      if (statSync(p).isDirectory()) return walk(p);
      return /\.(ts|tsx)$/.test(name) && !/\.test\.(ts|tsx)$/.test(name) ? [p] : [];
    });

  it("has no string literal starting with /web outside lib/routes.ts", () => {
    const literal = /["'`]\/web(?:\/|["'`])/;
    const offenders = walk(SRC)
      .filter((p) => relative(SRC, p) !== join("lib", "routes.ts"))
      .filter((p) => literal.test(readFileSync(p, "utf8")))
      .map((p) => relative(SRC, p));
    expect(offenders).toEqual([]);
  });
});
