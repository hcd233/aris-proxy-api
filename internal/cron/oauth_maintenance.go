package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	commonmodel "github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
)

// oauthMaintenanceCron 订阅账号维护任务：临期 token 主动刷新 + 额度探测（30 分钟节流）。
type oauthMaintenanceCron struct {
	cron      *cron.Cron
	credRepo  llmproxy.OAuthCredentialRepository
	tokens    service.OAuthTokenProvider
	providers map[enum.OAuthProvider]service.OAuthProviderClient
	locker    *lock.RedisLocker
	lockKey   string
}

// NewOAuthMaintenanceCron 构造订阅账号维护任务
func NewOAuthMaintenanceCron(
	credRepo llmproxy.OAuthCredentialRepository,
	tokens service.OAuthTokenProvider,
	providers map[enum.OAuthProvider]service.OAuthProviderClient,
	redisClient *redis.Client,
) Cron {
	return &oauthMaintenanceCron{
		cron:      cron.New(cron.WithLogger(newCronLoggerAdapter(constant.CronModuleOAuthMaintenance))),
		credRepo:  credRepo,
		tokens:    tokens,
		providers: providers,
		locker:    lock.NewLocker(redisClient),
	}
}

// Start 启动调度
func (c *oauthMaintenanceCron) Start(spec string) error {
	c.lockKey = fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleOAuthMaintenance)
	_, err := c.cron.AddFunc(spec, wrapCronFunc(constant.CronModuleOAuthMaintenance, c.locker, c.lockKey, LockOptions{}, c.sweep, constant.CronTriggerSourceScheduled))
	if err != nil {
		return err
	}
	c.cron.Start()
	return nil
}

// Trigger 手动触发一次维护
func (c *oauthMaintenanceCron) Trigger() bool {
	return TriggerWithLock(constant.CronModuleOAuthMaintenance, c.locker, c.lockKey, LockOptions{}, c.sweep)
}

// Stop 停止调度并等待运行中任务完成
func (c *oauthMaintenanceCron) Stop() {
	<-c.cron.Stop().Done()
}

// StopGracefully 仅停止调度，不等待运行中任务完成
func (c *oauthMaintenanceCron) StopGracefully() {
	c.cron.Stop()
}

// sweep 执行一次维护（转调可测的包级函数）
func (c *oauthMaintenanceCron) sweep(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error) {
	if err := SweepOAuthMaintenance(ctx, c.credRepo, c.tokens, c.providers); err != nil {
		return nil, err
	}
	return nil, nil
}

// SweepOAuthMaintenance 遍历 active 凭据：临期刷新 + 到期额度探测。
//
// 单账号失败只记日志不中断循环（一个账号的上游故障不应阻塞其余账号维护）；
// 探测按 constant.OAuthQuotaProbeInterval 节流，避免频繁打上游用量接口。
func SweepOAuthMaintenance(
	ctx context.Context,
	credRepo llmproxy.OAuthCredentialRepository,
	tokens service.OAuthTokenProvider,
	providers map[enum.OAuthProvider]service.OAuthProviderClient,
) error {
	log := logger.WithCtx(ctx)

	creds, err := credRepo.ListActive(ctx)
	if err != nil {
		return err
	}
	for _, cred := range creds {
		// 1. 临期主动刷新（token provider 内含提前量与分布式锁）
		if refreshErr := tokens.RefreshIfNeeded(ctx, cred.EndpointID()); refreshErr != nil {
			log.Warn("[OAuthMaintenance] Refresh expiring token failed",
				zap.Uint("endpointID", cred.EndpointID()), zap.Error(refreshErr))
		}

		// 2. 额度探测（节流窗口内跳过）
		if time.Since(cred.QuotaUpdatedAt()) < constant.OAuthQuotaProbeInterval {
			continue
		}
		probeOAuthQuota(ctx, cred, tokens, providers, credRepo)
	}
	return nil
}

// probeOAuthQuota 探测单个凭据的订阅窗口用量并写回快照（失败 fail-soft）
func probeOAuthQuota(
	ctx context.Context,
	cred *aggregate.OAuthCredential,
	tokens service.OAuthTokenProvider,
	providers map[enum.OAuthProvider]service.OAuthProviderClient,
	credRepo llmproxy.OAuthCredentialRepository,
) {
	log := logger.WithCtx(ctx)

	client := providers[cred.Provider()]
	if client == nil {
		return
	}
	access, err := tokens.GetAccessToken(ctx, cred.EndpointID())
	if err != nil {
		log.Warn("[OAuthMaintenance] Get access token for quota probe failed",
			zap.Uint("endpointID", cred.EndpointID()), zap.Error(err))
		persistProbeError(ctx, cred, err, credRepo)
		return
	}
	snapshot, err := client.ProbeQuota(ctx, access.Token, access.AccountRef)
	if err != nil {
		log.Warn("[OAuthMaintenance] Quota probe failed",
			zap.Uint("endpointID", cred.EndpointID()), zap.Error(err))
		persistProbeError(ctx, cred, err, credRepo)
		return
	}
	cred.SetQuota(*snapshot, time.Now())
	if err := credRepo.Save(ctx, cred); err != nil {
		log.Error("[OAuthMaintenance] Persist quota snapshot failed",
			zap.Uint("endpointID", cred.EndpointID()), zap.Error(err))
	}
}

// persistProbeError 记录额度探测失败原因（不改状态机）：写 last_error 供管理端展示，
// 否则用户无法察觉"额度为什么一直不刷新"（评审 M1）。
func persistProbeError(ctx context.Context, cred *aggregate.OAuthCredential, probeErr error, credRepo llmproxy.OAuthCredentialRepository) {
	cred.SetLastError(fmt.Sprintf(constant.OAuthReasonQuotaProbeFailed+"%v", probeErr))
	if err := credRepo.Save(ctx, cred); err != nil {
		logger.WithCtx(ctx).Error("[OAuthMaintenance] Persist probe error failed",
			zap.Uint("endpointID", cred.EndpointID()), zap.Error(err))
	}
}
