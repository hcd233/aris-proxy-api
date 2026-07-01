# TextExtractor — 协议无关的内容提取接口

在敏感词检查链路中引入 `TextExtractor` 接口，各协议 DTO 实现各自的文本提取方法，`BlockedContentChecker` 只做 AC 自动机匹配，不感知协议细节。

**Why TextExtractor, not UnifiedMessage.** 备选方案是让调用方先把协议 DTO 转成 `UnifiedMessage`，Checker 只认一种格式。这要求每次检查前都执行完整转换（包括 tool calls、reasoning 等非检查所需字段），且转换逻辑与 converter 层重复。`TextExtractor` 只提取检查所需的文本，轻量且不重复 converter 的职责。

**Why interface, not generic function.** 三种协议（OpenAI Chat、Anthropic Messages、OpenAI Response）的文本提取路径不同——OpenAI Chat 遍历 `messages[].content`，Anthropic Messages 遍历 `messages[].content[].text`，Response API 在 `input[]` 中。接口让每个协议封装自己的提取逻辑，Checker 的接口保持简单（`Check(TextExtractor) bool`）。

**Consequences:**
- `BlockedContentChecker` 从 usecase 层移除对具体 DTO 类型的依赖
- 新增协议只需实现 `TextExtractor`，不必修改 Checker
- Checker 的 3 个 adapter 让这个 seam 成为真正的多态 seam
