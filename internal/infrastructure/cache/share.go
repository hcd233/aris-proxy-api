// Package cache 分享缓存操作
package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/redis/go-redis/v9"
)

// shareRecord 分享记录（存储在 Redis Sorted Set 的 member 中）
type shareRecord struct {
	ShareID   string `json:"shareId"`
	SessionID uint   `json:"sessionId"`
	CreatedAt int64  `json:"createdAt"`
}

// ShareCache 分享缓存操作接口
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type ShareCache interface {
	CreateShare(ctx context.Context, userID, sessionID uint) (string, time.Time, error)
	GetShareSessionID(ctx context.Context, shareID string) (uint, error)
	DeleteShare(ctx context.Context, shareID string) error
	ListUserShares(ctx context.Context, userID uint, page, pageSize int) ([]*dto.ShareItem, *model.PageInfo, error)
}

type shareCache struct {
	cache *redis.Client
}

// NewShareCache 创建分享缓存操作实例
//
//	@param cache *redis.Client
//	@return ShareCache
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func NewShareCache(cache *redis.Client) ShareCache {
	return &shareCache{cache: cache}
}

// CreateShare 创建分享链接
//
//	@receiver s *shareCache
//	@param ctx context.Context
//	@param userID uint
//	@param sessionID uint
//	@return string shareID
//	@return time.Time expiresAt
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (s *shareCache) CreateShare(ctx context.Context, userID, sessionID uint) (string, time.Time, error) {
	shareID := uuid.New().String()
	key := fmt.Sprintf(constant.ShareKeyTemplate, shareID)
	userSharesKey := fmt.Sprintf(constant.UserSharesKeyTemplate, userID)
	expiresAt := time.Now().Add(constant.ShareTTL)

	record := &shareRecord{
		ShareID:   shareID,
		SessionID: sessionID,
		CreatedAt: time.Now().Unix(),
	}
	recordJSON, err := sonic.Marshal(record)
	if err != nil {
		return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, err, "failed to marshal share record")
	}

	pipe := s.cache.Pipeline()
	pipe.Set(ctx, key, sessionID, constant.ShareTTL)
	pipe.ZAdd(ctx, userSharesKey, redis.Z{
		Score:  float64(record.CreatedAt),
		Member: string(recordJSON),
	})
	pipe.Expire(ctx, userSharesKey, constant.ShareTTL)

	if _, execErr := pipe.Exec(ctx); execErr != nil {
		return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, execErr, "failed to create share")
	}

	return shareID, expiresAt, nil
}

// GetShareSessionID 获取分享链接对应的 sessionID
//
//	@receiver s *shareCache
//	@param ctx context.Context
//	@param shareID string
//	@return uint
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (s *shareCache) GetShareSessionID(ctx context.Context, shareID string) (uint, error) {
	key := fmt.Sprintf(constant.ShareKeyTemplate, shareID)
	val, err := s.cache.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, ierr.New(ierr.ErrDataNotExists, "share link not found or expired")
		}
		return 0, ierr.Wrap(ierr.ErrInternal, err, "failed to get share")
	}

	sessionID, parseErr := strconv.ParseUint(val, constant.DecimalBase, constant.ParseFloat64BitSize)
	if parseErr != nil {
		return 0, ierr.Wrap(ierr.ErrInternal, parseErr, "failed to parse session ID from share")
	}

	return uint(sessionID), nil
}

// DeleteShare 删除分享链接
//
//	@receiver s *shareCache
//	@param ctx context.Context
//	@param shareID string
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (s *shareCache) DeleteShare(ctx context.Context, shareID string) error {
	key := fmt.Sprintf(constant.ShareKeyTemplate, shareID)
	err := s.cache.Del(ctx, key).Err()
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "failed to delete share")
	}
	return nil
}

// ListUserShares 获取用户的所有分享链接
//
//	@receiver s *shareCache
//	@param ctx context.Context
//	@param userID uint
//	@param page int
//	@param pageSize int
//	@return []*dto.ShareItem
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (s *shareCache) ListUserShares(ctx context.Context, userID uint, page, pageSize int) ([]*dto.ShareItem, *model.PageInfo, error) {
	userSharesKey := fmt.Sprintf(constant.UserSharesKeyTemplate, userID)

	total, err := s.cache.ZCard(ctx, userSharesKey).Result()
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrInternal, err, "failed to count user shares")
	}

	start := int64((page - 1) * pageSize)
	stop := int64(page*pageSize - 1)

	results, zErr := s.cache.ZRevRange(ctx, userSharesKey, start, stop).Result()
	if zErr != nil {
		return nil, nil, ierr.Wrap(ierr.ErrInternal, zErr, "failed to list user shares")
	}

	items := make([]*dto.ShareItem, 0, len(results))
	for _, result := range results {
		var record shareRecord
		if unmarshalErr := sonic.Unmarshal([]byte(result), &record); unmarshalErr != nil {
			continue
		}

		shareKey := fmt.Sprintf(constant.ShareKeyTemplate, record.ShareID)
		exists, existsErr := s.cache.Exists(ctx, shareKey).Result()
		if existsErr != nil || exists == 0 {
			continue
		}

		ttl, ttlErr := s.cache.TTL(ctx, shareKey).Result()
		expiresAt := time.Time{}
		if ttlErr == nil && ttl > 0 {
			expiresAt = time.Now().Add(ttl)
		}

		items = append(items, &dto.ShareItem{
			ShareID:   record.ShareID,
			SessionID: record.SessionID,
			CreatedAt: time.Unix(record.CreatedAt, 0),
			ExpiresAt: expiresAt,
		})
	}

	pageInfo := &model.PageInfo{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}

	return items, pageInfo, nil
}
