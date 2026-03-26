# Session 总结定时任务实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现每天自动总结 session 记录的定时任务，为每个未总结的 session 生成 5-10 字的简短摘要

**Architecture:** 新增 SummarizerAgent 使用 eino ChatModelAgent 调用 OpenAI 模型生成总结；SessionSummarizeCron 定时任务使用 pond 协程池并发处理，凌晨 3:30 执行避开整点清洗任务

**Tech Stack:** robfig/cron/v3, cloudwego/eino, cloudwego/eino-ext/openai, alitto/pond/v2, GORM

---

## 文件结构规划

| 文件 | 说明 |
|------|------|
| `internal/infrastructure/database/model/session.go` | Session 模型新增 Summary 字段 |
| `internal/cron/summarizer.go` | eino SummarizerAgent 封装 |
| `internal/cron/session_summarize.go` | SessionSummarizeCron 定时任务 |
| `internal/cron/cron.go` | InitCronJobs 中启动新定时任务 |

---

## Task 1: 添加 eino 依赖

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: 添加 eino 依赖**

```bash
go get github.com/cloudwego/eino
go get github.com/cloudwego/eino-ext/components/model/openai
```

- [ ] **Step 2: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add eino and eino-ext dependencies"
```

---

## Task 2: Session 模型添加 Summary 字段

**Files:**
- Modify: `internal/infrastructure/database/model/session.go`

- [ ] **Step 1: 添加 Summary 字段**

```go
// Session 用户数据库模型
//
//	author centonhuang
//	update 2026-03-26 10:00:00
type Session struct {
	BaseModel
	ID         uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:用户ID"`
	APIKeyName string `json:"api_key_name" gorm:"column:api_key_name;not null;default:'';comment:API密钥名称"`
	MessageIDs []uint `json:"message_ids" gorm:"column:message_ids;not null;comment:消息ID列表;serializer:json"`
	ToolIDs    []uint `json:"tool_ids" gorm:"column:tool_ids;not null;comment:工具ID列表;serializer:json"`
	Summary    string `json:"summary" gorm:"column:summary;not null;default:'';comment:会话总结(5-10字)"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/infrastructure/database/model/session.go
git commit -m "feat(model): add summary field to session model"
```

---

## Task 3: 创建 SummarizerAgent

**Files:**
- Create: `internal/cron/summarizer.go`

- [ ] **Step 1: 创建 SummarizerAgent 文件**

```go
// Package cron Session总结Agent
//
//	author centonhuang
//	update 2026-03-26 10:00:00
package cron

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	openai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// SummarizerAgent Session总结Agent
//
//	@author centonhuang
//	@update 2026-03-26 10:00:00
type SummarizerAgent struct {
	agent adk.Agent
}

// NewSummarizerAgent 创建SummarizerAgent
//
//	@return *SummarizerAgent
//	@return error
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func NewSummarizerAgent() (*SummarizerAgent, error) {
	ctx := context.Background()

	// 创建OpenAI ChatModel
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:   config.OpenAIModel,
		APIKey:  config.OpenAIAPIKey,
		BaseURL: config.OpenAIBaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	// 配置ChatModelAgent
	agentConfig := &adk.ChatModelAgentConfig{
		Name:        "SessionSummarizer",
		Description: "An agent that summarizes conversation sessions into 5-10 characters.",
		Instruction: "你是一个对话总结助手。请将以下对话内容总结为5-10个字的简短摘要，捕捉对话的核心主题。只输出总结文字，不要添加任何解释或标点。",
		Model:       chatModel,
	}

	// 创建Agent
	agent, err := adk.NewChatModelAgent(ctx, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model agent: %w", err)
	}

	return &SummarizerAgent{agent: agent}, nil
}

// Summarize 总结对话内容
//
//	@receiver s *SummarizerAgent
//	@param ctx context.Context
//	@param content string 对话内容
//	@return summary string 5-10字总结
//	@return err error
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func (s *SummarizerAgent) Summarize(ctx context.Context, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "空会话", nil
	}

	// 准备输入消息
	messages := []schema.Message{
		schema.UserMessage(content),
	}

	input := &adk.AgentInput{
		Messages: messages,
	}

	// 创建Runner并执行
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: s.agent})
	iterator := runner.Query(ctx, input)

	var summary strings.Builder
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			return "", fmt.Errorf("agent execution error: %w", event.Err)
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			summary.WriteString(event.Output.MessageOutput.Message.Content)
		}
	}

	result := strings.TrimSpace(summary.String())
	if result == "" {
		return "无法总结", nil
	}

	return result, nil
}

// SummarizeWithRetry 带重试的总结
//
//	@receiver s *SummarizerAgent
//	@param ctx context.Context
//	@param content string
//	@param maxRetries int 最大重试次数
//	@return summary string
//	@return err error
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func (s *SummarizerAgent) SummarizeWithRetry(ctx context.Context, content string, maxRetries int) (string, error) {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			logger.Logger().Info("[SummarizerAgent] Retrying summary generation",
				zap.Int("attempt", i+1),
				zap.Int("maxRetries", maxRetries+1))
			time.Sleep(time.Second * time.Duration(i)) // 指数退避
		}

		summary, err := s.Summarize(ctx, content)
		if err == nil {
			return summary, nil
		}
		lastErr = err
		logger.Logger().Error("[SummarizerAgent] Summary generation failed",
			zap.Int("attempt", i+1),
			zap.Error(err))
	}

	return "", fmt.Errorf("failed after %d attempts: %w", maxRetries+1, lastErr)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/cron/summarizer.go
git commit -m "feat(cron): add SummarizerAgent using eino ChatModelAgent"
```

---

## Task 4: 创建 SessionSummarizeCron 定时任务

**Files:**
- Create: `internal/cron/session_summarize.go`

- [ ] **Step 1: 创建 SessionSummarizeCron 文件**

```go
// Package cron Session总结定时任务
//
//	author centonhuang
//	update 2026-03-26 10:00:00
package cron

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// SessionSummarizeCron Session总结定时任务
//
//	@author centonhuang
//	@update 2026-03-26 10:00:00
type SessionSummarizeCron struct {
	cron        *cron.Cron
	sessionDAO  *dao.SessionDAO
	messageDAO  *dao.MessageDAO
}

// NewSessionSummarizeCron 创建Session总结定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func NewSessionSummarizeCron() Cron {
	return &SessionSummarizeCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter("SessionSummarizeCron", logger.Logger())),
		),
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
	}
}

// Start 启动Session总结定时任务
//
//	@receiver c *SessionSummarizeCron
//	@return error
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func (c *SessionSummarizeCron) Start() error {
	// 每天凌晨3:30执行（与整点的deduplicate任务错峰）
	entryID, err := c.cron.AddFunc("30 3 * * *", c.summarize)
	if err != nil {
		logger.Logger().Error("[SessionSummarizeCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SessionSummarizeCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}

// summarize 执行Session总结逻辑
//
//	@receiver c *SessionSummarizeCron
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func (c *SessionSummarizeCron) summarize() {
	ctx := context.WithValue(context.Background(), constant.CtxKeyTraceID, uuid.New().String())
	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	// 1. 查询未总结的sessions
	sessions, err := c.getUnsummarizedSessions(db)
	if err != nil {
		log.Error("[SessionSummarizeCron] Failed to get unsummarized sessions", zap.Error(err))
		return
	}

	if len(sessions) == 0 {
		log.Info("[SessionSummarizeCron] No sessions to summarize")
		return
	}

	log.Info("[SessionSummarizeCron] Starting summarization", zap.Int("count", len(sessions)))

	// 2. 使用协程池并发处理
	pool := pond.New(config.PoolWorkers, config.PoolQueueSize)
	defer pool.StopAndWait()

	// 3. 提交任务到协程池
	for _, session := range sessions {
		s := session // 避免闭包陷阱
		pool.Submit(func() {
			c.processSession(ctx, db, s)
		})
	}

	log.Info("[SessionSummarizeCron] All summarization tasks submitted")
}

// getUnsummarizedSessions 获取未总结的sessions
//
//	@receiver c *SessionSummarizeCron
//	@param db *gorm.DB
//	@return []*dbmodel.Session
//	@return error
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func (c *SessionSummarizeCron) getUnsummarizedSessions(db *gorm.DB) ([]*dbmodel.Session, error) {
	var sessions []*dbmodel.Session
	err := db.Where("summary = ?", "").Where("deleted_at = 0").Find(&sessions).Error
	return sessions, err
}

// processSession 处理单个Session的总结（每个goroutine创建独立的SummarizerAgent）
//
//	@receiver c *SessionSummarizeCron
//	@param ctx context.Context
//	@param db *gorm.DB
//	@param session *dbmodel.Session
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func (c *SessionSummarizeCron) processSession(ctx context.Context, db *gorm.DB, session *dbmodel.Session) {
	log := logger.WithCtx(ctx)

	// 每个goroutine创建独立的SummarizerAgent（避免线程安全问题）
	summarizer, err := NewSummarizerAgent()
	if err != nil {
		log.Error("[SessionSummarizeCron] Failed to create summarizer agent",
			zap.Uint("sessionID", session.ID),
			zap.Error(err))
		return
	}

	// 获取session的消息内容
	content, err := c.getSessionContent(ctx, session)
	if err != nil {
		log.Error("[SessionSummarizeCron] Failed to get session content",
			zap.Uint("sessionID", session.ID),
			zap.Error(err))
		return
	}

	// 调用Agent生成总结（带3次重试）
	summary, err := summarizer.SummarizeWithRetry(ctx, content, 3)
	if err != nil {
		log.Error("[SessionSummarizeCron] Failed to generate summary",
			zap.Uint("sessionID", session.ID),
			zap.Error(err))
		return
	}

	// 更新数据库
	err = c.sessionDAO.Update(db, &dbmodel.Session{ID: session.ID}, map[string]interface{}{
		"summary": summary,
	})
	if err != nil {
		log.Error("[SessionSummarizeCron] Failed to update session summary",
			zap.Uint("sessionID", session.ID),
			zap.Error(err))
		return
	}

	log.Info("[SessionSummarizeCron] Session summarized successfully",
		zap.Uint("sessionID", session.ID),
		zap.String("summary", summary))
}

// getSessionContent 获取Session的消息内容
//
//	@receiver c *SessionSummarizeCron
//	@param ctx context.Context
//	@param session *dbmodel.Session
//	@return string 消息内容
//	@return error
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func (c *SessionSummarizeCron) getSessionContent(ctx context.Context, session *dbmodel.Session) (string, error) {
	if len(session.MessageIDs) == 0 {
		return "", nil
	}

	// 批量获取消息
	messages, err := c.messageDAO.BatchGetByField(database.GetDBInstance(ctx), "id", session.MessageIDs, []string{"id", "message"})
	if err != nil {
		return "", fmt.Errorf("failed to get messages: %w", err)
	}

	// 按MessageIDs顺序排序
	messageMap := lo.SliceToMap(messages, func(m *dbmodel.Message) (uint, *dbmodel.Message) {
		return m.ID, m
	})

	var contentParts []string
	for _, msgID := range session.MessageIDs {
		if msg, ok := messageMap[msgID]; ok && msg.Message != nil {
			// 提取消息文本内容
			text := extractMessageText(msg.Message)
			if text != "" {
				contentParts = append(contentParts, text)
			}
		}
	}

	return strings.Join(contentParts, "\n"), nil
}

// extractMessageText 从UnifiedMessage中提取文本内容
//
//	@param msg *dto.UnifiedMessage
//	@return string
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func extractMessageText(msg *dto.UnifiedMessage) string {
	if msg == nil || msg.Content == nil {
		return ""
	}

	// 优先使用Text字段
	if msg.Content.Text != "" {
		return msg.Content.Text
	}

	// 如果Parts不为空，拼接所有文本部分
	var texts []string
	for _, part := range msg.Content.Parts {
		if part.Type == "text" && part.Text != "" {
			texts = append(texts, part.Text)
		}
	}

	return strings.Join(texts, " ")
}
```

- [ ] **Step 2: 添加缺失的 import**

在文件顶部添加：
```go
import (
	"github.com/hcd233/aris-proxy-api/internal/dto"
)
```

- [ ] **Step 3: Commit**

```bash
git add internal/cron/session_summarize.go
git commit -m "feat(cron): add SessionSummarizeCron with pond worker pool"
```

---

## Task 5: 在 InitCronJobs 中启动新定时任务

**Files:**
- Modify: `internal/cron/cron.go`

- [ ] **Step 1: 修改 InitCronJobs 函数**

```go
// InitCronJobs 初始化定时任务
//
//	author centonhuang
//	update 2026-03-26 10:00:00
func InitCronJobs() {
	sessionDeduplicateCron := NewSessionDeduplicateCron()
	lo.Must0(sessionDeduplicateCron.Start())

	sessionSummarizeCron := NewSessionSummarizeCron()
	lo.Must0(sessionSummarizeCron.Start())

	logger.Logger().Info("[Cron] Init cron jobs")
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/cron/cron.go
git commit -m "feat(cron): register SessionSummarizeCron in InitCronJobs"
```

---

## Task 6: 验证编译

**Files:**
- All files

- [ ] **Step 1: 编译验证**

```bash
go build -o aris-proxy-api main.go
```

Expected: 编译成功，无错误

- [ ] **Step 2: Commit（如需要）**

如果有任何修复，提交：
```bash
git add .
git commit -m "fix: resolve compilation issues"
```

---

## 验证清单

- [ ] Session 模型包含 Summary 字段
- [ ] SummarizerAgent 使用 eino ChatModelAgent
- [ ] SessionSummarizeCron 每天凌晨 3:30 执行
- [ ] 使用 pond 协程池并发处理
- [ ] 单个失败不影响其他任务
- [ ] 模型调用失败重试 3 次
- [ ] InitCronJobs 启动新定时任务
- [ ] 项目可编译通过
