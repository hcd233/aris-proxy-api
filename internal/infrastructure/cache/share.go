// Package cache 分享缓存操作
package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/redis/go-redis/v9"
)

// shareRecord 分享记录（存储在 Redis Sorted Set 的 member 中）
type shareRecord struct {
	ShareID   string `json:"shareId"`
	SessionID uint   `json:"sessionId"`
	CreatedAt int64  `json:"createdAt"`
	TTL       int64  `json:"ttl"`
}

// ShareCache 分享缓存操作接口
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type ShareCache interface {
	CreateShare(ctx context.Context, userID, sessionID uint, ttl time.Duration) (string, time.Time, error)
	GetShareSessionID(ctx context.Context, shareID string) (uint, error)
	DeleteShare(ctx context.Context, userID uint, shareID string) error
	ListUserShares(ctx context.Context, userID uint, page, pageSize int) ([]*dto.ShareItem, *model.PageInfo, error)
	GetSessionShareID(ctx context.Context, sessionID uint) (string, error)
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
func (s *shareCache) CreateShare(ctx context.Context, userID, sessionID uint, ttl time.Duration) (string, time.Time, error) {
	if sessionID == 0 {
		return "", time.Time{}, ierr.New(ierr.ErrValidation, "sessionID must be greater than 0")
	}
	if ttl <= 0 {
		return "", time.Time{}, ierr.New(ierr.ErrValidation, "ttl must be greater than 0")
	}

	existingShareID, checkErr := s.GetSessionShareID(ctx, sessionID)
	if checkErr != nil {
		return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, checkErr, "failed to check existing share")
	}
	if existingShareID != "" {
		return "", time.Time{}, ierr.New(ierr.ErrDataExists, "session already has an active share")
	}

	now := time.Now()
	expiresAt := now.Add(ttl)

	// 防撞：用 SET NX 原子占位 share key，碰撞则重试。
	// 长度从 constant.ShareIDMinLen 起逐级提升到 constant.ShareIDMaxLen，
	// 每个长度尝试 constant.ShareIDMaxAttemptsPerLen 次。
	shareID, key, reserveErr := s.reserveShareID(ctx, sessionID, ttl)
	if reserveErr != nil {
		return "", time.Time{}, reserveErr
	}

	userSharesKey := fmt.Sprintf(constant.UserSharesKeyTemplate, userID)
	sessionSharesKey := fmt.Sprintf(constant.SessionSharesKeyTemplate, sessionID)

	record := &shareRecord{
		ShareID:   shareID,
		SessionID: sessionID,
		CreatedAt: now.Unix(),
		TTL:       int64(ttl.Seconds()),
	}
	recordJSON, err := sonic.Marshal(record)
	if err != nil {
		// 回滚已占位的 share key，避免长期残留
		s.cache.Del(ctx, key)
		return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, err, "failed to marshal share record")
	}

	pipe := s.cache.Pipeline()
	pipe.ZAdd(ctx, userSharesKey, redis.Z{
		Score:  float64(record.CreatedAt),
		Member: string(recordJSON),
	})
	pipe.SAdd(ctx, sessionSharesKey, shareID)
	pipe.Expire(ctx, sessionSharesKey, ttl)

	if _, execErr := pipe.Exec(ctx); execErr != nil {
		// 回滚已占位的 share key，避免索引/key 出现孤儿
		s.cache.Del(ctx, key)
		return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, execErr, "failed to create share")
	}

	return shareID, expiresAt, nil
}

// reserveShareID 通过 Redis SET NX 原子占位完成 shareID 防撞，
// 冲突时先在当前长度内重试，超过尝试次数再增加 1 位长度，最长不超过 constant.ShareIDMaxLen。
//
//	@receiver s *shareCache
//	@param ctx context.Context
//	@param sessionID uint
//	@return string shareID 已成功占位的短码
//	@return string key 已写入的 Redis share key（用于失败回滚）
//	@return error
//	@author centonhuang
//	@update 2026-05-28 20:10:00
func (s *shareCache) reserveShareID(ctx context.Context, sessionID uint, ttl time.Duration) (string, string, error) {
	for length := constant.ShareIDMinLen; length <= constant.ShareIDMaxLen; length++ {
		for attempt := 0; attempt < constant.ShareIDMaxAttemptsPerLen; attempt++ {
			shareID, genErr := util.GenerateShareID(sessionID, length)
			if genErr != nil {
				return "", "", genErr
			}
			key := fmt.Sprintf(constant.ShareKeyTemplate, shareID)
			ok, setErr := s.cache.SetNX(ctx, key, sessionID, ttl).Result()
			if setErr != nil {
				return "", "", ierr.Wrap(ierr.ErrInternal, setErr, "failed to reserve share key")
			}
			if ok {
				return shareID, key, nil
			}
			// 冲突：换一个 nonce 再来
		}
	}
	return "", "", ierr.New(ierr.ErrInternal, "failed to reserve unique shareID after retries")
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
	if sessionID == 0 {
		// 兜底：防御 GORM 零值 where 条件被忽略导致返回错位 session
		return 0, ierr.New(ierr.ErrInternal, "share record has invalid session id")
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
	var targetRecord shareRecord
	for _, m := range members {
		var record shareRecord
		if unmarshalErr := sonic.Unmarshal([]byte(m), &record); unmarshalErr != nil {
			continue
		}
		if record.ShareID == shareID {
			found = true
			targetMember = m
			targetRecord = record
			break
		}
	}

	if !found {
		return ierr.New(ierr.ErrDataNotExists, "share link not found or not owned by user")
	}

	sessionSharesKey := fmt.Sprintf(constant.SessionSharesKeyTemplate, targetRecord.SessionID)

	pipe := s.cache.Pipeline()
	pipe.Del(ctx, fmt.Sprintf(constant.ShareKeyTemplate, shareID))
	pipe.ZRem(ctx, userSharesKey, targetMember)
	pipe.SRem(ctx, sessionSharesKey, shareID)

	if _, execErr := pipe.Exec(ctx); execErr != nil {
		return ierr.Wrap(ierr.ErrInternal, execErr, "failed to delete share")
	}
	return nil
}

// ListUserShares 获取用户的所有分享链接（含 retention 窗口内已过期条目）
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
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	now := time.Now()
	retentionStart := now.Add(-constant.ShareExpiredRetention)
	minCreatedAt := now.Add(-constant.ShareTTLNeverExpire - constant.ShareExpiredRetention).Unix()
	scoreRange := &redis.ZRangeBy{
		Max: constant.RedisZRangePositiveInfinity,
		Min: strconv.FormatInt(minCreatedAt, constant.DecimalBase),
	}

	results, zErr := s.cache.ZRevRangeByScore(ctx, userSharesKey, scoreRange).Result()
	if zErr != nil {
		return nil, nil, ierr.Wrap(ierr.ErrInternal, zErr, "failed to list user shares")
	}

	// 解析当前页 member 为 shareRecord
	records := make([]shareRecord, 0, len(results))
	for _, result := range results {
		var record shareRecord
		if unmarshalErr := sonic.Unmarshal([]byte(result), &record); unmarshalErr != nil {
			continue
		}
		records = append(records, record)
	}

	// 批量查询当前页 share key 的存在性（Pipeline 消除 N+1）
	pipe := s.cache.Pipeline()
	existsCmds := make([]*redis.IntCmd, len(records))
	for i, r := range records {
		shareKey := fmt.Sprintf(constant.ShareKeyTemplate, r.ShareID)
		existsCmds[i] = pipe.Exists(ctx, shareKey)
	}

	if _, pipeErr := pipe.Exec(ctx); pipeErr != nil && !errors.Is(pipeErr, redis.Nil) {
		return nil, nil, ierr.Wrap(ierr.ErrInternal, pipeErr, "failed to batch check share keys")
	}

	// 仅过滤"被手动删除但尚未 TTL 过期"的边界记录；远过期项按各记录 TTL + retention 窗口过滤。
	items := make([]*dto.ShareItem, 0, len(records))
	for i, r := range records {
		createdAt := time.Unix(r.CreatedAt, 0)
		ttl := time.Duration(r.TTL) * time.Second
		if ttl <= 0 {
			ttl = constant.ShareTTLDefault
		}
		expiresAt := createdAt.Add(ttl)
		if expiresAt.Before(retentionStart) {
			continue
		}
		if existsCmds[i].Val() == 0 && !expiresAt.Before(now) {
			continue
		}

		items = append(items, &dto.ShareItem{
			ShareID:   r.ShareID,
			SessionID: r.SessionID,
			CreatedAt: createdAt,
			ExpiresAt: expiresAt,
		})
	}

	total := int64(len(items))
	start := (page - 1) * pageSize
	if start >= len(items) {
		items = []*dto.ShareItem{}
	} else {
		end := start + pageSize
		if end > len(items) {
			end = len(items)
		}
		items = items[start:end]
	}

	pageInfo := &model.PageInfo{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}

	return items, pageInfo, nil
}

func (s *shareCache) GetSessionShareID(ctx context.Context, sessionID uint) (string, error) {
	key := fmt.Sprintf(constant.SessionSharesKeyTemplate, sessionID)
	members, err := s.cache.SMembers(ctx, key).Result()
	if err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "failed to get session share ID")
	}
	if len(members) == 0 {
		return "", nil
	}
	return members[0], nil
}
