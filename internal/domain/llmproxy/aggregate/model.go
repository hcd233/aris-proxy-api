package aggregate

import (
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonagg "github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// Model 模型关联聚合根
//
// 记录对外暴露的模型别名（alias）与上游实际模型名（upstreamModel）和 endpoint 的关联。
// 同一 alias 可关联多条记录，解析时随机选择。
type Model struct {
	commonagg.Base

	alias           vo.EndpointAlias
	modelID         string
	upstreamModel   string
	endpointID      uint
	enabled         bool
	contextLength   int
	maxOutputTokens int
	capabilities    []enum.InputModality
	createdAt       time.Time
	updatedAt       time.Time
}

// CreateModel 构造 Model 聚合根
func CreateModel(id uint, alias vo.EndpointAlias, upstreamModel string, endpointID uint, enabled bool, contextLength, maxOutputTokens int, capabilities []enum.InputModality) (*Model, error) {
	if alias.IsEmpty() {
		return nil, ierr.New(ierr.ErrValidation, "model alias cannot be empty")
	}
	if upstreamModel == "" {
		return nil, ierr.New(ierr.ErrValidation, "model name cannot be empty")
	}
	if endpointID == 0 {
		return nil, ierr.New(ierr.ErrValidation, "endpoint id cannot be 0")
	}
	if contextLength < 0 {
		return nil, ierr.New(ierr.ErrValidation, "context length cannot be negative")
	}
	if maxOutputTokens < 0 {
		return nil, ierr.New(ierr.ErrValidation, "max output tokens cannot be negative")
	}
	if err := validateCapabilities(capabilities); err != nil {
		return nil, err
	}
	m := &Model{
		alias:           alias,
		modelID:         alias.String(),
		upstreamModel:   upstreamModel,
		endpointID:      endpointID,
		enabled:         enabled,
		contextLength:   contextLength,
		maxOutputTokens: maxOutputTokens,
		capabilities:    capabilities,
	}
	m.SetID(id)
	return m, nil
}

// validateCapabilities 校验模型能力集合：非空、必须含 text、成员必须合法
func validateCapabilities(capabilities []enum.InputModality) error {
	if len(capabilities) == 0 {
		return ierr.New(ierr.ErrValidation, "model capabilities cannot be empty")
	}
	if !lo.Contains(capabilities, enum.InputModalityText) {
		return ierr.New(ierr.ErrValidation, "model capabilities must contain text")
	}
	if !lo.Every(enum.InputModalities, capabilities) {
		return ierr.New(ierr.ErrValidation, "model capabilities contain invalid input modality")
	}
	return nil
}

func (m *Model) Alias() vo.EndpointAlias { return m.alias }
func (m *Model) ModelID() string         { return m.modelID }
func (m *Model) UpstreamModel() string   { return m.upstreamModel }
func (m *Model) EndpointID() uint        { return m.endpointID }
func (m *Model) Enabled() bool           { return m.enabled }
func (m *Model) ContextLength() int      { return m.contextLength }
func (m *Model) MaxOutputTokens() int    { return m.maxOutputTokens }
func (m *Model) Capabilities() []enum.InputModality {
	return m.capabilities
}
func (m *Model) CreatedAt() time.Time { return m.createdAt }
func (m *Model) UpdatedAt() time.Time { return m.updatedAt }

// SetModelID 设置业务模型 ID（仓储恢复用）
func (m *Model) SetModelID(modelID string) { m.modelID = modelID }

func (m *Model) SetTimestamps(createdAt, updatedAt time.Time) {
	m.createdAt = createdAt
	m.updatedAt = updatedAt
}

// Update 更新 Model 字段（仅非 nil 字段更新）
func (m *Model) Update(alias *vo.EndpointAlias, upstreamModel *string, endpointID *uint, enabled *bool, contextLength, maxOutputTokens *int, capabilities *[]enum.InputModality, modelID *string) error {
	if alias != nil {
		m.alias = *alias
	}
	if upstreamModel != nil {
		m.upstreamModel = *upstreamModel
	}
	if endpointID != nil {
		m.endpointID = *endpointID
	}
	if enabled != nil {
		m.enabled = *enabled
	}
	if contextLength != nil {
		m.contextLength = *contextLength
	}
	if maxOutputTokens != nil {
		m.maxOutputTokens = *maxOutputTokens
	}
	if capabilities != nil {
		if err := validateCapabilities(*capabilities); err != nil {
			return err
		}
		m.capabilities = *capabilities
	}
	if modelID != nil {
		if *modelID == "" {
			return ierr.New(ierr.ErrValidation, "model id cannot be empty")
		}
		m.modelID = *modelID
	}
	return nil
}
