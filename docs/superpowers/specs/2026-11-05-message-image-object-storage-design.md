# Message 图片对象存储设计

日期：2026-11-05
状态：用户已确认（第 3 版：按代码核对结果修正；导出新增图片；修复 Anthropic 图片落库）

## 1. 背景与目的

**OpenAI 协议**的图片（`image_url` 的 data URI、Responses 的 `input_image`）进入统一结构后，以 `data:<mime>;base64,<data>` 字符串存放在 `vo.UnifiedContentPart.ImageURL`，随 `messages.message`（text 列，JSON 序列化）整条落库。

**Anthropic 协议**的图片目前根本存不下来（已用探针确认）：

- 消息里直接带 `image` 块：`dto.extractAnthropicBlocks` 报 `unknown anthropic content block type: "image"`，这轮对话的 message 和 session 都不落库（转发不受影响）。
- `tool_result` 里带图：`extractToolResultContent` 只取文字，图被静默丢弃。

所以存量 base64 只来自 OpenAI 协议。Anthropic 的历史图片已丢失，无法通过迁移找回。

带来三个问题：

1. **数据库膨胀**：base64 图片把 `messages` 表撑大，拖慢 session 详情、cron、dataset 导出等所有读取 message 的链路。用户确认"必须要做了"。
2. **前端看不到图**：`web/src/components/chat/multimodal-parts.tsx` 的 `PartImage` 只渲染 http/https 链接（`isSafeUrl`），data URI 被拦掉。
3. **Anthropic 图片丢失**：如上。

目标：图片写入对象存储，message 只存引用；Web 端 session 详情页可以查看图片；Anthropic 图片正常落库；存量 base64 迁移出 `messages`，最后回收磁盘空间。

## 2. 目标、非目标、反目标

**目标**
- message 中的 base64 图片**一律**替换为引用，图片本体由 `blobs` 表跟踪：上传成功的存在对象存储中，未上传成功的暂存在 `blobs` 表里，由重试任务补传。
- 图片压缩到 ≤ 2MB 后再存储（用户决定：只存压缩版）。
- 同一张图只存一份、只上传一次。
- message checksum 去重对图片的"base64 形态"和"引用形态"给出同一结果。
- Anthropic 协议的消息图片和 `tool_result` 图片都能落库。
- session 详情页的图片默认显示占位卡片，**点击后才加载**。
- dataset 导出新增图片：内联回 base64（用户决定，这是新能力，现有导出不含图片）。
- 提供可重复执行、可断点续跑的存量迁移命令 `object migrate`，迁移完成后执行 `VACUUM FULL messages`。

**非目标**
- session **列表页**不显示缩略图。
- **分享页**不提供图片查看（用户决定只做详情页）。分享页和详情页共用消息组件，引用形态在分享页显示为不可点击的卡片。
- 外部 http/https 图片链接原样保留，不抓取不转存（抓取用户提供的 URL 有 SSRF 风险，而且这类图本来也不占库空间）。
- `input_audio`、`file`、Anthropic `document` 等其他附件本次不处理。注意：Anthropic 的 `document` / `search_result` / `container_upload` 块目前同样会让整轮对话不落库，这是既有缺陷，本次不修。
- ~~现有 `internal/infrastructure/storage` 下的 audio 用户目录（`ObjDAO`、COS SDK）不重构、不改动。~~（2026-11-06 已整体删除，见第 14 节）
- ~~不做孤儿对象回收（message 被删后图片不会跟着删），以后单独做。~~（2026-11-06 已在软删清理任务中实现，见第 14 节）
- 不保留原图。

**反目标**（技术上能跑，但出现即视为失败）
- 代理转发链路的延迟增加，或因对象存储故障导致请求失败。
- 图片丢失：任何时刻，message 中的引用都必须能在 `blobs` 表中找到对应行，且该行的图片要么已在对象存储中，要么在 `blobs.data` 中。
- 用户能看到不属于自己 session 的图片。
- 迁移导致现有 message 去重或 session 前缀合并整体失效。**不含图片的 message 的 checksum 必须逐字节不变。**
- 生产 pod（内存上限 512Mi）因解码图片 OOM。

## 3. 存储模型

### 3.1 新表 `blobs`

| 列 | 类型 | 说明 |
|---|---|---|
| `id` | bigserial | 主键 |
| `sha256` | char(64) | **原始字节**（上游实际收到的 base64 解码后字节）的 SHA256，hex 小写，唯一索引 |
| `status` | varchar | `pending` / `uploaded` / `failed` / `unprocessable`，见 3.2 |
| `data` | bytea, nullable | 未上传时暂存的图片字节；`uploaded` 后置 NULL |
| `storage_key` | varchar | 对象 key，如 `images/ab/cd/<sha256>.jpg`，不含域名和 bucket；写行时即确定 |
| `mime` | varchar | 存储版本的 MIME，按字节内容判定（见第 4 节），不信任客户端声明 |
| `size` | bigint | 存储版本的字节数 |
| `width` / `height` | int | 存储版本的像素尺寸，无法解码时为 0 |
| `original_size` | bigint | 原始字节数（用于观测压缩效果） |
| `attempts` | int | 上传尝试次数 |
| `last_error` | varchar | 最近一次上传失败原因（截断保存，不含凭据） |
| `next_retry_at` | timestamptz | 下次重试时间，`pending` 时有效，与 `status` 建复合索引 |
| `created_at` / `updated_at` | timestamptz | |

`data` 存二进制字节，不存 base64 文本：同样的图会小约 25%，读出时再按需编码。表只追加、改状态，不删行，不用软删。

### 3.2 状态机

```
            上传成功
pending ───────────────▶ uploaded（data = NULL）
   │  ▲
   │  └─ 重试未成功、尚未超过上限：attempts+1，按退避推后 next_retry_at
   ▼
failed（超过重试上限，data 保留；可由 object migrate --retry-failed 重置为 pending）

unprocessable：无法解码、且原始字节 > 2MB 的图片。按"只存 ≤2MB"的规则不上传，
               原始字节保留在 data 中，不参与重试。前端照常可看，导出照常内联。
```

**不变式**：`status = uploaded` ⇔ 对象存储中已有 `storage_key` 的对象，且 `data IS NULL`；其余状态 `data IS NOT NULL`。

状态写入规则：
- 只有在 PUT 成功**之后**，才能把状态改为 `uploaded` 并清空 `data`，两者放在同一条 UPDATE 中。
- 状态只允许从 `pending`/`failed` 变成 `uploaded`，不能反向；UPDATE 带 `WHERE status IN ('pending','failed')` 条件，防止并发覆盖。

### 3.3 message 中的引用形态

`vo.UnifiedContentPart` 的 `type=image_url` 部分：
- `ImageURL` 改存 `aris-blob:sha256:<hex>`（前缀定义为常量）。
- 新增 `ImageMime`（json `image_mime`）、`ImageSize`（json `image_size`），均为 omitempty，供前端占位卡片显示"类型 · 大小"，不需要额外查表。
- `ImageDetail` 保持不变。

选择哈希引用、不直接存 S3 链接的原因：bucket 私有，预签名链接会过期；链接里带厂商域名，换厂商要重写全表；URL 形态与 base64 形态无法统一 checksum；也无法区分"我们托管的图"和"用户的外部链接"。

### 3.4 对象存储客户端

- 使用仓库已有依赖 `minio-go` 作为 S3 兼容客户端（R2 / COS 的 S3 兼容 endpoint / AWS S3 / MinIO），新建独立组件，不复用 `ObjDAO` 及其 `userID` 分目录语义。
- 新配置（环境变量）：`OBJECT_STORAGE_ENDPOINT`、`OBJECT_STORAGE_BUCKET`、`OBJECT_STORAGE_REGION`（默认 `auto`）、`OBJECT_STORAGE_ACCESS_KEY`、`OBJECT_STORAGE_SECRET_KEY`、`OBJECT_STORAGE_TLS`（默认 true）、`OBJECT_STORAGE_PATH_STYLE`（默认 false）。
- bucket 私有，只通过预签名 GET 链接访问。bucket 由运维预先创建。
- **未配置时**：写入路径照常外置，`blobs` 行全部停在 `pending`，图片数据留在 `blobs.data`；重试任务直接跳过；前端与导出从 `data` 读取。服务启动不能因为没配置而 panic（现有 `storage.GetPlatform()` 未配置时会 panic，新组件不能走它）。配置无效时记录 error 日志并按未配置处理，不能影响代理启动。`object migrate` 在未配置时拒绝执行，因为那样只是把数据从一张表挪到另一张表。

## 4. 压缩规则

输入为原始字节，输出为不超过 2MB 的存储版本：

1. 先用 `image.DecodeConfig` 读尺寸，像素总数超过 5000 万的视为异常输入，不做解码（防解压炸弹），按第 6 步处理。
2. 原始字节 ≤ 2MB 且长边 ≤ 4096px：**原样存储**，不重新编码（避免截图文字被二次压糊）。
3. 否则解码（按 EXIF Orientation 摆正），长边缩到 ≤ 4096px。
4. 源图为 PNG 时先编码为 PNG，≤ 2MB 即采用；否则（或源图本就是 JPEG/WebP/其他）铺白底转 JPEG，质量按 90 → 80 → 70 → 60 依次尝试，直到 ≤ 2MB。仍超限就把长边再乘以 0.75，然后重复本步。
5. GIF 只取第一帧，按静态图处理。WebP 解码后输出 PNG/JPEG（纯 Go 没有 WebP 编码器）。
6. 解码失败、格式不支持或超过像素上限：原始字节 ≤ 2MB 就原样作为存储版本，正常进入上传流程；否则写为 `unprocessable`，原始字节存入 `data`，打 warn 日志。

**MIME 判定**：`mime` 用 `http.DetectContentType` 按字节判定，只接受 `image/png`、`image/jpeg`、`image/gif`、`image/webp`，其余一律记为 `application/octet-stream`。这样用户声明的 `image/svg+xml` 不会以可执行内容的类型存进 bucket，前端也不会渲染。

**内存**：一张 5000 万像素的图解码约占 200MB，而 pod 上限 512Mi。解码用进程内互斥锁串行化，同一时间只解码一张（不需解码的小图不受影响）。

压缩只决定存储版本是什么。sha256 始终按原始字节计算，所以调整压缩参数不影响去重。

## 5. 写入流程（新数据）

### 5.1 Anthropic 图片进入统一结构

修改 `internal/dto/unified_message.go`：

- `extractAnthropicBlocks` 增加 `image` 分支：`source.type=base64` 转成 data URI，`source.type=url` 取 URL，其他 source 类型跳过。
- 有图片时，消息内容用 `Parts`（文本合并成一个 text 部分在前，图片按序在后）；**没有图片时保持原来的 `Text` 形态**，保证纯文本消息的 checksum 不变。
- `extractToolResultContent` 同样收集嵌套的 `image` 块，规则相同。
- 用户消息同时带 `tool_result` 和顶层图片时（Claude Code 常见），顶层图片追加到 tool_result 内容的末尾。其他行为（如顶层文本被 tool_result 覆盖）保持现状。

代价（用户已知晓并接受）：进行中的对话里，带图的 `tool_result` 算出的 checksum 会变，这些对话在上线后可能各被拆出一个新 session，只发生一次。

### 5.2 图片外置

入口：`internal/infrastructure/pool/store_pool.go` 的 `runMessageStoreTask`。它本身已经在异步协程池中执行，不在代理转发路径上，所以对象存储的耗时和故障都不会影响请求。

在计算 checksum、开启事务之前，增加一步"图片外置"：

1. 遍历 `task.Messages` 中所有 `image_url` 部分，只处理 `data:...;base64,` 形态（外部链接和已有 `aris-blob:` 引用直接跳过）。
2. 解码 base64，计算原始字节的 sha256。base64 本身非法的部分保留原样，不外置，打 warn 日志（这类数据上游本身也会拒绝，极少见）。
3. 一个 task 内按 sha256 去重，然后批量查 `blobs`（只查元数据列，不读 `data`）：
   - 已存在（任意状态）：直接复用，不压缩、不上传。
4. 不存在的，逐张处理：
   1. 压缩（第 4 节）。
   2. 对象存储已配置时，尝试 PUT 一次（单张超时 10s）。
   3. PUT 成功：`INSERT ... status=uploaded, data=NULL ON CONFLICT (sha256) DO NOTHING`。
   4. PUT 失败或未配置：`INSERT ... status=pending, data=<存储版本>, attempts=1, next_retry_at=now+5min ON CONFLICT (sha256) DO NOTHING`。
   5. `unprocessable`：`INSERT ... status=unprocessable, data=<原始字节> ON CONFLICT (sha256) DO NOTHING`。
   6. 插入后重查该行元数据，以数据库中的 mime/size 为准写入引用（并发时对方可能先插入）。
5. 所有图片的 `blobs` 行都确认存在后，才把 message 副本中的对应部分替换为引用（3.3）。替换在副本上进行，不就地修改可能被其他代码持有的原对象。
6. **`blobs` 行写入失败**（数据库错误）是唯一保留 base64 的情况：该图在 message 中保持 data URI，之后由 `object migrate` 补外置。checksum 不受影响（第 7 节）。

`blobs` 行的写入不放进 message 主事务：行在 message 之前独立提交，即使后面 message 事务失败，也只是多出一条未被引用的 blob 行，不会出现悬空引用。

并发安全：两个 task 同时处理同一张新图时，可能都会 PUT（同 key 幂等），也可能一个插入 `uploaded`、另一个插入 `pending` 时撞上 ON CONFLICT 被跳过。无论哪种情况，最终行都满足不变式；若留下的是 `pending`，重试任务会再 PUT 一次（幂等）。

这一步对整个 task 设置 60s 总超时，避免对象存储卡住时协程池被占满；超时后剩余图片不再尝试 PUT，直接走 `pending` 分支。

## 6. 上传重试任务

新增 cron `BlobUploadRetryCron`，挂在现有 cron 框架下（分布式锁保证单实例执行，写法参考 `SessionTerminalCleanupCron`）：

- 每 5 分钟执行一次；对象存储未配置时直接跳过。
- 选取 `status='pending' AND next_retry_at <= now()` 的 ID，按 `next_retry_at` 升序，每批 50 个。**先取 ID，再逐行读 `data`**，避免一次把 100MB 读进内存。
- 逐行 PUT `data` 到 `storage_key`：
  - 成功：`UPDATE ... SET status='uploaded', data=NULL WHERE id=? AND status IN ('pending','failed')`。
  - 失败：`attempts+1`，记录 `last_error`，`next_retry_at` 按指数退避推后（5min → 15min → 1h → 6h，封顶 24h）。`attempts` 达到 10 后改为 `failed`，打 error 日志。
- 把待传行数、`failed` 行数和本次成功数写进 cron 审计的 metadata，便于在后台观察积压情况。

`failed` 行的图片照常可看、可导出，只是还留在数据库里。排查好对象存储问题后，用 `object migrate --retry-failed` 把它们重置为 `pending`。

## 7. checksum 去重

修改 `vo.ComputeMessageChecksum`：规范化阶段把每个 `image_url` 部分统一映射为 `sha256:<hex>`：

- `data:...;base64,` 形态：解码后计算原始字节 sha256；解码失败则保持原字符串。
- `aris-blob:sha256:<hex>` 形态：直接取 hex。
- 外部 URL：保持原样。
- `ImageMime`、`ImageSize`、data URI 里的 mime 标签都不参与计算。

效果：同一张图不管是 base64 还是引用，算出的 checksum 都一样；第 5.2 节第 6 步保留了 base64 的 message，与外置成功的 message 也是同一个 checksum。

规范化在 content 的副本上进行，不修改入参。**不含 `data:` 或 `aris-blob:` 图片的 message，checksum 必须与改动前逐字节一致**（用固定值测试钉死）。外置之后 checksum 是对引用形态计算的，不会重复解码。

这是 checksum 算法的变更：已有含 base64 图片的 message 的 checksum 必须在迁移中重算（第 9 节）。

## 8. 前端查看（点击加载）

### 8.1 后端接口

`GET /api/web/v1/session/image?id=<sessionID>&sha256=<hex>`（JWT 分区，遵循 `docs/agents/go-backend.md` 的路由规范）：

1. 校验 `sha256` 为 64 位小写 hex。
2. 权限复用 `GetSessionMetaByUserHandler`，与 `HandleListSessionMessages` 完全一致（owner / admin / demo 白名单）。
3. 校验该 sha256 确实被这个 session 的某条 message 引用（防止拿着自己的 session ID 去探测别人的图）；未引用返回"资源不存在"。
4. 查 `blobs`：
   - `uploaded`：生成 10 分钟有效的预签名 GET 链接，返回 `{ url, expiresAt, mime, size, width, height }`。
   - `pending` / `failed` / `unprocessable`：返回 `{ dataUrl, mime, size, width, height }`，内容来自 `blobs.data`。`mime` 不是 `image/*` 时 `dataUrl` 为空，前端显示"无法预览"。
   - 行不存在：返回资源不存在。

session 消息接口本身不碰对象存储，也不读 `blobs`。

Redis 里的 message 内容缓存（TTL 60 分钟）不需要主动失效：迁移后旧的 base64 记录最多再留 60 分钟，这段时间图片不显示（和现状一样），到期自动恢复。

### 8.2 前端

- `content-extract.ts`：识别 `aris-blob:sha256:` 前缀，`ContentPart` 增加 `image_mime`、`image_size`；`ToolResultInfo` 增加 `parts`（tool_result 里的图目前不会被提取）。
- `multimodal-parts.tsx`：引用形态渲染占位卡片，显示"类型 · 大小"和"点击查看"，不发任何图片请求。点击后调接口，拿到 `url` 或 `dataUrl` 再渲染 `<img>`。`url` 形态可在新标签页打开。
- `tool-calls.tsx`：工具结果展开区渲染 `result.parts`，这样 Claude Code 的截图也能看到。
- `dataUrl` 只接受接口返回的、以 `data:image/` 开头的值，不放开 `isSafeUrl` 对 message 原文中 data URI 的拦截。
- 链接过期（`img` 加载失败）后允许再次点击重新获取。
- 通过 React context 提供 `sessionId`：只有 `session-detail-client.tsx` 提供，分享页不提供，此时卡片不可点击。
- 存量里还没迁移的 data URI：维持现状（不显示）。迁移完成后就不存在了。

仓库目前没有 CSP 配置，`<img>` 加载也不需要 CORS，这两项不用改。

## 9. 存量迁移：`object migrate`

挂在现有 `cmd/server/object.go` 的 `objectCmd` 下：`server object migrate`。

**扫描**
- 不加 `deleted_at` 条件（软删的 message 也占空间，一并处理），按 `id` 升序游标分批，批大小默认 200。
- SQL 预过滤只选出含 data URI 图片的行：`message LIKE '%"image_url":"data:%'`（`message` 是 text 列，这是 sonic 序列化后的字节形态）。**先只取 ID，再逐行读取**，避免把一批大字段同时读进内存。
- 读取时用原始字符串扫描，逐行解析；解析失败的行记 error 日志并跳过，不中断整体迁移。
- 参数：`--batch-size`、`--start-id`（断点续跑）、`--limit`、`--dry-run`（只统计条数、去重后图片数与预估节省字节，不上传不写库）、`--retry-failed`（只把 `failed` 的 blob 重置为 `pending`，然后退出）。
- 每批结束输出进度日志：当前游标、已处理行数、新建 blob 数（分 uploaded / pending / unprocessable）、复用数、累计节省字节数。

**单行处理**
1. 对行内每张 data URI 图片执行与第 5.2 节相同的外置逻辑（复用同一组件）。上传失败的图进入 `pending`，由重试任务处理，**不影响这一行替换为引用**。
2. 生成新的 message JSON，按第 7 节重算 checksum（`modelID` 取该行的 `model_id`）：
   - 若 message 含 `ToolCalls`：迁移时拿不到原始的 tool schema，重算可能与在线算法不一致，所以**只替换 message、不改 checksum**。带图片的 assistant tool_call 消息实际上几乎不存在。
   - 若新 checksum 已被**其他行**占用（`idx_message_checksum` 唯一冲突，例如同一张图的 mime 标签不同）：**只替换 message、保留旧 checksum**。该行仍被 session 正常引用、内容正确，只是以后不再参与去重；不做跨 session 的 ID 重映射，规避高风险写操作。
   - 其他情况：message 与 check_sum 在同一条 UPDATE 中更新。
3. 写库前先把 message 序列化成 JSON 文本再 UPDATE，不要把结构体或切片直接交给 `Update(column, value)`（仓库已有 GORM serializer 踩坑先例）。
4. 只有 `blobs` 行写入失败的图片会留在 message 中，下次重跑时会再次被预过滤选中。

**幂等**：已是引用形态的部分不会被预过滤选中、也不会被处理；重复执行安全。

**与在线写入并存**：迁移期间服务正常运行。新版本上线后、迁移完成前这段时间，若有请求续写一个"图片还没迁移"的旧会话，新算法算出的 checksum 与旧行对不上，会插入一条内容相同的新 message 行，这次续写可能被拆成一个新 session。迁移时该旧行会走"checksum 冲突"分支。这个窗口通过"上线后立即执行迁移"来缩短，属于用户已确认的一次性代价。

## 10. dataset 导出

现状：`internal/application/dataset/converter/sharegpt.go` 的 `extractText` 只取文本，导出不含图片。本次新增：

- `ShareGPTConversation` 新增顶层 `images []string`（omitempty），内容为 data URI，按出现顺序排列。
- 图片在 message 的 `value` 中对应位置放 `<image>` 占位符，占位符个数与 `images` 长度一致（LLaMA-Factory 的 ShareGPT 多模态约定）。不含图片的会话，输出与现在逐字节一致。
- 取图：`aris-blob:` 引用从对象存储（`uploaded`）或 `blobs.data`（其他状态）取回；尚未迁移的 data URI 原样放入；外部 URL 原样放入。
- 一次导出内按 sha256 缓存已取到的图片，缓存总量上限 64MB，超限后清空重来（简单够用，pod 内存有限）。
- 单张图获取失败：跳过这张图及其占位符，打 warn 日志并在导出结束时汇总失败数，导出不中断。
- 格式预览（preview-format）是给浏览器看的，不取图，`images` 中显示 `data:<mime>;base64,<已省略 N KB>`，避免把几 MB 的字符串塞进页面。

已知限制：前端 `dataset/page.tsx` 把所有导出行累积在内存中再生成文件。含大量图片的导出可能撑爆浏览器标签页。本次不改，遇到时通过筛选条件缩小范围，后续可改为流式写文件。

## 11. 上线步骤

1. 配置对象存储并创建私有 bucket，在 `api.env` 中配置 `OBJECT_STORAGE_*`。
2. 执行 `database migrate`（建 `blobs` 表）。
3. 滚动部署新版本，新图片开始外置，Anthropic 图片开始落库。
4. 立即执行 `object migrate --dry-run` 评估规模，然后执行正式迁移；完成后再跑一次，补齐部署窗口期内产生的 base64 行（预期为 0 或极少）。
5. 确认 `messages` 中已无 data URI 图片（预过滤 SQL 计数为 0）。
6. 等重试任务把 `pending` 清空，并处理完 `failed`。这一步不影响 `VACUUM FULL messages`（图片已经不在 `messages` 里），但决定 `blobs` 表最终占多大空间。
7. **维护窗口执行 `VACUUM FULL messages`**：会持有 ACCESS EXCLUSIVE 锁，期间 messages 读写全部阻塞（session 落库会排队或失败，代理转发不受影响）。执行前确认磁盘剩余空间足以容纳表的新副本，并记录 `pg_total_relation_size('messages')` 的前后值。生产数据库写操作需届时单独授权。

`blobs` 表在大量 `data` 被置 NULL 后同样会留下空闲空间，必要时之后再对它执行一次 `VACUUM FULL blobs`（表小、锁时间短）。

## 12. 测试要求

- checksum：同一张图的 data URI、不同 mime 标签的 data URI、引用形态三者 checksum 相同；外部 URL 不受影响；纯文本消息与外部 URL 消息的 checksum 与改动前的固定值一致；入参不被修改。
- Anthropic：顶层图片（base64/url）、`tool_result` 内图片、两者并存都能转换；纯文本与不含图的 tool_result 仍是 `Text` 形态。
- 压缩：小图原样保留；大 PNG、大 JPEG、超大尺寸、GIF、WebP、带 EXIF 旋转的 JPEG 输出均 ≤ 2MB 且方向正确；超像素上限输入不解码；不可解码的大图进入 `unprocessable`；声明为 SVG 的内容 mime 不是 `image/svg+xml`。
- 写入：同一 task 内重复图只处理一次；已存在的 sha256 不上传；上传失败时写入 `pending` 且 `data` 非空、message 仍替换为引用（用假的对象存储实现注入失败）；`blobs` 写库失败时 message 保留 base64 且 checksum 不变；未配置对象存储时全部为 `pending`；入参 message 不被修改。
- 状态不变式：`uploaded` 行 `data` 恒为 NULL；`uploaded` 不会被覆盖回 `pending`。
- 重试任务：成功后转 `uploaded` 并清空 `data`；失败后 `attempts` 递增、`next_retry_at` 推后；达到上限转 `failed`；`--retry-failed` 可重置；未到 `next_retry_at` 的不处理。
- 迁移（sqlite）：幂等重跑；checksum 冲突分支；含 ToolCalls 分支；上传失败时该行仍完成替换；`--dry-run` 不写库；解析失败的行被跳过且不中断；迁移后的行能经 GORM 正常读回。
- 图片接口 e2e：owner 可取；`uploaded` 返回 url、`pending` 返回 dataUrl；他人 session 无权限；sha256 不在该 session 中时拒绝；sha256 格式非法时拒绝。沿用 `test/e2e/cross_tenant_session` 的真实路由装配模式。
- dataset 导出：两种状态的引用都被内联且占位符个数匹配；获取失败时跳过且导出完成；不含图片的会话输出不变；预览不取图。
- 前端：占位卡片在点击前不发请求；点击后 url / dataUrl 两种返回都能渲染；过期后可重试；分享页不可点击；tool_result 的图能显示。

## 13. 交给实施者的决定

- 常量名、函数内部结构（遵循仓库约定）。
- 单元测试 fixture 的具体构造。

## 14. 2026-11-06 CR 修订

本节覆盖前文中与之冲突的描述。

1. **伪造引用（安全）**：原实现只外置 `data:` 图片，客户端自带的 `aris-blob:sha256:<他人图片>` 会原样入库；而图片接口用 `message LIKE '%aris-blob:sha256:<hex>%'` 判断"会话引用了该图"，文本里出现这串字符也能通过。修订：
   - 落库前 `vo.NeutralizeImageRefs` 把所有 `aris-blob:` 前缀的 image_url 部分降级为文本部分，库里的引用只能由外置器生成。
   - 判断引用改为两步：`LIKE '%"image_url":"<完整引用>"%'` 预过滤（不解析 JSON，sqlite/PG 通用），再在 Go 里反序列化，只认 `type=image_url` 且值相等的部分。不直接用 jsonb：`messages.message` 是 text 列，按字段判断要逐行 `::jsonb` 解析，且 sqlite 测试库不支持。
2. **图片上传池**：第 5.2 节改为由 `blobstore.Uploader` 处理新图。
   - 服务进程使用独立 pond 池（`POOL_BLOB_WORKERS` 默认 2，`POOL_BLOB_QUEUE_SIZE` 默认 200），与消息/审计落库的 store 池隔离。落库任务不再等待压缩和上传：新图立即替换为引用（`image_mime`/`image_size` 取原图值），图片本体在内存中排队。
   - 池中处理：压缩 → PUT 一次 → 成功写 `uploaded` 行；失败写 `pending` 行（压缩后字节，attempts=1）；`unprocessable` 照旧。
   - 队列满、内存中原图总字节超过 64MB 或进程退出排空时，直接把**原图**写成 `pending` 行（attempts=0，立即到期），由重试任务压缩后补传；重试任务上传成功时写回压缩后的 mime/size/宽高/storage_key，压缩后仍超限则转 `unprocessable`。
   - 同一 sha 在内存中排队或处理时，后续提交直接复用，不重复处理。
   - 退出顺序：store 池排空 → 图片池排空（剩余图片不再压缩上传，直接落 pending）→ agent 池。
   - 已知代价：进程被强杀（OOM/SIGKILL）时内存中未处理的图片会丢失，message 中的引用变为悬空（接口返回"图片不存在"）。这放宽了第 2 节"图片丢失"反目标，用户已确认。
   - `object migrate` 使用同步模式（不经过池），保证 message 改写前 blobs 行已存在。第 5.2 节的 60s 任务总超时随之删除。
3. **数据集导出改为 NDJSON 流**：`GET /dataset/export` 返回 `application/x-ndjson`，每行一条 ShareGPT JSON，`X-Total-Sessions` 头给出匹配总数。服务端每批读 50 个会话的 message/tool，写完即 Flush；无匹配、鉴权失败等在流开始前以普通 JSON 业务错误返回。客户端以收到的行数少于总数判定中断。前端收集原始字节块合成 Blob，不再拼字符串。单会话内联的 data URI 总量上限 32MB（超出的图片连同 `<image>` 占位符跳过），跨会话缓存上限 64MB 降到 16MB。
4. **生命周期**：软删清理 cron 在硬删孤儿 message 前收集它们引用的图片，删除后对其中不再出现在任何 message（含软删行，子串匹配，宁多留不误删）中的图片删除对象与 blobs 行；对象删除失败或未配置对象存储时保留 `uploaded` 行。与在线写入之间无锁，秒级窗口内复用同一张图的新 message 可能丢图（已用 ponytail 注释标记）。
5. **unprocessable 不作预览**：图片接口对 `unprocessable` 行只返回元数据，不返回 dataUrl（原图可能远超 2MB），前端显示"图片过大，无法预览"；导出照常内联。
6. **audio 目录与旧对象存储删除**：OAuth 新用户注册时创建空 `audio/<userID>/` 目录是项目模板遗留，`ObjDAO` 的读写接口无调用方，且建目录失败会让登录失败。已删除 `internal/infrastructure/storage`、`AudioDirCreator`、`object bucket create` 命令、COS/MinIO 配置项与 `cos-go-sdk-v5` 依赖。新 objectstore 改为进程内单例 `objectstore.Default()`，fx、cron、CLI 共用。配置键改为点号风格（`object.storage.access.key` 等），环境变量名不变。
7. 其他：`TruncateError` 按字符截断；`s3Store.Get` 限读 2MB+1 字节；通用常量从 `constant/blob.go` 归位到 `sql.go`/`database.go`/`http.go`/`string.go`；格式预览占位文案改为英文 `<omitted N KB>`。
