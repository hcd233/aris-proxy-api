/** 仪表盘首页的 Next 路由路径（不含 basePath），用于 <Link> 与导航配置。 */
export const DASHBOARD_HOME_PATH = "/dashboard/";

/** 仪表盘首页的完整 URL（含 basePath），用于 window.location 整页跳转。 */
export const DASHBOARD_HOME_URL = "/web/dashboard/";

function withTrailingSlash(path: string): string {
  return path.endsWith("/") ? path : `${path}/`;
}

/** 导航项高亮判定：按路径段前缀匹配，兼容 usePathname 带或不带尾斜杠。 */
export function isNavItemActive(pathname: string, href: string): boolean {
  return withTrailingSlash(pathname).startsWith(withTrailingSlash(href));
}
