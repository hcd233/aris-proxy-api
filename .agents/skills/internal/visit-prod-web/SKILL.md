---
name: visit-prod-web
description: 通过 chrome-devtools MCP 用真实浏览器访问生产环境 Web 控制台 https://api.lvlvko.top/web/，检查登录状态、引导 OAuth2 登录（GitHub/Google）、浏览 Dashboard 及各管理页面（Sessions、API Keys、Models、Audit、Cron 等）。当用户说"打开/看一下线上 Web 后台/管理页面/dashboard/控制台"、"浏览器打开生产页面"、"看看线上页面长什么样"、"检查一下我登录了没有"时使用本 skill，即使没有提到"skill"或"浏览器"字样。
---

# visit-prod-web

用 chrome-devtools MCP 驱动的真实浏览器查看 aris-proxy-api 生产 Web 控制台，定位是**只读浏览**：访问页面、切换页面、查看内容。

## 适用边界

- 本 skill 只负责"看"：不点击会修改数据的按钮（创建、删除、保存、编辑、Logout），除非用户明确要求并确认影响。
- 涉及生产数据查询、日志排障、traceId 追踪走 `query-prod-log`；HTTP/API 调用走 `call-api`；K8s 服务状态走 `operate-prod-service`。

## 前置

chrome-devtools MCP 必须已连接。首次调用页面列表工具确认连接，同时看看浏览器里是否已有 `api.lvlvko.top` 的标签页——有就复用（保留登录态），别新开。

## 核心工作流

### 1. 打开生产 Web 控制台

浏览器里没有该站点标签页时：

```
chrome_devtools_new_page { "url": "https://api.lvlvko.top/web/" }
```

已有标签页则优先 `chrome_devtools_select_page` 复用。

### 2. 判断登录状态

用 `chrome_devtools_take_snapshot` 拿页面 a11y 快照，按特征判断：

- **已登录**：URL 是 `/web/` 或 `/web/<页面>`；快照里有左侧 `navigation`（Dashboard、Sessions、API Keys、Models、Cron Audit 等链接）和顶部用户信息（头像、用户名、角色、Logout 按钮）。
- **未登录**：URL 被重定向到 `/web/login/`；快照里只有 "Aris Proxy" 品牌区 + 两个登录按钮（GitHub / Google），没有侧边栏。

### 3. 未登录 → 引导用户登录

1. 从最新快照找到 GitHub 或 Google 登录按钮的 uid，点击：
   ```
   chrome_devtools_click { "uid": "<登录按钮uid>" }
   ```
2. 浏览器跳到 GitHub/Google 的 OAuth2 授权页。**这一步必须由真人完成**——授权页要用户输账号或选账号，AI 代替不了。
3. **明确提示用户**在浏览器里完成授权（如"请在打开的浏览器窗口中用 GitHub 完成登录"），然后停下等用户确认。
4. 用户授权后，授权页带 code/state 跳回 `/web/login/`，前端自动换 token 并跳到 `/web/`。
5. 重新 `chrome_devtools_take_snapshot` 确认已进入 Dashboard（出现侧边栏即成功）。

### 4. 已登录 → 浏览

- 想看某个页面：从**最新快照**找到对应导航链接的 uid → `chrome_devtools_click { "uid": ... }` → `chrome_devtools_take_snapshot` 看结果。
- 页面加载确认：`chrome_devtools_wait_for { "text": ["..."] }` 等关键文本出现再快照。
- 视觉确认：`chrome_devtools_take_screenshot`（默认视口；`"fullPage": true` 整页）。

## 关键机制（为什么这么做）

- **uid 每次快照都会重新分配**：a11y 树重建后 uid 不保证稳定，必须走"最新快照 → 拿 uid → 点击 → 再快照"循环，别复用旧 uid。
- **优先 take_snapshot 而非 take_screenshot**：快照是文本 a11y 树，自带可点击元素的 uid，token 开销远小于截图；截图只用于需要看视觉的场合。
- **登录必须真人操作**：OAuth2 授权页涉及账号密码/账号选择，agent 无法代替，所以"提示用户登录 + 等待确认"是流程一部分，不是可跳过步骤。
- **click 后可带 `"includeSnapshot": true`**：`chrome_devtools_click` 能在响应里直接带回新快照，省一次调用；不确定时显式 `take_snapshot` 更稳。

## 常见页面（侧边栏导航，2026-08 实测）

| 链接 | URL | 内容 |
|------|-----|------|
| Dashboard | /web/ | 资源概览（API Keys / Sessions / Endpoints / Models 计数）+ 调用趋势、成功率、Token 吞吐等图表 |
| Sessions | /web/sessions/ | 会话列表 |
| Shares | /web/shares/ | 分享 |
| API Keys | /web/apikeys/ | API Key 管理 |
| Endpoints | /web/endpoints/ | 端点管理 |
| Models | /web/models/ | 模型列表 |
| Blocked Words | /web/blocked/ | 拦截词 |
| Model Call Audit | /web/audit/model/ | 模型调用审计 |
| Cron Jobs | /web/cron/ | 定时任务 |
| Cron Audit | /web/audit/cron/ | 定时任务执行审计（traceId、状态、耗时） |
| Monitor | /web/monitor/ | 监控 |
| Training Data | /web/dataset/ | 训练数据 |
| Agent Traces | /web/trace/ | Agent 追踪 |
| Profile | /web/profile/ | 个人资料 |

## 边界与安全

- 不在回复中粘贴 JWT/cookie 等敏感值。
- 页面数据异常或用户想深挖线上问题，建议转 `query-prod-log` 按 traceId 排查。
- chrome-devtools 报未连接/超时时，提示用户启动浏览器并连接 MCP 后重试。
