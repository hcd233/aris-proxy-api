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
	"golang.org/x/oauth2"
)

// UserInfo 用户信息
type UserInfo interface {
	GetID() string
	GetName() string
	GetEmail() string
	GetAvatar() string
}

// Platform OAuth2 提供商接口
//
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
	GetUserInfo(ctx context.Context, token *oauth2.Token) (UserInfo, error)
}

// StateManager OAuth2 state管理器，防止CSRF攻击
type StateManager struct {
	states map[string]time.Time
	mu     sync.RWMutex
	ttl    time.Duration
}

// NewStateManager 创建State管理器
func NewStateManager() *StateManager {
	sm := &StateManager{
		states: make(map[string]time.Time),
		ttl:    constant.OAuthStateManagerTTL,
	}
	go sm.cleanup()
	return sm
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
	sm.mu.Unlock()

	return state, nil
}

// VerifyState 验证state是否有效（一次性使用）
func (sm *StateManager) VerifyState(state string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	createdAt, exists := sm.states[state]
	if !exists {
		return false
	}

	// 删除已使用的state（一次性）
	delete(sm.states, state)

	// 检查是否过期
	return time.Now().UTC().Sub(createdAt) <= sm.ttl
}

// cleanup 定期清理过期state
func (sm *StateManager) cleanup() {
	ticker := time.NewTicker(constant.OAuthStateCleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now().UTC()
		for state, createdAt := range sm.states {
			if now.Sub(createdAt) > sm.ttl {
				delete(sm.states, state)
			}
		}
		sm.mu.Unlock()
	}
}

// globalStateManager 全局state管理器实例
var globalStateManager = NewStateManager()

// GenerateOAuth2State 生成OAuth2 state
func GenerateOAuth2State() (string, error) {
	return globalStateManager.GenerateState()
}

// VerifyOAuth2State 验证OAuth2 state
func VerifyOAuth2State(state string) bool {
	return globalStateManager.VerifyState(state)
}
