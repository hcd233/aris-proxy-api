package vo

// APIKeyOwnerID Session 归属的 API Key ID 值对象。
//
// 归属判定的唯一权威依据。相比名称，ID 不可跨用户重复，因此不存在
// 「他人建同名 Key 即可越权」的问题（名称只在 (user_id, name) 维度唯一，
// 仅作展示字段保留在聚合的 ownerName 中，不参与鉴权）。
// 0 表示归属未知（2026-07 前后的存量会话）；自增主键从 1 起，故 0
// 天然不匹配任何真实 Key，此类会话仅 admin 可见。
//
//	@author centonhuang
//	@update 2026-09-27 10:00:00
type APIKeyOwnerID uint

// Uint 返回底层 ID
func (o APIKeyOwnerID) Uint() uint { return uint(o) }

// IsEmpty 归属是否缺失（0 为缺失）
func (o APIKeyOwnerID) IsEmpty() bool { return uint(o) == 0 }
