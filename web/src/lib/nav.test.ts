import { describe, expect, it } from "vitest";
import { DASHBOARD_HOME_PATH, DASHBOARD_HOME_URL, isNavItemActive } from "./nav";

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
