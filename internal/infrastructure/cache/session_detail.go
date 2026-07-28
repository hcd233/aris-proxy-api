// Package cache Session 详情缓存操作
package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/samber/mo/option"

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

func (s *sessionDetailCache) DeleteSessionMeta(ctx context.Context, sessionID uint) error {
	key := fmt.Sprintf(constant.SessionMetaKeyTemplate, sessionID)
	if err := s.cache.Del(ctx, key).Err(); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "failed to delete session meta cache")
	}
	return nil
}

// mgetRecords 按 key 模板批量 MGet 缓存记录；未命中或反序列化失败的 ID 归入 missing。
func mgetRecords[T any](ctx context.Context, cache *redis.Client, ids []uint, keyTemplate string) (hits map[uint]*T, missing []uint, err error) {
	if len(ids) == 0 {
		return map[uint]*T{}, nil, nil
	}
	keys := lo.Map(ids, func(id uint, _ int) string { return fmt.Sprintf(keyTemplate, id) })
	values, err := cache.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, ids, ierr.Wrap(ierr.ErrInternal, err, "failed to mget session detail cache")
	}
	hits = make(map[uint]*T, len(values))
	missing = make([]uint, 0, len(ids))
	for i, v := range values {
		raw, ok := v.(string)
		record := option.FlatMap(func(raw string) mo.Option[*T] {
			var r T
			if unmarshalErr := sonic.UnmarshalString(raw, &r); unmarshalErr != nil {
				return mo.None[*T]()
			}
			return mo.Some(&r)
		})(mo.TupleToOption(raw, ok))
		if rec, ok := record.Get(); ok {
			hits[ids[i]] = rec
		} else {
			missing = append(missing, ids[i])
		}
	}
	return hits, missing, nil
}

// msetRecords 按 key 模板批量 Pipeline Set 缓存记录，跳过 nil 记录。
func msetRecords[T any](ctx context.Context, cache *redis.Client, records []*T, keyTemplate string, idOf func(*T) uint) error {
	if len(records) == 0 {
		return nil
	}
	pipe := cache.Pipeline()
	for _, r := range records {
		if r == nil {
			continue
		}
		payload, err := sonic.MarshalString(r)
		if err != nil {
			return ierr.Wrap(ierr.ErrInternal, err, "failed to marshal session detail cache record")
		}
		pipe.Set(ctx, fmt.Sprintf(keyTemplate, idOf(r)), payload, constant.SessionDetailCacheTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "failed to pipeline set session detail cache")
	}
	return nil
}

func (s *sessionDetailCache) GetMessages(ctx context.Context, ids []uint) (results map[uint]*sessionport.MessageCacheRecord, missed []uint, err error) {
	return mgetRecords[sessionport.MessageCacheRecord](ctx, s.cache, ids, constant.MessageKeyTemplate)
}

func (s *sessionDetailCache) SetMessages(ctx context.Context, records []*sessionport.MessageCacheRecord) error {
	return msetRecords(ctx, s.cache, records, constant.MessageKeyTemplate, func(r *sessionport.MessageCacheRecord) uint { return r.ID })
}

func (s *sessionDetailCache) GetTools(ctx context.Context, ids []uint) (results map[uint]*sessionport.ToolCacheRecord, missed []uint, err error) {
	return mgetRecords[sessionport.ToolCacheRecord](ctx, s.cache, ids, constant.ToolKeyTemplate)
}

func (s *sessionDetailCache) SetTools(ctx context.Context, records []*sessionport.ToolCacheRecord) error {
	return msetRecords(ctx, s.cache, records, constant.ToolKeyTemplate, func(r *sessionport.ToolCacheRecord) uint { return r.ID })
}
