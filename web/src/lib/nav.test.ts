import { describe, expect, it } from "vitest";
import {
  DASHBOARD_HOME_PATH,
  DASHBOARD_HOME_URL,
  NAV_GROUPS,
  findActiveGroupKey,
  isNavItemActive,
  matchesNavQuery,
  resolveNavGroups,
  type NavAccess,
} from "./nav";

const access = (over: Partial<NavAccess> = {}): NavAccess => ({
  isAdmin: false,
  isDemo: false,
  isModuleOpen: () => false,
  ...over,
});
const hrefs = (a: NavAccess) =>
  resolveNavGroups(NAV_GROUPS, a).flatMap((g) => g.items.map((i) => i.href));

describe("isNavItemActive", () => {
  it("highlights dashboard on its own path with or without trailing slash", () => {
    expect(isNavItemActive("/dashboard/", DASHBOARD_HOME_PATH)).toBe(true);
    expect(isNavItemActive("/dashboard", DASHBOARD_HOME_PATH)).toBe(true);
  });

  it("does not highlight dashboard on other pages or the portal", () => {
    expect(isNavItemActive("/sessions/", DASHBOARD_HOME_PATH)).toBe(false);
    expect(isNavItemActive("/", DASHBOARD_HOME_PATH)).toBe(false);
  });

  it("keeps sibling audit tabs apart and matches nested pages", () => {
    expect(isNavItemActive("/audit/cron/", "/audit/model/")).toBe(false);
    expect(isNavItemActive("/sessions/detail/", "/sessions/")).toBe(true);
  });

  it("exposes the basePath-qualified url", () => {
    expect(DASHBOARD_HOME_URL).toBe("/web/dashboard/");
  });
});

describe("NAV_GROUPS", () => {
  it("pins the dashboard at the top level instead of hiding it in a one-item group", () => {
    const first = NAV_GROUPS[0];
    expect(first.pinned).toBe(true);
    expect(first.items.map((i) => i.href)).toEqual([DASHBOARD_HOME_PATH]);
  });

  it("has unique hrefs", () => {
    const all = NAV_GROUPS.flatMap((g) => g.items.map((i) => i.href));
    expect(new Set(all).size).toBe(all.length);
  });
});

describe("findActiveGroupKey", () => {
  it("returns the collapsible group that owns the current page", () => {
    expect(findActiveGroupKey("/sessions/detail/")).toBe("data");
    expect(findActiveGroupKey("/audit/cron/")).toBe("ops");
  });

  it("never reports a pinned group: it has no header to expand", () => {
    expect(findActiveGroupKey(DASHBOARD_HOME_PATH)).toBeUndefined();
  });

  it("returns undefined for unknown paths", () => {
    expect(findActiveGroupKey("/nope/")).toBeUndefined();
  });
});

describe("resolveNavGroups", () => {
  it("shows everything to admins, none locked", () => {
    const groups = resolveNavGroups(NAV_GROUPS, access({ isAdmin: true }));
    expect(groups.flatMap((g) => g.items).some((i) => i.locked)).toBe(false);
    expect(hrefs(access({ isAdmin: true }))).toContain("/users/");
  });

  it("hides admin-only pages from regular users and drops groups left empty", () => {
    const list = hrefs(access());
    expect(list).toContain("/sessions/");
    expect(list).not.toContain("/users/");
    expect(list).not.toContain("/trigger/");
    // 「系统管理」只剩个人资料一项，仍应保留
    expect(list).toContain("/profile/");
    // 「运维与自动化」全是 admin 专属，整组消失
    const keys = resolveNavGroups(NAV_GROUPS, access()).map((g) => g.key);
    expect(keys).not.toContain("ops");
  });

  it("shows everything to demo users but locks modules that are not opened", () => {
    const groups = resolveNavGroups(
      NAV_GROUPS,
      access({ isDemo: true, isModuleOpen: (m) => m === "dashboard" }),
    );
    const byHref = Object.fromEntries(
      groups.flatMap((g) => g.items).map((i) => [i.href, i.locked]),
    );
    expect(byHref["/dashboard/"]).toBe(false);
    expect(byHref["/sessions/"]).toBe(true);
    // 没有 demoModule 的页面对 demo 一律锁定
    expect(byHref["/apikeys/"]).toBe(true);
  });
});

describe("matchesNavQuery", () => {
  it("matches everything for an empty query", () => {
    expect(matchesNavQuery("  ", ["仪表盘"])).toBe(true);
  });

  it("matches case-insensitively against any haystack", () => {
    expect(matchesNavQuery("AUDIT", ["模型调用审计", "/audit/model/"])).toBe(true);
    expect(matchesNavQuery("审计", ["模型调用审计", "/audit/model/"])).toBe(true);
    expect(matchesNavQuery("nothing", ["模型调用审计", "/audit/model/"])).toBe(false);
  });
});
