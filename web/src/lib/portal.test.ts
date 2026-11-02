import { describe, expect, it } from "vitest";
import en from "../locales/en.json";
import zh from "../locales/zh.json";
import ja from "../locales/ja.json";
import { GITHUB_REPO_URL, PORTAL_PILLARS, portalPrimaryCta } from "./portal";

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

describe("portal copy", () => {
  it("points at the public repository", () => {
    expect(GITHUB_REPO_URL).toBe("https://github.com/hcd233/aris-proxy-api");
  });

  it("has three pillars of four items each", () => {
    expect(PORTAL_PILLARS.map((p) => p.key)).toEqual(["access", "curation", "governance"]);
    PORTAL_PILLARS.forEach((p) => expect(p.items).toHaveLength(4));
  });

  it("defines every portal.* key in en, zh and ja", () => {
    const keys = (d: Record<string, string>) =>
      Object.keys(d)
        .filter((k) => k.startsWith("portal."))
        .sort();
    expect(keys(en).length).toBeGreaterThan(0);
    expect(keys(zh)).toEqual(keys(en));
    expect(keys(ja)).toEqual(keys(en));
    for (const p of PORTAL_PILLARS) for (const k of p.items) expect(en).toHaveProperty([k]);
  });
});
