package aggregate

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonagg "github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// Model 模型关联聚合根
//
// 记录对外暴露的模型别名（alias）与上游实际模型名（model）和 endpoint 的关联。
// 同一 alias 可关联多条记录，解析时随机选择。
type Model struct {
	commonagg.Base

	alias      vo.EndpointAlias
	model      string
	endpointID uint
}

// CreateModel 构造 Model 聚合根
func CreateModel(id uint, alias vo.EndpointAlias, model string, endpointID uint) (*Model, error) {
	if alias.IsEmpty() {
		return nil, ierr.New(ierr.ErrValidation, "model alias cannot be empty")
	}
	if model == "" {
		return nil, ierr.New(ierr.ErrValidation, "model name cannot be empty")
	}
	if endpointID == 0 {
		return nil, ierr.New(ierr.ErrValidation, "endpoint id cannot be 0")
	}
	m := &Model{
		alias:      alias,
		model:      model,
		endpointID: endpointID,
	}
	m.SetID(id)
	return m, nil
}

func (*Model) AggregateType() string { return constant.AggregateTypeModel }

func (m *Model) Alias() vo.EndpointAlias { return m.alias }
func (m *Model) ModelName() string       { return m.model }
func (m *Model) EndpointID() uint        { return m.endpointID }
