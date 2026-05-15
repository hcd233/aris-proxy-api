// Package oauth2 Oauth2
package oauth2

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/oauth2/vo"
	"golang.org/x/oauth2"
)

// Platform OAuth2 提供商接口
//
// 实现 domain/oauth2/service.Platform 接口。
// 安全约束：所有实现必须通过 GetAuthURLWithState 携带一次性 state，
// 静态 state 形态（早期的 GetAuthURL()）已废除，避免 CSRF。
//
//	@author centonhuang
//	@update 2026-04-24 14:00:00
type Platform interface {
	// GetAuthURLWithState 获取携带一次性 state 的授权 URL
	GetAuthURLWithState(state string) string
	// ExchangeToken 通过授权码获取 Access Token
	ExchangeToken(ctx context.Context, code string) (*oauth2.Token, error)
	// GetUserInfo 获取用户信息
	GetUserInfo(ctx context.Context, token *oauth2.Token) (vo.OAuthUserInfo, error)
}

// StateManager OAuth2 state管理器，防止CSRF攻击
type StateManager struct {
	states map[string]time.Time
	mu     sync.RWMutex
	ttl    time.Duration
}

// NewStateManager 创建State管理器
func NewStateManager() *StateManager {
	return &StateManager{
		states: make(map[string]time.Time),
		ttl:    constant.OAuthStateManagerTTL,
	}
}

// GenerateState 生成随机state
func (sm *StateManager) GenerateState() (string, error) {
	b := make([]byte, constant.OAuthStateBytes)
	if _, err := rand.Read(b); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "generate random state")
	}
	state := hex.EncodeToString(b)

	sm.mu.Lock()
	sm.states[state] = time.Now().UTC()
	if len(sm.states) > constant.OAuthStateMaxPending {
		sm.cleanupExpired()
	}
	sm.mu.Unlock()

	return state, nil
}

// VerifyState 验证state是否有效（一次性使用）
//
// 返回 error 以区分「state 无效/过期」和「存储故障」。
// 当前内存实现不产生存储故障，但保留 error 返回值以符合 domain 接口契约。
func (sm *StateManager) VerifyState(state string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	createdAt, exists := sm.states[state]
	if !exists {
		return ierr.New(ierr.ErrUnauthorized, "oauth state not found")
	}

	delete(sm.states, state)

	if time.Now().UTC().Sub(createdAt) > sm.ttl {
		return ierr.New(ierr.ErrUnauthorized, "oauth state expired")
	}

	return nil
}

// cleanupExpired 清理过期state（调用方需持有锁）
func (sm *StateManager) cleanupExpired() {
	now := time.Now().UTC()
	for state, createdAt := range sm.states {
		if now.Sub(createdAt) > sm.ttl {
			delete(sm.states, state)
		}
	}
}

// globalStateManager 全局state管理器实例
var globalStateManager = NewStateManager()

// GenerateOAuth2State 生成OAuth2 state
func GenerateOAuth2State() (string, error) {
	return globalStateManager.GenerateState()
}

// VerifyOAuth2State 验证OAuth2 state
func VerifyOAuth2State(state string) error {
	return globalStateManager.VerifyState(state)
}
