package cron_test

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type mockCron struct {
	started       bool
	stopped       bool
	triggerResult bool
}

func (m *mockCron) Trigger() bool {
	return m.triggerResult
}

func (m *mockCron) Start(_ string) error {
	m.started = true
	return nil
}

func (m *mockCron) Stop() {
	m.stopped = true
}

func (m *mockCron) StopGracefully() {
	m.stopped = true
}

// TestInitCronJobs_AllDisabled 验证所有任务关闭时不会注册任何定时任务
func TestInitCronJobs_AllDisabled(t *testing.T) { //nolint:paralleltest // cron tests share global state
	origDedup := config.CronSessionTerminalCleanupEnabled
	origPurge := config.CronSoftDeletePurgeEnabled
	origThink := config.CronThinkExtractEnabled
	origRegistry := cron.DefaultCronRegistry
	defer func() {
		config.CronSessionTerminalCleanupEnabled = origDedup
		config.CronSoftDeletePurgeEnabled = origPurge
		config.CronThinkExtractEnabled = origThink
		cron.DefaultCronRegistry = origRegistry
	}()

	config.CronSessionTerminalCleanupEnabled = false
	config.CronSoftDeletePurgeEnabled = false
	config.CronThinkExtractEnabled = false

	cron.DefaultCronRegistry = []cron.CronRegistryEntry{
		{
			Name:    "SessionDeduplicate",
			Enabled: func() bool { return config.CronSessionTerminalCleanupEnabled },
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

	crons := cron.InitCronJobs(context.TODO(), nil, nil, nil, nil, nil, nil, nil)

	if CronInstanceCount(crons) != 0 {
		t.Fatalf("expected 0 cron instances when all disabled, got %d", CronInstanceCount(crons))
	}
}

// TestInitCronJobs_PartialEnabled 验证部分开启时只注册启用的任务
func TestInitCronJobs_PartialEnabled(t *testing.T) { //nolint:paralleltest // cron tests share global state
	origDedup := config.CronSessionTerminalCleanupEnabled
	origPurge := config.CronSoftDeletePurgeEnabled
	origThink := config.CronThinkExtractEnabled
	origRegistry := cron.DefaultCronRegistry
	defer func() {
		config.CronSessionTerminalCleanupEnabled = origDedup
		config.CronSoftDeletePurgeEnabled = origPurge
		config.CronThinkExtractEnabled = origThink
		cron.DefaultCronRegistry = origRegistry
	}()

	config.CronSessionTerminalCleanupEnabled = true
	config.CronSoftDeletePurgeEnabled = false
	config.CronThinkExtractEnabled = false

	cron.DefaultCronRegistry = []cron.CronRegistryEntry{
		{
			Name:    "SessionDeduplicate",
			Enabled: func() bool { return config.CronSessionTerminalCleanupEnabled },
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

	crons := cron.InitCronJobs(context.TODO(), nil, nil, nil, nil, nil, nil, nil)

	if CronInstanceCount(crons) != 1 {
		t.Fatalf("expected 1 cron instance, got %d", CronInstanceCount(crons))
	}
}

// TestInitCronJobs_AllEnabled 验证全部开启时注册所有任务
func TestInitCronJobs_AllEnabled(t *testing.T) { //nolint:paralleltest // cron tests share global state
	origDedup := config.CronSessionTerminalCleanupEnabled
	origPurge := config.CronSoftDeletePurgeEnabled
	origThink := config.CronThinkExtractEnabled
	origRegistry := cron.DefaultCronRegistry
	defer func() {
		config.CronSessionTerminalCleanupEnabled = origDedup
		config.CronSoftDeletePurgeEnabled = origPurge
		config.CronThinkExtractEnabled = origThink
		cron.DefaultCronRegistry = origRegistry
	}()

	config.CronSessionTerminalCleanupEnabled = true
	config.CronSoftDeletePurgeEnabled = true
	config.CronThinkExtractEnabled = true

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

	crons := cron.InitCronJobs(context.TODO(), nil, nil, nil, nil, nil, nil, nil)

	if CronInstanceCount(crons) != 1 {
		t.Fatalf("expected 1 cron instance, got %d", CronInstanceCount(crons))
	}
	if !mock.started {
		t.Fatal("expected mock cron to be started")
	}

	for _, c := range crons {
		c.Stop()
	}

	if !mock.stopped {
		t.Fatal("expected mock cron to be stopped")
	}
}

// TestStopCronJobs_Empty 验证空实例列表下停止不会 panic
func TestStopCronJobs_Empty(t *testing.T) { //nolint:paralleltest // cron tests share global state
	StopCronJobsWithContext(context.Background(), nil)
}

// StopCronJobsWithContext 停止全部定时任务（测试本地辅助，替代原生产包导出）。
func StopCronJobsWithContext(ctx context.Context, crons []cron.Cron) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, c := range crons {
			c.Stop()
		}
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CronInstanceCount 统计定时任务实例数（测试本地辅助，替代原生产包导出）。
func CronInstanceCount(crons []cron.Cron) int {
	return len(crons)
}

// TestCronManager_Trigger_NotFound 验证触发未注册任务返回 ErrDataNotExists
func TestCronManager_Trigger_NotFound(t *testing.T) {
	t.Parallel()
	m := cron.NewCronManager(cron.CronDeps{})
	err := m.Trigger("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown cron job")
	}
	bizErr := ierr.ToBizError(err, ierr.ErrInternal.BizError())
	if bizErr.Code != ierr.ErrDataNotExists.BizError().Code {
		t.Fatalf("expected ErrDataNotExists code, got %d", bizErr.Code)
	}
}

// TestCronManager_Trigger_Success 验证触发已注册任务成功
func TestCronManager_Trigger_Success(t *testing.T) {
	t.Parallel()
	m := cron.NewCronManager(cron.CronDeps{})
	mock := &mockCron{triggerResult: true}
	m.Register("TestCron", mock, "0 * * * *", nil)
	if err := m.Trigger("TestCron"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCronManager_Trigger_Locked 验证任务正在执行（拿不到锁）返回 ErrResourceLocked
func TestCronManager_Trigger_Locked(t *testing.T) {
	t.Parallel()
	m := cron.NewCronManager(cron.CronDeps{})
	mock := &mockCron{triggerResult: false}
	m.Register("TestCron", mock, "0 * * * *", nil)
	err := m.Trigger("TestCron")
	if err == nil {
		t.Fatal("expected error when lock cannot be acquired")
	}
	bizErr := ierr.ToBizError(err, ierr.ErrInternal.BizError())
	if bizErr.Code != ierr.ErrResourceLocked.BizError().Code {
		t.Fatalf("expected ErrResourceLocked code, got %d", bizErr.Code)
	}
}
