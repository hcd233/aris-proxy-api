// Package cron 提取消息中 <think> 标签内容到 reasoning_content 的定时任务
//
//	@author centonhuang
//	@update 2026-06-02 10:00:00
package cron

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var thinkTagOpen = "<think>"

var thinkRegexp = regexp.MustCompile(`(?s)<think>(.*?)</think>`)

// ThinkExtractCron 消息推理内容提取定时任务
//
//	@author centonhuang
//	@update 2026-06-02 10:00:00
type ThinkExtractCron struct {
	cron   *cron.Cron
	db     *gorm.DB
	locker lock.Locker
}

// NewThinkExtractCron 创建消息推理内容提取定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-06-02 10:00:00
func NewThinkExtractCron(db *gorm.DB, cache *redis.Client) Cron {
	return &ThinkExtractCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleThinkExtract)),
		),
		db:     db,
		locker: lock.NewLocker(cache),
	}
}

// Stop 停止消息推理内容提取定时任务
//
//	@receiver c *ThinkExtractCron
//	@author centonhuang
//	@update 2026-06-02 10:00:00
func (c *ThinkExtractCron) Stop() {
	if c.cron != nil {
		ctx := c.cron.Stop()
		<-ctx.Done()
	}
}

// Start 启动消息推理内容提取定时任务，每天凌晨00:00执行
//
//	@receiver c *ThinkExtractCron
//	@return error
//	@author centonhuang
//	@update 2026-06-02 10:00:00
func (c *ThinkExtractCron) Start() error {
	key := fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleThinkExtract)
	entryID, err := c.cron.AddFunc(constant.CronSpecThinkExtract, wrapCronFunc(c.locker, key, LockOptions{}, c.extract))
	if err != nil {
		logger.Logger().Error("[ThinkExtractCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[ThinkExtractCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}

// extract 执行推理内容提取逻辑
//
//	@receiver c *ThinkExtractCron
//	@param ctx context.Context
//	@author centonhuang
//	@update 2026-06-02 10:00:00
func (c *ThinkExtractCron) extract(ctx context.Context) {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)

	var lastID uint
	totalProcessed := 0

	for {
		var messages []*dbmodel.Message
		err := db.Model(&dbmodel.Message{}).
			Where(constant.DBConditionIDGreaterThan, lastID).
			Where(constant.DBConditionDeletedAtZero).
			Where(constant.DBJSONConditionAssistantRole).
			Where(constant.DBJSONConditionHasThinkTag).
			Where(constant.DBJSONConditionReasoningEmpty).
			Order(constant.DBOrderByID).
			Limit(config.SQLBatchSize).
			Find(&messages).Error
		if err != nil {
			log.Error("[ThinkExtractCron] Query error", zap.Error(err))
			return
		}

		if len(messages) == 0 {
			break
		}

		for _, msg := range messages {
			lastID = msg.ID

			if msg.Message == nil || msg.Message.ReasoningContent != "" {
				continue
			}

			extracted := extractThinkFromContent(msg.Message)
			if extracted == "" {
				continue
			}

			msg.Message.ReasoningContent = extracted
			if err := db.Model(&dbmodel.Message{ID: msg.ID}).Select([]string{constant.FieldMessage, constant.FieldUpdatedAt}).Updates(map[string]any{
				constant.FieldMessage:   msg.Message,
				constant.FieldUpdatedAt: time.Now().UTC(),
			}).Error; err != nil {
				log.Error("[ThinkExtractCron] Update error", zap.Uint("id", msg.ID), zap.Error(err))
				continue
			}
			totalProcessed++
		}

		if len(messages) < config.SQLBatchSize {
			break
		}

		log.Info("[ThinkExtractCron] Batch processed",
			zap.Int("batchSize", len(messages)),
			zap.Uint("lastID", lastID))
	}

	log.Info("[ThinkExtractCron] Extract completed", zap.Int("totalProcessed", totalProcessed))
}

// extractThinkFromContent 从消息内容中提取 <think> 标签内容并移除标签
//
//	@param msg *vo.UnifiedMessage
//	@return string 提取的推理内容
//	@author centonhuang
//	@update 2026-06-02 10:00:00
func extractThinkFromContent(msg *vo.UnifiedMessage) string {
	if msg.Content == nil {
		return ""
	}

	var thinkParts []string

	if msg.Content.Text != "" {
		if extracted, modified := extractAndRemoveThinkTags(msg.Content.Text); extracted != "" {
			thinkParts = append(thinkParts, extracted)
			msg.Content.Text = modified
		}
	}

	for i, p := range msg.Content.Parts {
		if p.Type != enum.ContentPartTypeText || !strings.Contains(p.Text, thinkTagOpen) {
			continue
		}
		if extracted, modified := extractAndRemoveThinkTags(p.Text); extracted != "" {
			thinkParts = append(thinkParts, extracted)
			msg.Content.Parts[i].Text = modified
		}
	}

	if len(thinkParts) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.Join(thinkParts, "\n"))
}

// extractAndRemoveThinkTags 从文本中提取 <think>...</think> 内容并移除标签
//
//	@param text string 原始文本
//	@return string 提取的推理内容
//	@return string 移除标签后的文本
//	@author centonhuang
//	@update 2026-06-02 10:00:00
func extractAndRemoveThinkTags(text string) (string, string) {
	matches := thinkRegexp.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return "", text
	}

	var innerParts []string
	for _, m := range matches {
		if trimmed := strings.TrimSpace(m[1]); trimmed != "" {
			innerParts = append(innerParts, trimmed)
		}
	}

	modified := thinkRegexp.ReplaceAllString(text, "")
	return strings.Join(innerParts, "\n"), strings.TrimSpace(modified)
}
