package oauthprovider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// oauthTokenProvider OAuthTokenProvider 实现：缓存 → DB → 锁内双检刷新。
type oauthTokenProvider struct {
	credRepo    llmproxy.OAuthCredentialRepository
	clients     map[enum.OAuthProvider]service.OAuthProviderClient
	accessCache service.OAuthAccessCache
	locker      lock.Locker
}

// NewOAuthTokenProvider 构造订阅账号 token 提供者
func NewOAuthTokenProvider(
	credRepo llmproxy.OAuthCredentialRepository,
	clients map[enum.OAuthProvider]service.OAuthProviderClient,
	accessCache service.OAuthAccessCache,
	locker lock.Locker,
) service.OAuthTokenProvider {
	return &oauthTokenProvider{credRepo: credRepo, clients: clients, accessCache: accessCache, locker: locker}
}

// GetAccessToken 返回可用 access token；临期时持锁刷新（双检防重复刷新）。
func (p *oauthTokenProvider) GetAccessToken(ctx context.Context, endpointID uint) (service.OAuthAccess, error) {
	if access, ok := p.accessCache.Get(ctx, endpointID); ok {
		return access, nil
	}

	cred, err := p.credRepo.FindByEndpointID(ctx, endpointID)
	if err != nil {
		return service.OAuthAccess{}, err
	}
	if cred == nil {
		return service.OAuthAccess{}, &model.OAuthCredentialError{Reason: constant.OAuthReasonCredentialNotFound}
	}
	if cred.Status() == enum.OAuthCredentialStatusReauthRequired {
		return service.OAuthAccess{}, &model.OAuthCredentialError{Reason: constant.OAuthReasonReauthRequired + cred.LastError()}
	}

	if cred.ExpiresAt().After(time.Now().Add(constant.OAuthTokenRefreshSkew)) {
		p.cacheAccess(ctx, cred)
		return accessOf(cred), nil
	}
	return p.refreshWithLock(ctx, cred)
}

// RefreshIfNeeded 临期主动刷新（cron 维护）；仅处理 active 凭据，未临期 no-op。
func (p *oauthTokenProvider) RefreshIfNeeded(ctx context.Context, endpointID uint) error {
	cred, err := p.credRepo.FindByEndpointID(ctx, endpointID)
	if err != nil {
		return err
	}
	if cred == nil || cred.Status() != enum.OAuthCredentialStatusActive {
		return nil
	}
	if cred.ExpiresAt().After(time.Now().Add(constant.OAuthTokenRefreshProactiveSkew)) {
		return nil
	}
	_, err = p.refreshWithLock(ctx, cred)
	return err
}

// refreshWithLock 持锁刷新：锁竞争时读缓存/旧 token 兜底，拿不到就快速失败（可切换）。
func (p *oauthTokenProvider) refreshWithLock(ctx context.Context, cred *aggregate.OAuthCredential) (service.OAuthAccess, error) {
	log := logger.WithCtx(ctx)
	endpointID := cred.EndpointID()
	lockKey := fmt.Sprintf(constant.OAuthRefreshLockKeyTemplate, endpointID)
	lockValue := strconv.FormatInt(time.Now().UnixNano(), constant.DecimalBase10)

	locked, lockErr := p.locker.Lock(ctx, lockKey, lockValue, constant.OAuthRefreshLockTTL)
	if lockErr != nil {
		// 锁基础设施故障 fail-open：直接刷新（可能与其他实例重复刷新，语义幂等）
		log.Warn("[OAuthTokenProvider] Refresh lock unavailable, refreshing without lock",
			zap.Uint("endpointID", endpointID), zap.Error(lockErr))
	}
	if !locked && lockErr == nil {
		// 他方正在刷新：优先用刚写好的缓存，其次用尚未真正过期的旧 token
		if access, ok := p.accessCache.Get(ctx, endpointID); ok {
			return access, nil
		}
		if cred.ExpiresAt().After(time.Now()) {
			return accessOf(cred), nil
		}
		return service.OAuthAccess{}, &model.OAuthCredentialError{Reason: constant.OAuthReasonRefreshInProgress}
	}
	if locked {
		defer func() { _ = p.locker.Unlock(ctx, lockKey, lockValue) }() //nolint:errcheck // 解锁失败仅影响锁 TTL 自然过期
	}

	// 双检：锁等待期间他方可能已完成刷新
	fresh, err := p.credRepo.FindByEndpointID(ctx, endpointID)
	if err != nil {
		return service.OAuthAccess{}, err
	}
	if fresh == nil {
		return service.OAuthAccess{}, &model.OAuthCredentialError{Reason: constant.OAuthReasonCredentialNotFound}
	}
	if fresh.ExpiresAt().After(time.Now().Add(constant.OAuthTokenRefreshSkew)) {
		p.cacheAccess(ctx, fresh)
		return accessOf(fresh), nil
	}

	client := p.clients[fresh.Provider()]
	if client == nil {
		return service.OAuthAccess{}, ierr.Newf(ierr.ErrInternal, "oauth provider client %q not wired", string(fresh.Provider()))
	}
	tokenSet, err := client.Refresh(ctx, fresh.RefreshToken())
	if err != nil {
		var protocolErr *service.OAuthProtocolError
		if errors.As(err, &protocolErr) && isReauthFailure(protocolErr) {
			fresh.MarkReauth(protocolErr.Reason)
			// 凭据不可用必须同时让缓存失效：主动刷新失败的窗口里缓存可能还有近半小时
			// 剩余 TTL，命中后会拿已失效的 token 继续转发（"凭据失效 ⇒ 缓存不可用"）
			p.accessCache.Delete(ctx, endpointID)
			if saveErr := p.credRepo.Save(ctx, fresh); saveErr != nil {
				log.Error("[OAuthTokenProvider] Persist reauth state failed", zap.Uint("endpointID", endpointID), zap.Error(saveErr))
			}
			return service.OAuthAccess{}, &model.OAuthCredentialError{Reason: constant.OAuthReasonReauthRequired + protocolErr.Reason}
		}
		// 网络/上游 5xx：保留凭据状态不变，等 cron 重试
		return service.OAuthAccess{}, &model.OAuthCredentialError{Reason: constant.OAuthReasonRefreshFailed + err.Error()}
	}

	fresh.ApplyTokens(tokenSet.AccessToken, tokenSet.RefreshToken, tokenSet.ExpiresAt)
	if err := p.credRepo.Save(ctx, fresh); err != nil {
		return service.OAuthAccess{}, err
	}
	p.cacheAccess(ctx, fresh)
	return accessOf(fresh), nil
}

// cacheAccess 按剩余有效期写缓存（失败仅降级，不阻断）
func (p *oauthTokenProvider) cacheAccess(ctx context.Context, cred *aggregate.OAuthCredential) {
	ttl := time.Until(cred.ExpiresAt()) - constant.OAuthAccessCacheGuard
	if ttl <= 0 {
		return
	}
	p.accessCache.Put(ctx, cred.EndpointID(), accessOf(cred), ttl)
}

// accessOf 聚合 → 取用值对象
func accessOf(cred *aggregate.OAuthCredential) service.OAuthAccess {
	return service.OAuthAccess{Token: cred.AccessToken(), AccountRef: cred.AccountRef(), ExpiresAt: cred.ExpiresAt()}
}

// isReauthFailure 判定上游拒绝是否属于"需重新授权"（凭据本身失效而非临时故障）
func isReauthFailure(protocolErr *service.OAuthProtocolError) bool {
	if protocolErr.StatusCode == http.StatusUnauthorized || protocolErr.StatusCode == http.StatusForbidden {
		return true
	}
	return protocolErr.Reason == constant.OAuthErrorInvalidGrant
}
