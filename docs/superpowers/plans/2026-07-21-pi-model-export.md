# Pi 模型配置导出实施计划

> **面向执行代理：** 必须使用 `superpowers:subagent-driven-development`（推荐）或 `superpowers:executing-plans` 逐任务执行本计划。所有步骤使用复选框跟踪。

**目标：** 在 Web 模型管理页增加 Pi 导出入口，生成可复制的 Bash 脚本，将当前页选中的模型合并写入 `~/.pi/agent/models.json`。

**架构：** 新增独立的 `ExportPiDialog` 组件，沿用现有导出弹窗的双栏布局、模型搜索多选、代码高亮和复制逻辑。模型页只负责维护弹窗开关和入口菜单；脚本在浏览器端根据表单状态即时生成，执行时由 Bash 调用 Python 标准库完成 JSON 备份和合并。

**技术栈：** Next.js 16.2.6、React 19、TypeScript、Tailwind v4、`highlight.js`、`lucide-react`、现有 `@lobehub/icons`、项目 `useT` 国际化。

## 全局约束

- 只修改 `web/` 前端和本次设计/计划文档，不增加后端接口或 DTO。
- Pi 配置文件固定默认为 `~/.pi/agent/models.json`，通过 `PI_MODELS_CONFIG` 支持路径覆盖。
- `models[].id` 和 `models[].name` 必须使用模型 `alias`。
- `reasoning` 必须为 `true`，`api` 必须为 `openai-completions`。
- `contextWindow` 无效时使用 `128000`，`maxTokens` 无效时使用 `16384`。
- 四个价格字段固定为 `0`，输入类型固定为 `["text"]`。
- 只导出模型页当前已经加载的模型，弹窗内由用户选择具体模型。
- 复用现有 UI 组件和 Tailwind 类名，不新增基础 UI 组件，不使用内联定值样式。
- 新增界面文案必须同步写入 `en.json`、`zh.json`、`ja.json`。
- 配置、备份和临时文件使用 `0600`，默认配置目录使用 `0700`，并通过 `os.replace` 原子替换配置文件。
- 选中模型存在重复 alias 时不生成脚本，界面显示重复 alias 提示。
- superpowers 生成的文档使用中文；必要的代码标识符、命令、路径、配置键和协议原文保留英文。
- 不执行 git commit，除非用户明确要求提交。

---

## 文件清单

### 新增文件

- `web/src/components/export-pi-dialog.tsx`：Pi 导出弹窗、配置表单、模型选择、脚本生成、代码预览和复制操作。

### 修改文件

- `web/src/app/(dashboard)/models/page.tsx`：引入 Pi 弹窗、增加开关状态、增加导出菜单项、挂载弹窗。
- `web/src/locales/en.json`：增加 Pi 导出的英文文案。
- `web/src/locales/zh.json`：增加 Pi 导出的中文文案。
- `web/src/locales/ja.json`：增加 Pi 导出的日文文案。

### 验证文件

- 不新增前端测试文件。当前 `web/package.json` 没有单测运行器；通过纯函数式的脚本内容检查、前端 lint、生产构建和 Chrome MCP 交互验证覆盖本次行为。

---

### 任务 1：实现 Pi 脚本生成器和导出弹窗

**文件：**

- 创建：`web/src/components/export-pi-dialog.tsx`

**接口：**

- 输入：
  - `open: boolean`
  - `onOpenChange: (open: boolean) => void`
  - `models: ModelItem[]`
- 输出：默认导出 React 组件 `ExportPiDialog`。
- 复用：`ModelItem`、`Button`、`Dialog`、`Input`、`Label`、`useT`、`highlight.js` Bash 语言和现有导出弹窗的布局约定。

- [ ] **步骤 1：复制现有导出弹窗的基础结构**

以 `web/src/components/export-dialog.tsx` 为结构参考，创建客户端组件并加入以下导入：

```tsx
"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ModelItem } from "@/lib/types";
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useT } from "@/lib/i18n";
import { Check, Copy, Search, Terminal, X } from "lucide-react";
import { Pi } from "@lobehub/icons";
```

如果当前版本的 `@lobehub/icons` 没有可用的 `Pi` 导出，不要新增图标依赖；改用 `Terminal` 图标作为 Pi 的入口和空状态图标，并保持其他导出组件不变。

- [ ] **步骤 2：实现模型到 Pi JSON 的映射**

在组件内定义脚本生成前使用的映射逻辑，确保数值默认值和字段固定值明确：

```tsx
function buildPiModels(models: ModelItem[]) {
  return models.map((model) => ({
    id: model.alias,
    name: model.alias,
    reasoning: true,
    input: ["text"],
    contextWindow: model.contextLength > 0 ? model.contextLength : 128000,
    maxTokens: model.maxOutputTokens > 0 ? model.maxOutputTokens : 16384,
    cost: {
      input: 0,
      output: 0,
      cacheRead: 0,
      cacheWrite: 0,
    },
  }));
}
```

不要把 `modelName` 写入 Pi 的 `id`，也不要为当前 DTO 未提供的 reasoning、价格或输入模态增加表单字段。

- [ ] **步骤 3：实现 Bash 脚本生成逻辑**

实现 `generateScript(providerId, baseUrl, apiKey, selectedModels)`，空数组返回空字符串；非空时生成可直接复制的 Bash 脚本。脚本必须包含以下完整行为：

```bash
#!/usr/bin/env bash
set -euo pipefail

export PROVIDER_ID="${PROVIDER_ID:-}"
export BASE_URL="${BASE_URL:-}"
export API_KEY="${API_KEY:-}"
PI_MODELS_CONFIG="${PI_MODELS_CONFIG:-$HOME/.pi/agent/models.json}"

python3 << 'PYEOF'
import json
import os
import shutil
import tempfile

config_path = os.path.expanduser(os.environ["PI_MODELS_CONFIG"])
provider_id = os.environ.get("PROVIDER_ID") or ${providerIdJson}
base_url = os.environ.get("BASE_URL") or ${baseUrlJson}
api_key = os.environ.get("API_KEY") or ${apiKeyJson}
models = json.loads(${JSON.stringify(modelsJson)})

config_dir = os.path.dirname(config_path) or "."
os.makedirs(config_dir, exist_ok=True)
if os.path.normpath(config_path) == os.path.expanduser("~/.pi/agent/models.json"):
    os.chmod(config_dir, 0o700)
if os.path.exists(config_path):
    shutil.copyfile(config_path, config_path + ".bak")
    os.chmod(config_path + ".bak", 0o600)
    with open(config_path, "r", encoding="utf-8") as file:
        config = json.load(file)
else:
    config = {}

providers = config.setdefault("providers", {})
provider = providers.setdefault(provider_id, {})
provider["baseUrl"] = base_url
provider["api"] = "openai-completions"
provider["apiKey"] = api_key

existing_models = provider.get("models", [])
selected_by_id = {model["id"]: model for model in models}
merged_models = [
    model
    for model in existing_models
    if model.get("id") not in selected_by_id
]
merged_models.extend(selected_by_id.values())
provider["models"] = merged_models

with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=config_dir, prefix=".models.json.", delete=False) as file:
    temp_path = file.name
    os.chmod(temp_path, 0o600)
    json.dump(config, file, indent=2, ensure_ascii=False)
    file.write("\n")
    file.flush()
    os.fsync(file.fileno())
os.replace(temp_path, config_path)
os.chmod(config_path, 0o600)
PYEOF
```

实际 TypeScript 模板中的 `${modelsJson}` 必须由 `JSON.stringify(buildPiModels(selectedModels), null, 2)` 生成并嵌入 Python 代码；provider ID、Base URL、API Key 的默认值也必须使用 JSON 字符串字面量注入，避免引号或特殊字符破坏生成脚本。脚本通过环境变量读取最终值，因此执行时修改环境变量即可覆盖弹窗默认值。

合并逻辑必须满足：

- 不存在配置文件时创建 `{ "providers": {} }` 结构。
- 存在配置文件时保留已有顶层字段和其他 provider。
- 同 ID 已有模型由本次选中模型覆盖。
- 未选中的已有模型继续保留。
- 新选中模型追加到 provider 模型数组。
- 写入前备份为同路径加 `.bak` 后缀。
- 配置、备份和临时文件使用 `0600`，默认配置目录使用 `0700`。
- 通过同目录临时文件、`flush`、`fsync` 和 `os.replace` 原子替换配置。
- 选中模型存在重复 alias 时阻止脚本生成，并显示重复 alias 提示。

- [ ] **步骤 4：实现弹窗状态与模型选择**

组件状态和派生值使用以下结构：

```tsx
const [providerId, setProviderId] = useState("aris-proxy");
const [baseUrl, setBaseUrl] = useState(() =>
  typeof window === "undefined" ? "" : `${window.location.origin}/api/openai/v1`
);
const [apiKey, setApiKey] = useState("YOUR_API_KEY");
const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
const [copied, setCopied] = useState(false);
const [modelSearch, setModelSearch] = useState("");

const filteredModels = useMemo(
  () =>
    modelSearch.trim()
      ? models.filter(
          (model) =>
            model.alias.toLowerCase().includes(modelSearch.toLowerCase()) ||
            model.modelName.toLowerCase().includes(modelSearch.toLowerCase())
        )
      : models,
  [models, modelSearch]
);

const selectedModels = useMemo(
  () => models.filter((model) => selectedIds.has(model.id)),
  [models, selectedIds]
);
```

模型列表使用复选框和“全选/清空”操作；弹窗关闭时清空选择、搜索和复制状态。脚本、HTML 高亮结果和行数均通过 `useMemo` 从当前状态生成；复制按钮沿用现有 `navigator.clipboard.writeText` 行为。

- [ ] **步骤 5：实现双栏响应式 UI**

使用与现有导出弹窗一致的容器尺寸和布局：

```tsx
<DialogContent
  showCloseButton={false}
  className="!max-w-[1040px] w-[calc(100vw-1.5rem)] h-[min(86vh,720px)] p-0 gap-0 overflow-hidden flex flex-col sm:!max-w-[1040px]"
>
  <DialogHeader className="shrink-0 flex-row items-center gap-3 px-6 py-4 border-b border-border">
    {/* Pi 图标、标题、描述和关闭按钮 */}
  </DialogHeader>
  <div className="flex flex-1 min-h-0 flex-col overflow-y-auto md:grid md:grid-cols-[minmax(0,1fr)_minmax(0,1.12fr)] md:overflow-hidden">
    <div className="md:min-h-0 md:overflow-y-auto md:border-r border-border px-6 py-5 space-y-6">
      {/* 连接信息和模型多选 */}
    </div>
    <div className="flex flex-col md:min-h-0 md:overflow-hidden bg-[#262624]">
      {/* Bash 工具栏、代码预览和底部提示 */}
    </div>
  </div>
</DialogContent>
```

左侧字段使用现有 `Label`、`Input`；右侧使用现有 Bash 高亮颜色类。动态按钮沿用现有 `Button` size 预留，不新增固定宽度翻译例外。

- [ ] **步骤 6：进行组件级静态检查**

运行：

```bash
cd web && npm run lint
```

预期：退出码为 0，且没有 React hooks、TypeScript 或 Next.js lint 错误。若发现 `@lobehub/icons` 没有 `Pi` 导出，按步骤 1 使用 `Terminal`，不要新增依赖。

---

### 任务 2：接入模型页导出入口

**文件：**

- 修改：`web/src/app/(dashboard)/models/page.tsx:1-120`
- 修改：`web/src/app/(dashboard)/models/page.tsx:251-330`
- 修改：`web/src/app/(dashboard)/models/page.tsx:646-662`

**接口：**

- 消费：任务 1 的默认导出组件 `ExportPiDialog`。
- 产生：模型页导出菜单中的 Pi 入口，以及传入当前 `models` 列表的弹窗实例。

- [ ] **步骤 1：增加导入和开关状态**

在现有导出组件导入旁增加：

```tsx
import ExportPiDialog from "@/components/export-pi-dialog";
```

在现有三个导出弹窗状态后增加：

```tsx
const [exportPiDialogOpen, setExportPiDialogOpen] = useState(false);
```

- [ ] **步骤 2：在导出菜单增加 Pi 菜单项**

在 Codex 菜单项后增加一个 `DropdownMenuItem`，保持已有 `items-start gap-2.5 rounded-lg px-2 py-2` 样式：

```tsx
<DropdownMenuItem
  onClick={() => setExportPiDialogOpen(true)}
  className="items-start gap-2.5 rounded-lg px-2 py-2"
>
  <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-gradient-to-b from-secondary to-muted">
    <Terminal className="size-4" />
  </span>
  <span className="flex min-w-0 flex-col gap-0.5">
    <span className="text-sm font-medium leading-none">
      {t("models.export_pi")}
    </span>
    <span className="truncate text-xs text-muted-foreground">
      {t("models.export_pi_hint")}
    </span>
  </span>
</DropdownMenuItem>
```

如果 `Terminal` 尚未在当前 import 列表中，加入现有 `lucide-react` 导入；不要引入第二套图标库。

- [ ] **步骤 3：挂载 Pi 弹窗**

在现有 `ExportCodexDialog` 后增加：

```tsx
<ExportPiDialog
  open={exportPiDialogOpen}
  onOpenChange={setExportPiDialogOpen}
  models={models}
/>
```

确认传入的是当前页面的 `models`，不新增额外请求，也不改变分页行为。

- [ ] **步骤 4：验证入口类型检查**

运行：

```bash
cd web && npm run lint
```

预期：退出码为 0；导入路径、弹窗属性和图标类型均无错误。

---

### 任务 3：补充三套国际化文案

**文件：**

- 修改：`web/src/locales/en.json`，现有 `models.export_*` 区域
- 修改：`web/src/locales/zh.json`，现有 `models.export_*` 区域
- 修改：`web/src/locales/ja.json`，现有 `models.export_*` 区域

**接口：**

- 消费：任务 1 和任务 2 使用的 `useT` 键。
- 产生：三种 Locale 下完整的 Pi 导出文案。

- [ ] **步骤 1：加入英文文案**

在 `web/src/locales/en.json` 的 Codex 导出文案附近加入：

```json
"models.export_pi": "Pi",
"models.export_pi_hint": "Generate a setup script for Pi",
"models.export_pi_title": "Export to Pi",
"models.export_pi_desc": "Generate a bash script to add selected models to Pi via ~/.pi/agent/models.json.",
"models.export_pi_empty_hint": "Pick models on the left to build the script",
"models.export_pi_script_filename": "pi-setup.sh",
"models.export_duplicate_aliases": "Selected models contain duplicate aliases. Select only one model for each alias."
```

- [ ] **步骤 2：加入中文文案**

在 `web/src/locales/zh.json` 的 Codex 导出文案附近加入：

```json
"models.export_pi": "Pi",
"models.export_pi_hint": "生成 Pi 配置脚本",
"models.export_pi_title": "导出到 Pi",
"models.export_pi_desc": "生成 bash 脚本，将选中的模型合并写入 ~/.pi/agent/models.json。",
"models.export_pi_empty_hint": "在左侧选择模型以生成脚本",
"models.export_pi_script_filename": "pi-setup.sh",
"models.export_duplicate_aliases": "所选模型包含重复的别名。每个别名只能选择一个模型。"
```

- [ ] **步骤 3：加入日文文案**

在 `web/src/locales/ja.json` 的 Codex 导出文案附近加入：

```json
"models.export_pi": "Pi",
"models.export_pi_hint": "Pi 用のセットアップスクリプトを生成",
"models.export_pi_title": "Pi にエクスポート",
"models.export_pi_desc": "選択したモデルを ~/.pi/agent/models.json に追加する bash スクリプトを生成します。",
"models.export_pi_empty_hint": "左側でモデルを選択してスクリプトを生成",
"models.export_pi_script_filename": "pi-setup.sh",
"models.export_duplicate_aliases": "選択したモデルに重複するエイリアスがあります。各エイリアスは 1 つだけ選択してください。"
```

- [ ] **步骤 4：检查 JSON 和键名一致性**

运行：

```bash
node -e 'for (const file of ["web/src/locales/en.json", "web/src/locales/zh.json", "web/src/locales/ja.json"]) JSON.parse(require("fs").readFileSync(file, "utf8"))'
```

预期：命令无输出并退出码为 0。随后搜索 `export_pi`，确认三份文件中的键名完全一致。

---

### 任务 4：验证生成脚本、构建和浏览器交互

**文件：**

- 检查：`web/src/components/export-pi-dialog.tsx`
- 检查：`web/src/app/(dashboard)/models/page.tsx`
- 检查：`web/src/locales/en.json`
- 检查：`web/src/locales/zh.json`
- 检查：`web/src/locales/ja.json`

**接口：**

- 消费：任务 1 至任务 3 的完整实现。
- 产生：可验证的 lint、构建和浏览器交互证据。

- [ ] **步骤 1：执行前端 lint**

运行：

```bash
cd web && npm run lint
```

预期：退出码为 0。

- [ ] **步骤 2：执行前端生产构建**

运行：

```bash
cd web && npm run build
```

预期：Next.js 静态构建成功，生成 `web/out/`，没有 TypeScript、静态导出或页面编译错误。

- [ ] **步骤 3：检查生成脚本的关键内容**

在浏览器中打开 Pi 导出弹窗，选择至少一个模型，检查右侧预览文本包含以下字面量：

```text
PI_MODELS_CONFIG
~/.pi/agent/models.json
PI_MODELS_CONFIG:-$HOME/.pi/agent/models.json
openai-completions
reasoning": true
contextWindow
maxTokens
cacheRead
cacheWrite
shutil.copyfile
selected_by_id
```

检查至少两种模型：一个 `contextLength` 和 `maxOutputTokens` 为正数的模型，以及一个无效限制值的模型；预览应分别出现实际值和 `128000`/`16384` 回退值。

- [ ] **步骤 4：使用 Chrome MCP 验证模型页入口**

启动本地前端或使用现有联调服务，打开模型页并验证：

1. “导出配置”菜单可打开。
2. 菜单中显示 Pi 目标和说明。
3. 点击 Pi 后弹窗打开，默认 Base URL 是当前站点的 `/api/openai/v1`。
4. 模型列表可搜索，选择和清空选择会更新脚本预览。
5. 未选择模型时右侧显示空状态，复制按钮不可用。
6. 选择模型后脚本预览显示，复制按钮可用并变为“已复制”。
7. 关闭再打开弹窗后，选择、搜索和复制状态已重置。
8. 切换中文、英文、日文后，Pi 文案存在且双栏布局没有明显跳变。

- [ ] **步骤 5：检查工作区差异**

运行：

```bash
git diff --check
git status --short
git diff --stat
```

预期：只有本计划列出的前端文件和中文文档出现变更；不删除或覆盖用户原有改动，不生成需要提交的 `web/out/` 或 `internal/web/dist/` 产物。

---

## 计划自审

- 需求覆盖：配置字段映射在任务 1，脚本备份和合并在任务 1，入口在任务 2，三语言文案在任务 3，lint/build/Chrome 验证在任务 4。
- 范围覆盖：没有增加后端接口、DTO、JSON 直接下载、全量模型加载、Pi OAuth 或未提供的高级配置。
- 占位符检查：脚本示例使用明确的 `${modelsJson}` 模板插值，步骤 3 已明确它必须由完整的 `JSON.stringify(buildPiModels(selectedModels), null, 2)` 生成，不得把模板标记原样写入最终脚本。
- 类型一致性：组件输入为 `open`、`onOpenChange`、`models`，模型页按同名属性传入；脚本生成函数接收 provider ID、Base URL、API Key 和 `ModelItem[]`。
- 验证可执行性：项目没有单测运行器，因此不新增虚假的测试命令，改用静态脚本内容检查、lint、build 和 Chrome MCP 覆盖行为。
