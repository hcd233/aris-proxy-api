package session_owner_id

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// TestMessageStoreTaskCarriesAPIKeyID 存储任务必须携带 APIKeyID。
//
// 归属落库的唯一来源是中间件注入的 CtxKeyAPIKeyID；任务结构体漏该字段
// 会让 sessions.api_key_id 恒为 0，新会话对普通用户不可见。
func TestMessageStoreTaskCarriesAPIKeyID(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), constant.CtxKeyAPIKeyID, uint(42))

	task := &dto.MessageStoreTask{
		Ctx:        ctx,
		APIKeyName: "my-key",
		APIKeyID:   util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID),
	}

	if task.APIKeyID != 42 {
		t.Errorf("APIKeyID = %d, want 42", task.APIKeyID)
	}
}

// TestCopyContextValuesPreservesAPIKeyID 异步任务的 ctx 复制必须保留 api key id。
//
// 存储走协程池，ctx 经 util.CopyContextValues 脱离请求生命周期；
// 若该键未被复制，异步落库时取到 0。
func TestCopyContextValuesPreservesAPIKeyID(t *testing.T) {
	t.Parallel()
	src := context.WithValue(context.Background(), constant.CtxKeyAPIKeyID, uint(7))
	dst := util.CopyContextValues(src)

	if got := util.CtxValueUint(dst, constant.CtxKeyAPIKeyID); got != 7 {
		t.Errorf("copied ctx api key id = %d, want 7", got)
	}
}
