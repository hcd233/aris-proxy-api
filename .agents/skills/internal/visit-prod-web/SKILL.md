---
name: visit-prod-web
description: 通过 ego-lite（`ego-browser` CLI）用真实浏览器访问生产环境 Web 控制台 https://api.lvlvko.top/web/，检查登录状态、引导 OAuth2 登录（GitHub/Google）、浏览 Dashboard 及各管理页面（Sessions、API Keys、Models、Audit、Cron 等）。当用户说"打开/看一下线上 Web 后台/管理页面/dashboard/控制台"、"浏览器打开生产页面"、"看看线上页面长什么样"、"检查一下我登录了没有"时使用本 skill，即使没有提到"skill"或"浏览器"字样。
---

# visit-prod-web

用 ego-lite 驱动的真实浏览器查看 aris-mass 生产 Web 控制台，定位是**只读浏览**：访问页面、切换页面、查看内容。

## 适用边界

- 本 skill 只负责"看"：不点击会修改数据的按钮（创建、删除、保存、编辑、Logout），除非用户明确要求并确认影响。
- 涉及生产数据查询、日志排障、traceId 追踪走 `query-prod-log`；HTTP/API 调用走 `call-api`；K8s 服务状态走 `operate-prod-service`。

## 前置

1. 先加载 `ego-browser` skill，并执行 `ego-browser skill` 读取当前安装版本的使用指南——API 与选项以该指南为准，本 skill 只记录本项目的具体用法。
2. 浏览器脚本一律交给 `ego-browser nodejs` 执行（heredoc 或 `-e`）。不要用 Playwright、不要自己 launch Chrome。
3. 整个浏览过程只用一个 TaskSpace，名字固定 `visit-prod-web`，后续轮次靠它续接，登录态得以保留。

## 核心工作流

### 1. 打开生产 Web 控制台

```bash
ego-browser nodejs <<'EOF'
const task = await taskSpace("visit-prod-web");
const page = task.page("p1");
await page.goto("https://api.lvlvko.top/web/");
console.log({ spaceId: task.spaceId, page: page.label, url: await page.url() });
console.log(await page.snapshot());
EOF
```

如果 space 里已经有 `api.lvlvko.top` 的页签，用 `await task.tabs()` 找到它、必要时 `adopt` 复用，不要新开 Page。

### 2. 判断登录状态

用 `page.snapshot()` 的 a11y 快照判断：

- **已登录**：URL 是 `/web/` 或 `/web/<页面>`；快照里有左侧 `navigation`（Dashboard、Sessions、API Keys、Models、Cron Audit 等链接）和顶部用户信息（头像、用户名、角色、Logout 按钮）。
- **未登录**：URL 被重定向到 `/web/login/`；快照里只有 "Aris MaaS" 品牌区 + 两个登录按钮（GitHub / Google），没有侧边栏。

### 3. 未登录 → 引导用户登录

1. 从最新快照确认登录按钮，点击：
   ```js
   await page.click("loc=role:button[name='GitHub']", { label: "GitHub 登录" });
   ```
2. 浏览器跳到 GitHub/Google 的 OAuth2 授权页。**这一步必须由真人完成**——授权页要用户输账号或选账号，AI 代替不了。
3. 调用 `await task.handOff()` 结束本轮，并**明确提示用户**在 ego-lite 窗口完成授权（如"请在 ego lite 窗口里用 GitHub 完成登录"），然后停下等用户确认。
4. 用户确认后，用之前打印的 `spaceId` 续接（handOff 后 space 归用户）：
   ```js
   const task = await takeOverTaskSpace(<spaceId>);
   const page = task.userPage();
   console.log(await page.snapshot());
   ```
5. 快照出现侧边栏即登录成功、已进入 Dashboard。

### 4. 已登录 → 浏览

- 切页面：从**最新快照**拿 ref 或构造语义选择器 → `await page.click("@12", { label: "打开 Sessions" })` → 再 `page.snapshot()` 看结果。
- 等页面就绪：`await page.waitForURL("**/web/sessions/**")` 或 `await page.waitForSelector("loc=role:link[name='Sessions']", { state: "visible" })`。
- 视觉确认：`await page.screenshot({ path: "/tmp/prod-web.png" })`（必须是绝对路径）；只看内容用 `page.snapshot({ scope: "full_page" })` 拿整页快照。

## 关键机制（为什么这么做）

- **ref 每次快照都会重新分配**：快照重建后 `@12` 不保证稳定，必须走"最新快照 → 拿 ref/选择器 → 点击 → 再快照"循环，别复用旧 ref。
- **优先 `snapshot()` 而非 `screenshot()`**：快照是文本 a11y 树，自带可操作 ref，token 开销远小于截图；截图只用于需要看视觉的场合。
- **登录必须真人操作**：OAuth2 授权页涉及账号密码/账号选择，agent 无法代替，所以"`handOff()` + 提示用户 + 等待确认"是流程的一部分，不是可跳过步骤。
- **每轮脚本都要打印 `spaceId` 和最后一次快照**：变量不跨进程保留，下一轮只能靠 `spaceId` + 快照续上。
- **结束时才 `finish()`**：浏览完成后 `await task.finish({ keep: [] })`；中途中断（等用户授权）不要调用 `finish()`，否则登录态和 space 一起没了。

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
- `ego-browser` 未安装或连不上时，读 `ego-browser` skill 的 `references/install.md`，按对应系统步骤处理，不要退回 chrome-devtools MCP。
