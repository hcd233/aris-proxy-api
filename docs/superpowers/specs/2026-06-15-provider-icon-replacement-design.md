# Provider 图标替换设计

## 背景

前端管理界面中，Endpoints 和 Audit 页面使用纯文字枚举值标识 OpenAI / Anthropic 协议，阅读负担较重。用户希望将相关 provider 标识替换为品牌图标，在保持信息密度的同时弱化文字负担。

## 目标

1. 在 Endpoints 页面用 OpenAI / Anthropic 品牌图标替换 provider 相关文字。
2. 在 Audit 页面用 OpenAI / Anthropic 品牌图标替换 protocol 列的 provider 文字。
3. 保持现有 claude.ai 风格的整体界面设计不变。
4. 保证可访问性（aria-label / title）。

## 非目标

- 不改动其他管理表格页（Sessions、API Keys、Models、Shares、Blocked）。
- 不改动能按钮、表头、分页器等其他文字。
- 不引入新的 npm 图标库；品牌图标以内联 SVG 组件形式存在。

## 图标规范

使用用户确认的官方品牌 logo SVG：

### OpenAI Logo

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" fill="currentColor">
  <path d="M224.32 114.24a56 56 0 0 0-60.07-76.57a56 56 0 0 0-96.32 13.77a56 56 0 0 0-36.25 90.32A56 56 0 0 0 69 217a56.4 56.4 0 0 0 14.59 2a56 56 0 0 0 8.17-.61a56 56 0 0 0 96.31-13.78a56 56 0 0 0 36.25-90.32Zm-41.47-59.81a40 40 0 0 1 28.56 48a51 51 0 0 0-2.91-1.81L164 74.88a8 8 0 0 0-8 0l-44 25.41V81.81l40.5-23.38a39.76 39.76 0 0 1 30.35-4M144 137.24l-16 9.24l-16-9.24v-18.48l16-9.24l16 9.24ZM80 72a40 40 0 0 1 67.53-29c-1 .51-2 1-3 1.62L100 70.27a8 8 0 0 0-4 6.92V128l-16-9.24ZM40.86 86.93a39.75 39.75 0 0 1 23.26-18.36A56 56 0 0 0 64 72v51.38a8 8 0 0 0 4 6.93l44 25.4L96 165l-40.5-23.43a40 40 0 0 1-14.64-54.64m32.29 114.64a40 40 0 0 1-28.56-48c.95.63 1.91 1.24 2.91 1.81L92 181.12a8 8 0 0 0 8 0l44-25.41v18.48l-40.5 23.38a39.76 39.76 0 0 1-30.35 4M176 184a40 40 0 0 1-67.52 29.05c1-.51 2-1.05 3-1.63L156 185.73a8 8 0 0 0 4-6.92V128l16 9.24Zm39.14-14.93a39.75 39.75 0 0 1-23.26 18.36c.07-1.14.12-2.28.12-3.43v-51.38a8 8 0 0 0-4-6.93l-44-25.4l16-9.24l40.5 23.38a40 40 0 0 1 14.64 54.64"/>
</svg>
```

### Anthropic Logo

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">
  <path d="M17.304 3.541h-3.672l6.696 16.918H24Zm-10.608 0L0 20.459h3.744l1.37-3.553h7.005l1.369 3.553h3.744L10.536 3.541Zm-.371 10.223L8.616 7.82l2.291 5.945Z"/>
</svg>
```

### 组件化

新增 `web/src/components/provider-icon.tsx`：

```tsx
export function OpenAIIcon({ className }: { className?: string }) { ... }
export function AnthropicIcon({ className }: { className?: string }) { ... }
export function ProviderIcon({ protocol, className }: { protocol: string; className?: string }) { ... }
```

`ProviderIcon` 根据 protocol 字符串前缀匹配：
- `openai-*` → `<OpenAIIcon />`
- `anthropic-*` → `<AnthropicIcon />`
- 其他 → 保留原文字或 fallback

尺寸规范：
- 表格内：默认 `size-4`（16px）
- 需要与文字并排时：额外加 `shrink-0`
- 颜色：继承 `currentColor`，跟随父元素文字色

## 具体改动

### 1. Endpoints 页面（`web/src/app/(dashboard)/endpoints/page.tsx`）

#### 1.1 Supported APIs 列

**当前**：Badge 内文字
- `OpenAI / Chat Completions`
- `OpenAI / Response`
- `Anthropic / Messages`

**优化后**：图标 + 短文字
- `<OpenAIIcon /> Chat Completions`
- `<OpenAIIcon /> Response`
- `<AnthropicIcon /> Messages`

同时影响：
- 桌面端 Table
- 移动端卡片（`isMobile` 分支）

#### 1.2 Base URL 列

**当前**：
```
O: https://api.openai.com/v1
A: https://api.anthropic.com
```

**优化后**：
```
<OpenAIIcon /> https://api.openai.com/v1
<AnthropicIcon /> https://api.anthropic.com
```

需要移除 `O:` / `A:` 文字前缀，保留 tooltip（已经存在）。

### 2. Audit 页面（`web/src/app/(dashboard)/audit/page.tsx`）

#### 2.1 Protocol 列

**当前**：
```
<div className="text-xs">{log.apiProtocol || "—"}</div>
<div className="text-xs text-muted-foreground/70">{log.upstreamProtocol || "—"}</div>
```

显示例如 `openai-chat-completion`、`anthropic-message`。

**优化后**：
- `apiProtocol` 前加 `<ProviderIcon protocol={log.apiProtocol} />`
- `upstreamProtocol` 前加 `<ProviderIcon protocol={log.upstreamProtocol} />`
- 保留协议后缀文字（`chat-completion`、`response`、`message`），避免信息丢失。

同时影响：
- 桌面端 Table
- 移动端展开详情中的 "Upstream" / "API Protocol" 字段

## 可访问性

- icon-only 或 icon + 文字组合中，文字本身已说明含义，无需额外 aria-label。
- 若某处仅展示 icon 而无文字，必须加 `title` 和 `aria-label`。
- 不改动现有 tooltip 行为。

## 视觉验收标准

- Endpoints 表格中 Supported APIs 列显示图标 + 短文字，排列整齐。
- Endpoints Base URL 列图标与 URL 对齐，无 `O:` / `A:` 前缀。
- Audit Protocol 列图标与协议文字对齐，未识别 protocol 仍显示原文字。
- 移动端卡片视图与桌面端保持一致风格。
- 暗色模式下图标颜色随主题文字色正确变化。

## 文件清单

- 新增：`web/src/components/provider-icon.tsx`
- 修改：`web/src/app/(dashboard)/endpoints/page.tsx`
- 修改：`web/src/app/(dashboard)/audit/page.tsx`

## 不做的改动

- 不修改 Endpoints / Audit 以外的页面。
- 不将图标用于 Models、Sessions、API Keys、Shares、Blocked 等页面。
- 不修改按钮文字、表头文字、分页器。
