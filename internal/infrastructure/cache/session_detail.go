// Package cache Session 详情缓存操作
package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type sessionDetailCache struct {
	cache *redis.Client
}

// NewSessionDetailCache 创建 session 详情缓存操作实例
//
//	@param cache *redis.Client
//	@return SessionDetailCache
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewSessionDetailCache(cache *redis.Client) sessionport.SessionDetailCache {
	return &sessionDetailCache{cache: cache}
}

func (s *sessionDetailCache) GetSessionMeta(ctx context.Context, sessionID uint) (*sessionport.SessionMetaCacheRecord, error) {
	key := fmt.Sprintf(constant.SessionMetaKeyTemplate, sessionID)
	val, err := s.cache.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrInternal, err, "failed to get session meta cache")
	}
	var record sessionport.SessionMetaCacheRecord
	if unmarshalErr := sonic.UnmarshalString(val, &record); unmarshalErr != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, unmarshalErr, "failed to unmarshal session meta cache")
	}
	return &record, nil
}

func (s *sessionDetailCache) SetSessionMeta(ctx context.Context, record *sessionport.SessionMetaCacheRecord) error {
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

func (s *sessionDetailCache) GetMessages(ctx context.Context, ids []uint) (map[uint]*sessionport.MessageCacheRecord, []uint, error) {
	if len(ids) == 0 {
		return map[uint]*sessionport.MessageCacheRecord{}, nil, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = fmt.Sprintf(constant.MessageKeyTemplate, id)
	}
	values, err := s.cache.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, ids, ierr.Wrap(ierr.ErrInternal, err, "failed to mget messages cache")
	}
	hits := make(map[uint]*sessionport.MessageCacheRecord, len(values))
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
		var record sessionport.MessageCacheRecord
		if unmarshalErr := sonic.UnmarshalString(raw, &record); unmarshalErr != nil {
			missing = append(missing, ids[i])
			continue
		}
		hits[ids[i]] = &record
	}
	return hits, missing, nil
}

func (s *sessionDetailCache) SetMessages(ctx context.Context, records []*sessionport.MessageCacheRecord) error {
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

func (s *sessionDetailCache) GetTools(ctx context.Context, ids []uint) (map[uint]*sessionport.ToolCacheRecord, []uint, error) {
	if len(ids) == 0 {
		return map[uint]*sessionport.ToolCacheRecord{}, nil, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = fmt.Sprintf(constant.ToolKeyTemplate, id)
	}
	values, err := s.cache.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, ids, ierr.Wrap(ierr.ErrInternal, err, "failed to mget tools cache")
	}
	hits := make(map[uint]*sessionport.ToolCacheRecord, len(values))
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
		var record sessionport.ToolCacheRecord
		if unmarshalErr := sonic.UnmarshalString(raw, &record); unmarshalErr != nil {
			missing = append(missing, ids[i])
			continue
		}
		hits[ids[i]] = &record
	}
	return hits, missing, nil
}

func (s *sessionDetailCache) SetTools(ctx context.Context, records []*sessionport.ToolCacheRecord) error {
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
