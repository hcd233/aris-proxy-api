# Port 层作为 DTO 权威源

`internal/application/*/port/` 中的 View 类型作为跨层传输的单一事实源（single source of truth），`internal/dto/` 中对等类型改为 type alias 引用 port 类型，删除 handler 层的 port→dto 字段映射。

**Why port, not dto.** dto 包存放 Huma 框架所需的请求/响应绑定类型，port 包的 View 类型承载了领域语义（permission 校验后的投影、聚合后的统计结果）。让 port 当权威源意味着领域概念不因 HTTP 绑定层的存在而重复定义。如果反过来（dto 当权威源），application 层会依赖 presentation 层，违反依赖方向。

**Why not keep both.** 当前 8 对近同构类型（如 `AuditLogItem` vs `AuditLogView`）在 handler 中用 `lo.Map` 逐字段拷贝，映射不产生任何转换、校验或业务逻辑——纯粹是结构性仪式，约 500 行的维护负担。

**Considered Options:**
- dto 为权威源：违反分层依赖方向（application → presentation）
- 保留两者：持续维护成本，且当前映射无业务价值
- 合并到一个共享 types 包：引入新的架构概念，不如复用已有的 port 层
