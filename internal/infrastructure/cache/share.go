// Package cache 分享缓存操作
package cache

import (
	"context"
	"errors"
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
	DeleteShare(ctx context.Context, userID uint, shareID string) error
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
	now := time.Now()
	shareID := uuid.New().String()
	key := fmt.Sprintf(constant.ShareKeyTemplate, shareID)
	userSharesKey := fmt.Sprintf(constant.UserSharesKeyTemplate, userID)
	expiresAt := now.Add(constant.ShareTTL)

	record := &shareRecord{
		ShareID:   shareID,
		SessionID: sessionID,
		CreatedAt: now.Unix(),
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
	// 不设置 userSharesKey 的全局 TTL：避免后续创建的分享重置 TTL 导致早期分享的索引提前过期。
	// 已过期的 share:{uuid} 会自然失效，ListUserShares 会惰性过滤。

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
		if errors.Is(err, redis.Nil) {
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

// DeleteShare 删除分享链接（含归属校验）
//
//	@receiver s *shareCache
//	@param ctx context.Context
//	@param userID uint
//	@param shareID string
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (s *shareCache) DeleteShare(ctx context.Context, userID uint, shareID string) error {
	userSharesKey := fmt.Sprintf(constant.UserSharesKeyTemplate, userID)

	// 从用户维度的 sorted set 中查找该 shareID 对应的 member
	members, err := s.cache.ZRange(ctx, userSharesKey, 0, -1).Result()
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "failed to lookup user shares")
	}

	found := false
	var targetMember string
	for _, m := range members {
		var record shareRecord
		if unmarshalErr := sonic.Unmarshal([]byte(m), &record); unmarshalErr != nil {
			continue
		}
		if record.ShareID == shareID {
			found = true
			targetMember = m
			break
		}
	}

	if !found {
		return ierr.New(ierr.ErrDataNotExists, "share link not found or not owned by user")
	}

	// 原子删除 share key 和 sorted set member
	pipe := s.cache.Pipeline()
	pipe.Del(ctx, fmt.Sprintf(constant.ShareKeyTemplate, shareID))
	pipe.ZRem(ctx, userSharesKey, targetMember)

	if _, execErr := pipe.Exec(ctx); execErr != nil {
		return ierr.Wrap(ierr.ErrInternal, execErr, "failed to delete share")
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

	// 解析所有 member 为 shareRecord
	var records []shareRecord
	for _, result := range results {
		var record shareRecord
		if unmarshalErr := sonic.Unmarshal([]byte(result), &record); unmarshalErr != nil {
			continue
		}
		records = append(records, record)
	}

	// 批量查询所有 share key 的存在性和 TTL（Pipeline 消除 N+1）
	pipe := s.cache.Pipeline()
	existsCmds := make([]*redis.IntCmd, len(records))
	ttlCmds := make([]*redis.DurationCmd, len(records))
	for i, r := range records {
		shareKey := fmt.Sprintf(constant.ShareKeyTemplate, r.ShareID)
		existsCmds[i] = pipe.Exists(ctx, shareKey)
		ttlCmds[i] = pipe.TTL(ctx, shareKey)
	}

	if _, pipeErr := pipe.Exec(ctx); pipeErr != nil && !errors.Is(pipeErr, redis.Nil) {
		return nil, nil, ierr.Wrap(ierr.ErrInternal, pipeErr, "failed to batch check share keys")
	}

	now := time.Now()
	items := make([]*dto.ShareItem, 0, len(records))
	for i, r := range records {
		if existsCmds[i].Val() == 0 {
			// share key 已过期，惰性过滤
			continue
		}

		expiresAt := time.Time{}
		if ttl := ttlCmds[i].Val(); ttl > 0 {
			expiresAt = now.Add(ttl)
		}

		items = append(items, &dto.ShareItem{
			ShareID:   r.ShareID,
			SessionID: r.SessionID,
			CreatedAt: time.Unix(r.CreatedAt, 0),
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
