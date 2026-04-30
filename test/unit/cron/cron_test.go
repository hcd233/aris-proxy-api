package cron_test

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/cron"
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
func TestInitCronJobs_AllDisabled(t *testing.T) {
	origDedup := config.CronSessionDeduplicateEnabled
	origSum := config.CronSessionSummarizeEnabled
	origScore := config.CronSessionScoreEnabled
	origPurge := config.CronSoftDeletePurgeEnabled
	defer func() {
		config.CronSessionDeduplicateEnabled = origDedup
		config.CronSessionSummarizeEnabled = origSum
		config.CronSessionScoreEnabled = origScore
		config.CronSoftDeletePurgeEnabled = origPurge
	}()

	config.CronSessionDeduplicateEnabled = false
	config.CronSessionSummarizeEnabled = false
	config.CronSessionScoreEnabled = false
	config.CronSoftDeletePurgeEnabled = false

	cron.StopCronJobs()
	cron.InitCronJobs()

	if cron.CronInstanceCount() != 0 {
		t.Fatalf("expected 0 cron instances when all disabled, got %d", cron.CronInstanceCount())
	}
}

// TestInitCronJobs_PartialEnabled 验证部分开启时只注册启用的任务
func TestInitCronJobs_PartialEnabled(t *testing.T) {
	origDedup := config.CronSessionDeduplicateEnabled
	origSum := config.CronSessionSummarizeEnabled
	origScore := config.CronSessionScoreEnabled
	origPurge := config.CronSoftDeletePurgeEnabled
	origRegistry := cron.DefaultCronRegistry
	defer func() {
		config.CronSessionDeduplicateEnabled = origDedup
		config.CronSessionSummarizeEnabled = origSum
		config.CronSessionScoreEnabled = origScore
		config.CronSoftDeletePurgeEnabled = origPurge
		cron.DefaultCronRegistry = origRegistry
	}()

	config.CronSessionDeduplicateEnabled = true
	config.CronSessionSummarizeEnabled = false
	config.CronSessionScoreEnabled = true
	config.CronSoftDeletePurgeEnabled = false

	cron.StopCronJobs()

	cron.DefaultCronRegistry = []cron.CronRegistryEntry{
		{
			Name:    "SessionDeduplicate",
			Enabled: func() bool { return config.CronSessionDeduplicateEnabled },
			Factory: func() cron.Cron { return &mockCron{} },
		},
		{
			Name:    "SessionSummarize",
			Enabled: func() bool { return config.CronSessionSummarizeEnabled },
			Factory: func() cron.Cron { return &mockCron{} },
		},
		{
			Name:    "SessionScore",
			Enabled: func() bool { return config.CronSessionScoreEnabled },
			Factory: func() cron.Cron { return &mockCron{} },
		},
		{
			Name:    "SoftDeletePurge",
			Enabled: func() bool { return config.CronSoftDeletePurgeEnabled },
			Factory: func() cron.Cron { return &mockCron{} },
		},
	}

	cron.InitCronJobs()

	if cron.CronInstanceCount() != 2 {
		t.Fatalf("expected 2 cron instances, got %d", cron.CronInstanceCount())
	}
}

// TestInitCronJobs_AllEnabled 验证全部开启时注册所有任务
func TestInitCronJobs_AllEnabled(t *testing.T) {
	origDedup := config.CronSessionDeduplicateEnabled
	origSum := config.CronSessionSummarizeEnabled
	origScore := config.CronSessionScoreEnabled
	origPurge := config.CronSoftDeletePurgeEnabled
	origRegistry := cron.DefaultCronRegistry
	defer func() {
		config.CronSessionDeduplicateEnabled = origDedup
		config.CronSessionSummarizeEnabled = origSum
		config.CronSessionScoreEnabled = origScore
		config.CronSoftDeletePurgeEnabled = origPurge
		cron.DefaultCronRegistry = origRegistry
	}()

	config.CronSessionDeduplicateEnabled = true
	config.CronSessionSummarizeEnabled = true
	config.CronSessionScoreEnabled = true
	config.CronSoftDeletePurgeEnabled = true

	cron.StopCronJobs()

	mock := &mockCron{}
	cron.DefaultCronRegistry = []cron.CronRegistryEntry{
		{
			Name:    "TestCron",
			Enabled: func() bool { return true },
			Factory: func() cron.Cron { return mock },
		},
	}

	cron.InitCronJobs()

	if cron.CronInstanceCount() != 1 {
		t.Fatalf("expected 1 cron instance, got %d", cron.CronInstanceCount())
	}
	if !mock.started {
		t.Fatal("expected mock cron to be started")
	}

	cron.StopCronJobs()

	if !mock.stopped {
		t.Fatal("expected mock cron to be stopped")
	}
}

// TestStopCronJobs_Empty 验证空实例列表下停止不会 panic
func TestStopCronJobs_Empty(t *testing.T) {
	cron.StopCronJobs()
}
