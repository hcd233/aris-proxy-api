// Package aggregate 定义聚合根公共契约。
//
// 仅封装聚合根的身份（ID + AggregateType）。事件驱动 / Outbox 机制尚未进入生产
// 链路，本包暂不暴露事件记录 API，待下一轮重构再按需补齐，避免留下死代码。
//
//	@author centonhuang
//	@update 2026-04-23 10:45:00
package aggregate

// Root 聚合根接口
//
//	@author centonhuang
//	@update 2026-04-23 10:45:00
type Root interface {
	// AggregateID 聚合根唯一标识；未持久化时可为 0
	AggregateID() uint
	// AggregateType 聚合类型，通常由具体聚合返回常量
	AggregateType() string
}

// Base 聚合根公共字段嵌入体
//
//	@author centonhuang
//	@update 2026-04-23 10:45:00
type Base struct {
	id uint
}

// AggregateID 返回聚合根 ID
//
//	@receiver b *Base
//	@return uint
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func (b *Base) AggregateID() uint { return b.id }

// SetID 在聚合首次持久化后回填自增 ID
//
//	@receiver b *Base
//	@param id uint
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func (b *Base) SetID(id uint) { b.id = id }
