package model

import "time"

// ModelCallAudit 模型调用审计数据库模型
//
//	@author centonhuang
//	@update 2026-06-21 10:00:00
//
// 索引说明（perf/model-call-audit-indexes-2026-08-02）：
//   - CreatedAt 在此重声明以覆盖 BaseModel.CreatedAt，仅为给本表挂 created_at 索引 tag，
//     避免污染其他继承 BaseModel 的表。
//   - 索引名改为 idx_mca_*：旧名 idx_api_key_id_created_at / idx_model_id_created_at
//     已作为单列索引存在，GORM AutoMigrate 检测到同名索引会跳过创建，无法升级为复合索引；
//     换新名后 AutoMigrate 会创建复合索引，旧单列索引残留（冗余但无害）。
//   - idx_mca_created_at 覆盖 admin 列表默认 created_at 倒序分页；
//     idx_mca_apikey_created / idx_mca_model_created 覆盖按 api_key_id/model_id 过滤 + 时间倒序及趋势聚合。
type ModelCallAudit struct {
	BaseModel
	CreatedAt                time.Time `json:"created_at" gorm:"column:created_at;comment:创建时间;index:idx_mca_apikey_created,priority:2,sort:desc;index:idx_mca_model_created,priority:2,sort:desc;index:idx_mca_created_at,sort:desc"`
	APIKeyID                 uint      `json:"api_key_id" gorm:"column:api_key_id;not null;comment:API密钥ID;index:idx_mca_apikey_created,priority:1"`
	ModelID                  string    `json:"model_id" gorm:"column:model_id;not null;default:'';comment:业务模型ID(创建默认=alias);index:idx_mca_model_created,priority:1"`
	UpstreamProtocol         string    `json:"upstream_protocol" gorm:"column:upstream_protocol;not null;default:'';comment:上游协议(openai-chat-completion/openai-response/anthropic-message)"`
	APIProtocol              string    `json:"api_protocol" gorm:"column:api_protocol;not null;default:'';comment:接口层协议(openai-chat-completion/openai-response/anthropic-message)"`
	Endpoint                 string    `json:"endpoint" gorm:"column:endpoint;not null;default:'';comment:调用模型的 Endpoint 名"`
	InputTokens              int       `json:"input_tokens" gorm:"column:input_tokens;not null;default:0;comment:输入token数"`
	OutputTokens             int       `json:"output_tokens" gorm:"column:output_tokens;not null;default:0;comment:输出token数"`
	CacheCreationInputTokens int       `json:"cache_creation_input_tokens" gorm:"column:cache_creation_input_tokens;not null;default:0;comment:缓存写入token数"`
	CacheReadInputTokens     int       `json:"cache_read_input_tokens" gorm:"column:cache_read_input_tokens;not null;default:0;comment:缓存命中token数"`
	FirstTokenLatencyMs      int64     `json:"first_token_latency_ms" gorm:"column:first_token_latency_ms;not null;default:0;comment:首token延迟(ms)，非流式为总延迟"`
	StreamDurationMs         int64     `json:"stream_duration_ms" gorm:"column:stream_duration_ms;not null;default:0;comment:流式传输持续时间(ms)，非流式为0"`
	UserAgent                string    `json:"user_agent" gorm:"column:user_agent;not null;default:'';comment:请求客户端User-Agent"`
	UpstreamStatusCode       int       `json:"upstream_status_code" gorm:"column:upstream_status_code;not null;default:0;comment:上游HTTP状态码：200成功，>0为上游返回码，-1为连接错误，0为未知错误"`
	ErrorMessage             string    `json:"error_message" gorm:"column:error_message;not null;default:'';comment:错误信息，成功时为空"`
	TraceID                  string    `json:"trace_id" gorm:"column:trace_id;not null;default:'';comment:请求追踪ID;index"`
}
