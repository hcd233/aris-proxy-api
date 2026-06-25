package pool_manager

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	mcaggregate "github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
)

// fakeAuditRepo 仅实现 Save，用于断言审计写路径确实经 AuditRepository 聚合仓储。
// 内嵌接口以满足其余只读方法（本测试不会触达）。
type fakeAuditRepo struct {
	modelcall.AuditRepository
	saved chan *mcaggregate.ModelCallAudit
}

func (f *fakeAuditRepo) Save(_ context.Context, audit *mcaggregate.ModelCallAudit) error {
	f.saved <- audit
	return nil
}

// TestPoolManager_SubmitModelCallAuditTask_WritesViaRepository 验证审计写路径已统一经
// modelcall.AuditRepository.Save（候选3：消解写/读不对称），且任务字段正确映射到聚合。
func TestPoolManager_SubmitModelCallAuditTask_WritesViaRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{saved: make(chan *mcaggregate.ModelCallAudit, 1)}
	pm := pool.NewPoolManager(nil, repo)

	if err := pm.SubmitModelCallAuditTask(&dto.ModelCallAuditTask{
		Ctx:                 context.Background(),
		ModelID:             7,
		Model:               "gpt-test",
		UpstreamProtocol:    enum.ProtocolOpenAIChatCompletion,
		APIProtocol:         enum.ProtocolOpenAIChatCompletion,
		Endpoint:            "ep-test",
		InputTokens:         11,
		OutputTokens:        22,
		FirstTokenLatencyMs: 33,
		StreamDurationMs:    44,
		UpstreamStatusCode:  200,
	}); err != nil {
		t.Fatalf("SubmitModelCallAuditTask() error: %v", err)
	}

	select {
	case audit := <-repo.saved:
		if audit.ModelID() != 7 {
			t.Errorf("ModelID = %d, want 7", audit.ModelID())
		}
		if audit.Model() != "gpt-test" {
			t.Errorf("Model = %q, want gpt-test", audit.Model())
		}
		if audit.Endpoint() != "ep-test" {
			t.Errorf("Endpoint = %q, want ep-test", audit.Endpoint())
		}
		if audit.Tokens().Input() != 11 || audit.Tokens().Output() != 22 {
			t.Errorf("tokens = (%d,%d), want (11,22)", audit.Tokens().Input(), audit.Tokens().Output())
		}
		if audit.Latency().FirstTokenMs() != 33 || audit.Latency().StreamMs() != 44 {
			t.Errorf("latency = (%d,%d), want (33,44)", audit.Latency().FirstTokenMs(), audit.Latency().StreamMs())
		}
		if audit.Status().UpstreamStatusCode() != 200 {
			t.Errorf("status code = %d, want 200", audit.Status().UpstreamStatusCode())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("audit task was not persisted via AuditRepository.Save within timeout")
	}
}
