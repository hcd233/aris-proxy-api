# Web Frontend

管理后台前端（`web/`，Next.js 静态导出 + Tailwind v4 + shadcn/ui）。本文件是前端领域的词汇表（glossary），只定义术语，不含实现细节。后端领域术语见根目录 `CONTEXT.md`。

## i18n & Layout Stability（国际化与布局稳定性）

**Locale（语言区域）**:
当前激活的 UI 语言，取值 `en` / `zh` / `ja`。首次访问按浏览器语言探测，之后持久化在 `localStorage("locale")`。反映到 `<html lang>` 上，是所有 per-locale 行为的单一事实来源。
_Avoid_: language, lang code

**Switch Flicker（切换闪烁）**:
用户主动切换语言时感知到的布局跳变。根因是元素盒模型尺寸随翻译变化，而非文字内容本身替换。本工作的目标是消除它。
_Avoid_: language flash, transition glitch

**Locale-Stable（语言切换稳定）**:
一个组件在激活 Locale 改变时宽高均不变的属性。是本工作对每个结构性组件的追求目标。
_Avoid_: i18n-safe, translation-proof

**Rigid Element（刚性元素）**:
交互类 UI 骨架元素（动作按钮、徽章、分页触发器等），其宽度按分类预留，跨语言不位移。侧边栏导航项不属此类（侧边栏容器本身定宽）。
_Avoid_: fixed element, locked component

**Elastic Element（弹性元素）**:
承载自然语言文本的块（描述、对话框正文、表格描述列等），随翻译长度自由换行；其高度由布局规则稳定，关键内容永不截断。
_Avoid_: fluid element, auto-size block

**Category Reserve（分类预留）**:
按组件类别统一预留尺寸（如「主操作按钮」「徽章」各一档），而非按每条翻译键单独预留。代价是短翻译语言下出现少量空白，收益是零构建工具、可预测、易维护。
_Avoid_: per-key reservation, dynamic min-width

**Font Scale（字号缩放）**:
对 CJK Locale 等比下调字号，使中/日文字形在相同 utility 下的视觉高度对齐拉丁字母。仅作用于字号，不影响 rem 基准与间距。
_Avoid_: font zoom, cjk shrink

**Layout-Pattern Height Fix（布局高度约定）**:
按布局类型分别约束高度：表格表头不换行 + 单行单元格截断带提示；卡片网格等高 + 描述限两行；对话框正文按最长语言的行数预留最小高度。自由描述文本不受约束。
_Avoid_: row lock, height freeze

## Dataset Page（训练数据页）

**Stepper（向导步骤指示器）**:
Dataset 页面顶部的三段式进度指示器，标记 `Configure → Review → Export` 三个阶段。纯视觉叙事设备，不做导航控制——所有步骤同时可见，用户自由滚动。改筛选条件时统计原地刷新。
_Avoid_: wizard controller, step locker

**Filter-Explore-Export Flow（筛选-探索-导出流）**:
Dataset 页面的用户心智模型。用户先配置筛选条件（Step 1），再查看数据分布和样本预览确认质量（Step 2），最后导出 ShareGPT JSONL（Step 3）。"探索"先于"导出"——统计信息应比导出按钮先进入视野。
_Avoid_: filter-export shortcut

**Export Confirmation Panel（导出确认面板）**:
Step 3 中的导出确认区域，展示筛选摘要（分数阈值、模型、时间范围）、匹配总数、输出格式说明，配合主导出按钮。将导出从"底部浮动按钮"升级为"确认动作"，强化最终步骤的仪式感。
_Avoid_: export bar, download footer

## Trace Install（Trace 安装）

**Trace Install Popover（Trace 安装入口）**:
Trace 页面（`trace/page.tsx`）的安装入口。点击后展示一段简短的安装命令预览：`curl -fsSL <origin>/install.sh | sh`，复制后用户在终端执行。命令不含票据与 API Key；API Key 由用户通过 `aris` CLI 的交互式配置（`aris trace init`）输入，不进入安装脚本。安装脚本由后端 `GET /install.sh` 提供（自包含的 trace 客户端安装脚本，公开访问）。
_Avoid_: codex hook dialog, setup script generator, install wizard

> 已知债务：早期设计（`POST /api/v1/trace/client/ticket` 单次下载票据 + uname 探测 + 原子安装）未落地，当前为简化版；若未来需要约束下载者身份/审计，再按该方案演进。

## Models Page（模型管理页）

**Model Alias（模型别名）**:
模型对外暴露的本地别名，客户端请求时作为 model 参数传入（如 `gpt-4o`）。同一 alias 可关联多个 endpoint 做负载均衡。
_Avoid_: exposed name, local model name

**Model ID（业务模型 ID）**:
模型在网关内唯一的业务标识（默认等于 alias，可更新），用于审计/会话等模块引用模型，区别于数据库主键 `id`。
_Avoid_: model key, business id

**Upstream Model（上游模型名）**:
转发到上游 Endpoint 时真正发送的模型名（如 `gpt-4o-2024-08-06`），区别于对外 Alias。前端字段 `upstreamModel` / 数据库列 `upstream_model`（旧名 `model`/`modelName`，2026-08 统一命名）。
_Avoid_: model name, upstream alias

## Theme（主题皮肤）

**Theme（主题皮肤）**:
整套视觉皮肤的命名集合，取值 `anthropic`（默认，暖色纸质）| `moonshot`（深空科技，固定深色）。持久化于 `localStorage("theme")`，由根布局内联脚本在 hydration 前反映到 `<html data-theme>`，是所有 per-theme 行为的单一事实来源。与明暗模式正交：moonshot 主题不受 `.dark` class 影响。整页根容器携带 `page-surface` 标记类，moonshot 下背景透明以透出星空；小组件不携带。
_Avoid_: color scheme, dark mode toggle
