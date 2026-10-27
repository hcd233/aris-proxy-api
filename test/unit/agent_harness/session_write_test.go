package agent_harness

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
)

func TestBuildSessionRowCapturesClientUA(t *testing.T) {
	t.Parallel()

	const ua = "ZCode/3.15.3 ai-sdk/provider-utils/4.0.39 runtime/node.js/24"
	ctx := context.WithValue(context.Background(), constant.CtxKeyClient, ua)
	task := &dto.MessageStoreTask{Ctx: ctx, APIKeyID: 7, APIKeyName: "main"}

	row := pool.BuildSessionRow(task, pool.SessionRowColumns{MessageIDs: []uint{1, 2}})

	if row.ClientUA != ua {
		t.Errorf("ClientUA = %q, want %q", row.ClientUA, ua)
	}
	if row.APIKeyID != 7 {
		t.Errorf("APIKeyID = %d, want 7", row.APIKeyID)
	}
}

func TestBuildSessionRowWithoutClientUAYieldsEmpty(t *testing.T) {
	t.Parallel()

	row := pool.BuildSessionRow(&dto.MessageStoreTask{Ctx: context.Background()}, pool.SessionRowColumns{})

	if row.ClientUA != "" {
		t.Errorf("ClientUA = %q, want empty", row.ClientUA)
	}
}
