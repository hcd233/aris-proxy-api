import { describe, expect, it } from "vitest";
import en from "../locales/en.json";
import zh from "../locales/zh.json";
import ja from "../locales/ja.json";
import {
  GITHUB_REPO_URL,
  PORTAL_AUDIT_ROWS,
  PORTAL_CAPABILITIES,
  PORTAL_CLIENTS,
  PORTAL_FEATURES,
  PORTAL_GATEWAY_STAGES,
  PORTAL_MODELS,
  PORTAL_ROUTE,
  PORTAL_STEPS,
  PORTAL_TTY_LINES,
  PORTAL_UPSTREAMS,
  PORTAL_VENDORS,
  portalPrimaryCta,
} from "./portal";

describe("portalPrimaryCta", () => {
  it("shows login while auth is loading even if a stale user exists", () => {
    expect(portalPrimaryCta({ isLoading: true, hasUser: true })).toEqual({
      href: "/login/",
      labelKey: "portal.cta.login",
    });
  });

  it("shows login for anonymous visitors", () => {
    expect(portalPrimaryCta({ isLoading: false, hasUser: false })).toEqual({
      href: "/login/",
      labelKey: "portal.cta.login",
    });
  });

  it("routes signed-in users to the dashboard", () => {
    expect(portalPrimaryCta({ isLoading: false, hasUser: true })).toEqual({
      href: "/dashboard/",
      labelKey: "portal.cta.console",
    });
  });
});

describe("portal demo data", () => {
  it("points at the public repository", () => {
    expect(GITHUB_REPO_URL).toBe("https://github.com/hcd233/aris-proxy-api");
  });

  it("lists the five export targets as clients", () =>
    expect(PORTAL_CLIENTS.map((c) => c.label)).toEqual([
      "Claude Code",
      "Codex",
      "OpenCode",
      "Pi",
      "DeepSeek Harness",
    ]));

  it("uses deepseek icon for DeepSeek Harness", () =>
    expect(PORTAL_CLIENTS.find((c) => c.id === "deepseek-harness")?.icon).toBe("deepseek"));

  it("expands nine upstreams including both subscriptions", () => {
    expect(PORTAL_UPSTREAMS).toHaveLength(9);
    expect(PORTAL_UPSTREAMS.map((u) => u.labelKey).filter(Boolean)).toEqual([
      "portal.upstream.codex_sub",
      "portal.upstream.claudecode_sub",
    ]);
    expect(PORTAL_VENDORS.map((v) => v.label)).toEqual([
      "OpenAI",
      "Anthropic",
      "DeepSeek",
      "Qwen",
      "Hy",
      "MiniMax",
      "GLM",
    ]);
  });

  it("keeps every demo model id lowercase", () => {
    const ids = [...Object.values(PORTAL_MODELS), ...PORTAL_AUDIT_ROWS.map((r) => r.model)];
    ids.forEach((m) => expect(m).toBe(m.toLowerCase()));
    expect(PORTAL_MODELS.minimax).toBe("minimax-m3.1-flash");
  });

  it("routes alias sonnet over two priority tiers", () => {
    expect(PORTAL_ROUTE.alias).toBe("sonnet");
    expect(PORTAL_ROUTE.tiers.map((t) => t.endpoints.map((e) => e.weight))).toEqual([
      [70, 30],
      [100, 100],
    ]);
  });

  it("never says --target in the export demo", () =>
    expect(PORTAL_TTY_LINES.some((l) => l.text.includes("--target"))).toBe(false));

  it("keeps implementation jargon out of feature cards", () => {
    for (const k of PORTAL_FEATURES)
      for (const f of ["title", "desc"] as const)
        expect((zh as Record<string, string>)[`portal.feature.${k}.${f}`]).not.toMatch(
          /checksum|内容寻址|去重/,
        );
  });
});

describe("portal copy", () => {
  const keys = (d: Record<string, string>) =>
    Object.keys(d)
      .filter((k) => k.startsWith("portal."))
      .sort();

  it("defines every portal.* key in en, zh and ja", () => {
    expect(keys(en).length).toBeGreaterThan(0);
    expect(keys(zh)).toEqual(keys(en));
    expect(keys(ja)).toEqual(keys(en));
  });

  it("has copy for every data-driven key", () => {
    const required = [
      ...PORTAL_FEATURES.flatMap((k) => [`portal.feature.${k}.title`, `portal.feature.${k}.desc`]),
      ...PORTAL_STEPS.flatMap((k) => [
        `portal.step.${k}.title`,
        `portal.step.${k}.desc`,
        `portal.step.${k}.window`,
      ]),
      ...PORTAL_CAPABILITIES.flatMap((c) => [
        `portal.cap.${c.key}.eyebrow`,
        `portal.cap.${c.key}.title`,
        `portal.cap.${c.key}.desc`,
        ...c.chips,
      ]),
      ...PORTAL_GATEWAY_STAGES,
      ...PORTAL_UPSTREAMS.flatMap((u) => (u.labelKey ? [u.labelKey] : [])),
    ];
    for (const k of required) expect(en).toHaveProperty([k]);
  });
});
