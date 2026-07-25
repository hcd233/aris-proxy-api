# CodeGraph 与 Serena

> **使用场景**：代码搜索、代码理解、跨文件符号重构，以及开发经验的读取和沉淀。

## CodeGraph：代码搜索与影响分析

CodeGraph 是代码知识图谱工具，负责建立项目结构、符号和调用关系上下文。

### 工具选择

- `codegraph_codegraph_status`：检查索引状态。
- `codegraph_codegraph_files`：查看项目文件树；探索目录结构时优先使用。
- `codegraph_codegraph_context`：根据任务描述获取入口、相关符号、调用者、被调用者和关键源码；这是代码理解的首选入口。
- `codegraph_codegraph_search`：按名称搜索函数、方法、类、接口、类型、变量、路由或组件。
- `codegraph_codegraph_callers`：查找调用指定符号的函数或方法。
- `codegraph_codegraph_callees`：查找指定符号调用的函数或方法。
- `codegraph_codegraph_impact`：分析修改指定符号的潜在影响范围。
- `codegraph_codegraph_node`：查看单个符号的定义和源码。
- `codegraph_codegraph_explore`：集中查看多个相关符号的源码和关系。

### 强制使用规则

- 代码搜索必须使用 CodeGraph，尤其是符号、调用关系和影响范围搜索。
- 不得用 `grep`、`rg` 或 `find` 替代 CodeGraph 的代码搜索。
- 命令行搜索仅用于 CodeGraph 不覆盖的非代码文件、配置文本、日志或已知字符串的精确检查。
- 推荐顺序：`codegraph_context` → `codegraph_explore` → `callers` / `callees` / `impact` → Serena 精确修改。

## Serena：语义级编辑与工程记忆

Serena 是基于语言服务的代码浏览、诊断和语义编辑工具。

### 工具选择

- `serena_list_memories`、`serena_read_memory`：读取已有工程经验。
- `serena_get_symbols_overview`、`serena_find_symbol`、`serena_find_declaration`：按文件或符号定位代码。
- `serena_find_referencing_symbols`、`serena_find_implementations`：查找引用和接口实现。
- `serena_rename_symbol`：跨文件语义重命名并更新引用。
- `serena_replace_symbol_body`：替换已确认符号的实现体。
- `serena_replace_in_files`：跨文件批量替换；有潜在误替换风险时先使用 `dry_run=true`。
- `serena_insert_before_symbol`、`serena_insert_after_symbol`：围绕符号插入代码。
- `serena_safe_delete_symbol`：确认无引用后删除符号。
- `serena_get_diagnostics_for_file`：检查语法、类型和语言服务诊断。
- `serena_write_memory`：沉淀可复用的工程经验。

### 强制使用规则

- 跨文件代码或符号重构必须使用 Serena，不得通过手工逐文件替换规避语义工具。
- 开发、排障或重构开始前，必须先使用 `serena_list_memories`，再用相关的 `serena_read_memory` 读取历史经验。
- 准备提交代码前，必须使用 `serena_write_memory` 沉淀稳定、可复用的经验，包括架构决策、约束、坑点、验证方式或排障结论。
- 没有相关历史经验时可以继续工作，但任务完成前应判断是否产生了值得沉淀的新经验。
- 不得把凭据、个人信息、生产敏感数据或仅对当前临时状态有用的内容写入 memory。

## 推荐工作流

```text
serena_list_memories / serena_read_memory
    ↓
CodeGraph 搜索并理解代码
    ↓
Serena 查找符号、引用和实现
    ↓
Serena 执行跨文件重构
    ↓
测试、lint 和诊断验证
    ↓
serena_write_memory 沉淀工程经验
    ↓
提交代码
```

## 与基础工具的边界

- CodeGraph 负责“代码在哪里、如何关联、修改会影响什么”。
- Serena 负责“按语义查找、重命名、重构、诊断和记录经验”。
- `read`、`edit`、`bash` 等基础工具仍可用于读取和修改文档、配置、脚本，以及不涉及跨文件符号语义的简单改动。
