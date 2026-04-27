// Package modelcall ModelCall 域根（仓储接口）
//
// TODO: 此域尚未被 use case 层接入。LLM 代理当前通过 pool.SubmitModelCallAuditTask() 直接写入审计记录。
// 计划在后续迭代中将审计写入迁移至 aggregate + repository 模式，届时本包将被激活。
package modelcall

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
)

// AuditRepository ModelCallAudit 聚合仓储接口
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type AuditRepository interface {
	// Save 持久化审计聚合（首次 Save 后回填 ID）
	Save(ctx context.Context, audit *aggregate.ModelCallAudit) error
}
