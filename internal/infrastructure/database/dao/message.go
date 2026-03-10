// Package dao Message DAO
//
//	author centonhuang
//	update 2026-03-10 10:00:00
package dao

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MessageDAO 消息数据访问对象
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type MessageDAO struct {
	baseDAO[model.Message]
}

// StoreMessageChain 存储消息链（带链式去重）
//
//	@receiver dao *MessageDAO
//	@param ctx context.Context
//	@param apiKeyName string API Key名称（用户隔离）
//	@param messages []*dto.ChatCompletionMessageParam 用户请求消息列表
//	@param response *dto.ChatCompletionMessageParam 模型回复消息
//	@return error
//	@author centonhuang
//	@update 2026-03-10 10:00:00
func (dao *MessageDAO) StoreMessageChain(ctx context.Context, apiKeyName string, messages []*dto.ChatCompletionMessageParam, response *dto.ChatCompletionMessageParam) error {
	logger := logger.WithCtx(ctx)

	// 2. 构建完整消息链（用户消息 + 模型回复）
	fullChain := make([]*dto.ChatCompletionMessageParam, 0, len(messages)+1)
	fullChain = append(fullChain, messages...)
	if response != nil {
		fullChain = append(fullChain, response)
	}

	if len(fullChain) == 0 {
		logger.Info("[MessageDAO] empty message chain, nothing to store")
		return nil
	}

	// 3. 计算每条消息的校验和
	checksums := make([]string, len(fullChain))
	for i, msg := range fullChain {
		checksum, err := util.ComputeMessageChecksum(msg)
		if err != nil {
			logger.Error("[MessageDAO] failed to compute checksum", zap.Int("index", i), zap.Error(err))
			checksum = "" // 计算失败时使用空字符串，继续处理
		}
		checksums[i] = checksum
	}

	db := database.GetDBInstance(ctx)

	// 4. 从后往前查找第一个不匹配的消息索引
	insertStartIndex := 0
	preMessageID := uint(0)

	for i := len(fullChain) - 1; i >= 0; i-- {
		// 查询数据库中是否存在相同(apiKeyName, checkSum)的消息
		var existingMsg model.Message
		err := db.Where("api_key_name = ? AND check_sum = ?", apiKeyName, checksums[i]).
			Where("deleted_at = 0").
			First(&existingMsg).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// 消息不存在，标记为需要插入的起点
				insertStartIndex = i
				preMessageID = 0
				continue
			}
			logger.Error("[MessageDAO] failed to query message", zap.Int("index", i), zap.Error(err))
			return err
		}

		// 消息存在，检查PreMessageID是否匹配
		if i == 0 {
			// 这是第一条消息，PreMessageID应该为0
			if existingMsg.PreMessageID == 0 {
				// 完全匹配，这条及之后的都不需要插入
				insertStartIndex = -1
				preMessageID = existingMsg.ID
				break
			}
			// PreMessageID不匹配，从这里开始插入
			insertStartIndex = i
			preMessageID = 0
		} else {
			// 不是第一条，需要继续检查前一条
			if i == len(fullChain)-1 {
				// 最后一条消息的PreMessageID需要特殊处理
				// 暂时记录，等待前一条消息确定
				preMessageID = existingMsg.ID
			}
		}
	}

	if insertStartIndex == -1 {
		logger.Info("[MessageDAO] all messages already exist, skip insertion")
		return nil
	}

	// 5. 重新遍历确定正确的preMessageID链
	// 需要找到insertStartIndex之前的那条消息的ID作为起始preMessageID
	if insertStartIndex > 0 {
		// 查询insertStartIndex-1对应的消息是否存在
		var prevMsg model.Message
		err := db.Where("api_key_name = ? AND check_sum = ?", apiKeyName, checksums[insertStartIndex-1]).
			Where("deleted_at = 0").
			First(&prevMsg).Error
		switch err {
		case nil:
			preMessageID = prevMsg.ID
		case gorm.ErrRecordNotFound:
			// 前一条消息应该存在，如果不存在说明逻辑有问题
			logger.Warn("[MessageDAO] previous message not found", zap.Int("index", insertStartIndex-1))
			preMessageID = 0
		default:
			logger.Error("[MessageDAO] failed to query previous message", zap.Error(err))
			return err
		}
	}

	// 6. 构建需要插入的消息记录
	messagesToInsert := make([]*model.Message, 0, len(fullChain)-insertStartIndex)

	for i := insertStartIndex; i < len(fullChain); i++ {
		msg := fullChain[i]
		msgModel := &model.Message{
			APIKeyName:   apiKeyName,
			Message:      *msg,
			CheckSum:     checksums[i],
			PreMessageID: preMessageID,
		}
		messagesToInsert = append(messagesToInsert, msgModel)
	}

	if len(messagesToInsert) == 0 {
		logger.Info("[MessageDAO] no messages to insert")
		return nil
	}

	// 7. 使用事务批量插入消息
	err := db.Transaction(func(tx *gorm.DB) error {
		for i, msgModel := range messagesToInsert {
			if err := tx.Create(msgModel).Error; err != nil {
				logger.Error("[MessageDAO] failed to create message", zap.Int("index", insertStartIndex+i), zap.Error(err))
				return err
			}
			// 更新preMessageID用于下一条消息的链式关联
			preMessageID = msgModel.ID
			// 更新消息记录中的PreMessageID
			if i < len(messagesToInsert)-1 {
				messagesToInsert[i+1].PreMessageID = preMessageID
			}
		}
		return nil
	})

	if err != nil {
		logger.Error("[MessageDAO] failed to insert messages", zap.Error(err))
		return err
	}

	logger.Info("[MessageDAO] message chain stored successfully",
		zap.Int("total", len(fullChain)),
		zap.Int("inserted", len(messagesToInsert)),
		zap.Int("skipped", insertStartIndex))

	return nil
}
