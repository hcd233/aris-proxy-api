// Package model defines the database schema for the model.
//
//	update 2026-04-09 10:00:00
package model

// ModelCallAudit 模型调用审计数据库模型
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type ModelCallAudit struct {
	BaseModel
	APIKeyID                  uint   `json:"api_key_id" gorm:"column:api_key_id;not null;index:idx_api_key_id_created_at"`
	ModelID                   uint   `json:"model_id" gorm:"column:model_id;not null;index:idx_model_id_created_at"`
	Model                     string `json:"model" gorm:"column:model;not null;index:idx_model_created_at"`
	UpstreamProvider          string `json:"upstream_provider" gorm:"column:upstream_provider;not null"`
	APIProvider               string `json:"api_provider" gorm:"column:api_provider;not null"`
	InputTokens               int    `json:"input_tokens" gorm:"column:input_tokens;default:0"`
	OutputTokens              int    `json:"output_tokens" gorm:"column:output_tokens;default:0"`
	CacheCreationInputTokens  int    `json:"cache_creation_input_tokens" gorm:"column:cache_creation_input_tokens;default:0"`
	CacheReadInputTokens      int    `json:"cache_read_input_tokens" gorm:"column:cache_read_input_tokens;default:0"`
	FirstTokenLatencyMs      int64  `json:"first_token_latency_ms" gorm:"column:first_token_latency_ms;default:0"`
	StreamDurationMs          int64  `json:"stream_duration_ms" gorm:"column:stream_duration_ms;default:0"`
	UserAgent                string `json:"user_agent" gorm:"column:user_agent;not null;default:''"`
	UpstreamStatusCode       int    `json:"upstream_status_code" gorm:"column:upstream_status_code;default:0"`
	ErrorMessage             string `json:"error_message" gorm:"column:error_message;not null;default:''"`
	TraceID                  string `json:"trace_id" gorm:"column:trace_id;not null;default:'';index"`
}
