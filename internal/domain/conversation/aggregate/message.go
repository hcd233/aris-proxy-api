// Package aggregate Conversation 域聚合根
//
// Conversation 域采用**内容寻址**模式：Message/Tool 是独立聚合根，
// 通过 Checksum 唯一标识。同一份消息内容在整个系统中只存一条记录，
// 由多个 Session 通过 ID 引用。
//
// 这与传统 DDD "聚合根独占实体" 模式不同，是针对 LLM 对话场景的优化。
//
//	@author centonhuang
//	@update 2026-04-22 19:30:00
package aggregate

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
)

// Message 消息聚合根
//
// 不可变：创建后内容不可修改。通过 Checksum 做去重主键。
// Model 字段仅对 assistant 消息非空（记录上游 model，用于审计查询）。
//
//	@author centonhuang
//	@update 2026-04-22 19:30:00
type Message struct {
	aggregate.Base

	content  *vo.UnifiedMessage
	model    string // upstream model，仅 assistant 角色有
	checksum string
}

// RecordMessage 构造一条待持久化的 Message 聚合
//
// 注意：Checksum 必须由调用方传入（通常通过 ConversationDedupService 计算），
// 这样可以保证跨调用一致的 schema-aware checksum 策略。
//
//	@param content *vo.UnifiedMessage
//	@param upstreamModel string 仅 assistant 消息需要，其他传空字符串
//	@param checksum string
//	@return *Message
//	@return error
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func RecordMessage(content *vo.UnifiedMessage, upstreamModel, checksum string) (*Message, error) {
	if content == nil {
		return nil, ierr.New(ierr.ErrValidation, "message content is nil")
	}
	if checksum == "" {
		return nil, ierr.New(ierr.ErrValidation, "message checksum is empty")
	}
	return &Message{
		content:  content,
		model:    upstreamModel,
		checksum: checksum,
	}, nil
}

// RestoreMessage 从仓储重建 Message 聚合（不产生事件）
//
//	@param id uint
//	@param content *vo.UnifiedMessage
//	@param upstreamModel string
//	@param checksum string
//	@return *Message
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func RestoreMessage(id uint, content *vo.UnifiedMessage, upstreamModel, checksum string) *Message {
	m := &Message{content: content, model: upstreamModel, checksum: checksum}
	m.SetID(id)
	return m
}

// AggregateType 实现 aggregate.Root 接口
func (*Message) AggregateType() string { return constant.AggregateTypeMessage }

// Content 返回消息内容
func (m *Message) Content() *vo.UnifiedMessage { return m.content }

// Model 返回上游 model 名
func (m *Message) Model() string { return m.model }

// Checksum 返回校验和
func (m *Message) Checksum() string { return m.checksum }
