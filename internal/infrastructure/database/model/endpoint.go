package model

// Endpoint 上游端点数据库模型
type Endpoint struct {
	BaseModel
	ID                          uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:端点ID"`
	Name                        string `json:"name" gorm:"column:name;not null;uniqueIndex:idx_endpoint_name_deleted,priority:1;comment:端点名称"`
	OpenaiBaseURL               string `json:"openai_base_url" gorm:"column:openai_base_url;not null;comment:OpenAI协议baseURL"`
	AnthropicBaseURL            string `json:"anthropic_base_url" gorm:"column:anthropic_base_url;not null;comment:Anthropic协议baseURL"`
	APIKey                      string `json:"api_key" gorm:"column:api_key;not null;comment:上游API密钥"`
	SupportOpenAIChatCompletion bool   `json:"support_openai_chat_completion" gorm:"column:support_openai_chat_completion;not null;default:true;comment:支持/chat/completions"`
	SupportOpenAIResponse       bool   `json:"support_openai_response" gorm:"column:support_openai_response;not null;default:false;comment:支持/responses"`
	SupportAnthropicMessage     bool   `json:"support_anthropic_message" gorm:"column:support_anthropic_message;not null;default:false;comment:支持/messages"`
	DeletedAt                   int64  `json:"deleted_at" gorm:"column:deleted_at;default:0;uniqueIndex:idx_endpoint_name_deleted,priority:2;comment:删除时间"`
}
