package model

// ModelCallAudit 模型调用审计数据库模型
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type ModelCallAudit struct {
	BaseModel
	APIKeyID                 uint   `json:"api_key_id" gorm:"column:api_key_id;not null;comment:API密钥ID;index:idx_api_key_id_created_at,priority:1"`
	ModelID                  uint   `json:"model_id" gorm:"column:model_id;not null;comment:模型端点ID;index:idx_model_id_created_at,priority:1"`
	Model                    string `json:"model" gorm:"column:model;not null;default:'';comment:对外暴露的模型别名;index:idx_model_created_at,priority:1"`
	UpstreamProvider         string `json:"upstream_provider" gorm:"column:upstream_provider;not null;default:'';comment:上游提供商(openai/anthropic)"`
	APIProvider              string `json:"api_provider" gorm:"column:api_provider;not null;default:'';comment:接口层协议(openai/anthropic)"`
	InputTokens              int    `json:"input_tokens" gorm:"column:input_tokens;not null;default:0;comment:输入token数"`
	OutputTokens             int    `json:"output_tokens" gorm:"column:output_tokens;not null;default:0;comment:输出token数"`
	CacheCreationInputTokens int    `json:"cache_creation_input_tokens" gorm:"column:cache_creation_input_tokens;not null;default:0;comment:缓存写入token数"`
	CacheReadInputTokens     int    `json:"cache_read_input_tokens" gorm:"column:cache_read_input_tokens;not null;default:0;comment:缓存命中token数"`
	FirstTokenLatencyMs      int64  `json:"first_token_latency_ms" gorm:"column:first_token_latency_ms;not null;default:0;comment:首token延迟(ms)，非流式为总延迟"`
	StreamDurationMs         int64  `json:"stream_duration_ms" gorm:"column:stream_duration_ms;not null;default:0;comment:流式传输持续时间(ms)，非流式为0"`
	UserAgent                string `json:"user_agent" gorm:"column:user_agent;not null;default:'';comment:请求客户端User-Agent"`
	UpstreamStatusCode       int    `json:"upstream_status_code" gorm:"column:upstream_status_code;not null;default:0;comment:上游HTTP状态码，成功为200，-1表示连接错误"`
	ErrorMessage             string `json:"error_message" gorm:"column:error_message;not null;default:'';comment:错误信息，成功时为空"`
	TraceID                  string `json:"trace_id" gorm:"column:trace_id;not null;default:'';comment:请求追踪ID;index"`
}
