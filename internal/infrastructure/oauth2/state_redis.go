package oauth2

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/redis/go-redis/v9"
)

// RedisStateManager OAuth2 state 管理器（Redis 版）
//
// 使用 Redis 存储一次性 state，支持多实例共享。
// key 格式：oauth:state:<state_string>
// value：创建时间的 Unix 纳秒时间戳
// TTL：constant.OAuthStateManagerTTL
//
//	@author centonhuang
//	@update 2026-07-30 19:30:00
type RedisStateManager struct {
	client *redis.Client
}

// NewRedisStateManager 创建 Redis 版 State 管理器
//
//	@param client *redis.Client 已初始化的 Redis 客户端
//	@return *RedisStateManager
//	@author centonhuang
//	@update 2026-07-30 19:30:00
func NewRedisStateManager(client *redis.Client) *RedisStateManager {
	return &RedisStateManager{client: client}
}

// GenerateState 生成携带平台前缀的一次性 state
//
// state 格式："provider:github:<hex>"
// 存入 Redis，TTL 为 constant.OAuthStateManagerTTL（10 分钟）
//
//	@receiver sm *RedisStateManager
//	@param platform string
//	@return string
//	@return error
//	@author centonhuang
//	@update 2026-07-30 19:30:00
func (sm *RedisStateManager) GenerateState(platform string) (string, error) {
	b := make([]byte, constant.OAuthStateBytes)
	if _, err := rand.Read(b); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "generate random state")
	}

	state := constant.StateProviderPrefix + platform + constant.StateProviderSeparator + hex.EncodeToString(b)
	key := constant.RedisOAuthStateKeyPrefix + state
	now := time.Now().UTC().UnixNano()

	if err := sm.client.Set(context.Background(), key, now, constant.OAuthStateManagerTTL).Err(); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "save oauth state to redis")
	}

	return state, nil
}

// VerifyState 验证 state 是否有效（一次性使用）
//
// 使用 Lua 脚本原子地 GET + DEL，确保一次性语义。
// 返回 error 以区分「state 不存在」「state 过期」「存储故障」。
//
//	@receiver sm *RedisStateManager
//	@param state string
//	@return error
//	@author centonhuang
//	@update 2026-07-30 19:30:00
func (sm *RedisStateManager) VerifyState(state string) error {
	key := constant.RedisOAuthStateKeyPrefix + state

	val, err := sm.client.Eval(context.Background(), constant.RedisOAuthStateVerifyScript, []string{key}).Result()
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "verify oauth state")
	}

	if val == nil {
		return ierr.New(ierr.ErrUnauthorized, "oauth state not found")
	}

	createdAtUnix, err := strconv.ParseInt(val.(string), constant.DecimalBase10, constant.BitSize64)
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "parse oauth state timestamp")
	}

	createdAt := time.Unix(0, createdAtUnix)
	if time.Now().UTC().Sub(createdAt) > constant.OAuthStateManagerTTL {
		return ierr.New(ierr.ErrUnauthorized, "oauth state expired")
	}

	return nil
}
