// Package modelcall ModelCall 域根（仓储接口）
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
