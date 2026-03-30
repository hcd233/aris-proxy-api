// Package agent Agent相关能力封装
//
//	author centonhuang
//	update 2026-03-26 10:00:00
package agent

import (
	"context"
	"strings"
	"time"

	openai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// Summarizer 对话总结器
//
//	@author centonhuang
//	@update 2026-03-26 10:00:00
type Summarizer struct {
	agent adk.Agent
}

// NewSummarizer 创建Summarizer
//
//	@return *Summarizer
//	@return error
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func NewSummarizer() (*Summarizer, error) {
	ctx := context.Background()

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:   config.OpenAIModel,
		APIKey:  config.OpenAIAPIKey,
		BaseURL: config.OpenAIBaseURL,
	})
	if err != nil {
		return nil, err
	}

	agentConfig := &adk.ChatModelAgentConfig{
		Name:        "SessionSummarizer",
		Description: "将对话会话总结为5-10个中文字符的摘要",
		Instruction: "你是一个对话总结助手。请严格遵循以下规则：\n1. 将对话内容总结为5-10个中文字的简短摘要\n2. 必须使用中文输出，禁止使用英文或其他语言\n3. 捕捉对话的核心主题\n4. 只输出总结文字，不要添加任何解释、标点或前缀",
		Model:       chatModel,
	}

	agent, err := adk.NewChatModelAgent(ctx, agentConfig)
	if err != nil {
		return nil, err
	}

	return &Summarizer{agent: agent}, nil
}

// Summarize 总结对话内容
//
//	@receiver s *Summarizer
//	@param ctx context.Context
//	@param content string 对话内容
//	@return summary string 5-10字总结
//	@return err error
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func (s *Summarizer) Summarize(ctx context.Context, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "空会话", nil
	}

	messages := []*schema.Message{
		schema.UserMessage(content),
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: s.agent})
	iterator := runner.Run(ctx, messages)

	var summary strings.Builder
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			return "", event.Err
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			msg, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				return "", err
			}
			if msg != nil {
				summary.WriteString(msg.Content)
			}
		}
	}

	result := strings.TrimSpace(summary.String())

	return result, nil
}

// SummarizeWithRetry 带重试的总结
//
//	@receiver s *Summarizer
//	@param ctx context.Context
//	@param content string
//	@param maxRetries int 最大重试次数
//	@return summary string
//	@return err error
//	@author centonhuang
//	@update 2026-03-26 10:00:00
func (s *Summarizer) SummarizeWithRetry(ctx context.Context, content string, maxRetries int) (string, error) {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			logger.Logger().Info("[Summarizer] Retrying summary generation",
				zap.Int("attempt", i+1),
				zap.Int("maxRetries", maxRetries+1))
			time.Sleep(time.Second * time.Duration(i))
		}

		summary, err := s.Summarize(ctx, content)
		if err == nil {
			return summary, nil
		}
		lastErr = err
		logger.Logger().Error("[Summarizer] Summary generation failed",
			zap.Int("attempt", i+1),
			zap.Error(err))
	}

	return "", lastErr
}
