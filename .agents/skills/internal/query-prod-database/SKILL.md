---
name: query-prod-database
description: Use when the user asks to inspect, count, diagnose, or query the production PostgreSQL database running in Docker on aris-proxy-api. Also use when a production database read or explicitly authorized data write is requested, or when the user asks to create/drop an index on the production database.
---

# query-prod-database

通过 SSH 进入生产服务器，在 PostgreSQL Docker 容器内查询生产数据库。默认只读；除索引 DDL（需用户明确授权）外，其他 DDL、权限变更和破坏性操作永久禁止。**连接生产服务器前必须先加载 `login-prod-server` skill** 获取 SSH 方式、环境布局、凭据读取规则与统一安全基线；本 skill 不再重复连接细节。

## 适用范围

适用于：

- 查询生产 PostgreSQL 数据；
- 查看表结构、表数量、统计信息和执行计划；
- 查询生产问题所需的数据库状态；
- 用户明确授权后的数据写操作。

不适用于：

- 索引以外的数据库结构变更或权限管理。

## 前置

1. 加载 `login-prod-server` skill，确认 SSH 方式、环境布局、凭据读取规则与安全基线；
2. 连接：`ssh ubuntu@api.lvlvko.top`；
3. 确认 PostgreSQL 容器（当前为 `postgresql`，勿假设不变）：`docker ps --format '{{.Names}}\t{{.Image}}\t{{.Status}}'`，找不到明确目标时停止并报告；
4. 从 `/home/ubuntu/code/aris-proxy-api/env/api.env` 读取 `POSTGRES_*` 连接键（读取与脱敏规则见 `login-prod-server`）。

## 只读 SQL：默认直接执行

默认允许的操作：

- `SELECT`（包括 PostgreSQL 系统目录和 `information_schema` 查询）；
- `SHOW`、`EXPLAIN`、`EXPLAIN ANALYZE`；
- 只读事务：`BEGIN READ ONLY`、`COMMIT`；
- PostgreSQL 的只读元数据查询。

只读查询也要保护生产环境：

- 非必要不要执行大表 `COUNT(*)`、无条件全表扫描或高成本 `EXPLAIN ANALYZE`；
- 优先使用统计信息（例如 `pg_stat_user_tables.n_live_tup`）回答规模类问题，并明确说明是估算值；
- 大结果集使用聚合、分页或合理的 `LIMIT`；
- 设置合理的 `statement_timeout`；
- 使用 `psql --no-psqlrc --set=ON_ERROR_STOP=1`；
- 不执行 `psql` 元命令：`\\!`、`\\copy`、`\\o`、`\\include` 等；
- 多语句 SQL 按高风险处理，逐条确认每条语句都是只读。

推荐执行形态（在生产服务器上执行，密码通过环境变量传入容器，不作为命令参数）：

```bash
set -a
. /home/ubuntu/code/aris-proxy-api/env/api.env
set +a

docker exec -i \
  -e PGPASSWORD="$POSTGRES_PASSWORD" \
  postgresql \
  psql --no-psqlrc \
    --set=ON_ERROR_STOP=1 \
    --username="$POSTGRES_USER" \
    --dbname="$POSTGRES_DATABASE" \
    --command="SET statement_timeout = '10000ms'; <READ_ONLY_SQL>"
```

不要在回复中回显完整连接命令中的密码、DSN 或 Secret 内容。若 shell 历史、审计日志或进程列表可能暴露凭据，停止并改用更安全的临时环境传递方式。

## 写操作：必须先获得明确授权

以下数据写操作不能直接执行：

- `INSERT`、`UPDATE`、`DELETE`、`MERGE`；
- `CALL` 或其他可能改变数据的函数调用；
- 任何包含写操作的事务或多语句 SQL。

执行前必须：

1. 原样向用户完整展示待执行 SQL；
2. 说明目标数据库、schema/table、筛选条件和预期影响范围；
3. 说明是否在事务中执行、是否需要先 `SELECT` 预览；
4. 等待用户对这段完整 SQL 的明确授权；
5. 只执行用户授权的原始 SQL，不自行增加、删除或改写语句；
6. 执行后报告成功/失败和数据库返回结果，不泄露敏感字段。

如果用户只说“执行一下”“可以”“继续”，但没有看到完整 SQL，先展示 SQL 并再次要求确认。

对于可能影响大量行的写操作，授权前必须先查询并展示影响行数或候选记录；无法安全估计时拒绝执行。

## 索引 DDL 操作：必须获得明确授权

以下索引结构变更不能直接执行，但经用户明确授权后可以执行：

- `CREATE INDEX`（含 `CREATE UNIQUE INDEX`、`CREATE INDEX CONCURRENTLY`）；
- `DROP INDEX`（含 `DROP INDEX CONCURRENTLY`）。

执行前必须：

1. 原样向用户完整展示待执行 SQL；
2. 说明目标数据库、schema/table、索引名称、索引列/表达式；
3. 说明预期影响：是否在事务中执行、是否需要先 `SELECT` 预览（如查询 `pg_indexes`、`pg_stat_user_tables`）；
4. 对生产大表说明锁风险与耗时预期：推荐使用 `CONCURRENTLY` 避免阻塞读写；`CONCURRENTLY` 不能在事务块内执行，且失败会遗留无效索引，需向用户说明；
5. 等待用户对这段完整 SQL 的明确授权；
6. 只执行用户授权的原始 SQL，不自行增加、删除或改写语句；
7. 执行后报告成功/失败和数据库返回结果，不泄露敏感字段。

注意：

- `DROP INDEX` 前建议先展示 `pg_indexes` 中该索引的定义供用户确认；若索引被约束依赖（如 UNIQUE/PRIMARY KEY 依赖的索引），直接 `DROP` 会失败，需向用户说明原因；
- 索引 DDL 执行期间可能持有锁，尽量选择低峰期执行；
- 如果用户只说“执行一下”“可以”“继续”，但没有看到完整 SQL，先展示 SQL 并再次要求确认。

## 永久禁止

无论用户是否授权，永久禁止执行：

- `DROP`、`TRUNCATE`（仅 `DROP INDEX` 例外，见“索引 DDL 操作”一节）；
- `CREATE`、`ALTER`、`RENAME`、`COMMENT`（仅 `CREATE INDEX` 例外，见“索引 DDL 操作”一节）；
- `GRANT`、`REVOKE`、`SECURITY LABEL`；
- 创建/修改数据库、schema、table、view、function、trigger、role 或 extension；
- `VACUUM FULL`、`CLUSTER`、`REINDEX` 等可能产生显著锁或生产影响的维护操作；
- `COPY`/`\\copy` 导出生产数据；
- `psql` Shell 元命令或任何宿主机命令注入；
- 绕过上述规则改用其他数据库连接路径。

发现 SQL 同时包含允许和禁止语句时，整批拒绝执行。

## 敏感数据与输出

- 不输出数据库密码、连接串、Secret、API Key、JWT 或完整凭证；
- 查询结果包含密码、Token、API Key、Cookie、Authorization 等字段时，先脱敏再展示；
- 不为方便查询而把整表敏感数据导出到本地文件；
- 只返回完成任务所需的最小字段和摘要；
- 结果需说明是精确值还是估算值。例如 `pg_stat_user_tables.n_live_tup` 是估算行数，不等同于 `COUNT(*)`。

## 失败处理

- SSH 失败：报告连接错误，不尝试使用裸 IP；
- PostgreSQL 容器不存在或不健康：停止并报告；
- 配置缺失或认证失败：停止，不猜测密码；
- SQL 解析不明确、混入未知语句或可能写入：停止并要求澄清/授权；
- 查询超时或失败：返回错误摘要，不自动重试可能产生副作用的 SQL；
- 不要为了“完成任务”降低只读和永久禁止规则。
