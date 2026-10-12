# Web 主题动效「双世界」设计（Anthropic 纸与墨 / Moonshot 月光与引力）

> 日期：2026-10-12 ｜ 状态：待评审 ｜ 范围：`web/` 前端 ｜ 分支：`feature/theme-motion-worlds-2026-10-12`

## 1. 背景与目标

现状：两套主题共用一套「Silk Paper」动效（`globals.css` 的 `rise/fade/scale-in/shimmer` + `.stagger-rise`，expo-out 缓动），不区分主题；moonshot 有 Canvas 2D 星空（指针吸引 + 滚动视差）与玻璃拟态；主题切换只有 350ms 颜色过渡（`html.theme-transition`）。

目标：

1. 两套主题拥有**强差异**的动效人格——缓动、时长、进场编排、悬停/按压反馈、专属效果全部独立，切换主题时「手感」一起变。
2. **语言层**：按主题定义动效 token 与通用组件动效（按钮/卡片/弹窗/菜单/Toast/表格/骨架/页面进场），业务页面零改动自动继承。
3. **签名时刻**（5 个）：主题转场、登录页、Dashboard 指标与图表入场、侧栏导航指示器、Playground 流式输出。
4. **环境舞台**：每个主题一个 WebGL 背景场景，背景感知用户注意力。
5. **只做自动降级**，不提供用户动效开关。

## 2. 决策记录

| 议题 | 结论 |
|---|---|
| 覆盖范围 | 语言层 + 签名时刻 |
| 差异度 | 强差异，按最大胆的方向设计 |
| 技术边界 | 原生（CSS / WAAPI / View Transitions / Canvas）+ `motion` 库 + WebGL |
| 兜底策略 | 只自动降级，无用户开关 |
| 整体方向 | 方案 A「双世界」：每个主题独立的 WebGL 场景与动效语法，共享基础设施；背景感知注意力 |
| 被否方案 | B「同一 shader 材质变形」（两种观感互相妥协，组件层只能中等差异）；C「背景克制」（未发挥 WebGL，不符合"最张扬"） |
| WebGL 实现 | 手写极简 WebGL2 渲染器（全屏三角形 + 每场景一个 program），不引入 ogl |
| 触屏设备 | `(pointer: coarse)` 默认归入 `lite` 档 |
| 交付 | 一份 spec，拆 5 个可独立上线的 PR（见 §10） |

## 3. 术语（同步写入 `web/CONTEXT.md`）

- **Motion World（动效世界）**：一个主题的完整动效人格，含 token、组件语法、环境场景与签名时刻。anthropic → `atelier`，moonshot → `lunar`。
- **Motion Tier（动效档位）**：`immersive | lite | still`，反映到 `<html data-motion-tier>`，是所有降级行为的单一事实来源。运行时只降不升。
- **Ambient Stage（环境舞台）**：全屏单 canvas 背景层，承载当前世界的 WebGL 场景或其降级形态。
- **Ambient Presence（环境存在感）**：注意力信号的输出值 `0.35..1`，用户操作时背景退后，空闲时浮现。
- **Ambient Pulse（环境脉冲）**：UI 事件发给舞台的一次性效果请求（墨滴、引力波纹、跃迁、聚焦）。
- **Signature Moment（签名时刻）**：为单个高光场景专门编排的动效序列。

_Avoid_: animation level, background effect, fx

## 4. 架构

### 4.1 分层与分工

原则：**能用 CSS 做的不用 motion，能用 motion 做的不用 WebGL。**

```
<html data-theme="anthropic|moonshot" data-motion-tier="immersive|lite|still">
│
├─ ① 档位判定   内联脚本首帧前用廉价信号判定 → AmbientStage 挂载后补测 WebGL2
│               → 运行时帧率调速器 / context lost 只降不升
│
├─ ② 环境舞台   AmbientStage（替换 ParticleBackground）
│     单 canvas · 单 WebGL2 上下文 · 全屏三角形
│     ├─ atelier 场景（纸纤维 + 台灯斜光 + 墨滴）
│     ├─ lunar 场景（月球 + 三层视差星空 + 星云 + 引力透镜）
│     ├─ 注意力信号 → uPresence
│     └─ 脉冲总线 emitAmbientPulse() → uPulse[4] / uFocus / uIntro
│
├─ ③ 动效语法层（纯 CSS）  motion-anthropic.css / motion-moonshot.css
│     按 [data-theme] 定义 token 与 keyframes；按 data-slot + data-open/data-closed
│     驱动 base-ui 弹层进出场；业务页零改动继承
│
├─ ④ 交互委托层  InteractionLayer（根布局挂载一次）
│     单个文档级 pointermove/pointerdown（passive，rAF 合帧），
│     按 data-slot / data-interactive / data-magnetic 命中目标，
│     写 CSS 变量（光点、倾斜、动态投影）或调用 motion 的 animate()（磁吸弹簧）
│
├─ ⑤ 物理组件   motion/react（LazyMotion + m）：导航指示器 layoutId、MetricNumber
│
└─ ⑥ 签名时刻   主题转场（View Transitions）/ 登录 / Dashboard / 侧栏 / Playground
```

### 4.2 档位判定

```
still     ← prefers-reduced-motion: reduce
lite      ← navigator.connection.saveData
            || navigator.deviceMemory ≤ 4
            || navigator.hardwareConcurrency ≤ 4
            || (pointer: coarse)
            || WebGL2 不可用（AmbientStage 挂载时补测）
immersive ← 其余
```

- 内联脚本（与 `THEME_INIT_SCRIPT` 同处、同模式）只用廉价信号，首帧前写入 `data-motion-tier`；WebGL2 探测放在 AmbientStage 挂载时，失败则降为 `lite`。
- 判定为纯函数 `resolveMotionTier(signals)`；内联脚本 `MOTION_TIER_INIT_SCRIPT` 是它的手写 ES5 镜像（生产压缩使 `toString` 不可靠），由全组合对拍单测保证一致。
- 运行时降级来源：帧率调速器、`webglcontextlost`、`prefers-reduced-motion` 变为 `reduce`（→ `still`）。**只降不升**，恢复要等刷新页面，避免档位来回抖动。

### 4.3 环境舞台 AmbientStage

- 替换 `components/theme/particle-background.tsx`，挂载位置不变（根布局），容器沿用 `.particle-bg` 的 `fixed inset-0 -z-10 pointer-events-none`，改名 `.ambient-stage`。
- **渲染器 `gl.ts`**：WebGL2、单个全屏三角形、每个场景一个 fragment program，首次切换到该场景时编译并缓存。
- **公共 uniform**：`uTime`、`uResolution`、`uPointer`（平滑后）、`uScroll`、`uPresence`、`uPulse[4]`（vec4：x, y, 起始时间, 类型）、`uFocus`（0..1）、`uIntro`（0..1，台灯亮起 / 月出）。
- **渲染分辨率**：默认 0.75× DPR（背景本身柔焦，画质损失不可见）。
- **帧率**：环境态封顶 30fps；脉冲与 intro 动画期间临时提到 60fps。
- **帧率调速器**：监测浏览器 rAF 节奏（与自身 30fps 封顶无关）。滚动 3s 平均低于 45fps → 渲染分辨率降到 0.5×；再持续 3s 仍低于 45fps → 降到 `lite`。
- **暂停**：`document.hidden` 时停止 rAF。
- **注意力信号 `attention.ts`**：
  - `keydown / input / wheel / scroll / pointerdown` 视为「操作中」，`uPresence` 在 300ms 内降到 0.35；
  - 空闲 8s 后用 2s 回升到 1；
  - `pointermove` 不算操作（它驱动台灯跟随与视差）。
- **脉冲总线**：模块级 `emitAmbientPulse({ kind, x, y })`，不经 React context。kind 取值：`primary`（主按钮点击）、`route`（导航跳转）、`focus-on` / `focus-off`（弹窗开合）、`intro`（场景入场）。各场景自行映射：
  - atelier：`primary` / `route` → 墨滴；
  - lunar：`primary` → 引力波纹，`route` → 跃迁。

### 4.4 文件组织

| 路径 | 内容 |
|---|---|
| `src/lib/motion/tier.ts` | `resolveMotionTier` 纯函数、`MOTION_TIER_INIT_SCRIPT`、运行时降级 API |
| `src/lib/motion/tokens.ts` | 两个世界的弹簧参数、时长常量（JS 侧唯一来源） |
| `src/lib/motion/attention.ts` | 注意力信号状态机（纯函数 reducer + 事件绑定） |
| `src/lib/motion/pulse.ts` | `emitAmbientPulse` 总线 |
| `src/lib/motion/fps-governor.ts` | 帧率调速器状态机（纯函数） |
| `src/components/ambient/ambient-stage.tsx` | 舞台组件、档位联动、场景切换 |
| `src/components/ambient/gl.ts` | 极简 WebGL2 渲染器 |
| `src/components/ambient/scenes/atelier.ts`、`lunar.ts` | shader 源码字符串 + uniform 映射 |
| `src/components/ambient/fallback-2d.ts` | 现有 Canvas 2D 星空迁移而来（lunar 的 lite / still） |
| `src/components/motion/interaction-layer.tsx` | 交互委托层 |
| `src/components/motion/metric-number.tsx` | 替代 `count-up.tsx` |
| `src/components/motion/split-text.tsx` | 逐字动画（读屏安全） |
| `src/styles/motion-base.css`、`motion-anthropic.css`、`motion-moonshot.css` | 动效语法层，由 `globals.css` 引入；现有 Silk Paper 规则迁入 `motion-anthropic.css` |

新依赖仅 `motion@^14`（peer 支持 React 19）。

### 4.5 连带改动

1. anthropic 下 `.page-surface` 也改为透明（三档一致：`immersive` 透出 GL 纸面，`lite / still` 透出 `body` 上的静态纸纹）；GL 底色严格等于 `--background`，GL 失效时视觉无断层。卡片保持不透明白底（纸上叠纸）。
2. `docs/agents/web-frontend.md` 中「不要引入 View Transitions API」收窄为「语言切换不使用 View Transitions」；主题切换允许使用。
3. `components/ui/` 下弹层组件（dialog / alert-dialog / sheet / popover / dropdown-menu / select / tooltip）删除 tw-animate 的 `data-open:animate-in …` / `data-closed:animate-out …` 类，进出场改由动效语法层按 `data-slot` 定义；`Button` 增加 `data-variant={variant}` 属性，供委托层与 CSS 命中。
4. 现有全局 `prefers-reduced-motion` 守卫保留，作为最底层保险。

## 5. Anthropic「纸与墨 · Atelier」

**性格**：东西落到纸上，墨水在纸里晕开，什么都不回弹。物理隐喻来自三种材质：纸（抬起、投影）、墨（晕染、描线）、活字（压印）。

### 5.1 Token

| 类别 | 值 |
|---|---|
| 缓动 | `--ease-settle: cubic-bezier(.22,1,.36,1)`（进场）· `--ease-ink: cubic-bezier(.65,0,.35,1)`（描线）· `--ease-lift: cubic-bezier(.4,0,1,1)`（退场） |
| 时长 | 压印 90ms · 微交互 180ms · 基础 320ms · 进场 520ms · 描线 700ms；退场取进场的 60% |
| 弹簧（JS） | `stiffness 260 / damping 32 / mass 1`（ζ≈0.99，临界阻尼、零过冲） |
| 位移 / 错峰 | 进场上浮 8px；子项错峰 40ms，最多 8 项 |

### 5.2 组件语汇

| 组件 | 动效 |
|---|---|
| 页面进场「落纸」 | 内容上浮 8px + 淡入；页面标题额外「墨迹定影」`blur(1.5px)→0` |
| 按钮「活字压印」 | 按下 `translateY(1px)` + 内阴影 `inset 0 1px 2px rgb(31 26 20/.18)`，外阴影收起；ghost/outline 悬停时底色以 `::before scaleX` 从左墨洗铺开；主按钮点击 → `emitAmbientPulse('primary')` 落墨 |
| 链接与可点文本「手绘下划线」 | `main a:not([data-slot=button])` 与 `[data-variant=link]`：悬停时带抖动的 SVG 手绘线以 `scaleX`（origin-left）画出 |
| 卡片「纸张抬起」 | 悬停上浮 2px，阴影 xs→md；投影方向随台灯（§5.3 动态投影） |
| 弹窗「纸片落桌」 | 进场 `translateY(12px) scale(.985) rotate(-.4deg)` → 落定，阴影长出，420ms；退场上抬 6px + 淡出，220ms；遮罩暖褐 18% + `blur(2px)` |
| 菜单 / Select / Popover「纸张展开」 | 从触发侧以 `clip-path: inset()` 展开，260ms；前 8 项错峰 18ms |
| Tooltip | 淡入 + 上浮 2px，140ms，不缩放 |
| Toast「便签」 | 右侧滑入 16px + 0.8° 歪斜后摆正，420ms；关闭 200ms。通过 sonner 的 `data-mounted` / `data-removed` 覆写，保留其堆叠逻辑 |
| 表格「印刷」 | 行挂载时逐行显现（错峰 16ms，`nth-child` 上限 12）；悬停行左侧 2px 墨条 `scaleY` 进入；翻页即新行挂载，自然复用 |
| 骨架屏「墨洗」 | 2.2s 暖色慢速晕染，替换冷色扫光 |
| Switch | 滑块临界阻尼；轨道从滑块位置墨水铺开 |
| 焦点环 | `outline-offset` 4px→2px 收紧，150ms |

### 5.3 WebGL 场景 `atelier`

- **纸面**：fbm 各向异性纤维噪声作高度图 → 法线 → Lambert；亮度起伏 ±3%，6% 暖色暗角，dither 防色带；底色 = `--background`。
- **台灯**：光源方向跟随 `uPointer`，约 1.2s 临界阻尼滞后；空闲时 ±2° 缓慢呼吸。
- **墨滴**：最多 4 滴并存；域扭曲噪声边缘的扩散环，峰值透明度 6%，色相取 `--primary` 混暖褐；1.4s 晕开，留极淡墨痕 6s 干透。
- **台灯亮起（intro）**：`uIntro` 0→1，800ms，用于主题转场后与登录页。
- **动态投影**：指针停顿 120ms 后，把台灯方向写入 `:root` 的 `--light-x / --light-y`；`[data-slot=card]`、弹窗、Popover 的投影偏移（≤ ±3px）用 600ms 过渡转向。只在停顿时更新，样式重算极少；`lite` 档关闭。
- **注意力**：`uPresence` 低时台灯停止跟随、墨滴变淡。
- **lite / still**：`.ambient-stage` 上叠一层静态 SVG `feTurbulence` 纸纹（低透明度），无动画。

**可读性底线**：卡片不透明；直接落在纸面的文字所在区域亮度起伏 ≤ 3%，对比度 ≥ 4.5:1。

## 6. Moonshot「月光与引力 · Lunar Orbit」

**性格**：元素悬浮在引力场里，光把它们从黑暗中显影。核心语汇：景深（对焦）、引力（磁吸、倾斜、带过冲弹簧）、光（光扫、辉光、显影）。

### 6.1 Token

| 类别 | 值 |
|---|---|
| 缓动 | `--ease-drift: cubic-bezier(.16,1,.3,1)`（长尾进场）· `--ease-orbit`（CSS `linear()` 模拟欠阻尼弹簧：对 `stiffness 170 / damping 18` 的阶跃响应等间隔采样 40 点生成，约 5% 过冲；生成脚本与单测共用同一采样函数）· `--ease-fall: cubic-bezier(.7,0,.84,0)`（退场） |
| 时长 | 微交互 200ms · 基础 380ms · 进场 760ms · 环境 ≥ 1200ms；退场取进场的 60% |
| 弹簧（JS） | 通用 `170 / 18`（ζ≈0.69，约 5% 过冲）；磁吸 `300 / 20`（ζ≈0.58，约 10% 过冲） |
| 景深 | 进场起点 `scale(.97) blur(8px) opacity 0`；子项错峰 60ms，最多 8 项 |

### 6.2 组件语汇

| 组件 | 动效 |
|---|---|
| 页面进场「对焦」 | 从深空模糊对焦到清晰；`lite` 档去掉 blur，只留缩放 + 淡入 |
| 按钮「引力磁吸」 | 命中 `[data-slot=button][data-variant=default]` 与 `[data-magnetic]`：指针进入 60px 被吸近（≤ 4px，磁吸弹簧），文字反向 1.5px 视差；按下 `scale(.96)` 过冲回弹；点击自点击点扩散白色冲击波环（500ms）+ `emitAmbientPulse('primary')`。ghost 按钮内部有跟随指针的柔光点（`--mx/--my`）。仅 `(hover: hover) and (pointer: fine)` |
| 卡片「月面倾斜 + 光扫边框」 | 所有卡片：边框高光随指针靠近亮起（`@property --angle` 驱动 conic 渐变，悬停进入时扫一圈 1.2s）。仅 `[data-interactive]` 卡片（统计卡、上游分组卡）做 3D 倾斜（≤ 4°，perspective 900px）+ 镜面高光（白 6%）。表格卡与长文本卡不倾斜 |
| 弹窗「从黑暗中浮现」 | 进场 `scale(.92) blur(16px)` → 对焦（`--ease-orbit`），520ms，同时 `emitAmbientPulse('focus-on')`（月球变暗 40%、星空减速）；退场坠回深空 240ms；遮罩黑 55% + `blur(6px)` |
| 菜单 / Select / Popover「光束展开」 | 从触发点 `scaleY(.9)` 展开，前沿一条 1px 白光扫过，280ms；悬停项左侧白光条 |
| Tooltip | `scale(.96)` 淡入 + 2px 辉光，160ms |
| Toast「彗星」 | 右侧切入，身后光尾先拉长后收缩，600ms；关闭时变暗滑出 |
| 表格「扫描显影」 | 容器挂载时水平扫描线自上而下扫一次，行随之显影（错峰 20ms，上限 12）；悬停行微提亮 + 左侧 1px 光线 |
| 骨架屏「遥测扫描」 | 暗色占位上细亮扫描线循环横扫，1.4s |
| Switch | 滑块过冲弹到位；打开后滑块白色光晕，轨道由内亮起 |
| 焦点环 | 白 35% 的 2px 环 + 8px 外发光，聚焦瞬间脉冲一次（300ms） |

### 6.3 WebGL 场景 `lunar`

- **月球**：屏幕空间解析球体（`z = √(1−r²)` 求法线，无需光线步进），位于右下角，半径约 0.9 倍视口高，裁切后露出约 40%；多层 Voronoi + fbm 陨坑法线扰动，月海暗斑。
- **光照**：太阳方向约 90s 周期漂移并轻微响应指针；软明暗分界线、背光面极淡地照、边缘轮廓光；亮面染 `#FFEED9`，最高亮度封顶 0.22。
- **星空**：shader 内三层哈希网格星点 + 闪烁；三层按 ±12 / 6 / 3px 做指针深度视差与滚动视差；指针处**引力透镜**（附近星点沿径向轻微偏折）。
- **星云**：冷蓝灰 `#A8BDD8`（4%）+ 奶油色（3%）fbm 云，缓慢漂移。
- **脉冲**：`route` → 跃迁（星点沿径向拉丝 450ms 后回弹）；`primary` → 点击点引力波纹 600ms；`focus-on/off` → 聚焦态。
- **月出（intro）**：`uIntro` 0→1，月球从下缘升起，1.2s（登录页 1.4s）。
- **注意力**：`uPresence` 低时月球变暗 30%、星空减速 50%、关闭透镜；空闲后月光缓慢呼吸。
- **lite**：`fallback-2d.ts`（现有 Canvas 2D 星空）+ CSS 径向渐变静态月影。**still**：Canvas 2D 静态一帧 + 静态月影。

**可读性底线**：月球只在右下角，左上标题区保持纯黑；卡片保持现有玻璃拟态（`blur(23px) brightness(.6)`）；直接落在背景上的文字对比度 ≥ 4.5:1。

## 7. 签名时刻

### 7.1 主题转场「日蚀 / 墨染」（约 1100ms）

- **入口**：`setTheme(next, origin?)`，`ThemeSwitcher` 传入按钮中心坐标。支持 View Transitions 且档位不是 `still` 时：
  1. 在 `:root` 写入 `--vt-x / --vt-y / --vt-r`（`--vt-r` = 原点到最远屏幕角的距离），并写入 `data-vt="eclipse|ink"`；
  2. 调用 `document.startViewTransition(() => flushSync(() => { setThemeState(next); mountEdgeLayer(); }))`；
  3. 等 `transition.finished` 后移除 `data-vt` 与边缘层。
  4. 连点时对进行中的转场调用 `skipTransition()`，保证可中断。
- **边缘层**：一个仅在新状态存在的全屏元素，带独立 `view-transition-name: vt-edge`。View Transition 的新画面是实时渲染的，所以可用 `@property --r` 动画驱动边缘半径，不存在大位图放大问题。
- **切到 moonshot「日蚀」**：`::view-transition-new(root)` 以 `clip-path: circle(var(--r) at var(--vt-x) var(--vt-y))` 扩张（`--ease-drift`）；边缘层是白色日冕光环；随后场景 intro 触发月出。
- **切到 anthropic「墨染」**：新画面以 60px 羽化径向 `mask-image` 揭开；边缘层是一圈暖褐色「湿墨带」（SVG `feTurbulence` 纤维纹理作背景、环形渐变遮罩）；随后台灯亮起。
- **降级**：不支持 View Transitions 或 `still` 档 → 现有 `theme-transition` 350ms 颜色过渡；`lite` 档 → 只做圆形揭开，不加边缘层。
- **语言切换不变**：仍由 `LocaleFade` 负责，不使用 View Transitions。

### 7.2 登录页

| | anthropic「封面压印」（约 1.4s） | moonshot「月出」（约 1.8s） |
|---|---|---|
| 编排 | 台灯亮起 → 品牌标压印（`scale 1.08→1`，同位置落墨）→ 衬线标题逐字「排字」（错峰 24ms，上浮 + 定影）→ 描述 → OAuth 按钮依次落定 | 星空淡入 → 月出（分界光扫过月面）→ 标题逐字对焦（错峰 30ms，`blur(10px)→0`）→ 登录卡从深空浮现 + 边框光扫一圈 → OAuth 按钮带磁吸 |
| 点击 OAuth | 压印 + 墨滴；跳转前按钮下方循环描线作为等待反馈 | 跃迁拉丝作为「发射」 |

- 跳转立即发起，**不为动效延迟**。
- 同一会话再次访问时编排压缩到 40% 时长（`sessionStorage("motion.login.seen")`）。
- `SplitText`：用 `Intl.Segmenter` 按字素切分（CJK 安全）；完整文本放在 `sr-only` span，逐字 span 设 `aria-hidden`。

### 7.3 Dashboard 指标与图表

- **编排**：页头 → 统计卡错峰 → 图表区。统计卡标记 `data-interactive`。
- **`MetricNumber`**（替代 `CountUp`，`tabular-nums`）：
  - anthropic「里程表」：每一位是竖向 0–9 数字带，用 `translateY` 滚动，从右到左错峰 30ms，临界阻尼；刷新时只滚动变化的位，方向随增减。
  - moonshot「解码锁定」：随机跳字后从左到右逐位锁定，锁定位闪一下白光（单次，衰减 300ms）；刷新时只重解码变化的位（300ms）。
  - `still` 档直接显示终值。实现沿用 `CountUp` 的 ref + DOM 直写模式，避免 setState-in-effect。
- **图表**：
  - 首次入场用自定义描线：给 recharts 线条路径注入 `pathLength`，用 CSS `stroke-dashoffset` 描线；无法注入时在挂载后用 `getTotalLength()` 测量。
  - 之后的数据更新沿用 recharts 自带过渡（`line-chart-card.tsx` 现有 600ms）。
  - anthropic：钢笔描线 900ms（`--ease-ink`）→ 面积区墨水渗入；柱子从基线临界阻尼升起；hover 十字线是铅笔细线。
  - moonshot：「点火」描线，带发光头和强辉光，画完后辉光衰减到常态；面积区以星云渐变显现；柱子过冲升起、顶边发光；十字线是光束，数据点带光晕。

### 7.4 侧栏导航指示器

- 激活项内渲染 `m.span layoutId="nav-indicator"`，layout 特性通过 `LazyMotion` 懒加载；桌面侧栏与移动 Sheet 各包一个 `LayoutGroup`，互不串扰。
- **anthropic「荧光笔」**：边缘不规则的暖色马克笔痕（SVG `mask-image`），临界阻尼滑动，落定时从左到右刷出一笔。
- **moonshot「轨道」**：发光玻璃胶囊 + 1px 内光，欠阻尼弹簧滑动并略冲过头；另有刚度更低的 `layoutId="nav-indicator-trail"` 余辉副本（透明度 35%）形成彗尾；激活图标白色辉光。
- **分组展开**：遵守「宽高不加过渡」契约，容器瞬时展开，子项按各自世界的语汇错峰显现；收起时反向执行，速度更快。
- `route` 脉冲由 AmbientStage 在 `pathname` 变化时自动发出，位置取 1s 内最后一次 `pointerdown`（导航项点击即落在该项上），无近期点击时取视口中心。

### 7.5 Playground 流式输出

- **约束**：`MarkdownLite` 每个 chunk 都整体重新解析，不能逐 token 动画。统一采用「尾部」方案：流式中的 assistant 内容容器带 `data-streaming`，在其最后一个文本块（列表取最后一个 `li`；代码块 / 表格放在块后）的 `::after` 上同时承载两样东西：
  - **尾部渐变**：3em 宽，用 `margin-left: -3em` 叠在最后几个字上；
  - **光标**：`border-right` 画出，闪烁只动画 `border-right-color`。
- **anthropic「湿墨」**：
  - 尾部叠暖色渐变（`mix-blend-mode: multiply`），最新几个字像湿墨，光标前移后自然「干透」；
  - 光标是 2px 宽的墨条，做「蘸墨」式明暗闪烁；
  - 思考中显示三个依次点下的墨点；
  - 完成时湿墨 600ms 干透。
- **moonshot「显影」**：
  - 尾部叠白色辉光（`mix-blend-mode: plus-lighter`），新字像刚显影；
  - 光标是会脉冲的光点；
  - 思考中显示一个绕环运行的小卫星；
  - 完成时辉光消退，消息边框扫过一道光。
- **新一轮对话进场**：anthropic 便签上滑；moonshot 从深空对焦浮现（替换 `chat-turn.tsx` 现有的 `animate-in fade-in slide-in-from-bottom-1`）。

## 8. 性能、无障碍与容错

### 8.1 性能预算

| 项 | 预算 | 保证手段 |
|---|---|---|
| 包体积增量 | gzip 后合计 ≤ 40KB | `LazyMotion` + `m`：首屏约 6KB，`domAnimation` 约 15KB 懒加载，`domMax`（layout）约 10KB 懒加载；GL 渲染器 + 两套 shader ≤ 12KB。构建后对比 `out/_next/static/chunks` 前后体积 |
| GL 单帧 | M1 级集显 ≤ 4ms | 0.75× DPR；环境态 30fps 封顶；JS 每帧只更新 uniform（≤ 1ms） |
| 调速器 | 3s 平均 < 45fps → 0.5× → 再 3s → `lite` | 监测 rAF 节奏 |
| 布局 | CLS 增量为 0 | 只动画 `transform / opacity / filter / clip-path / mask`；CSS 变量写入节流 |
| 交互 | 不增加 INP | 磁吸、倾斜走委托层 + motion `animate()`，不触发 React 重渲染 |

已知成本：moonshot 玻璃卡片的 `backdrop-filter` 叠在动态 canvas 上，每帧需要重新模糊（现状已存在）；30fps 封顶使其减半，调速器兜底。

### 8.2 无障碍

- `still` 档对应 OS 减弱动态，全局守卫保留作底线。
- 不出现每秒超过 3 次的闪烁（WCAG 2.3.1）；跃迁、视差幅度有上限，`still` 档全部关闭。
- 动画不抢焦点，焦点环始终可见；逐字动画对读屏器透明（§7.2）。
- 磁吸 / 倾斜 / 光点只在 `(hover: hover) and (pointer: fine)` 设备启用；所有交互都不依赖 hover 才能完成。

### 8.3 容错

| 故障 | 处理 |
|---|---|
| WebGL 初始化失败 / shader 编译错误 | `console.warn` 一次，降为 `lite` |
| 运行中 `webglcontextlost` | 降为 `lite`，本会话不再重试 |
| 不支持 View Transitions 或调用抛错 | 直接切换主题 + 现有颜色过渡 |
| motion 懒加载特性失败 | `m` 组件按静态渲染 |
| sonner 覆写与其堆叠逻辑冲突 | 退化为只改缓动与时长，不改位移 |

## 9. 验证

1. **vitest 单测**（纯函数）：
   - `resolveMotionTier`：减弱动态 / 省流量 / 内存 / 核数 / 粗指针 / WebGL2 各输入组合 → 档位；
   - `fps-governor`：帧样本序列 → 动作（保持 / 降分辨率 / 降档），只降不升；
   - `attention`：事件序列 + 时间推进 → `uPresence` 取值；
   - `MetricNumber` 变位计算：新旧值 → 需要滚动的位与方向；
   - `--ease-orbit` 的 `linear()` 采样：过冲峰值在 4%–6%。
2. `npm run lint && npm run test && npm run build && npm run format:check`，并对比构建产物体积增量 ≤ 40KB（gzip）。
3. **程序化断言**（chrome-headless-shell + swiftshader）：
   - 注入脚本编译两套 shader，把编译日志输出到 DOM 并提取断言无错误；
   - `--virtual-time-budget` 快进到动画结束，断言两主题 × 三档位下没有元素停在 `opacity: 0`，`data-motion-tier` 判定符合预期。
4. **next-dev-loop**：`get_errors` 确认运行时零报错。
5. **真机手感复验**（用户执行，附检查清单）：缓动手感、两主题辨识度、长时间停留的可读性、风扇与耗电、触屏设备进入 `lite`。

## 10. 交付拆分

| PR | 内容 | 验收 |
|---|---|---|
| 1 地基 | 档位判定 + 内联脚本、注意力信号、脉冲总线、调速器、AmbientStage + `gl.ts` + atelier / lunar 场景 + 降级形态；替换 ParticleBackground；anthropic `.page-surface` 透明 | 单测通过；两场景 shader 编译无错；三档位渲染正确 |
| 2 组件动效 | `motion-*.css` 两套语法层；ui 弹层组件移除 tw-animate 类；`Button` 加 `data-variant`；InteractionLayer（光点、倾斜、磁吸、动态投影）；页面进场与 stagger 按主题切换 | 业务页零改动生效；无元素卡在 opacity 0；CLS 无增量 |
| 3 转场 · 登录 · 导航 | View Transitions 主题转场 + 边缘层；登录页编排 + `SplitText`；侧栏 `layoutId` 指示器 | 转场可中断且降级正确；读屏器读到完整标题 |
| 4 Dashboard | `MetricNumber` 替代 `CountUp`；图表首次入场描线 | 刷新只动变化位；数据更新沿用 recharts 过渡 |
| 5 Playground | 流式尾部湿墨 / 显影、光标、思考态、完成态、新一轮进场 | 长文本流式无掉帧；完成后无残留伪元素 |

每个 PR 同步更新 `web/CONTEXT.md` 术语与 `docs/agents/web-frontend.md` 中受影响的契约条目。

## 11. 不做的事（YAGNI）

- 用户动效开关（已明确不要）。
- 音效、第三套主题、anthropic 暗色入口。
- share 页专属签名时刻（自动继承组件动效与环境舞台）。
- 逐 token 的流式动画（受 `MarkdownLite` 整体重解析约束，用尾部方案替代）。
- 侧栏分组的高度动画（遵守宽高不加过渡的契约）。

## 12. 实现偏差记录

| PR | 偏差 | 原因 |
|---|---|---|
| 1 | 内联脚本改为 `resolveMotionTier` 的手写镜像 + 对拍单测 | 生产压缩使 `Function.prototype.toString` 不可靠 |
| 1 | 档位内联脚本独立成第二个 `<script>`，不与 `THEME_INIT_SCRIPT` 拼接 | 后者来自 `"use client"` 模块，在 server component 中是客户端引用，拼接会 stringify 成抛错函数（运行时矩阵验证捕获） |
| 1 | 档位 DOM 存储与 hook 拆到 `tier-store.ts` | 保持 `tier.ts` 可在 node 下单测 |
| 1 | `route` 脉冲由舞台监听路由自动发出 | 覆盖所有跳转来源，PR3 无需接线 |
| 1 | 新增 `uDrift` 漂移时钟 uniform | 操作中减速、聚焦时再减速，避免缩放 `uTime` 造成跳变 |
| 1 | atelier 降级纸纹挂在 `.ambient-stage` 而非 `body` | 与 GL 场景同层，切换档位不叠加两层纹理 |
| 2 | 磁吸弹簧用自写 `springStep`（`lib/motion/spring.ts`），PR2 不引入 motion | 逐帧积分只需二十行；motion 仅在确有布局动画需求的 PR3（`layoutId`）引入 |
| 2 | anthropic 只有 `data-interactive` 卡片悬停上浮，其余卡片只加深阴影 | 大量 Card 包裹表格，悬停时整块位移会干扰阅读（与 moonshot「表格卡不倾斜」同理） |
| 2 | 上游分组块不标 `data-interactive` | 它不是 Card 且内含列表，倾斜/上浮会干扰阅读；只标记 Dashboard 统计卡 |
| 2 | ghost/outline「墨洗从左铺开」简化为墨色缓动的背景过渡 | 伪元素铺色会与各处自定义的 `hover:bg-*` 颜色冲突 |
| 2 | moonshot 页面包裹层用独立的 `--animate-page`（透明度 + 缩放，无 blur） | 祖先元素上的 filter 会成为玻璃卡片的 backdrop root，整个入场期间玻璃失效；blur 改放在内部块上 |
| 2 | 表格悬停墨条 / 光线用首格 inset 阴影淡入，而非 `scaleY` | `tr` 上的伪元素定位不可靠，inset 阴影零布局成本 |
| 2 | Switch 轨道「从滑块位置铺开」简化为缓动颜色过渡 | Switch 根节点的 `::after` 已用作扩大点击区域 |
| 3 | 侧栏指示器用自写 FLIP + `springStep`，不引入 motion 的 `layoutId` | 单个元素只需追踪纵向位置，二十余行即可，且直接复用两套世界的弹簧 token；整个特性因此零新增依赖 |
| 3 | 侧栏分组收起不做退场动画 | 条件渲染卸载无法做退场，补退场需要引入 AnimatePresence 类机制，收益小 |
| 3 | 登录页「再次访问压缩到 40%」由页内首帧脚本写 `html[data-login-seen]` | 静态导出的 HTML 在 hydration 前就开始播放动画，hydration 后再改节奏会中途跳变 |
| 3 | Google / Demo 按钮标 `data-magnetic` | 让三个 OAuth 入口在 moonshot 下都有磁吸与冲击波、在 anthropic 下都落墨，与 spec「OAuth 按钮带磁吸」一致 |
| 4 | 图表描线沿用 recharts 自带的描线动画，时长统一 600ms，仅按世界切换缓动 | recharts 的入场与数据更新共用同一时长，单独拉长入场需要额外状态；描线本身就是 recharts Line 的默认入场 |
| 4 | moonshot 线条的「发光头」未实现，只做点火辉光衰减 | recharts 不暴露逐帧路径进度，发光头需自绘覆盖层，收益与复杂度不匹配 |
| 4 | 「柱子升起」落在 Dashboard 的自定义占比条（横向，`scaleX` 自左生长） | Dashboard 的柱状信息由 StackedRatioBar 呈现，没有 recharts Bar |
| 4 | `still` 档关闭 recharts 动画 | recharts 动画在 JS 中执行，全局 reduced-motion 守卫无效 |
