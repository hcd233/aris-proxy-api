# Session 人工评分功能设计

**日期**: 2026-06-03
**作者**: centonhuang

## 背景

当前 `sessions` 表已有 LLM 自动评分的三维度字段（`coherence_score`/`depth_score`/`value_score`/`total_score`），通过 cron 任务 + agent 自动评分。现需改为**人工评分**模式：

1. 删除无用的三维度评分列，新增单一 `score` 列（1-5 整数）
2. 提供后端 API 接口支持人工评分
3. 前端展示评分并支持评分操作

## 影响范围

### 数据库

```sql
ALTER TABLE sessions DROP COLUMN coherence_score;
ALTER TABLE sessions DROP COLUMN depth_score;
ALTER TABLE sessions DROP COLUMN value_score;
ALTER TABLE sessions DROP COLUMN total_score;
ALTER TABLE sessions ADD COLUMN score SMALLINT DEFAULT NULL;
```

### 后端改动

| 层 | 文件 | 改动 |
|---|---|---|
| **Model** | `internal/infrastructure/database/model/session.go` | 删除 `CoherenceScore`/`DepthScore`/`ValueScore`/`TotalScore`；新增 `Score *int` |
| **VO** | `internal/domain/session/vo/summary_score.go` | `SessionScore` 简化为 `score *int` + `at *time.Time` + `errMsg`；重构构造/恢复函数 |
| **Aggregate** | `internal/domain/session/aggregate/session.go` | 调整 `UpdateScore` 签名 |
| **Repository 接口** | `internal/domain/session/repository.go` | 调整 `UpdateScore` 签名 |
| **Repository 实现** | `internal/infrastructure/repository/session_repository.go` | `applyScore`/`toSessionAggregate`/`UpdateScore` 适配新 VO |
| **常量** | `internal/common/constant/sql.go` | 删除 `FieldCoherenceScore`/`FieldDepthScore`/`FieldValueScore`/`FieldTotalScore`；新增 `FieldScore` |
| **常量** | `internal/common/constant/session.go` | 删除 `ScoreVersion`/`ScoreMaxRetries`/`CronModuleSessionScore`/`CronSpecSessionScore` |
| **Agent** | `internal/infrastructure/agent/scorer.go` | 删除整个文件（LLM 自动评分不再需要） |
| **Cron** | `internal/cron/session_score.go` | 删除整个文件 |
| **Pool** | `internal/infrastructure/pool/agent_pool.go` | 删除 `SubmitScoreTask` 方法 |
| **DTO** | `internal/dto/session.go` | `SessionSummary`/`SessionMetadata`/`SessionDetail` 新增 `Score *int`/`ScoredAt *time.Time` |
| **DTO** | `internal/dto/session_share.go` | `ShareSessionMetadata` 新增 `Score *int` |
| **新 DTO** | `internal/dto/session.go` | 新增 `ScoreSessionReq`/`ScoreSessionRsp` |
| **Handler** | `internal/handler/session.go` | 新增 `HandleScoreSession`；SessionHandler 接口扩展 |
| **Router** | `internal/router/session.go` | 注册 `POST /score` 路由 |
| **Bootstrap** | `internal/bootstrap/container.go` | 删除 `SessionScoreCron` 注册 |
| **DTO 任务** | `internal/dto/task.go` | 删除 `ScoreTask` |
| **Web 嵌入** | `internal/router/web.go` | 分享页 DTO 映射需包含 Score |

### 前端改动

| 文件 | 改动 |
|---|---|
| `web/src/lib/types.ts` | `SessionSummary`/`SessionMetadata`/`ShareSessionMetadata` 新增 `score?: number`/`scoredAt?: string`；新增 `ScoreSessionReq`/`ScoreSessionRsp` |
| `web/src/lib/api-client.ts` | 新增 `api.scoreSession(body)` 方法 |
| `web/src/app/(dashboard)/sessions/page.tsx` | 列表新增评分列/标签 |
| `web/src/components/session-detail/session-detail-client.tsx` | Header 区域：已评分展示分数，未评分渲染 1-5 星评分栏 |
| `web/src/app/share/page.tsx` | 分享页展示评分（只读） |

## API 设计

### POST /api/v1/session/score

- **鉴权**: JWT
- **请求体**:
  ```json
  { "sessionId": 123, "score": 4 }
  ```
- **响应**:
  ```json
  { "sessionId": 123, "score": 4, "scoredAt": "2026-06-03T10:00:00Z" }
  ```
- **校验**: score 必须在 1-5 范围内；session 必须存在且属于当前用户
- **幂等**: 重复评分覆盖之前的值

## 前端交互设计

### Session 列表页
- 新增"评分"列（桌面端表格）/ "评分"行（移动端卡片）
- 已评分：显示 `★ 4` 标签（金色）
- 未评分：显示 `-`

### Session 详情页
- Header 区域（返回按钮右侧 / meta 行）
- 已评分：显示分数标签 `★ 4`
- 未评分：显示 5 颗星交互评分栏
  - 悬停/点击高亮星星
  - 点击后调接口，成功 toast "评分成功"，星星变为已选状态
  - 失败 toast 错误信息

## 自审

- [x] 无 TBD/TODO
- [x] 无内部矛盾
- [x] 范围可控，无过度抽象
- [x] 明确改动的每个文件和字段
