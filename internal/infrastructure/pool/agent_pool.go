// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-06-03 10:00:00
package pool

import (
	"context"

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
