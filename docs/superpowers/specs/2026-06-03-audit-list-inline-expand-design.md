# Audit 列表内联展开优化设计

日期：2026-06-03  
状态：approaching  
分支：feature/audit-list-optimize-2026-06-03

## 1. 问题

当前 audit 列表桌面端展示 11 列（Time / Model / Provider / User / API Key / Status / Tokens / Cache / Latency / TraceID），信息密度过高，一眼难以定位异常请求。用户核心场景为**排障追查**（按 traceId/时间定位错误），需要"状态与错误优先"。

## 2. 目标

- 主行精简至核心字段（Status / Time / Model / User / API Key / Tokens），降低视觉噪音
- 次级信息（Provider / Cache / Latency / TraceID / ErrorMessage）放入可展开的详情行
- 异常行（状态码非 200 / 首 token 延迟 > 3000ms）自动展开详情行并高亮问题字段
- 增加快速过滤芯片：All / Failed / Success

## 3. 设计决策

| 项目 | 决定 |
|------|------|
| 展开方式 | 表格内联展开（detail row 跨全列），非侧边面板 |
| 主行字段 | Status → Time → Model → User → API Key → Tokens |
| 详情行字段 | Provider / Cache / Latency / TraceID / ErrorMessage / UA |
| 自动展开条件 | ① upstreamStatusCode != 200 ② firstTokenLatencyMs > 3000 |
| 高亮规则 | 错误行：Status Badge 红 + ErrorMessage 红字；高延迟行：Latency 数值红字 |
| 手动切换 | 点击主行任意位置切换详情行显隐 |
| 移动端 | 卡片 + 内联展开，布局与桌面保持一致信息架构 |

## 4. 不涉及

- 后端接口不变（`ListAuditLogsRsp` / `AuditLogItem` 字段不变）
- 不修改排序、分页、时间范围、搜索逻辑
- 不新增图表或统计

## 5. UI 骨架

```
┌─ 筛选区 ───────────────────────────────────┐
│ [All] [Failed] [Success]  │  TimeRangePicker │  [🔍 Search...] │
└─────────────────────────────────────────────┘

┌─ 表格（桌面端）────────────────────────────┐
│ Status │ Time       │ Model   │ User  │ Key  │ Tokens      │
├────────┼────────────┼─────────┼───────┼──────┼─────────────┤
│ ✓ 200  │ 14:32:01   │ gpt-4o  │ cent  │ prod │ 1.2k↑/567↓ │  ← 正常行（折叠）
│ ✓ 200  │ 14:31:58   │ claude3 │ admin │ dev  │ 3k↑/890↓   │
│ ✕ 500  │ 14:31:30   │ gpt-4o  │ cent  │ prod │ 0↑/0↓      │  ← 错误行（自动展开）
│ ┊──────┴────────────┴─────────┴───────┴──────┴─────────────┊
│ ┊ Provider: openai · openai  │ Cache: —                     ┊
│ ┊ Latency: — · —             │ TraceID: abc-def-123 (copy) ┊
│ ┊ Error: Connection timeout                                ┊  ← 红色高亮
│ ┊ UA: Chrome/125                                            ┊
├────────┼────────────┼─────────┼───────┼──────┼─────────────┤
│ ✓ 200  │ 14:30:01   │ gpt-4o  │ cent  │ prod │ 1.2k↑/567↓ │  ← 高延迟行（自动展开）
│ ┊──────┴────────────┴─────────┴───────┴──────┴─────────────┊
│ ┊ Provider: openai · openai  │ Cache: 200↑/50↓              ┊
│ ┊ Latency: 4500ms · 8000ms   │ TraceID: xyz-456 (copy)    ┊  ← 延迟红色高亮
│ ┊ UA: Chrome/125                                            ┊
└────────────────────────────────────────────────────────────┘
```

## 6. 组件拆分

不新增文件，所有改动在 `web/src/app/(dashboard)/audit/page.tsx` 内完成：
- 新增 `isExpanded` 状态（Set<number>，存储展开的 log id）
- 新增 `shouldAutoExpand(log)` 判断函数
- 重构 `<TableBody>` 内渲染逻辑：每行变为主行 + 条件详情行
- 新增快速过滤芯片栏
- 重构移动端卡片布局适配相同逻辑

## 7. 验收

- [ ] 桌面端正常行默认折叠，点击展开/折叠
- [ ] 错误行（statusCode != 200）自动展开，ErrorMessage 红色
- [ ] 高延迟行（firstTokenLatencyMs > 3000ms）自动展开，Latency 红色
- [ ] 快速过滤 All/Failed/Success 芯片可用
- [ ] 移动端卡片同逻辑
- [ ] lint + typecheck 通过
