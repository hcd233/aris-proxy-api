# Pi 模型配置导出设计

## 需求范围

在 Web 模型管理页增加 Pi 导出目标。导出使用独立弹窗，沿用现有
OpenCode、Codex、Claude Code 导出的交互方式，为当前模型列表页生成 Bash
配置脚本。

弹窗允许用户选择模型、编辑 Provider ID、基础地址和 API 密钥，实时预览
脚本并复制。此次改动不增加后端接口，也不修改模型 DTO。

## Pi 配置

生成的脚本会更新 `~/.pi/agent/models.json`，写入以下 provider 结构：

```json
{
  "providers": {
    "aris-proxy": {
      "baseUrl": "https://example.com/api/openai/v1",
      "api": "openai-completions",
      "apiKey": "YOUR_API_KEY",
      "models": [
        {
          "id": "model-alias",
          "name": "model-alias",
          "reasoning": true,
          "input": ["text"],
          "contextWindow": 128000,
          "maxTokens": 64000,
          "cost": {
            "input": 0,
            "output": 0,
            "cacheRead": 0,
            "cacheWrite": 0
          }
        }
      ]
    }
  }
}
```

字段映射规则：

- `models[].id` 和 `models[].name` 使用模型 `alias`。
- `contextWindow` 使用 `contextLength`；值不大于 0 时回退为 `128000`。
- `maxTokens` 使用 `maxOutputTokens`；值不大于 0 时回退为 `16384`。
- `reasoning` 固定为 `true`。
- `input` 固定为 `["text"]`。
- 四个 `cost` 字段均为 `0`，因为当前模型 DTO 没有 Pi 价格元数据。
- `api` 固定为 `openai-completions`。
- 默认基础地址为当前站点 origin 加 `/api/openai/v1`。
- 默认 Provider ID 为 `aris-proxy`，默认 API 密钥为 `YOUR_API_KEY`。

## 脚本行为

- 默认目标文件为 `~/.pi/agent/models.json`。
- 可通过 `PI_MODELS_CONFIG` 覆盖目标文件路径。
- 目标文件不存在时，脚本会创建父目录和文件。
- 目标文件已存在时，修改前先复制为 `<目标文件>.bak`。
- 保留已有的顶层配置和其他 provider。
- 将选中的 provider 与生成的 provider 配置合并。
- 模型按 `id` 合并；本次选中的模型覆盖相同 ID 的已有模型，未选中的已有
  模型继续保留。
- Provider ID、基础地址和 API 密钥通过 Shell 变量暴露，并可使用
  `PROVIDER_ID`、`BASE_URL`、`API_KEY` 覆盖。
- 生成脚本中的 JSON 操作只使用 Python 标准库。

## 界面改动

- 在模型页的导出下拉菜单中增加 Pi 选项。
- 新增独立的 `ExportPiDialog` 组件，不改动现有导出弹窗的实现。
- 复用当前双栏导出布局：左侧为连接字段和可搜索的多选模型列表，右侧为
  高亮 Bash 预览和复制操作。
- 弹窗关闭时重置已选模型、搜索文本和复制状态。
- 为 Pi 菜单项、弹窗标题和描述、空状态及相关标签补充英文、简体中文、
  日文翻译。
- 保持现有响应式行为和国际化布局稳定约定。

## 测试与验证

实现完成后验证以下内容：

- 未选择模型时不生成脚本。
- 已选模型的 alias 正确出现在 Pi 模型的 ID 和名称中。
- 正数模型限制值及无效值的默认回退值生成正确。
- 生成脚本包含 Pi 配置路径覆盖、备份、provider 合并、模型合并和环境变量
  覆盖逻辑。
- `cd web && npm run lint` 通过。
- `cd web && npm run build` 通过。
- 使用 Chrome MCP 在模型页验证导出菜单、Pi 弹窗、模型选择、脚本预览和复制
  交互。

## 非本次范围

- 自动加载当前页之外的全部模型。
- 直接下载 JSON 文件。
- 后端接口或 DTO 改动。
- Pi OAuth、自定义请求头、模型级兼容参数，或当前模型数据未提供的价格配置。
