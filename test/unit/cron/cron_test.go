package cron_test

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type mockCron struct {
	started bool
	stopped bool
}

func (m *mockCron) Start() error {
	m.started = true
	return nil
}

func (m *mockCron) Stop() {
	m.stopped = true
}

// TestInitCronJobs_AllDisabled 验证所有任务关闭时不会注册任何定时任务
func TestInitCronJobs_AllDisabled(t *testing.T) { //nolint:paralleltest // cron tests share global state
	origDedup := config.CronSessionDeduplicateEnabled
	origPurge := config.CronSoftDeletePurgeEnabled
	origThink := config.CronThinkExtractEnabled
	defer func() {
		config.CronSessionDeduplicateEnabled = origDedup
		config.CronSoftDeletePurgeEnabled = origPurge
		config.CronThinkExtractEnabled = origThink
	}()

	config.CronSessionDeduplicateEnabled = false
	config.CronSoftDeletePurgeEnabled = false
	config.CronThinkExtractEnabled = false

	cron.StopCronJobsWithContext(context.Background(), nil)
	crons := cron.InitCronJobs(context.TODO(), nil, nil, nil, nil)

	if cron.CronInstanceCount(crons) != 0 {
		t.Fatalf("expected 0 cron instances when all disabled, got %d", cron.CronInstanceCount(crons))
	}
}

// TestInitCronJobs_PartialEnabled 验证部分开启时只注册启用的任务
func TestInitCronJobs_PartialEnabled(t *testing.T) { //nolint:paralleltest // cron tests share global state
	origDedup := config.CronSessionDeduplicateEnabled
	origPurge := config.CronSoftDeletePurgeEnabled
	origThink := config.CronThinkExtractEnabled
	origRegistry := cron.DefaultCronRegistry
	defer func() {
		config.CronSessionDeduplicateEnabled = origDedup
		config.CronSoftDeletePurgeEnabled = origPurge
		config.CronThinkExtractEnabled = origThink
		cron.DefaultCronRegistry = origRegistry
	}()

	config.CronSessionDeduplicateEnabled = true
	config.CronSoftDeletePurgeEnabled = false
	config.CronThinkExtractEnabled = false

	cron.DefaultCronRegistry = []cron.CronRegistryEntry{
		{
			Name:    "SessionDeduplicate",
			Enabled: func() bool { return config.CronSessionDeduplicateEnabled },
			Factory: func(_ *gorm.DB, _ *pool.PoolManager, _ *redis.Client, _ conversation.ThinkExtractRepository) cron.Cron {
				return &mockCron{}
			},
		},
		{
			Name:    "SoftDeletePurge",
			Enabled: func() bool { return config.CronSoftDeletePurgeEnabled },
			Factory: func(_ *gorm.DB, _ *pool.PoolManager, _ *redis.Client, _ conversation.ThinkExtractRepository) cron.Cron {
				return &mockCron{}
			},
		},
		{
			Name:    "ThinkExtract",
			Enabled: func() bool { return config.CronThinkExtractEnabled },
			Factory: func(_ *gorm.DB, _ *pool.PoolManager, _ *redis.Client, _ conversation.ThinkExtractRepository) cron.Cron {
				return &mockCron{}
			},
		},
	}

	crons := cron.InitCronJobs(context.TODO(), nil, nil, nil, nil)

	if cron.CronInstanceCount(crons) != 1 {
		t.Fatalf("expected 1 cron instance, got %d", cron.CronInstanceCount(crons))
	}
}

// TestInitCronJobs_AllEnabled 验证全部开启时注册所有任务
func TestInitCronJobs_AllEnabled(t *testing.T) { //nolint:paralleltest // cron tests share global state
	origDedup := config.CronSessionDeduplicateEnabled
	origPurge := config.CronSoftDeletePurgeEnabled
	origThink := config.CronThinkExtractEnabled
	origRegistry := cron.DefaultCronRegistry
	defer func() {
		config.CronSessionDeduplicateEnabled = origDedup
		config.CronSoftDeletePurgeEnabled = origPurge
		config.CronThinkExtractEnabled = origThink
		cron.DefaultCronRegistry = origRegistry
	}()

	config.CronSessionDeduplicateEnabled = true
	config.CronSoftDeletePurgeEnabled = true
	config.CronThinkExtractEnabled = true

	cron.StopCronJobsWithContext(context.Background(), nil)

	mock := &mockCron{}
	cron.DefaultCronRegistry = []cron.CronRegistryEntry{
		{
			Name:    "TestCron",
			Enabled: func() bool { return true },
			Factory: func(_ *gorm.DB, _ *pool.PoolManager, _ *redis.Client, _ conversation.ThinkExtractRepository) cron.Cron {
				return mock
			},
		},
	}

	crons := cron.InitCronJobs(context.TODO(), nil, nil, nil, nil)

	if cron.CronInstanceCount(crons) != 1 {
		t.Fatalf("expected 1 cron instance, got %d", cron.CronInstanceCount(crons))
	}
	if !mock.started {
		t.Fatal("expected mock cron to be started")
	}

	cron.StopCronJobsWithContext(context.Background(), crons)

	if !mock.stopped {
		t.Fatal("expected mock cron to be stopped")
	}
}

// TestStopCronJobs_Empty 验证空实例列表下停止不会 panic
func TestStopCronJobs_Empty(t *testing.T) { //nolint:paralleltest // cron tests share global state
	cron.StopCronJobsWithContext(context.Background(), nil)
}
