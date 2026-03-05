package dto

// OpenAIModel OpenAI模型信息
//
//	@author centonhuang
//	@update 2025-11-12 10:00:00
type OpenAIModel struct {
	ID      string `json:"id" doc:"The model identifier"`
	Created int64  `json:"created" doc:"Unix timestamp (in seconds) when the model was created"`
	Object  string `json:"object" doc:"The object type, always model"`
	OwnedBy string `json:"owned_by" doc:"The organization that owns the model"`
}

// ListModelsResponse OpenAI模型列表响应体
//
//	@author centonhuang
//	@update 2025-11-12 10:00:00
type ListModelsResponse struct {
	Body struct {
		Object string         `json:"object" doc:"The object type, always list"`
		Data   []*OpenAIModel `json:"data" doc:"List of model objects"`
	}
}
