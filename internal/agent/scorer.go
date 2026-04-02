// Package agent Agent相关能力封装
//
//	author centonhuang
//	update 2026-04-02 10:00:00
package agent

import (
	"context"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	openai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// ScoreResult 评分结果
//
//	@author centonhuang
//	@update 2026-04-02 10:00:00
type ScoreResult struct {
	Coherence int `json:"coherence"`
	Depth     int `json:"depth"`
	Value     int `json:"value"`
}

// Total 计算总分
//
//	@return float64
//	@author centonhuang
//	@update 2026-04-02 10:00:00
func (s *ScoreResult) Total() float64 {
	return float64(s.Coherence+s.Depth+s.Value) / 3.0
}

// Scorer Session评分器
//
//	@author centonhuang
//	@update 2026-04-02 10:00:00
type Scorer struct {
	agent adk.Agent
}

// NewScorer 创建Scorer
//
//	@return *Scorer
//	@return error
//	@author centonhuang
//	@update 2026-04-02 10:00:00
func NewScorer() (*Scorer, error) {
	ctx := context.Background()

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:     config.OpenAIModel,
		APIKey:    config.OpenAIAPIKey,
		BaseURL:   config.OpenAIBaseURL,
		MaxTokens: lo.ToPtr(constant.ScoreMaxTokens),
	})
	if err != nil {
		return nil, err
	}

	agentConfig := &adk.ChatModelAgentConfig{
		Name:        constant.SessionScorerAgentName,
		Description: constant.SessionScorerAgentDescription,
		Instruction: constant.SessionScorerAgentInstruction,
		Model:       chatModel,
	}

	agent, err := adk.NewChatModelAgent(ctx, agentConfig)
	if err != nil {
		return nil, err
	}

	return &Scorer{agent: agent}, nil
}

// Score 对对话内容进行评分
//
//	@receiver s *Scorer
//	@param ctx context.Context
//	@param content string 对话内容
//	@return *ScoreResult 评分结果
//	@return error
//	@author centonhuang
//	@update 2026-04-02 10:00:00
func (s *Scorer) Score(ctx context.Context, content string) (*ScoreResult, error) {
	if strings.TrimSpace(content) == "" {
		return &ScoreResult{Coherence: 0, Depth: 0, Value: 0}, nil
	}

	messages := []*schema.Message{
		schema.UserMessage(content),
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: s.agent})
	iterator := runner.Run(ctx, messages)

	var resultBuilder strings.Builder
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			return nil, event.Err
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			msg, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				return nil, err
			}
			if msg != nil {
				resultBuilder.WriteString(msg.Content)
			}
		}
	}

	result := strings.TrimSpace(resultBuilder.String())

	// 解析JSON结果
	var scoreResult ScoreResult
	if err := sonic.Unmarshal([]byte(result), &scoreResult); err != nil {
		logger.Logger().Error("[Scorer] Failed to parse score result",
			zap.String("result", result),
			zap.Error(err))
		return nil, err
	}

	// 验证评分范围
	scoreResult.Coherence = lo.Clamp(scoreResult.Coherence, 1, 10)
	scoreResult.Depth = lo.Clamp(scoreResult.Depth, 1, 10)
	scoreResult.Value = lo.Clamp(scoreResult.Value, 1, 10)

	return &scoreResult, nil
}

// ScoreWithRetry 带重试的评分
//
//	@receiver s *Scorer
//	@param ctx context.Context
//	@param content string
//	@param maxRetries int 最大重试次数
//	@return *ScoreResult
//	@return error
//	@author centonhuang
//	@update 2026-04-02 10:00:00
func (s *Scorer) ScoreWithRetry(ctx context.Context, content string, maxRetries int) (*ScoreResult, error) {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			logger.Logger().Info("[Scorer] Retrying score generation",
				zap.Int("attempt", i+1),
				zap.Int("maxRetries", maxRetries+1))
			time.Sleep(time.Second * time.Duration(i))
		}

		result, err := s.Score(ctx, content)
		if err == nil {
			return result, nil
		}
		lastErr = err
		logger.Logger().Error("[Scorer] Score generation failed",
			zap.Int("attempt", i+1),
			zap.Error(err))
	}

	return nil, lastErr
}
