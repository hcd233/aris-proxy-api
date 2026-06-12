package aggregate

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonagg "github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/samber/mo"
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
	enabled    bool
	createdAt  time.Time
	updatedAt  time.Time
}

// CreateModel 构造 Model 聚合根
func CreateModel(id uint, alias vo.EndpointAlias, model string, endpointID uint, enabled bool) (*Model, error) {
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
		enabled:    enabled,
	}
	m.SetID(id)
	return m, nil
}

func (*Model) AggregateType() string { return enum.AggregateTypeModel }

func (m *Model) Alias() vo.EndpointAlias { return m.alias }
func (m *Model) ModelName() string       { return m.model }
func (m *Model) EndpointID() uint        { return m.endpointID }
func (m *Model) Enabled() bool           { return m.enabled }
func (m *Model) CreatedAt() time.Time    { return m.createdAt }
func (m *Model) UpdatedAt() time.Time    { return m.updatedAt }

func (m *Model) SetTimestamps(createdAt, updatedAt time.Time) {
	m.createdAt = createdAt
	m.updatedAt = updatedAt
}

// Update 更新 Model 字段（仅 Some 字段更新）
func (m *Model) Update(alias mo.Option[vo.EndpointAlias], model mo.Option[string], endpointID mo.Option[uint], enabled mo.Option[bool]) {
	alias.ForEach(func(v vo.EndpointAlias) { m.alias = v })
	model.ForEach(func(v string) { m.model = v })
	endpointID.ForEach(func(v uint) { m.endpointID = v })
	enabled.ForEach(func(v bool) { m.enabled = v })
}
