package dto

// HarnessRuleItem 一条 harness 规则
type HarnessRuleItem struct {
	ID      uint   `json:"id" doc:"规则 ID"`
	Pattern string `json:"pattern" doc:"UA 前缀（入库统一小写）"`
	Name    string `json:"name" doc:"harness 展示名"`
}

// ListHarnessRulesReq 规则列表请求
type ListHarnessRulesReq struct{}

// ListHarnessRulesRsp 规则列表响应
type ListHarnessRulesRsp struct {
	CommonRsp
	Rules []*HarnessRuleItem `json:"rules" doc:"规则列表（按前缀长度降序，越具体的规则越靠前）"`
}

// CreateHarnessRuleReq 新增规则请求
type CreateHarnessRuleReq struct {
	Body *CreateHarnessRuleReqBody `json:"body" doc:"Request body"`
}

// CreateHarnessRuleReqBody 新增规则请求体
type CreateHarnessRuleReqBody struct {
	Pattern string `json:"pattern" required:"true" minLength:"1" maxLength:"200" doc:"UA 前缀（大小写不敏感，入库转小写）"`
	Name    string `json:"name" required:"true" minLength:"1" maxLength:"64" doc:"harness 展示名（Other 为保留名）"`
}

// CreateHarnessRuleRsp 新增规则响应
type CreateHarnessRuleRsp struct {
	CommonRsp
	Rule *HarnessRuleItem `json:"rule" doc:"新建的规则"`
}

// UpdateHarnessRuleReq 修改规则请求
type UpdateHarnessRuleReq struct {
	ID   uint                      `query:"id" required:"true" minimum:"1" doc:"规则 ID"`
	Body *UpdateHarnessRuleReqBody `json:"body" doc:"Request body"`
}

// UpdateHarnessRuleReqBody 修改规则请求体
type UpdateHarnessRuleReqBody struct {
	Pattern string `json:"pattern" required:"true" minLength:"1" maxLength:"200" doc:"UA 前缀（大小写不敏感，入库转小写）"`
	Name    string `json:"name" required:"true" minLength:"1" maxLength:"64" doc:"harness 展示名（Other 为保留名）"`
}

// UpdateHarnessRuleRsp 修改规则响应
type UpdateHarnessRuleRsp struct {
	CommonRsp
	Rule *HarnessRuleItem `json:"rule" doc:"修改后的规则"`
}

// DeleteHarnessRuleReq 删除规则请求
type DeleteHarnessRuleReq struct {
	ID uint `query:"id" required:"true" minimum:"1" doc:"规则 ID"`
}

// DeleteHarnessRuleRsp 删除规则响应
type DeleteHarnessRuleRsp struct {
	CommonRsp
}

// PreviewHarnessReq UA 预览请求
type PreviewHarnessReq struct {
	Body *PreviewHarnessReqBody `json:"body" doc:"Request body"`
}

// PreviewHarnessReqBody UA 预览请求体
type PreviewHarnessReqBody struct {
	UA string `json:"ua" required:"true" minLength:"1" maxLength:"512" doc:"待解析的原始 User-Agent"`
}

// PreviewHarnessRsp UA 预览响应
type PreviewHarnessRsp struct {
	CommonRsp
	Harness string           `json:"harness" doc:"最终展示名（未命中为 Other）"`
	Rule    *HarnessRuleItem `json:"rule,omitempty" doc:"命中的规则（未命中时为空）"`
}
