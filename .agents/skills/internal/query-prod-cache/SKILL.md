---
name: query-prod-cache
description: Use when the user asks to inspect, count, diagnose, or query the production Redis cache running in Docker on aris-proxy-api. Also use when a production cache read, TTL/key inspection, or explicitly authorized cache data write (e.g. deleting a specific key) is requested. Trigger on mentions of Redis, cache, keys, TTL, sessions, rate limit buckets, or "查一下缓存".
---

# query-prod-cache

通过 SSH 进入生产服务器，在 Redis Docker 容器内查询生产缓存。默认只读；结构破坏、实例级维护和危险操作永久禁止，数据写操作（如按 key 删除、更新）必须先获得明确授权。

## 适用范围

适用于：

- 查询生产 Redis 缓存数据（session 元数据、message/tool 缓存、限流令牌桶、JWT 用户缓存等）；
- 查看 key 是否存在、类型、TTL、内存占用（`MEMORY USAGE`）、编码方式（`OBJECT ENCODING`）；
- 按前缀用 `SCAN` 统计 key 数量、查看 key 分布；
- 查看实例健康与统计信息（`INFO`、`DBSIZE`、`CLIENT LIST`）；
- 用户明确授权后的缓存数据写操作（如删除单个误置 key、修正某条缓存值）。

不适用于：

- 任何实例级结构变更、配置变更或破坏性操作。

## 连接入口

始终使用域名，不得使用裸 IP：

```bash
ssh ubuntu@api.lvlvko.top
```

生产 Redis 当前已验证的容器名为 `redis`（镜像 redis:8.4.0，由 1Panel 管理，数据/配置在 `/opt/1panel/apps/redis/redis/`），但不要假设容器名永远不变；执行前先确认：

```bash
docker ps --format '{{.Names}}\t{{.Image}}\t{{.Status}}'
```

选择镜像为 Redis 且状态健康/运行中的容器。找不到明确目标时停止并向用户报告，不要猜测。

## 获取连接配置

优先从生产服务器读取应用自身的配置（应用就是用这套凭据连 Redis 的，与 redis-cli 需要的一致）：

```text
/home/ubuntu/code/aris-proxy-api/env/api.env
```

常见键包括：

```text
REDIS_HOST
REDIS_PORT
REDIS_PASSWORD
REDIS_DB
```

注意：生产上**没有**独立的 `redis.env`（只有 `redis.env.template`），容器由 1Panel 部署，密码在容器启动参数 `--requirepass` 和配置文件 `/opt/1panel/apps/redis/redis/conf/redis.conf` 中；不要假设 compose 文件里的 `env/redis.env` 存在。应用固定使用 `0` 号库（`internal/common/constant/database.go` 中 `RedisDB = 0`，与 redis-cli 默认库一致），`REDIS_DB` 仅作参考。如果部署链路明确由 Kubernetes 管理，可以检查对应 ConfigMap/Secret；读取 Secret 时不得输出解码后的值。不要把密码写入 skill、脚本、Git、聊天消息或命令行参数中。

只允许输出配置键名或 `<redacted>`，例如：

```bash
awk -F= 'tolower($1) ~ /(redis|cache)/ {print $1"=<redacted>"}' \
  /home/ubuntu/code/aris-proxy-api/env/api.env
```

## 只读命令：默认直接执行

默认允许的操作：

- 字符串：`GET`、`MGET`、`EXISTS`、`TTL`、`PTTL`、`TYPE`、`STRLEN`；
- 哈希：`HGET`、`HGETALL`、`HLEN`、`HKEYS`、`HVALS`、`HSCAN`；
- 列表：`LRANGE`、`LLEN`；
- 集合：`SMEMBERS`、`SCARD`、`SSCAN`；
- 有序集合：`ZRANGE`、`ZREVRANGE`、`ZCARD`、`ZSCORE`、`ZCOUNT`、`ZRANGEBYSCORE`、`ZSCAN`；
- 实例信息：`PING`、`INFO`、`DBSIZE`、`MEMORY USAGE`、`OBJECT ENCODING`、`OBJECT IDLETIME`、`CLIENT LIST`、`CONFIG GET`（只读，`CONFIG SET` 永久禁止）；
- 游标遍历：`SCAN`（替代 `KEYS`）。

只读查询也要保护生产环境：

- 用 `SCAN` 遍历 key，**绝不使用 `KEYS`**——`KEYS` 会阻塞整个实例，生产上等同事故；
- 大集合优先用游标命令（`HSCAN`/`SSCAN`/`ZSCAN`）或合理 `COUNT`/`LIMIT`，不要一次性拉全量；
- `INFO`、`CLIENT LIST` 等直接返回，不做多余聚合；
- 不执行阻塞命令：`BLPOP`、`BRPOP`、`SUBSCRIBE`、`PSUBSCRIBE`、`MONITOR`、`WAIT` 等会挂起连接或长时间占用资源；
- 单条命令一次执行，不使用管道（pipeline）；`EVAL`/`EVALSHA`/`FCALL` 脚本和 `--pipe` 按高风险处理（见下）；
- 需要统计某前缀下 key 数量时，用 `SCAN MATCH` 迭代计数，并在结果中说明是扫描计数而非 `DBSIZE` 精确值。

推荐执行形态（在生产服务器上执行，密码通过环境变量传入容器，不作为命令参数；`REDISCLI_AUTH` 是 redis-cli 官方支持的环境变量，避免密码出现在进程列表）：

```bash
set -a
. /home/ubuntu/code/aris-proxy-api/env/api.env
set +a

docker exec -i \
  -e REDISCLI_AUTH="$REDIS_PASSWORD" \
  redis \
  redis-cli --no-auth-warning --raw <READ_ONLY_COMMAND>
```

不要在回复中回显完整连接命令中的密码或 Secret 内容。若 shell 历史、审计日志或进程列表可能暴露凭据，停止并改用更安全的临时环境传递方式。

### 常用 key 前缀（帮助定位，来自 `internal/common/constant/rediskey.go`）

```text
jwt:user:%d             JWT 用户缓存
tb:%s:%s:%v             限流令牌桶
trace:client:ticket:%s  客户端 ticket
scanner:ban:%s          扫描器封禁
scanner:strike:%s       扫描器击打计数
share:%s                会话分享
user_shares:%d          用户分享列表
session_shares:%d       会话分享列表
cron:lock:%s            cron 任务互斥锁
session:meta:%d         session 元数据缓存（含 messageIDs/toolIDs）
message:%d              message 详情缓存
tool:%d                 tool 详情缓存
metrics:runtime:data:%s 运行时指标快照（ZSET）
```

例如按前缀统计 key 数量：

```bash
redis-cli --scan --pattern 'session:meta:*' | wc -l
```

## 写操作：必须先获得明确授权

以下缓存数据写操作不能直接执行：

- `SET`、`MSET`、`SETEX`、`SETNX`、`GETSET`、`APPEND`、`INCR`/`INCRBY`/`DECR` 等值修改；
- `DEL`、`UNLINK`（删除指定 key）；
- `EXPIRE`、`PEXPIRE`、`EXPIREAT`、`PERSIST` 等 TTL 修改；
- `RENAME`、`COPY`、`RESTORE` 等 key 操作；
- 哈希/列表/集合/有序集合的任何写命令（`HSET`、`LPUSH`、`SADD`、`ZADD` 等）；
- 任何可能写入数据的 Lua 脚本（`EVAL`/`EVALSHA`/`FCALL`）或多条命令组合。

执行前必须：

1. 原样向用户完整展示待执行的命令及其完整参数；
2. 说明目标 key（或 key 前缀与匹配范围）、操作类型和预期影响范围；
3. 说明是否需要先 `SCAN`/`GET` 预览将影响的 key 与数据；
4. 等待用户对这条完整命令的明确授权；
5. 只执行用户授权的原始命令，不自行增加、删除或改写参数；
6. 执行后报告成功/失败和 Redis 返回结果，不泄露敏感字段。

如果用户只说“执行一下”“可以”“继续”，但没有看到完整命令，先展示命令并再次要求确认。

对于可能影响大量 key 的写操作（如按前缀批量删除），授权前必须先查询并展示将影响的 key 数量与示例；无法安全估计时拒绝执行。

## 永久禁止

无论用户是否授权，永久禁止执行：

- `FLUSHALL`、`FLUSHDB`——清空整个实例/库；
- `SHUTDOWN`、`DEBUG`（如 `DEBUG SEGFAULT`）、`MONITOR`——宕机或泄露流量；
- `CONFIG SET`、`CONFIG RESETSTAT`——修改实例配置或重置统计；
- `SLAVEOF`/`REPLICAOF`、`FAILOVER`——复制拓扑变更；
- `MIGRATE`、`MOVE`、`SWAPDB`——跨实例/跨库搬移数据；
- `SAVE`、`BGSAVE`、`BGREWRITEAOF`——fork 与磁盘 I/O 可能显著影响生产实例；
- `KEYS`、`FLUSH*` 的等价替代（如 `DEBUG CHANGE-REPL-ID` 等危险命令）；
- `EVAL`/`EVALSHA`/`FCALL` 脚本、`--pipe` 批量导入、交互式 `!` shell 转义——脚本内容无法逐条审计，等同多语句注入；
- `redis-cli` 的宿主命令注入：`--pipe`、`-x` 读 stdin、交互模式 `!` 执行 shell 命令；
- 绕过上述规则改用其他连接路径（如应用调试接口、直连其他容器）。

发现命令同时包含允许和禁止操作时，整批拒绝执行。

## 敏感数据与输出

- 不输出 Redis 密码、连接串、Secret、API Key、JWT 或完整凭证；
- 缓存中常驻敏感数据：`jwt:user:*`（JWT 载荷）、`share:*`（分享令牌）、`trace:client:ticket:*`（客户端 ticket）、session 数据中的 Token/Authorization 字段。查询结果包含这类字段时，先脱敏再展示；
- 不为方便查询而把整库数据导出到本地文件；
- 只返回完成任务所需的最小字段和摘要（如仅 key 名、TTL、类型、大小）；
- 结果需说明是精确值还是估算值：`SCAN` 计数、`DBSIZE` 是精确值；`MEMORY USAGE` 是近似内存占用；不要拿扫描计数冒充精确行数。

## 失败处理

- SSH 失败：报告连接错误，不尝试使用裸 IP；
- Redis 容器不存在或不健康：停止并报告；
- 配置缺失或认证失败（如 `NOAUTH`、`WRONGPASS`）：停止，不猜测密码；
- 命令不明确、混入写操作或涉及禁止命令：停止并要求澄清/授权；
- 命令超时或失败：返回错误摘要，不自动重试可能产生副作用的命令；
- 不要为了“完成任务”降低只读和永久禁止规则。
