/**
 * 站内路由与 basePath 的单一来源。
 *
 * `next/link`、`next/navigation` 会自动补 basePath，业务路径一律写不带前缀的形式（如 `/login/`）。
 * 只有绕过路由器的场景才需要完整 URL：`window.location` 整页跳转、原生 `<a href>`、
 * 对外展示/复制的分享链接。这些场景统一经 `withBasePath` 拼接，不要手写 `/web`。
 *
 * BASE_PATH 必须与 `next.config.ts` 的 `basePath` 一致（由 routes.test.ts 守护）。
 */
export const BASE_PATH = "/web";

/** 登录页的 Next 路由路径（不含 basePath）。 */
export const LOGIN_PATH = "/login/";

/** 公开分享页的 Next 路由路径（不含 basePath），在 `(dashboard)` 路由组之外，匿名可访问。 */
export const SHARE_PATH = "/share/";

/** 会话详情页的 Next 路由路径（不含 basePath），会话 id 走 `?id=` 查询串（静态导出无法预渲染动态段）。 */
export const SESSION_DETAIL_PATH = "/sessions/detail/";

/** 给站内路由路径补上 basePath，得到可用于整页跳转 / 原生 `<a>` 的 URL。`path` 必须以 `/` 开头。 */
export function withBasePath(path: string): string {
  return `${BASE_PATH}${path}`;
}
