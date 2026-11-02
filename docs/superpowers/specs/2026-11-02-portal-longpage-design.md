# Portal 长页重设计

> 日期：2026-11-02 ｜ 分支：`feature/portal-longpage-redesign-2026-11-02`
> 定稿视觉稿：[2026-11-02-portal-longpage-mockup.html](2026-11-02-portal-longpage-mockup.html)（浏览器直接打开，底部胶囊切换两个主题）

## 1. 意图

现在的公开门户页（`/web/`）只有导航、Hero、一张链路示意、三张支柱卡和页脚。用户的评价是「很丑，不够惊艳，不美观和现代化」。

这次把它扩成一张现代 SaaS 落地页：

- **结构和动效**整体参考 Lusmu（https://lusmu.diarmuradi.com/）：
  - 悬浮药丸导航；
  - 每个区块都是「小标签 → 大标题 → 副文案」的三层节奏；
  - Hero 下方放功能卡跑马灯；
  - 内容滚动进入视口时淡入上浮；
  - 接入步骤做成手风琴，配一个演示窗口。
- **anthropic 主题的样式**参考 teal-treasury（https://21stdev-my-components-one.vercel.app/#teal-treasury-landing）：
  - 暖纸色底；
  - 大号细字重标题；
  - 带呼吸光点的小标签；
  - Hero 背后一团柔光；
  - 收尾 CTA 用桃色到淡紫的渐变卡。
- **moonshot 主题**保持现有的黑底玻璃语言，在上面套用同一套结构和动效。

页面的任务是让访客在一屏一屏的滚动中看懂三件事：Aris 是什么、能做什么、怎么接入。看懂之后去登录控制台或 GitHub。

## 2. 目标、非目标、反目标

**目标**

- 达到用户认可的「惊艳、现代」观感。以定稿视觉稿为准，两个主题都要做到。
- 每个能力都配图解或示例，不只是列要点。
- 尽量复用 web 端已有组件，例如评分用 `ScoreDots`、对话气泡用 `Bubble`、厂商图标用 `ProviderIcon`、数字用 `MetricNumber`、进度条用 `Progress`、标签用 `Badge`，让门户和控制台看起来是同一个产品。

**非目标**

- 不新增后端接口。页面上所有示例数据都是写死的演示内容。
- 不做价格、用户评价、团队、FAQ、数字指标区块。前三项是 Lusmu 有的，后两项是讨论中被用户删掉的。
- 不改登录、控制台等其他页面。

**反目标**（技术上能跑，但做出来就算失败）

- 观感仍然平淡，例如区块之间节奏单调，或者动效只有淡入。
- 文案写成技术说明书。功能卡要讲用户能得到什么，「内容寻址去重」「checksum」这类实现细节不出现在跑马灯里。
- 为了动效引入新的动画库或 WebGL/Canvas 背景。
- 切换主题或语言时布局跳动。

## 3. 约束（来自项目契约 `docs/agents/web-frontend.md`）

- **静态导出**：`output: "export"`，`basePath: "/web"`。页面仍是 `"use client"` 组件，不引入需要 SSR 的依赖。
- **样式**：只用 Tailwind v4 + `cn()`，颜色走 `globals.css` 里的主题变量，不写死 hex，不用内联 `style` 写定值。逐元素的 `--i` 一类 CSS 变量可以例外，这是 `SplitText` 已有的做法。
- **图标**：通用图标用 `lucide-react`，厂商和客户端图标统一走 `ProviderIcon`（底层是 `@lobehub/icons`）。
- **动效**：
  - 动效强度以 `<html data-motion-tier>`（`immersive | lite | still`）为唯一档位来源，JS 侧读 `useMotionTier()`。
  - 能用 CSS 就不用 JS。
  - 入场动画用 `backwards` 填充，结束后元素上不残留 transform。
  - 定时轮播要在 `still` 档停下。
- **页面背景保持纯色**：契约禁止全页纹理、渐变、粒子和 canvas，除非用户明确要求。定稿视觉稿里 moonshot 的星空是全页背景，Hero 柔光是区块内装饰，两者的处理见 §10 第 1 条。
- **i18n**：所有文案都要有 zh / en / ja 三份，并且切换语言时布局不能跳动（Category Reserve、Font Scale）。用了 `truncate` 的元素必须包在 Tooltip 里。
- **验收命令**：`npm run lint && npm run test && npm run build`；CI 还会跑 `format:check`。

## 4. 页面结构（用户逐项确认过）

自上而下：

| # | 区块 | 锚点 |
|---|---|---|
| 1 | 悬浮药丸导航 | — |
| 2 | Hero | — |
| 3 | 功能卡跑马灯 | — |
| 4 | 兼容生态：两行图标跑马灯 | — |
| 5 | 请求链路 | `#flow` |
| 6 | 四大能力 | `#capabilities` |
| 7 | 接入步骤 | `#get-started` |
| 8 | 收尾 CTA | — |
| 9 | 大页脚 | — |

接入步骤放在能力之后，这是我定的，理由是「先讲能做什么，再讲怎么接」。用户已看过视觉稿，没有提出异议。

## 5. 各区块规格

区块之间统一用以下节奏：顶部是带呼吸光点的 eyebrow 小标签，接着是大号细字重的 h2，再接一段约 620px 宽的 muted 副文案，整体居中。各区块滚动进入视口时淡入并上浮（moonshot 下的限制见 §8）。

### 5.1 悬浮药丸导航

- 吸顶，距顶部约 14px，最大宽度约 720px 的圆角药丸，玻璃或纸面底加柔和阴影。
- 内容从左到右：品牌标识（沿用现有品牌）、三个锚点链接（链路 / 能力 / 接入步骤）、`PreferenceControls compact`、GitHub 图标按钮、主 CTA。
- 主 CTA 沿用 `portalPrimaryCta` 的逻辑：认证加载中按未登录展示，已登录时显示「进入控制台」。
- 窄屏下隐藏锚点链接，`sm` 以下隐藏 CTA，与现有行为一致。

### 5.2 Hero

- eyebrow「MaaS 网关 · 数据沉淀」+ h1「一个地址，接入所有大模型」+ 副文案 + 两个按钮（主 CTA、GitHub 源码）。文案沿用现有 `portal.*` key。
- h1 逐字或逐词升起：复用 `SplitText`（按字形拆分，CJK 安全），在 motion 语法层给 portal 标题补一组 `.split-char` 规则（参考 `.login-title .split-char`）。
- Hero 背后放一团缓慢漂移的径向柔光，只占 Hero 区块内部；anthropic 用桃色和淡紫，moonshot 用冷蓝灰和暖白。`still` 档时柔光静止。

### 5.3 功能卡跑马灯

横向无限滚动，悬停暂停，单张卡悬停时上浮。共 6 张卡，每张上半部是一块迷你插图，下面一行标题加一行说明，文案只讲用户能得到什么：

| 卡片 | 插图 | 标题 / 说明 |
|---|---|---|
| ① | 地址栏胶囊（显示「已连接」）+ 7 个厂商图标轮流浮动 | 一个地址，用上所有模型 / 换模型只改别名，不用再管各家的差异。 |
| ② | 两个对话气泡（复用 `Bubble`）+ 会话元信息 + `ScoreDots` 评分 | 每次对话，自动存成会话 / 随时回看、打分，也能生成链接分享给别人。 |
| ③ | 迷你终端 `aris model export` + 客户端胶囊 | 一条命令，配好客户端 / 不用再手动复制地址和密钥。 |
| ④ | Token / 估算费用 / 成功率三个数（`MetricNumber`）+ 一条面积曲线 | 用量和花费，一眼看清 / 每次调用都有记录，趋势和费用一目了然。 |
| ⑤ | 三个上游健康行，其中一个红点脉冲「已暂停 · 自动恢复中」 | 上游出问题，自动止损 / 故障上游被自动隔离，恢复后自动放行。 |
| ⑥ | 筛选标签 + 「1,204 段对话已筛出」+ 进度条（`Progress`） | 好对话，导出成训练数据 / 按评分、模型和时间筛选，一键导出。 |

### 5.4 兼容生态条

- 标题「兼容你正在用的客户端与模型厂商」。下面两行图标卡（图标 + 名字），两行反向滚动，悬停时暂停。
- 客户端一行：Claude Code、Codex、OpenCode、Pi、**DeepSeek Harness**（用 DeepSeek 图标）。这 5 个是 `aris model export` 实际支持的目标。
- 厂商一行：OpenAI、Anthropic、DeepSeek、Qwen、**Hy**（腾讯混元图标）、MiniMax、GLM（ZAI 图标）。

### 5.5 请求链路

- 一张大卡，三栏布局：
  - 左栏「客户端」5 个节点；
  - 中间是 Aris 核心，内含「鉴权 / 路由 / 熔断 / 审计」4 个小格；
  - 右栏「上游供应商」9 个节点：OpenAI、Anthropic、DeepSeek、Qwen、Hy、MiniMax、GLM、**Codex 订阅**、**Claude Code 订阅**。
- 每个节点都要有图标和名字。
- 所有节点用流动虚线连到核心：用 SVG 贝塞尔曲线，端点按 DOM 实测位置计算，窗口 resize 时重算。
- 循环演示一次请求：
  1. 随机一个客户端节点高亮，光点沿虚线流向核心；
  2. 核心的 4 个小格依次点亮；
  3. 光点流向随机一个上游，该上游节点高亮。
- `still` 档时只保留静态虚线，不跑演示。

### 5.6 四大能力

采用拼图网格：7 列宽 + 5 列宽交错排布，第一行是接入（7）和沉淀（5），第二行是治理（5）和容错（7）。每张卡由小标签、一句标题、一句说明、1 到 2 块示例面板组成，底部是一排要点标签。

**接入：「一个别名，背后是一组上游」**

- 示例 · 模型别名路由：别名 `sonnet` 下分两层优先级。
  - 优先级 1：Anthropic 权重 70、Claude Code 订阅 权重 30；
  - 优先级 2：DeepSeek 权重 100、GLM 权重 100。
  - 选中的端点按权重轮换高亮。
  - 下方注释：「同一会话 5 分钟内优先粘在同一个上游，避免上下文缓存失效。」
- 示例 · 协议互转（含流式）：左边是 OpenAI 格式请求 `POST …/chat/completions {"model":"sonnet","stream":true}`，右边是 Anthropic 格式 `POST /v1/messages {"model":"claude-sonnet-5-5","stream":true}`，中间放 ⇄。
- 要点标签：优先级分层、同层加权、会话亲和、OpenAI ⇄ Anthropic、Codex 订阅 / Claude Code 订阅。

**沉淀：「对话不再用完就丢」**

- 示例 · 一段会话：标题加 `ScoreDots`，下面是标签（38 条消息、6 个工具、claude-sonnet-5-5、分享链接）。
- 示例 · 导出 ShareGPT JSONL：筛选标签（评分 ≥ 4、claude-opus-5-5、近 30 天）、一段 JSONL 片段、`Progress`，底部写「保留多轮上下文、推理与工具调用 · 1,204 条」。
- 要点标签：消息去重存储、评分、分享链接、SFT 数据集。能力区可以提「去重」，跑马灯里不提。

**治理：「谁调用了什么，一清二楚」**

- 示例 · 调用审计（实时）：一张滚动表，新行从顶部滑入，最多显示 4 行。每行依次是：厂商图标、模型名、Token、首字延迟、状态码，其中 502 标红。
- 四级权限阶梯：pending、demo、user、admin，柱子逐级升高，admin 用强调色。
- 触发词命中后的处理：拦截（返回拦截消息）、忽略（放行但不存储）、捕获（只留历史不转发）。
- 要点标签：每次调用可追踪、运行时监控、100 次/分钟限流、Demo 账户。

**容错：「一个上游出问题，不拖垮整个网关」**

- 图解 · 熔断的一次完整过程：三个状态卡 CLOSED → OPEN → HALF-OPEN 用虚线相连。下面是 10 格请求条和实时错误率，再下面一行说明文字。脚本循环如下：
  1. 请求逐格变绿或变红，错误率随之上涨；
  2. 错误率到 60%（≥50%）时切到 OPEN，说明「后续请求立刻返回 503 + Retry-After」；
  3. 进入 HALF-OPEN，放行 1 个探测请求；
  4. 探测成功，回到 CLOSED。
- 图解 · 并发隔离：32 格占用条随机波动，配文「每个上游最多 32 个并发，满了等 1 秒后快速失败」。
- 要点标签：≥10 次请求且错误率 ≥50% 触发、30 秒后半开、503 + Retry-After、429 不计入熔断、进行中的流不受影响。

### 5.7 接入步骤：「四步，从零到可追溯」

- 布局是左右两栏。
- **左栏**是手风琴步骤列表：当前步骤展开说明文字和标签，底部有一条 7 秒进度条，走完自动切到下一步，也可以点击切换；非当前步骤半透明。
- **右栏**是一个吸顶的窗口（带窗口栏和标题），内容跟随当前步骤切换：

| 步骤 | 左栏说明（要点） | 右栏窗口 |
|---|---|---|
| 01 登录，拿到 API Key | GitHub / Google 登录，管理员审核后在 API Keys 页生成密钥 | 控制台 · API Keys：名称、打码的密钥 + 「已复制」、权限、创建时间、「密钥只显示这一次」 |
| 02 接入上游，定义模型别名 | 填 Base URL 和密钥，或 OAuth 授权 Codex / Claude Code 订阅；给模型起别名，绑定上游并设置优先级和权重 | 控制台 · 上游：3 个端点行（API Key / OAuth + 健康点）；别名 `sonnet` 的 3 条绑定（P1·70 / P1·30 / P2·100） |
| 03 装上 aris，一键配好客户端 | 一行命令安装，向导里填服务地址和密钥；`aris model export` 写入所选客户端配置并自动备份 | 终端逐行打字：`curl -fsSL https://aris.example.com/install.sh \| sh` → 下载并 sha256 校验 → `Server host` / `API key ✓` → `Configuration completed` → `aris model export` → 多选客户端 → 写入 `~/.claude/settings.json` 和 `~/.codex/config.toml`（.bak 已备份） |
| 04 照常使用，数据自动沉淀 | 每次调用被审计、每段对话存成会话，可评分、分享、导出 | 会话卡（`Bubble` 对话 + `ScoreDots` + 模型 / 消息数 / Token / 费用）、一条调用审计记录、导出进度 |

- 终端内容必须与真实 CLI 一致：
  - 安装脚本最后会 `exec aris init --host`；
  - `aris model export` **没有** `--target` 参数，是交互式多选。

### 5.8 收尾 CTA

- 一张大圆角卡：h2「把所有大模型，收进一个地址」、一句副文案、两个按钮。
- 卡片底色：anthropic 是桃色到淡紫的线性渐变，moonshot 是黑底卡片右上角一团冷光。

### 5.9 大页脚

- 四栏：品牌加一句话、产品、资源、开源。
- 底部放一个超大号「Aris」字标，用渐隐的文字渐变做装饰。
- 页脚链接只指向确实存在的目标：控制台、页内锚点、GitHub。没有文档站就不放「文档」链接。

## 6. 演示数据（写死，集中放在 `src/lib/portal.ts`）

- **模型名一律小写**，取自 models.dev（2026-11-02）各厂商的最新型号：

| 厂商 | 模型名 |
|---|---|
| Anthropic | claude-sonnet-5-5、claude-opus-5-5 |
| OpenAI | gpt-6.1-sol |
| DeepSeek | deepseek-v4-pro |
| Qwen | qwen3.8-max |
| MiniMax | minimax-m3.1-flash |
| GLM | glm-5.3 |
| Hy | hy4-preview |

- 别名 `sonnet` 保留。它代表用户自己起的别名，用来说明「别名背后对应一组真实模型」。
- 用到的事实都已和代码 / README 核对：
  - 路由：优先级分层 + 层内加权 + 会话亲和 TTL 5 分钟。
  - 熔断：60s 窗口，≥10 次请求且 ≥50% 错误时打开，30s 后半开，放行 1 个探测。
  - 隔离：每个 key 最多 32 并发，等待 1s；返回 503 + Retry-After，429 不计入熔断。
  - 限流：100 次/分钟。
  - 权限：四级。
  - 触发词：deny / omit / capture 三种动作。
  - 导出：ShareGPT JSONL。
  - 客户端：5 个 export 目标。

## 7. 组件复用对照

| 用途 | 复用 |
|---|---|
| 厂商 / 客户端图标 | `ProviderIcon`。需要在 `providerMap` 补 `claudecode`、`codex`、`opencode`、`pi` 四个 key（`@lobehub/icons` 已有 `ClaudeCode` / `Codex` / `OpenCode` / `Pi`）。注意 `findProviderKey` 是前缀匹配，`claudecode` 不会被 `claude` 误吃（后面跟的是小写字母），补完后要加单测确认。 |
| 评分 | `ScoreDots`。门户上做成可交互的本地状态（访客可以点着玩），`onClear` 清空即可，`scoring` 恒为 false。 |
| 对话气泡 | `ui/bubble`（`Bubble` / `BubbleContent`） |
| 数字 | `MetricNumber`（契约禁止再写 CountUp 一类的计数器） |
| 进度条 | `ui/progress` |
| 标签 | `ui/badge` |
| 按钮 | `ui/button` |
| 逐字入场 | `motion/split-text` |
| 主题 / 语言切换 | `PreferenceControls` |
| 数值格式化 | `formatTokenCount` / `formatInteger` / `formatCost` / `formatPct` |

迷你插图（地址栏、终端、状态机、并发格、审计行、路由树）属于门户专用的展示件，放在 `src/components/portal/` 下。不要放进 `components/ui/`。

## 8. 主题映射

- **anthropic**：
  - 沿用现有的暖纸色 token（`--background #faf8f5`、`--primary #b5543a` 等），标题用现有的 `font-display`（Source Serif），细字重、字距收紧；
  - 卡片是纸面白底加暖色柔和阴影；
  - eyebrow 用金棕色加呼吸光点。
  - 浅色和 `.dark` 都要能看，dark 下所有颜色都走 token 自动适配。
- **moonshot**：
  - 沿用现有的黑底和玻璃卡片 token，标题字体沿用 moonshot 的覆盖规则；
  - h1 用白到灰的文字渐变；
  - 强调色用 `--chart-3 / --chart-4`（暖白 / 冷蓝灰）。
  - moonshot 下不要在包含玻璃卡片的祖先元素上做 `filter` 动画，入场时的 blur 只能放在内部块上。

## 9. 动效与档位

| 效果 | immersive | lite | still |
|---|---|---|---|
| Hero 逐字升起、区块滚动淡入 | ✓ | ✓（只保留透明度和位移，不做 blur） | 直接显示 |
| 两组跑马灯 | ✓ | ✓ | 静止，横向可滚 |
| 链路光点、核心点灯 | ✓ | 只保留虚线流动 | 静态虚线 |
| 能力卡内循环（路由轮换、审计流、熔断脚本、并发格） | ✓ | ✓ | 停在一个有代表性的静态帧 |
| 步骤自动轮播、终端打字 | ✓ | ✓ | 不自动轮播，终端直接显示完整内容 |
| 卡片悬停上浮、Hero 柔光漂移 | ✓ | ✓ | 无 |

其他要求：

- 所有定时器在组件卸载时清理；页面不可见时（`document.hidden`）暂停 JS 驱动的循环。
- 跑马灯用纯 CSS（内容复制一份，再做 `translateX(-50%)` 的无限动画）。

## 10. 用户决定

1. **moonshot 全页星空：保留**（用户 2026-11-02 明确确认，属于契约「页面背景保持纯色」的用户授权例外）。
   - 范围只限门户页：挂在门户根容器上，用 `html[data-theme="moonshot"]` 加门户专属类名命中，不碰 `body` 和全局 `.page-surface`，控制台、登录、分享页一律不受影响。
   - 实现方式：多层 `radial-gradient` 点阵铺成静态 CSS 背景。不用 canvas、WebGL 或 JS 逐帧绘制，所以不触及契约「没有环境背景运行时」那一条。
   - 档位：`immersive` 下允许极慢的视差或闪烁；`lite` 和 `still` 下只保留静态星点。
   - anthropic 主题不加星空。
2. **Hero 柔光**：两个主题都保留，作为 Hero 区块内部的装饰，不铺满全页。
3. 回写 `docs/agents/web-frontend.md` 动效契约，在「页面背景保持纯色」一条后补一句例外说明：「门户页 moonshot 星空为用户授权例外，限门户根容器、纯 CSS 静态」。这样以后不会被误当成违规删掉。

## 11. 留给实现者

- 区块的具体间距、断点下的折叠方式（移动端两栏改单栏、链路图改纵向）、时长和缓动的微调。
- i18n key 的命名：沿用 `portal.*` 前缀，三语文案由实现者撰写，中文以本文档为准。
- 组件文件怎么拆：门户组件放在 `components/portal/` 下，按区块拆分。

## 11.1 质量底线（来自 frontend-design / impeccable / ui-ux-pro-max / shadcn 规范）

- **可访问性**
  - 装饰性图标加 `aria-hidden`；独立出现的图标（如 GitHub 按钮）要有可访问名称。
  - 键盘焦点可见，悬浮导航不遮挡焦点元素。
  - 正文与背景对比度 ≥ 4.5:1，三种主题（anthropic 浅色、anthropic dark、moonshot）都要单独核对。
- **自动轮播可暂停**
  - 跑马灯在悬停、键盘聚焦时暂停，`still` 档静止。
  - 步骤轮播在用户点击某一步后停止自动切换。
  - 演示动画对读屏器隐藏（`aria-hidden`），关键信息另有文字表达。
- **文案**
  - 按钮文字后面不手工拼「→」，箭头一律用 lucide 图标配 `data-icon="inline-end"`。
  - en / ja 的小标签不用全大写字母。
  - 按钮和链接的文字直接说明点下去会发生什么。
- **shadcn 规范**
  - 使用语义颜色 token；用 `gap-*` 而不是 `space-*`；宽高相等时用 `size-*`。
  - 组件内的图标不加尺寸类。
  - 标签状态用 `Badge`，分隔线用 `Separator`。
- **布局与动效**
  - 交互的按压、悬停效果不改变布局尺寸。
  - 入场动效只在首次进入视口时播放一次。
- **自检**：完成后跑一次 `impeccable detect`，修掉它报的问题。

## 12. 验收

- `npm run lint && npm run test && npm run build`、`npm run format:check` 全部通过。
- `ProviderIcon` 新增 key 的单测通过。
- 用无头浏览器按 1440 / 1280 / 390 三档宽度，各在 anthropic 浅色、anthropic dark、moonshot 三种主题下程序化断言：
  - 无脚本错误；
  - 无横向溢出；
  - 卡片内无文字溢出；
  - 图标无缺失；
  - 链路虚线数 = 客户端数 + 上游数（14 条）。
- 分别在 still / lite 档下确认无自动轮播、无残留 transform。
- 切换 zh / en / ja 时导航和按钮不跳动。
- 用户在浏览器里看实际页面后认可观感。
