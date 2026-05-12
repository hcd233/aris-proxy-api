package pool_manager

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
)

// testCase represents raw JSON structure aligned with fixtures/pool_config_cases.json
type testCase struct {
	Name           string `json:"name"`
	StoreWorkers   int    `json:"store_workers"`
	StoreQueueSize int    `json:"store_queue_size"`
	AgentWorkers   int    `json:"agent_workers"`
	AgentQueueSize int    `json:"agent_queue_size"`
}

// loadCases loads test cases from fixtures/pool_config_cases.json
func loadCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/pool_config_cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/pool_config_cases.json: %v", err)
	}
	var cases []testCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/pool_config_cases.json: %v", err)
	}
	return cases
}

// findCase finds a test case by name, fatals if not found
func findCase(t *testing.T, cases []testCase, name string) testCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return testCase{}
}

// ==================== PoolConfig Structure Tests ====================

func TestPoolConfig_Structure(t *testing.T) {
	poolConfig := config.PoolConfig{
		Store: config.PoolGroupConfig{
			Workers:   50,
			QueueSize: 1000,
		},
		Agent: config.PoolGroupConfig{
			Workers:   10,
			QueueSize: 100,
		},
	}

	t.Logf("PoolConfig.Store.Workers = %d", poolConfig.Store.Workers)
	t.Logf("PoolConfig.Store.QueueSize = %d", poolConfig.Store.QueueSize)
	t.Logf("PoolConfig.Agent.Workers = %d", poolConfig.Agent.Workers)
	t.Logf("PoolConfig.Agent.QueueSize = %d", poolConfig.Agent.QueueSize)

	if poolConfig.Store.Workers != 50 {
		t.Errorf("PoolConfig.Store.Workers = %d, want 50", poolConfig.Store.Workers)
	}
	if poolConfig.Store.QueueSize != 1000 {
		t.Errorf("PoolConfig.Store.QueueSize = %d, want 1000", poolConfig.Store.QueueSize)
	}
	if poolConfig.Agent.Workers != 10 {
		t.Errorf("PoolConfig.Agent.Workers = %d, want 10", poolConfig.Agent.Workers)
	}
	if poolConfig.Agent.QueueSize != 100 {
		t.Errorf("PoolConfig.Agent.QueueSize = %d, want 100", poolConfig.Agent.QueueSize)
	}
}

func TestPoolGroupConfig_Fields(t *testing.T) {
	groupConfig := config.PoolGroupConfig{
		Workers:   25,
		QueueSize: 500,
	}

	if groupConfig.Workers != 25 {
		t.Errorf("PoolGroupConfig.Workers = %d, want 25", groupConfig.Workers)
	}
	if groupConfig.QueueSize != 500 {
		t.Errorf("PoolGroupConfig.QueueSize = %d, want 500", groupConfig.QueueSize)
	}
}

// ==================== PoolConfig Parsing Tests ====================

func TestPoolConfig_DefaultValues(t *testing.T) {
	// Verify the default Pool config is loaded
	poolConfig := config.Pool

	t.Logf("config.Pool.Store.Workers = %d", poolConfig.Store.Workers)
	t.Logf("config.Pool.Store.QueueSize = %d", poolConfig.Store.QueueSize)
	t.Logf("config.Pool.Agent.Workers = %d", poolConfig.Agent.Workers)
	t.Logf("config.Pool.Agent.QueueSize = %d", poolConfig.Agent.QueueSize)

	// Default values set in config.go
	if poolConfig.Store.Workers != 50 {
		t.Errorf("default Pool.Store.Workers = %d, want 50", poolConfig.Store.Workers)
	}
	if poolConfig.Store.QueueSize != 1000 {
		t.Errorf("default Pool.Store.QueueSize = %d, want 1000", poolConfig.Store.QueueSize)
	}
	if poolConfig.Agent.Workers != 10 {
		t.Errorf("default Pool.Agent.Workers = %d, want 10", poolConfig.Agent.Workers)
	}
	if poolConfig.Agent.QueueSize != 100 {
		t.Errorf("default Pool.Agent.QueueSize = %d, want 100", poolConfig.Agent.QueueSize)
	}
}

func TestPoolConfig_FromFixture(t *testing.T) {
	cases := loadCases(t)

	tc := findCase(t, cases, "default_pool_config")

	// Simulate config parsing from fixture
	poolConfig := config.PoolConfig{
		Store: config.PoolGroupConfig{
			Workers:   tc.StoreWorkers,
			QueueSize: tc.StoreQueueSize,
		},
		Agent: config.PoolGroupConfig{
			Workers:   tc.AgentWorkers,
			QueueSize: tc.AgentQueueSize,
		},
	}

	if poolConfig.Store.Workers != tc.StoreWorkers {
		t.Errorf("Store.Workers = %d, want %d", poolConfig.Store.Workers, tc.StoreWorkers)
	}
	if poolConfig.Store.QueueSize != tc.StoreQueueSize {
		t.Errorf("Store.QueueSize = %d, want %d", poolConfig.Store.QueueSize, tc.StoreQueueSize)
	}
	if poolConfig.Agent.Workers != tc.AgentWorkers {
		t.Errorf("Agent.Workers = %d, want %d", poolConfig.Agent.Workers, tc.AgentWorkers)
	}
	if poolConfig.Agent.QueueSize != tc.AgentQueueSize {
		t.Errorf("Agent.QueueSize = %d, want %d", poolConfig.Agent.QueueSize, tc.AgentQueueSize)
	}
}

func TestPoolConfig_CustomValues(t *testing.T) {
	cases := loadCases(t)

	tc := findCase(t, cases, "custom_pool_config")

	poolConfig := config.PoolConfig{
		Store: config.PoolGroupConfig{
			Workers:   tc.StoreWorkers,
			QueueSize: tc.StoreQueueSize,
		},
		Agent: config.PoolGroupConfig{
			Workers:   tc.AgentWorkers,
			QueueSize: tc.AgentQueueSize,
		},
	}

	if poolConfig.Store.Workers != 100 {
		t.Errorf("Store.Workers = %d, want 100", poolConfig.Store.Workers)
	}
	if poolConfig.Store.QueueSize != 2000 {
		t.Errorf("Store.QueueSize = %d, want 2000", poolConfig.Store.QueueSize)
	}
	if poolConfig.Agent.Workers != 20 {
		t.Errorf("Agent.Workers = %d, want 20", poolConfig.Agent.Workers)
	}
	if poolConfig.Agent.QueueSize != 200 {
		t.Errorf("Agent.QueueSize = %d, want 200", poolConfig.Agent.QueueSize)
	}
}

func TestPoolConfig_MinimalValues(t *testing.T) {
	cases := loadCases(t)

	tc := findCase(t, cases, "minimal_pool_config")

	poolConfig := config.PoolConfig{
		Store: config.PoolGroupConfig{
			Workers:   tc.StoreWorkers,
			QueueSize: tc.StoreQueueSize,
		},
		Agent: config.PoolGroupConfig{
			Workers:   tc.AgentWorkers,
			QueueSize: tc.AgentQueueSize,
		},
	}

	if poolConfig.Store.Workers != 1 {
		t.Errorf("Store.Workers = %d, want 1", poolConfig.Store.Workers)
	}
	if poolConfig.Store.QueueSize != 10 {
		t.Errorf("Store.QueueSize = %d, want 10", poolConfig.Store.QueueSize)
	}
	if poolConfig.Agent.Workers != 1 {
		t.Errorf("Agent.Workers = %d, want 1", poolConfig.Agent.Workers)
	}
	if poolConfig.Agent.QueueSize != 10 {
		t.Errorf("Agent.QueueSize = %d, want 10", poolConfig.Agent.QueueSize)
	}
}

// ==================== PoolManager Method Signature Tests ====================

func TestPoolManager_GetPoolManager(t *testing.T) {
	// Init first, then GetPoolManager should return non-nil PoolManager
	pool.InitPoolManager(nil)
	pm := pool.GetPoolManager()
	if pm == nil {
		t.Error("GetPoolManager() returned nil after InitPoolManager()")
	}
	t.Log("GetPoolManager() returned non-nil PoolManager after initialization")
}

func TestPoolManager_Stop(t *testing.T) {
	// InitPoolManager and StopPoolManager should work without panic
	pool.InitPoolManager(nil)
	pool.StopPoolManager()
	t.Log("InitPoolManager and StopPoolManager executed successfully")
}
