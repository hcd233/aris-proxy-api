package aggregate

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/vo"
)

// ReconstructAuditInput 从持久层重建聚合的输入参数
//
//	@author centonhuang
//	@update 2026-06-21 10:00:00
type ReconstructAuditInput struct {
	APIKeyID              uint
	ModelID               uint
	Model                 string
	UpstreamProtocol      enum.ProtocolType
	APIProtocol           enum.ProtocolType
	Endpoint              string
	Tokens                vo.TokenBreakdown
	Latency               vo.CallLatency
	Status                vo.CallStatus
	UserAgent             string
	TraceID               string
	CompressionEnabled    bool
	CompressedTokens      int
	CompressionStrategies []string
	CreatedAt             time.Time
}

// ReconstructAudit 从持久层数据重建审计聚合（用于读查询）
//
//	@param input ReconstructAuditInput
//	@return *ModelCallAudit
//	@author centonhuang
//	@update 2026-05-11 10:00:00
func ReconstructAudit(input ReconstructAuditInput) *ModelCallAudit {
	return &ModelCallAudit{
		apiKeyID:              input.APIKeyID,
		modelID:               input.ModelID,
		model:                 input.Model,
		upstreamProtocol:      input.UpstreamProtocol,
		apiProtocol:           input.APIProtocol,
		endpoint:              input.Endpoint,
		tokens:                input.Tokens,
		latency:               input.Latency,
		status:                input.Status,
		userAgent:             input.UserAgent,
		traceID:               input.TraceID,
		compressionEnabled:    input.CompressionEnabled,
		compressedTokens:      input.CompressedTokens,
		compressionStrategies: input.CompressionStrategies,
		createdAt:             input.CreatedAt,
	}
}
