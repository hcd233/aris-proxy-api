# Message 图片对象存储 实施计划

> **执行方式：** 按用户规则不使用子 Agent，由主会话使用 superpowers:executing-plans 逐任务实施。步骤用复选框（`- [ ]`）跟踪。

**目标：** message 中的 base64 图片外置到 S3 兼容对象存储（失败的暂存 `blobs` 表），checksum 对两种形态一致；Anthropic 图片正常落库；详情页点击查看图片；导出内联图片；提供存量迁移命令。

**架构：** `internal/infrastructure/blobstore` 提供压缩、外置（Externalizer）、读取（Reader）、重试、迁移，被 `PoolManager`、cron、cobra 命令、应用层复用。应用层只依赖 `internal/domain/blob` 的接口。message 只存 `aris-blob:sha256:<hex>`。

**技术栈：** Go 1.25、gorm、minio-go v7（已有）、disintegration/imaging + golang.org/x/image（新增）、huma + fiber v3、Next.js + vitest。

**设计文档：** `docs/superpowers/specs/2026-11-05-message-image-object-storage-design.md`（执行前必读，本计划不重复其中的流程细节）。

## 全局约束

- 存储版本 ≤ 2MB（`2 << 20` 字节）；长边 ≤ 4096px；解码像素上限 5000 万；JPEG 质量 90 → 80 → 70 → 60；仍超限时长边 × 0.75。
- sha256 始终按**原始字节**计算；hex 小写。引用前缀 `aris-blob:sha256:`。
- 预签名有效期 10 分钟；单张 PUT 超时 10s；单 task 外置总超时 60s。
- 重试：cron `*/5 * * * *`，每批 50，退避 5min/15min/1h/6h/24h，`attempts` 达 10 转 `failed`。
- 不含图片的 message，checksum 必须逐字节不变；不含图片的会话，导出输出必须逐字节不变。
- 项目硬约束：错误走 `ierr`（禁 `errors.New`/`fmt.Errorf`）；JSON 用 `sonic`；业务包禁本地 `const`（放 `internal/common/constant` / `enum`）；测试只放 `test/unit/<topic>/`、`test/e2e/<topic>/`，只用标准库 `testing`，禁 `time.Sleep`；日志前缀 `[PascalCase]`；凭据只走环境变量，日志不得输出。
- 编辑 Go 文件前跑 `sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path <file>`（不得过滤输出）；加载 `golang-naming`、`golang-code-style`、`golang-samber-lo`、`ponytail`；改 fx / cobra / viper / DTO 时分别加载 `golang-uber-fx`、`golang-spf13-cobra`、`golang-spf13-viper`、`huma-dto-conventions`。
- pre-commit hook 跑全仓 vet/test/lint，且会把已跟踪改动并入当前提交，所以每个任务结束时工作区必须能完整编译通过，接口变更与其全部调用方同批提交。

## 重点防范（设计文档未写明、最容易出事的输入）

1. 带参数或不规范的 data URI：`data:image/png;charset=utf-8;base64,...`、无 padding、含换行。应能解析，且与规范写法得到同一 sha256。（Task 1）
2. 截断或只有文件头的图片（如 `iVBORw0KGgo=`，解码器可能报错甚至 panic）。应原样存储，不能 panic，不能丢图。（Task 3）
3. 同一张图在一条消息里出现两次且 `detail` 不同；`image_url` 为空串（Responses 的 `file_id` 图片）。两处都替换且各自保留 detail；空串不动。（Task 5）
4. demo 用户取白名单内外会话的图片。白名单校验必须和消息接口一样生效。（Task 8）
5. `uploaded` 行但对象存储后来被取消配置。接口返回业务错误，导出跳过该图，都不能 panic。（Task 8、Task 9）

---

### Task 1: 引用类型与 checksum 规范化

**文件：**
- 新建：`internal/common/constant/blob.go`、`internal/common/enum/blob_status.go`、`internal/common/vo/image_ref.go`
- 修改：`internal/common/vo/unified_message.go:72-82`、`internal/common/vo/checksum.go:59-97`
- 测试：`test/unit/message_checksum/image_checksum_test.go`

**接口：**
- 产出（常量）：`BlobRefPrefix`、`BlobChecksumPrefix="sha256:"`、`DataURLPrefix="data:"`、`DataURLBase64Marker=";base64,"`、`BlobStorageKeyTemplate="images/%s/%s/%s%s"`、`BlobMaxStoredBytes`、`BlobMaxLongEdge`、`BlobMaxDecodePixels`、`BlobDownscaleFactor`、`BlobJPEGQualities`、`BlobPutTimeout`、`BlobTaskTimeout`、`BlobPresignTTL`、`BlobRetryBatchSize`、`BlobRetryMaxAttempts`、`BlobRetryBackoff`、`BlobLastErrorMaxLen=512`、`BlobExportCacheMaxBytes=64<<20`、`BlobMigrateDefaultBatchSize=200`、`ShareGPTImagePlaceholder="<image>"`。
- 产出（枚举）：`enum.BlobStatus`，取值 `BlobStatusPending/Uploaded/Failed/Unprocessable`。
- 产出（vo）：`UnifiedContentPart.ImageMime string`（`image_mime,omitempty`）、`ImageSize int64`（`image_size,omitempty`）；
  `ParseImageDataURI(url string) (mime string, raw []byte, ok bool)`、`ParseImageRef(url string) (sha string, ok bool)`、`ImageRefOf(sha string) string`、`SHA256Hex(raw []byte) string`。

- [ ] **Step 1: 开工准备**

```bash
git worktree add .worktrees/message-image-storage -b feature/message-image-storage-2026-11-05
mkdir -p .worktrees/message-image-storage/internal/web/dist && echo '<!doctype html>' > .worktrees/message-image-storage/internal/web/dist/index.html
ln -s ../../../web/node_modules .worktrees/message-image-storage/web/node_modules
cp docs/superpowers/specs/2026-11-05-message-image-object-storage-design.md .worktrees/message-image-storage/docs/superpowers/specs/
cp docs/superpowers/plans/2026-11-05-message-image-object-storage.md .worktrees/message-image-storage/docs/superpowers/plans/
```

之后所有路径都以 worktree 为根（改文件前核对路径，防止误改主工作区）。读根目录 `CONTEXT.md` 与 `web/CONTEXT.md`。

- [ ] **Step 2: 在未改 checksum 的代码上记录固定值**

写 `TestChecksumGoldenUnchanged`，覆盖 4 类消息：纯文本；text + 外部 URL 图片的 `Parts`；带 `ToolCallID` 的 tool 消息；带 `ToolCalls` 的 assistant（`modelID="gpt-4"`）。先用空字符串占位，运行后把实际值填回测试，确认在旧代码上 PASS。这些值是"不含图片的 message 不变"的唯一证据，**必须在改 checksum 前取得**。

- [ ] **Step 3: 写失败的测试**

```go
func TestChecksumImageFormsEqual(t *testing.T)            // data URI、不同 mime 标签的 data URI、ImageRefOf(sha) + ImageMime/ImageSize 三者相等
func TestChecksumDifferentImagesDiffer(t *testing.T)
func TestChecksumExternalURLKeepsLiteral(t *testing.T)    // 不同外部 URL 不相等，且不被当作图片哈希
func TestChecksumDoesNotMutateInput(t *testing.T)         // 算完后入参 Parts[i].ImageURL 不变
func TestChecksumInvalidBase64Stable(t *testing.T)        // "data:image/png;base64,@@@" 不 panic、结果稳定
func TestParseImageDataURI(t *testing.T)
// 表驱动：规范写法、charset 参数、无 padding、含 \n 四种写法 sha 相同且 ok；
// 非 base64 的 "data:image/svg+xml,<svg/>"、"https://x/a.png"、"" 的 ok=false
func TestParseImageRef(t *testing.T)                      // 合法；长度不对、大写、非 hex、无前缀均 ok=false
```

- [ ] **Step 4: 运行确认失败**

运行：`go test -count=1 ./test/unit/message_checksum/`
预期：编译失败，`ParseImageDataURI` 未定义。

- [ ] **Step 5: 实现**

`ParseImageDataURI`：要求 `DataURLPrefix` 开头，用 `strings.Index` 找 `DataURLBase64Marker`，mime 取前缀之后到第一个 `;`；先去掉 `\r\n`，再依次试 `base64.StdEncoding`、`base64.RawStdEncoding`。

`checksum.go`：新增 `normalizeImageParts(*UnifiedContent) *UnifiedContent`，**没有 `data:` 或引用形态的部分时返回原指针**，否则返回新 `UnifiedContent`（部分逐个复制，`ImageURL` 映射为 `BlobChecksumPrefix+hex`，清空 `ImageMime`、`ImageSize`）。在现有 Content/ReasoningContent 规范化之后调用。

- [ ] **Step 6: 运行确认通过**

运行：`go test -count=1 ./test/unit/message_checksum/ ./test/unit/message_dedup/`
预期：全部 PASS，含 Step 2 的固定值用例。

- [ ] **Step 7: 提交**

```bash
git add -A internal/common test/unit/message_checksum docs/superpowers
git commit -m "feat(blob): 新增图片引用类型并统一 checksum 规范化"
```

---

### Task 2: Anthropic 图片进入统一结构

**文件：**
- 修改：`internal/dto/unified_message.go:204-318`
- 测试：`test/unit/anthropic_image/anthropic_image_test.go`

**接口：**
- 消费：`constant.DataURLTemplate`、`enum.ImageSourceTypeBase64/URL`、`enum.AnthropicContentBlockTypeImage`。
- 产出：`FromAnthropicMessage`、`FromAnthropicResponse` 的行为变化，签名不变。

- [ ] **Step 1: 写失败的测试**

```go
func TestTopLevelBase64Image(t *testing.T)
// text + image → Parts[0]={text}, Parts[1].ImageURL=="data:image/png;base64,iVBORw0KGgo="，err==nil
func TestTopLevelURLImage(t *testing.T)        // Parts[0].ImageURL=="https://x/a.png"
func TestImageOnlyMessage(t *testing.T)        // 只有 image，无 text part
func TestToolResultWithImage(t *testing.T)     // ToolCallID 正确；Parts=[text "截图如下", image]
func TestToolResultImageOnly(t *testing.T)     // Content 非 nil，只有 image part
func TestToolResultPlusTopLevelImage(t *testing.T) // 顶层图追加到 tool_result 内容末尾
func TestTextOnlyKeepsTextForm(t *testing.T)   // 无图：Content.Text 非空且 len(Parts)==0（含 tool_result）
func TestImageWithNilSourceSkipped(t *testing.T) // Source==nil 不报错、不产生 part
```

- [ ] **Step 2: 运行确认失败**

运行：`go test -count=1 ./test/unit/anthropic_image/`
预期：FAIL，`unknown anthropic content block type: "image"`。

- [ ] **Step 3: 实现**

新增私有函数 `anthropicImagePart(block *AnthropicContentBlock) *vo.UnifiedContentPart`（source 为 nil 或类型未知时返回 nil）与 `buildPartsContent(texts []string, images []*vo.UnifiedContentPart) *vo.UnifiedContent`（无图时返回 `Text` 形态，无文本也无图时返回 nil）。`extractAnthropicBlocks` 增加 image 分支；`extractToolResultContent` 收集嵌套图片；tool_result 分支把顶层图片追加到内容末尾（`Text` 形态先转成一个 text part）。

- [ ] **Step 4: 运行确认通过**

运行：`go test -count=1 ./test/unit/anthropic_image/ ./test/unit/converter/ ./test/unit/message_dedup/ ./test/unit/unified_response/`
预期：PASS。若已有用例断言 `image` 块报错，那是旧缺陷的固化，改断言并在提交说明中注明。

- [ ] **Step 5: 提交**

```bash
git add internal/dto test/unit
git commit -m "fix(anthropic): 消息和 tool_result 中的图片正常落库"
```

---

### Task 3: 图片压缩

**文件：**
- 新建：`internal/infrastructure/blobstore/compress.go`
- 测试：`test/unit/blobstore/compress_test.go`

**接口：**
- 产出：
```go
type CompressResult struct {
    Data          []byte
    Mime          string
    Width, Height int
    Unprocessable bool // 无法解码且 > 2MB：Data 为原始字节
}
func Compress(raw []byte) CompressResult
func StorageKey(sha, mime string) string // images/ab/cd/<sha>.jpg；扩展名 .png/.jpg/.gif/.webp/.bin
```

- [ ] **Step 1: 添加依赖**

运行：`go get github.com/disintegration/imaging@v1.6.2 golang.org/x/image`
预期：`go.mod` 新增两项，`go build ./...` 通过。

- [ ] **Step 2: 写失败的测试**

测试内用噪声像素生成图片（噪声不易压缩，才能超过 2MB）。

```go
func TestSmallImageKeptVerbatim(t *testing.T)     // bytes.Equal(res.Data, raw)；Mime=image/png；宽高正确
func TestLargePNGUnder2MB(t *testing.T)           // len(res.Data)<=2<<20；可解码
func TestLargeJPEGUnder2MB(t *testing.T)          // Mime=image/jpeg
func TestOversizeEdgeDownscaled(t *testing.T)     // 5000x100 小体积 PNG：长边==4096
func TestAnimatedGIFFirstFrame(t *testing.T)      // >2MB 的 GIF：输出可解码、≤2MB、Mime≠image/gif
func TestWebPDecoded(t *testing.T)                // 用极小 WebP 字节：≤2MB 时原样，Mime=image/webp
func TestExifOrientationApplied(t *testing.T)     // 5000x1000 JPEG 插入 Orientation=6 的 APP1：输出 Height>Width
func TestPixelBombNotDecoded(t *testing.T)        // 头部声明 20000x20000 的小 PNG：不解码，原样存储，Width==0
func TestUndecodableLargeIsUnprocessable(t *testing.T) // 3MB 随机字节：Unprocessable==true，Data 为原字节
func TestTruncatedImageKept(t *testing.T)         // base64 解码 "iVBORw0KGgo="：不 panic，原样存储，Mime=image/png
func TestSVGNotTrusted(t *testing.T)              // "<svg xmlns=...>"：Mime==application/octet-stream
func TestStorageKey(t *testing.T)                 // "images/ab/cd/abcd...e.jpg"
```

- [ ] **Step 3: 运行确认失败**

运行：`go test -count=1 ./test/unit/blobstore/`
预期：编译失败，`Compress` 未定义。

- [ ] **Step 4: 实现**

`mime` 用 `http.DetectContentType` 判定并按白名单收窄。流程照设计文档第 4 节。解码用 `imaging.Decode(r, imaging.AutoOrientation(true))`（空白导入 `golang.org/x/image/webp` 注册格式），缩放用 `imaging.Fit(img, edge, edge, imaging.Lanczos)`，转 JPEG 前用 `imaging.OverlayCenter(imaging.New(w, h, color.White), img, 1)` 铺白底。

解码到编码的部分放在包级 `sync.Mutex` 内，并用 `defer recover()` 把解码器 panic 转成"解码失败"分支。

- [ ] **Step 5: 运行确认通过**

运行：`go test -count=1 ./test/unit/blobstore/`
预期：PASS。

- [ ] **Step 6: 提交**

```bash
git add go.mod go.sum internal/infrastructure/blobstore test/unit/blobstore
git commit -m "feat(blob): 新增图片压缩，存储版本不超过 2MB"
```

---

### Task 4: blobs 表、DAO、对象存储客户端

**文件：**
- 新建：`internal/domain/blob/blob.go`、`internal/infrastructure/database/model/blob.go`、`internal/infrastructure/database/dao/blob.go`、`internal/infrastructure/objectstore/s3.go`
- 修改：`internal/infrastructure/database/model/base.go:21-38`（加入 `&Blob{}`）、`internal/config/config.go`、`env/api.env.template`
- 测试：`test/unit/blobstore/dao_test.go`、`test/unit/objectstore/s3_test.go`

**接口：**
- 产出（domain）：
```go
type ObjectStore interface {
    Put(ctx context.Context, key, contentType string, data []byte) error
    Get(ctx context.Context, key string) ([]byte, error)
    PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}
type ImageView struct {
    Mime          string
    Size          int64
    Width, Height int
    URL           string     // uploaded
    ExpiresAt     *time.Time // uploaded
    Data          []byte     // 其他状态
}
type ImageReader interface {
    IsReferenced(ctx context.Context, messageIDs []uint, sha string) (bool, error)
    Locate(ctx context.Context, sha string) (*ImageView, error) // 无此行返回 nil, nil
    ReadBytes(ctx context.Context, sha string) (mime string, data []byte, err error)
}
```
- 产出（model）：`dbmodel.Blob`，列见设计文档 3.1。**不嵌入 `BaseModel`**（不软删）；`SHA256` 唯一索引 `idx_blob_sha256`；`(status, next_retry_at)` 复合索引 `idx_blob_status_retry`；`Data []byte` 不写 `type`，让方言决定。
- 产出（DAO，`dao.GetBlobDAO()`）：`FindMetaBySHA256s(db, shas) ([]*dbmodel.Blob, error)`（不查 `data`）、`InsertIgnoreConflict(db, *dbmodel.Blob) error`、`GetBySHA256(db, sha) (*dbmodel.Blob, error)`（无行返回 nil, nil）、`GetByID(db, id)`、`ListDueRetryIDs(db, now time.Time, limit int) ([]uint, error)`、`MarkUploaded(db, id) error`、`MarkRetry(db, id, attempts int, lastError string, next time.Time) error`、`MarkFailed(db, id, attempts int, lastError string) error`、`ResetFailed(db, now time.Time) (int64, error)`、`CountByStatus(db, status) (int64, error)`。
- 产出（config）：`ObjectStorageEndpoint/Bucket/Region/AccessKey/SecretKey string`、`ObjectStorageTLS/PathStyle bool`（键 `object.storage.endpoint` 等；默认 region=`auto`、tls=true、path_style=false）。
- 产出（objectstore）：`NewFromConfig() blob.ObjectStore`。endpoint 为空返回字面量 `nil`（避免 typed-nil）；配置无效时记 error 日志并返回 `nil`，不 panic，不发起网络请求。

- [ ] **Step 1: 写失败的测试**

```go
// dao_test.go（sqlitetest.OpenPrivateMemoryDB + AutoMigrate(&dbmodel.Blob{})）
func TestInsertIgnoreConflictKeepsFirst(t *testing.T)  // 第二次插入同 sha 无错误，status 仍为第一次的值
func TestFindMetaOmitsData(t *testing.T)               // Data==nil，Mime/Size 正确
func TestMarkUploadedClearsData(t *testing.T)          // status=uploaded 且 Data==nil
func TestMarkRetryCannotOverwriteUploaded(t *testing.T)// 对 uploaded 行调用 MarkRetry/MarkFailed 后状态不变
func TestListDueRetryIDs(t *testing.T)                 // 只返回 pending 且到期的，按 next_retry_at 升序，受 limit 限制
func TestResetFailed(t *testing.T)                     // failed→pending、attempts=0、next_retry_at=now；返回行数
// s3_test.go（httptest 服务器模拟 S3，region=us-east-1，tls=false）
func TestNewFromConfigUnconfiguredIsNil(t *testing.T)  // endpoint 空：store==nil
func TestNewFromConfigInvalidIsNil(t *testing.T)       // endpoint="http://bad host"：nil，不 panic
func TestPutSendsKeyAndContentType(t *testing.T)       // PUT 路径含 bucket 和 key；Content-Type=image/png；body 一致
func TestGetReturnsBody(t *testing.T)
func TestPresignGet(t *testing.T)                      // URL 含 key、X-Amz-Signature、X-Amz-Expires=600
```

- [ ] **Step 2: 运行确认失败**

运行：`go test -count=1 ./test/unit/blobstore/ ./test/unit/objectstore/`
预期：编译失败。

- [ ] **Step 3: 实现**

`MarkRetry`、`MarkFailed`、`MarkUploaded` 的 UPDATE 都带 `status IN ('pending','failed')`。`ResetFailed` 只作用于 `failed`。`Put` 用 `minio.PutObjectOptions{ContentType, DisableMultipart: true}`；`minio.Options.BucketLookup` 在 `PathStyle=true` 时取 `BucketLookupPath`，否则 `BucketLookupAuto`。

`env/api.env.template` 增加 `OBJECT_STORAGE_*` 七项（值用 `xxx` 占位）。

- [ ] **Step 4: 运行确认通过**

运行：`go test -count=1 ./test/unit/blobstore/ ./test/unit/objectstore/ && go build ./...`
预期：PASS。

- [ ] **Step 5: 提交**

```bash
git add internal env test/unit
git commit -m "feat(blob): 新增 blobs 表与 S3 兼容对象存储客户端"
```

---

### Task 5: 图片外置并接入落库

**文件：**
- 新建：`internal/infrastructure/blobstore/externalizer.go`
- 修改：`internal/infrastructure/pool/pool.go:29-61`、`internal/infrastructure/pool/store_pool.go:41-62`、`internal/bootstrap/modules/infra.go:23-73`；调用方 `test/unit/pool_manager/pool_manager_test.go`、`test/unit/pool_manager/audit_write_test.go`、`test/unit/session_owner_id/store_pool_write_test.go`、`test/unit/message_dedup/message_dedup_test.go`（新增参数传 `nil`）
- 测试：`test/unit/blobstore/externalizer_test.go`、`test/unit/blobstore/fake_store_test.go`、`test/unit/blobstore/store_pool_test.go`

**接口：**
- 消费：`Compress`、`StorageKey`、`dao.GetBlobDAO()`、`blob.ObjectStore`、`vo.ParseImageDataURI`。
- 产出：
```go
type ExternalizeStats struct {
    Uploaded, Pending, Unprocessable, Reused, Failed int
    SavedBytes int64 // message 中被移走的 data URI 字节数
}
func NewExternalizer(db *gorm.DB, store blob.ObjectStore) *Externalizer
func (e *Externalizer) Externalize(ctx context.Context, msgs []*vo.UnifiedMessage) ([]*vo.UnifiedMessage, ExternalizeStats)
```
  返回新切片；无图片的消息保持原指针；不修改入参。
- 产出：`pool.NewPoolManager(db, auditRepo, demoAccessAuditRepo, sessionCache, externalizer *blobstore.Externalizer)`，`externalizer` 为 nil 时跳过外置。
- 产出（fx，`infra.go`）：`NewObjectStore() blob.ObjectStore`、`NewBlobExternalizer(db, store) *blobstore.Externalizer`。

- [ ] **Step 1: 写失败的测试**

`fake_store_test.go`：内存 `fakeStore`，带 `puts map[string]int`、`failPut bool`、`presignErr error`。

```go
func TestExternalizeUploadsAndReplaces(t *testing.T)
// status=uploaded、Data==nil；ImageURL==ImageRefOf(sha)；ImageMime/ImageSize 非空；入参未变
func TestExternalizeDedupWithinTask(t *testing.T)     // 同图两条消息：puts==1
func TestExternalizeReusesExisting(t *testing.T)      // 预置 blobs 行：puts==0，Reused==1
func TestExternalizeRepeatedImageKeepsDetail(t *testing.T) // 同图 detail=low/high：都替换且 detail 各自保留
func TestExternalizeSkipsNonData(t *testing.T)        // 外部 URL、已有引用、空串、纯文本：原指针返回
func TestExternalizeInvalidBase64Kept(t *testing.T)   // 保留原字符串，Failed==1
func TestExternalizePutFailureGoesPending(t *testing.T)
// status=pending、Data 非空、attempts==1、next_retry_at≈now+5min；message 仍为引用
func TestExternalizeNoStoreGoesPending(t *testing.T)  // store==nil：全部 pending
func TestExternalizeUnprocessable(t *testing.T)       // 3MB 随机字节：status=unprocessable，Data 为原始字节，无 PUT
func TestExternalizeDBFailureKeepsBase64(t *testing.T)
// 不建 blobs 表：message 保持 data URI，Failed==1；checksum 与引用形态相同
func TestExternalizeTaskTimeoutGoesPending(t *testing.T)
// e.TaskTimeout=10ms，fakeStore.Put 阻塞到 ctx 结束；两张不同的图：都写成 pending，puts<=1（数据库写入不受超时影响）
// store_pool_test.go
func TestStoreTaskPersistsReference(t *testing.T)
// 经 pool.NewPoolManager 提交任务（沿用 session_owner_id 的等待方式）：messages 行不含 "base64"，blobs 有 1 行
```

- [ ] **Step 2: 运行确认失败**

运行：`go test -count=1 ./test/unit/blobstore/`
预期：编译失败，`NewExternalizer` 未定义。

- [ ] **Step 3: 实现 Externalizer**

流程照设计文档 5.2。`Externalizer` 导出字段 `TaskTimeout`（构造时取 `constant.BlobTaskTimeout`，测试可覆盖）。PUT 用 `context.WithTimeout(taskCtx, constant.BlobPutTimeout)`，`taskCtx` 由 `TaskTimeout` 派生；`taskCtx` 到期后不再 PUT。**数据库操作用外层 `ctx`**，不受 `taskCtx` 超时影响，否则超时后图片连 `pending` 都写不进去。

- [ ] **Step 4: 接入 PoolManager**

`runMessageStoreTask` 开头：`messages := task.Messages`，`pm.externalizer != nil` 时替换为外置结果，统计非零时打 Info 日志（只记数量）。后续 `lo.Map` 改用 `messages`。其余逻辑不动，`upgradeReasoningContent` 自然拿到的是引用形态。

- [ ] **Step 5: 运行确认通过**

运行：`go test -count=1 ./test/unit/blobstore/ ./test/unit/pool_manager/ ./test/unit/session_owner_id/ ./test/unit/message_dedup/ && go build ./... && go vet ./test/...`
预期：PASS。

- [ ] **Step 6: 验证 fx 依赖图**

运行：`go test -count=1 ./test/unit/bootstrap/`（已有 `fx.ValidateApp` 冒烟测试）
预期：PASS。

- [ ] **Step 7: 提交**

```bash
git add internal test/unit
git commit -m "feat(blob): 落库前把 base64 图片外置为引用"
```

---

### Task 6: 上传重试 cron

**文件：**
- 新建：`internal/infrastructure/blobstore/retry.go`、`internal/cron/blob_upload_retry.go`
- 修改：`internal/cron/cron.go:152`（注册）、`internal/common/model/cron.go:9-17`、`internal/common/constant/cron.go`、`internal/config/config.go`、`env/api.env.template`、`web/src/app/(dashboard)/audit/cron/page.tsx:54`
- 测试：`test/unit/blobstore/retry_test.go`

**接口：**
- 产出：
```go
type RetryStats struct{ Uploaded, Pending, Failed int64 }
func RetryUploads(ctx context.Context, db *gorm.DB, store blob.ObjectStore, now time.Time) (RetryStats, error)
func NewBlobUploadRetryCron(db *gorm.DB, cache *redis.Client) Cron
```
- 产出：`CronCallAuditMetadata` 增加 `UploadedBlobs/PendingBlobs/FailedBlobs`（json `uploaded_blobs_count`、`pending_blobs_count`、`failed_blobs_count`）；`constant.CronModuleBlobUploadRetry`、`CronSpecBlobUploadRetry`、`CronDescriptionBlobUploadRetry`；`config.CronBlobUploadRetryEnabled`（键 `cron.blob.upload_retry.enabled`，默认 true）。

- [ ] **Step 1: 写失败的测试**

```go
func TestRetrySuccessMarksUploaded(t *testing.T)   // status=uploaded、Data==nil；对象内容与原 Data 相同
func TestRetryFailureBacksOff(t *testing.T)        // attempts=1→2，next_retry_at==now+15min，last_error 非空
func TestRetryBackoffCapped(t *testing.T)          // attempts=8→9，next_retry_at==now+24h
func TestRetryReachesMaxGoesFailed(t *testing.T)   // attempts=9→10：status=failed，Data 保留
func TestRetrySkipsNotDue(t *testing.T)            // next_retry_at 在未来：无 PUT
func TestRetrySkipsUnprocessableAndFailed(t *testing.T)
func TestRetryNilStoreNoop(t *testing.T)           // store==nil：无错误，状态不变
func TestRetryRowWithoutDataSkipped(t *testing.T)  // 脏行 Data==nil：不 panic，记录错误后继续处理下一行
func TestRetryLastErrorTruncated(t *testing.T)     // len(last_error)<=512
```

- [ ] **Step 2: 运行确认失败**

运行：`go test -count=1 -run TestRetry ./test/unit/blobstore/`
预期：编译失败。

- [ ] **Step 3: 实现**

先 `ListDueRetryIDs` 取 ID，再逐行 `GetByID` 读 `data`。退避 `BlobRetryBackoff[min(attempts-1, len-1)]`，其中 `attempts` 是本次失败后的新值。

cron 外壳照抄 `session_terminal_cleanup.go`，用 `objectstore.NewFromConfig()` 取 store，分布式锁由 `wrapCronFunc` 提供，不需要 `FOR UPDATE`。前端 `audit/cron/page.tsx` 的指标标签补上三个新字段。

- [ ] **Step 4: 运行确认通过**

运行：`go test -count=1 ./test/unit/blobstore/ ./test/unit/cron... && go build ./...`
预期：PASS。若有用例断言 cron 注册总数，同步更新。

- [ ] **Step 5: 提交**

```bash
git add internal env web/src test/unit
git commit -m "feat(blob): 新增上传重试定时任务"
```

---

### Task 7: 存量迁移命令

**文件：**
- 新建：`internal/infrastructure/blobstore/migrator.go`
- 修改：`cmd/server/object.go`、`internal/common/constant/blob.go`（LIKE 模式常量 `BlobMigrateLikePattern = "%\"image_url\":\"data:%"`）
- 测试：`test/unit/blobstore/migrator_test.go`

**接口：**
- 消费：`Externalizer.Externalize`、`vo.ComputeMessageChecksum`、`BlobDAO.ResetFailed`。
- 产出：
```go
type MigrateOptions struct { BatchSize int; StartID uint; Limit int; DryRun bool }
type MigrateStats struct {
    Scanned, Migrated, ParseFailed, ChecksumKept int64
    LastID uint
    Blobs ExternalizeStats
}
func NewMigrator(db *gorm.DB, externalizer *Externalizer) *Migrator
func (m *Migrator) Run(ctx context.Context, opts MigrateOptions) (MigrateStats, error)
```
- 产出（CLI，用户明确要求）：`server object migrate [--batch-size] [--start-id] [--limit] [--dry-run] [--retry-failed]`。

- [ ] **Step 1: 写失败的测试**

```go
func TestMigrateReplacesAndRecomputesChecksum(t *testing.T)
// 迁移后经 dbmodel.Message（GORM serializer）读回：ImageURL 为引用；CheckSum==ComputeMessageChecksum(读回, row.ModelID, nil)
func TestMigrateIdempotent(t *testing.T)            // 再跑一次：Scanned==0，blobs 行数不变
func TestMigrateIncludesSoftDeleted(t *testing.T)   // deleted_at≠0 的行同样被处理
func TestMigrateChecksumConflictKeepsOld(t *testing.T)
// 预置同内容引用行占用新 checksum：消息被替换，旧 checksum 保留，ChecksumKept==1
func TestMigrateToolCallsKeepsChecksum(t *testing.T) // 含 ToolCalls：消息被替换，checksum 不变
func TestMigratePutFailureStillReplaces(t *testing.T)// failPut：消息仍为引用，blob 为 pending
func TestMigrateBlobWriteFailureRetriedLater(t *testing.T)
// 第一次没有 blobs 表：消息保留 base64；建表后重跑：完成替换
func TestMigrateParseFailureSkipped(t *testing.T)    // 一行坏 JSON：ParseFailed==1，其他行照常处理
func TestMigrateDryRunWritesNothing(t *testing.T)    // messages 不变，blobs 为空，无 PUT；统计非零
func TestMigrateStartIDAndLimit(t *testing.T)
func TestMigrateTextOnlyUntouched(t *testing.T)      // 不含图片的行 updated_at、check_sum 不变
```

- [ ] **Step 2: 运行确认失败**

运行：`go test -count=1 -run TestMigrate ./test/unit/blobstore/`
预期：编译失败。

- [ ] **Step 3: 实现 Migrator**

流程照设计文档第 9 节：游标查 ID（`id > cursor AND message LIKE ?`，升序，不加 `deleted_at` 条件）→ 逐行 `db.Table("messages").Select("id, model_id, message, check_sum").Scan` 到私有具名结构体（`Message string`）→ `sonic` 解析。

写入用 `db.Table("messages").Where(id).Updates(map)`，**用 `Table` 不用 `Model`，绕开字段 serializer**，`message` 传 JSON 文本。先带 `check_sum` 更新；`errors.Is(err, gorm.ErrDuplicatedKey)` 时去掉 `check_sum` 重试一次，`ChecksumKept++`。

`DryRun` 只解析和统计，不调用 Externalizer，游标照常前进。某批取到的 ID 数不足 `BatchSize`，或已处理行数达到 `Limit` 时结束循环。

- [ ] **Step 4: 接入 cobra**

在 `objectCmd` 下新增 `migrateObjectCmd`。真实迁移要求 `objectstore.NewFromConfig()` 非 nil，否则打 error 并退出非零；`--dry-run` 不要求配置；`--retry-failed` 调 `ResetFailed` 后退出。每批结束的进度日志放在 Migrator 内。

- [ ] **Step 5: 运行确认通过**

运行：`go test -count=1 ./test/unit/blobstore/ && go run ./cmd/server object migrate --help`
预期：PASS；帮助文本列出 5 个参数。

- [ ] **Step 6: 提交**

```bash
git add internal cmd test/unit
git commit -m "feat(blob): 新增 object migrate 存量迁移命令"
```

---

### Task 8: 取图接口

**文件：**
- 新建：`internal/infrastructure/blobstore/reader.go`、`internal/application/session/query/session_image_query.go`
- 修改：`internal/application/session/port/handler.go`、`internal/dto/session.go`、`internal/handler/session.go`、`internal/router/session.go:16-117`、`internal/bootstrap/modules/application.go`、`internal/bootstrap/modules/handler.go:122-152`、`internal/bootstrap/modules/infra.go`、`test/e2e/cross_tenant_session/fixture_test.go`
- 测试：`test/unit/blobstore/reader_test.go`、`test/unit/session_image/session_image_test.go`、`test/e2e/cross_tenant_session/session_image_test.go`

**接口：**
- 消费：`blob.ImageReader`、`sessionport.GetSessionMetaByUserHandler`。
- 产出：
```go
func NewReader(db *gorm.DB, store blob.ObjectStore) blob.ImageReader

// port
type GetSessionImageQuery struct {
    UserID uint; IsAdmin, IsDemo bool; AllowedSessionIDs []uint; SessionID uint; SHA256 string
}
type SessionImageView struct {
    URL, DataURL, Mime string; Size int64; Width, Height int; ExpiresAt *time.Time
}
type GetSessionImageHandler interface {
    Handle(ctx context.Context, q GetSessionImageQuery) (*SessionImageView, error)
}
// dto
type GetSessionImageReq struct {
    SessionID uint   `query:"id" required:"true" minimum:"1" doc:"Session ID"`
    SHA256    string `query:"sha256" required:"true" pattern:"^[0-9a-f]{64}$" doc:"图片哈希"`
}
type GetSessionImageRsp struct { CommonRsp; Image *SessionImage `json:"image,omitempty"` }
```
  `SessionImage` 的 JSON 字段：`url`、`dataUrl`、`expiresAt`、`mime`、`size`、`width`、`height`。
- 路由：`GET /image`（`OperationID: "getSessionImage"`，`LimitUserPermissionWithDemoMiddleware(..., enum.PermissionUser, enum.DemoModuleSessions, ...)`）。
- `SessionHandler` 新增 `HandleGetSessionImage`；`SessionDependencies` 新增 `GetImage`。

- [ ] **Step 1: 写失败的测试**

```go
// reader_test.go
func TestLocateUploadedReturnsURL(t *testing.T)      // URL 非空，ExpiresAt≈now+10min，Data==nil
func TestLocatePendingReturnsData(t *testing.T)
func TestLocateMissingReturnsNil(t *testing.T)       // nil, nil
func TestLocateUploadedNoStoreErrors(t *testing.T)   // store==nil：err≠nil，不 panic
func TestIsReferenced(t *testing.T)                  // 只在给定 messageIDs 内匹配；其他消息引用同图时返回 false
func TestReadBytes(t *testing.T)                     // uploaded 走 store.Get；pending 走 Data；都不存在返回 err
// session_image_test.go（stub meta handler + fake reader）
func TestPassesDemoScopeToMetaQuery(t *testing.T)    // IsDemo、AllowedSessionIDs 原样传给 meta 查询
func TestMetaRejectionPropagates(t *testing.T)       // 返回 ErrNoPermission 时不调用 reader
func TestUnreferencedIsNotFound(t *testing.T)        // ErrDataNotExists
func TestNonImageMimeHasNoDataURL(t *testing.T)      // mime=application/octet-stream：DataURL==""
func TestDataURLBuilt(t *testing.T)                  // "data:image/png;base64,..." 前缀
// session_image_test.go（e2e，生产路由装配）
func TestSessionImageOwnerUploaded(t *testing.T)     // tokenA：url 非空，dataUrl 为空
func TestSessionImageOwnerPending(t *testing.T)      // dataUrl 以 data:image/ 开头
func TestSessionImageOtherUserRejected(t *testing.T) // tokenB：requireRejected（同名 Key 的攻击面）
func TestSessionImageAdminAllowed(t *testing.T)
func TestSessionImageUnreferencedRejected(t *testing.T) // A 自己的 session + 别人的图：bizCodeDataNotExists
func TestSessionImageBadSHA256(t *testing.T)         // 大写或短串：HTTP 422
```

fixture 改动：`AutoMigrate` 加 `&dbmodel.Blob{}`；构造 `blobstore.NewReader(db, fakeStore)`，经 `sessionquery.NewGetSessionImageHandler(getMetaByUser, reader)` 接入 `GetImage`。

- [ ] **Step 2: 运行确认失败**

运行：`go test -count=1 ./test/unit/session_image/ ./test/e2e/cross_tenant_session/`
预期：编译失败。

- [ ] **Step 3: 实现**

`IsReferenced`：`SELECT 1 FROM messages WHERE id IN ? AND message LIKE ? LIMIT 1`，模式为 `%` + 引用 + `%`（sha 已校验为 hex，不含通配符）。

Handler 照搬 `HandleListSessionMessages:450-476`：`IsAdmin: isAdmin || isDemo`，`resolveDemoAccess` 取白名单。

- [ ] **Step 4: 运行确认通过**

运行：`go test -count=1 ./test/unit/blobstore/ ./test/unit/session_image/ ./test/e2e/cross_tenant_session/ && go test -count=1 ./test/unit/bootstrap/`
预期：PASS。

- [ ] **Step 5: 验证测试能捕获越权**

临时注释掉 `IsReferenced` 校验，确认 `TestSessionImageUnreferencedRejected` FAIL；用**手工改回**的方式恢复（不要用 `git checkout`，会丢掉同文件的其他改动）。

- [ ] **Step 6: 提交**

```bash
git add internal test
git commit -m "feat(session): 新增按需获取会话图片的接口"
```

---

### Task 9: dataset 导出内联图片

**文件：**
- 新建：`internal/application/dataset/query/export_images.go`
- 修改：`internal/application/dataset/converter/sharegpt.go`、`internal/application/dataset/query/dataset_query.go:70-239,305`、`internal/bootstrap/modules/application.go:484-486`、`test/e2e/cross_tenant_session/fixture_test.go:232`
- 测试：`test/unit/dataset_image/dataset_image_test.go`、`test/e2e/cross_tenant_session/dataset_image_test.go`

**接口：**
- 消费：`blob.ImageReader.ReadBytes`、`vo.ParseImageRef`。
- 产出：
```go
type ImageResolver func(part *vo.UnifiedContentPart) (string, bool) // 第二个返回值 false：跳过这张图及其占位符
func ConvertSessionWithImages(messages []*session.MessageDetailProjection, tools []*session.ToolDetailProjection, resolve ImageResolver) *ShareGPTConversation
```
  `ConvertSession` 保持原签名，等价于 `resolve == nil`（不产生占位符和 `images`）。`ShareGPTConversation` 增加 `Images []string`（`images,omitempty`）。
- 产出：`NewExportDatasetHandler(readRepo, apiKeyRepo, images blob.ImageReader)`。

- [ ] **Step 1: 写失败的测试**

```go
func TestNilResolverOutputUnchanged(t *testing.T)    // 带图消息：无 images，value 无占位符，JSON 与改动前相同
func TestPlaceholderMatchesImages(t *testing.T)      // text + 2 图：value 含 2 个 <image>，len(Images)==2，顺序一致
func TestResolverFalseSkipsBoth(t *testing.T)        // 一张返回 false：占位符和 images 都少一个
func TestToolRoleImageIncluded(t *testing.T)
func TestTextOnlyHasNoImagesField(t *testing.T)      // JSON 不含 "images"
func TestExportResolverRef(t *testing.T)             // 引用→data:<mime>;base64,<内容>
func TestExportResolverDataURIAndExternalPassThrough(t *testing.T)
func TestExportResolverCachesAndCaps(t *testing.T)   // 同图只 ReadBytes 一次；超 64MB 后清空，缓存总量不超限
func TestExportResolverReadFailure(t *testing.T)     // 返回 false，failed 计数 +1，无 panic
func TestPreviewResolverNoFetch(t *testing.T)        // 不调用 reader；"data:image/png;base64,<已省略 2 KB>"
// e2e（生产路由 + 真实导出）
func TestDatasetExportInlinesImages(t *testing.T)
// uploaded 与 pending 各一张：SSE data 行 images 长度 2，均为 data URI
func TestDatasetExportMissingBlobStillCompletes(t *testing.T) // 引用无对应行：收到 done 事件
```

- [ ] **Step 2: 运行确认失败**

运行：`go test -count=1 ./test/unit/dataset_image/ ./test/e2e/cross_tenant_session/`
预期：编译失败。

- [ ] **Step 3: 实现**

`extractText` 接收 resolver 和图片收集切片：遍历 `Parts`，`image_url` 部分在 resolver 非 nil 且返回 true 时追加占位符行和图片。`buildConversation` 增加 `resolve` 参数；预览传预览 resolver，导出传 `exportImageResolver`（持有 `ctx`、`reader`、按 sha256 的缓存、已用字节数、`failed` 计数）。`writeSSEDone` 前若 `failed>0` 打 warn。缓存超过 `BlobExportCacheMaxBytes` 时整表清空（`// ponytail: 超限整表清空，够用；需要时换 LRU`）。

- [ ] **Step 4: 运行确认通过**

运行：`go test -count=1 ./test/unit/dataset_image/ ./test/e2e/cross_tenant_session/ && go test -count=1 ./test/unit/bootstrap/`
预期：PASS。

- [ ] **Step 5: 提交**

```bash
git add internal test
git commit -m "feat(dataset): 导出时内联图片为 base64"
```

---

### Task 10: 前端点击加载

**文件：**
- 新建：`web/src/components/chat/blob-image.ts`、`web/src/components/chat/blob-image.test.ts`、`web/src/components/chat/session-image-context.tsx`
- 修改：`web/src/components/chat/content-extract.ts`、`web/src/components/chat/multimodal-parts.tsx`、`web/src/components/chat/tool-calls.tsx:89-126`、`web/src/components/session-detail/session-detail-client.tsx:436-444`、`web/src/lib/api-client.ts`、`web/src/lib/types.ts`、`web/src/locales/{zh,en,ja}.json`

**接口：**
- 消费：`GET ${API_PREFIX}/session/image?id=&sha256=`。
- 产出：
```ts
export function parseBlobRef(url: string | undefined): string | null; // 64 位小写 hex，否则 null
export function formatImageMeta(mime?: string, size?: number): string; // "PNG · 1.2 MB"；缺失时返回空串
export function pickImageSrc(rsp: { url?: string; dataUrl?: string }): string | null;
// url 经 isSafeUrl；dataUrl 必须以 data:image/ 开头；否则 null
export function SessionImageProvider(props: { sessionId: number; children: React.ReactNode }): JSX.Element;
export function useSessionImageId(): number | null;
// api-client
async getSessionImage(sessionId: number, sha256: string): Promise<GetSessionImageRsp>;
```
  `ToolResultInfo` 增加 `parts?: ContentPart[]`。
- i18n 键：`chat.image_click_to_view`、`chat.image_unavailable`、`chat.image_load_failed`（三种语言都加）。

- [ ] **Step 1: 写失败的测试**

```ts
describe("parseBlobRef", ...)    // 合法；大写、短串、无前缀、data URI、undefined → null
describe("formatImageMeta", ...) // ("image/png", 1258291) → "PNG · 1.2 MB"；("image/jpeg", 512) → "JPEG · 512 B"；缺失 → ""
describe("pickImageSrc", ...)
// https 通过；javascript:alert(1) → null；data:image/png;base64,x 通过；
// data:text/html;base64,x → null；data:image/svg+xml 按 data:image/ 规则通过但只用于 <img>；{} → null
describe("buildToolResultsByID", ...) // Parts 带图的 tool_result：parts 含图片，text 为文本
```

- [ ] **Step 2: 运行确认失败**

运行：`cd web && npm run test -- blob-image`
预期：FAIL，模块不存在。

- [ ] **Step 3: 实现**

`multimodal-parts.tsx` 增加 `PartBlobImage({ sha, mime, size })`，状态 `idle | loading | ready | failed`：

- `useSessionImageId()` 为 null：渲染不可点击卡片（`chat.image_unavailable`）。
- `idle`：渲染 `PartIconCard` 样式的按钮，显示类型大小和"点击查看"。**此时不发任何请求**。
- 点击后调用 `api.getSessionImage`，`pickImageSrc` 得到地址后渲染 `<img>`；`img` 的 `onError` 或接口错误转 `failed`，可再次点击。
- `image_url` 分支先走 `parseBlobRef`，未命中再走现有 `PartImage`。

`tool-calls.tsx` 在展开区渲染 `result.parts`；有图片时不显示"空输出"提示。`session-detail-client.tsx` 用 `SessionImageProvider` 包住 `SessionTranscript`。分享页不改。

- [ ] **Step 4: 运行门禁**

运行：`make web-lint`
预期：lint、format:check、typecheck、test 全部通过（含 `shadcn-native.test.ts` 与截断规则）。

- [ ] **Step 5: 运行时验证**

用 esbuild 把 `MultimodalParts` 打包成临时 harness，放在 `web/.harness/`（入口须在项目内才能解析 react，不提交），接口用桩返回，通过 `chrome-headless-shell` + CDP 验证：

1. 点击前，CDP `Network.requestWillBeSent` 中没有 `/session/image` 和图片请求。
2. 点击后 `<img>` 出现且 `naturalWidth>0`（url、dataUrl 各一次）。
3. 接口返回失败后可再次点击重试。
4. 没有 Provider 时卡片不可点击。

验证完删除 `web/.harness/`。

- [ ] **Step 6: 提交**

```bash
git add web/src
git commit -m "feat(web): 会话详情页点击后加载图片"
```

---

### Task 11: 文档与整体验证

**文件：**
- 修改：`docs/agents/go-backend.md:99`（Session 路由表加 `GET /image`）、`docs/agents/architecture.md`（写入路径和 cron）、`CONTEXT.md`（术语：图片外置、Blob、引用形态）

- [ ] **Step 1: 更新文档**

只写上述事实，不新建 README 类文件。

- [ ] **Step 2: 全量验证**

```bash
make test
make lint
make web-lint
```

预期：全部通过。`cmd | tail` 会吞退出码，不要用管道判断结果。golangci-lint 报陈旧路径时先 `$(go env GOPATH)/bin/golangci-lint cache clean`。

- [ ] **Step 3: 用真实 PostgreSQL 验证 SQL**

单测跑在 sqlite 上，有三处 SQL 需要在 PostgreSQL 上确认：`blobs` 的 AutoMigrate（`[]byte`→`bytea`）、`ON CONFLICT`、`LIKE` 预过滤。本地用 `docker run postgres` 起临时实例，跑 `database migrate`，构造一条 base64 消息，执行 `object migrate --dry-run` 和正式迁移（MinIO 容器当对象存储），确认消息被替换、对象存在、重跑 `Scanned=0`。

- [ ] **Step 4: 过度工程审查**

使用 `ponytail-review` 审查全部 diff，按结果删减。

- [ ] **Step 5: 提交并询问合并方式**

```bash
git add docs CONTEXT.md
git commit -m "docs(blob): 补充图片对象存储相关说明"
```

询问用户提 PR 还是直接合并。**不要自行推送、部署或操作生产。** 上线步骤见设计文档第 11 节，其中配置生产环境、执行迁移和 `VACUUM FULL` 都需要用户届时单独授权。
