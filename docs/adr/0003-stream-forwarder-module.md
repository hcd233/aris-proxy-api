# StreamForwarder — 协议转发编排独立于 usecase

将 LLM 代理请求的流式转发编排（SSE 帧化、审计尾部、Token 上报、错误处理）从 `openAIUseCase`/`anthropicUseCase` 的 6 个 forward 方法中提取为独立的 `forwarder` 模块。

**Why extract, not inline.** 当前每个协议路径（Chat native、Chat→Anthropic、Response native 等）的 forward 方法中，约 70% 的编排逻辑（SSE 帧化、审计记录、Token 扣减）是重复的。协议间真正变化的是转换函数（chunk→event 的映射），而不是编排流程。提取后，新增协议路径只需提供转换函数 + 配置，不需要复刻整个 forward 方法。

**Why independent module, not shared method on usecase.** 放在 usecase 内部方法中虽然改动更小，但会导致 `openAIUseCase` 继续膨胀（当前已承载 5 个正交关注点）。独立模块让编排逻辑可被两个 usecase 共同注入，且可独立测试（不需要构造完整的 usecase 及其所有依赖）。

**Considered Options:**
- 保持现状：重复逻辑继续分散在 6 个方法中，新增协议需复刻
- usecase 内部共享方法：改动最小，但 usecase struct 继续膨胀
- 独立 forwarder 模块（选择）：编排单一化，usecase 收缩为路由 + 配置
