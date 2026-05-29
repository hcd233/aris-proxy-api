// Package cache Session 详情缓存操作
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
)

// SessionMetaCacheRecord 是 session 元数据的缓存载荷。
//
// MessageIDs/ToolIDs 是 cache 内部字段，不直接透出给 API 响应：
// metadata 接口只透出 messageCount = len(MessageIDs)；
// message/tool 分页接口在内部读它们做 offset+limit 切片。
type SessionMetaCacheRecord struct {
	ID         uint              `json:"id"`
	APIKeyName string            `json:"apiKeyName"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	MessageIDs []uint            `json:"messageIds"`
	ToolIDs    []uint            `json:"toolIds"`
}

// MessageCacheRecord 单条 message 缓存载荷
type MessageCacheRecord struct {
	ID        uint               `json:"id"`
	Model     string             `json:"model"`
	Message   *vo.UnifiedMessage `json:"message"`
	CreatedAt time.Time          `json:"createdAt"`
}

// ToolCacheRecord 单条 tool 缓存载荷
type ToolCacheRecord struct {
	ID        uint            `json:"id"`
	Tool      *vo.UnifiedTool `json:"tool"`
	CreatedAt time.Time       `json:"createdAt"`
}

// SessionDetailCache session 详情相关的缓存接口
//
// 设计原则：
//
//   - Get* 系列：cache miss 不算 error；error 仅代表 Redis 通信故障，调用方应当 fallback 到 DB
//
//   - Set* 系列：用 Pipeline 批量写入；Redis 故障不阻断主流程
//
//   - message / tool 是不可变的，缓存内容一旦写入 TTL 内永远有效
//
//     @author centonhuang
//     @update 2026-05-29 14:00:00
type SessionDetailCache interface {
	GetSessionMeta(ctx context.Context, sessionID uint) (*SessionMetaCacheRecord, error)
	SetSessionMeta(ctx context.Context, record *SessionMetaCacheRecord) error

	GetMessages(ctx context.Context, ids []uint) (hits map[uint]*MessageCacheRecord, missing []uint, err error)
	SetMessages(ctx context.Context, records []*MessageCacheRecord) error

	GetTools(ctx context.Context, ids []uint) (hits map[uint]*ToolCacheRecord, missing []uint, err error)
	SetTools(ctx context.Context, records []*ToolCacheRecord) error
}

type sessionDetailCache struct {
	cache *redis.Client
}

// NewSessionDetailCache 创建 session 详情缓存操作实例
//
//	@param cache *redis.Client
//	@return SessionDetailCache
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewSessionDetailCache(cache *redis.Client) SessionDetailCache {
	return &sessionDetailCache{cache: cache}
}

func (s *sessionDetailCache) GetSessionMeta(ctx context.Context, sessionID uint) (*SessionMetaCacheRecord, error) {
	key := fmt.Sprintf(constant.SessionMetaKeyTemplate, sessionID)
	val, err := s.cache.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrInternal, err, "failed to get session meta cache")
	}
	var record SessionMetaCacheRecord
	if unmarshalErr := sonic.UnmarshalString(val, &record); unmarshalErr != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, unmarshalErr, "failed to unmarshal session meta cache")
	}
	return &record, nil
}

func (s *sessionDetailCache) SetSessionMeta(ctx context.Context, record *SessionMetaCacheRecord) error {
	if record == nil {
		return ierr.New(ierr.ErrValidation, "session meta record cannot be nil")
	}
	payload, err := sonic.MarshalString(record)
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "failed to marshal session meta cache")
	}
	key := fmt.Sprintf(constant.SessionMetaKeyTemplate, record.ID)
	if setErr := s.cache.Set(ctx, key, payload, constant.SessionDetailCacheTTL).Err(); setErr != nil {
		return ierr.Wrap(ierr.ErrInternal, setErr, "failed to set session meta cache")
	}
	return nil
}

func (s *sessionDetailCache) GetMessages(ctx context.Context, ids []uint) (map[uint]*MessageCacheRecord, []uint, error) {
	if len(ids) == 0 {
		return map[uint]*MessageCacheRecord{}, nil, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = fmt.Sprintf(constant.MessageKeyTemplate, id)
	}
	values, err := s.cache.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, ids, ierr.Wrap(ierr.ErrInternal, err, "failed to mget messages cache")
	}
	hits := make(map[uint]*MessageCacheRecord, len(values))
	missing := make([]uint, 0, len(ids))
	for i, v := range values {
		if v == nil {
			missing = append(missing, ids[i])
			continue
		}
		raw, ok := v.(string)
		if !ok {
			missing = append(missing, ids[i])
			continue
		}
		var record MessageCacheRecord
		if unmarshalErr := sonic.UnmarshalString(raw, &record); unmarshalErr != nil {
			missing = append(missing, ids[i])
			continue
		}
		hits[ids[i]] = &record
	}
	return hits, missing, nil
}

func (s *sessionDetailCache) SetMessages(ctx context.Context, records []*MessageCacheRecord) error {
	if len(records) == 0 {
		return nil
	}
	pipe := s.cache.Pipeline()
	for _, r := range records {
		if r == nil {
			continue
		}
		payload, err := sonic.MarshalString(r)
		if err != nil {
			return ierr.Wrap(ierr.ErrInternal, err, "failed to marshal message cache")
		}
		key := fmt.Sprintf(constant.MessageKeyTemplate, r.ID)
		pipe.Set(ctx, key, payload, constant.SessionDetailCacheTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "failed to pipeline set messages cache")
	}
	return nil
}

func (s *sessionDetailCache) GetTools(ctx context.Context, ids []uint) (map[uint]*ToolCacheRecord, []uint, error) {
	if len(ids) == 0 {
		return map[uint]*ToolCacheRecord{}, nil, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = fmt.Sprintf(constant.ToolKeyTemplate, id)
	}
	values, err := s.cache.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, ids, ierr.Wrap(ierr.ErrInternal, err, "failed to mget tools cache")
	}
	hits := make(map[uint]*ToolCacheRecord, len(values))
	missing := make([]uint, 0, len(ids))
	for i, v := range values {
		if v == nil {
			missing = append(missing, ids[i])
			continue
		}
		raw, ok := v.(string)
		if !ok {
			missing = append(missing, ids[i])
			continue
		}
		var record ToolCacheRecord
		if unmarshalErr := sonic.UnmarshalString(raw, &record); unmarshalErr != nil {
			missing = append(missing, ids[i])
			continue
		}
		hits[ids[i]] = &record
	}
	return hits, missing, nil
}

func (s *sessionDetailCache) SetTools(ctx context.Context, records []*ToolCacheRecord) error {
	if len(records) == 0 {
		return nil
	}
	pipe := s.cache.Pipeline()
	for _, r := range records {
		if r == nil {
			continue
		}
		payload, err := sonic.MarshalString(r)
		if err != nil {
			return ierr.Wrap(ierr.ErrInternal, err, "failed to marshal tool cache")
		}
		key := fmt.Sprintf(constant.ToolKeyTemplate, r.ID)
		pipe.Set(ctx, key, payload, constant.SessionDetailCacheTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "failed to pipeline set tools cache")
	}
	return nil
}
