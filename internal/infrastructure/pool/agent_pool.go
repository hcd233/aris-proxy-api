// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-04-26 14:00:00
package pool

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/agent"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// getSessionRepo 返回绑定调用方 context 的 Session 仓储。
//
//	@param ctx context.Context
//	@return session.SessionRepository
//	@author centonhuang
//	@update 2026-05-12 19:33:00
func (pm *PoolManager) getSessionRepo(ctx context.Context) session.SessionRepository {
	return repository.NewSessionRepository(pm.db.WithContext(ctx))
}

// SubmitSummarizeTask 提交 Session 总结任务到协程池
//
//	@receiver pm *PoolManager
//	@param task *dto.SummarizeTask
//	@return error
//	@author centonhuang
//	@update 2026-04-26 14:00:00
func (pm *PoolManager) SubmitSummarizeTask(task *dto.SummarizeTask) error {
	log := logger.WithCtx(task.Ctx)

	summarizer := agent.GetSummarizer()

	return pm.agentPool.Go(func() {
		summary, err := summarizer.SummarizeWithRetry(task.Ctx, task.Content, constant.SummarizeMaxRetries)
		if err != nil {
			log.Error("[AgentPool] Failed to generate summary", zap.Uint("sessionID", task.SessionID), zap.Error(err))
			failedSummary := vo.NewSessionSummary("", err.Error())
			if updateErr := pm.getSessionRepo(task.Ctx).UpdateSummary(task.Ctx, task.SessionID, failedSummary); updateErr != nil {
				log.Error("[AgentPool] Failed to update summarize_error", zap.Uint("sessionID", task.SessionID), zap.Error(updateErr))
			}
			return
		}

		if summary == "" {
			log.Error("[AgentPool] Summary is empty", zap.Uint("sessionID", task.SessionID))
			return
		}

		successSummary := vo.NewSessionSummary(summary, "")
		if err := pm.getSessionRepo(task.Ctx).UpdateSummary(task.Ctx, task.SessionID, successSummary); err != nil {
			log.Error("[AgentPool] Failed to update session summary", zap.Uint("sessionID", task.SessionID), zap.Error(err))
			return
		}

		log.Info("[AgentPool] Session summarized successfully", zap.Uint("sessionID", task.SessionID), zap.String("summary", summary))
	})
}

// SubmitScoreTask 提交 Session 评分任务到协程池
//
//	@receiver pm *PoolManager
//	@param task *dto.ScoreTask
//	@return error
//	@author centonhuang
//	@update 2026-04-26 14:00:00
func (pm *PoolManager) SubmitScoreTask(task *dto.ScoreTask) error {
	log := logger.WithCtx(task.Ctx)

	scorer := agent.GetScorer()

	return pm.agentPool.Go(func() {
		result, err := scorer.ScoreWithRetry(task.Ctx, task.Content, constant.ScoreMaxRetries)
		if err != nil {
			log.Error("[AgentPool] Failed to generate score", zap.Uint("sessionID", task.SessionID), zap.Error(err))
			failedScore := vo.NewFailedSessionScore(err.Error(), time.Now())
			if updateErr := pm.getSessionRepo(task.Ctx).UpdateScore(task.Ctx, task.SessionID, failedScore); updateErr != nil {
				log.Error("[AgentPool] Failed to update score_error", zap.Uint("sessionID", task.SessionID), zap.Error(updateErr))
			}
			return
		}

		if result == nil {
			log.Info("[AgentPool] Skipping score for empty content", zap.Uint("sessionID", task.SessionID))
			return
		}

		score, scoreErr := vo.NewSessionScore(float64(result.Coherence), float64(result.Depth), float64(result.Value), constant.ScoreVersion, time.Now())
		if scoreErr != nil {
			log.Error("[AgentPool] Invalid score values", zap.Uint("sessionID", task.SessionID), zap.Error(scoreErr))
			return
		}

		if err := pm.getSessionRepo(task.Ctx).UpdateScore(task.Ctx, task.SessionID, score); err != nil {
			log.Error("[AgentPool] Failed to update session score", zap.Uint("sessionID", task.SessionID), zap.Error(err))
			return
		}

		log.Info("[AgentPool] Session scored successfully",
			zap.Uint("sessionID", task.SessionID),
			zap.Float64("coherence", float64(result.Coherence)),
			zap.Float64("depth", float64(result.Depth)),
			zap.Float64("value", float64(result.Value)),
			zap.Float64("total", result.Total()))
	})
}
