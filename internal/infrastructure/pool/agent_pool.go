// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-04-05 10:00:00
package pool

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/agent"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// SubmitSummarizeTask 提交 Session 总结任务到协程池
//
//	@receiver pm *PoolManager
//	@param task *dto.SummarizeTask
//	@return error
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func (pm *PoolManager) SubmitSummarizeTask(task *dto.SummarizeTask) error {
	log := logger.WithCtx(task.Ctx)
	db := database.GetDBInstance(task.Ctx)

	summarizer := agent.GetSummarizer()

	return pm.agentPool.Go(func() {
		summary, err := summarizer.SummarizeWithRetry(task.Ctx, task.Content, constant.SummarizeMaxRetries)
		if err != nil {
			log.Error("[AgentPool] Failed to generate summary", zap.Uint("sessionID", task.SessionID), zap.Error(err))
			// 记录失败原因
			sessionDAO := dao.GetSessionDAO()
			_ = sessionDAO.Update(db, &dbmodel.Session{ID: task.SessionID}, map[string]any{
				"summarize_error": err.Error(),
			})
			return
		}

		if summary == "" {
			log.Error("[AgentPool] Summary is empty", zap.Uint("sessionID", task.SessionID))
			return
		}

		sessionDAO := dao.GetSessionDAO()
		err = sessionDAO.Update(db, &dbmodel.Session{ID: task.SessionID}, map[string]any{
			"summary": summary,
		})
		if err != nil {
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
//	@update 2026-04-09 10:00:00
func (pm *PoolManager) SubmitScoreTask(task *dto.ScoreTask) error {
	log := logger.WithCtx(task.Ctx)
	db := database.GetDBInstance(task.Ctx)

	scorer := agent.GetScorer()

	return pm.agentPool.Go(func() {
		result, err := scorer.ScoreWithRetry(task.Ctx, task.Content, constant.ScoreMaxRetries)
		if err != nil {
			log.Error("[AgentPool] Failed to generate score", zap.Uint("sessionID", task.SessionID), zap.Error(err))
			// 记录失败原因
			sessionDAO := dao.GetSessionDAO()
			_ = sessionDAO.Update(db, &dbmodel.Session{ID: task.SessionID}, map[string]any{
				"score_error": err.Error(),
			})
			return
		}

		if result == nil {
			log.Info("[AgentPool] Skipping score for empty content", zap.Uint("sessionID", task.SessionID))
			return
		}

		sessionDAO := dao.GetSessionDAO()
		err = sessionDAO.Update(db, &dbmodel.Session{ID: task.SessionID}, map[string]any{
			"coherence_score": result.Coherence,
			"depth_score":     result.Depth,
			"value_score":     result.Value,
			"total_score":     result.Total(),
			"score_version":   constant.ScoreVersion,
			"scored_at":       lo.ToPtr(time.Now()),
		})
		if err != nil {
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
