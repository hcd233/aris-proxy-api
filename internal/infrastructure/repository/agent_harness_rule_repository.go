package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/agentharness"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

type agentHarnessRuleRepository struct {
	db *gorm.DB
}

// NewAgentHarnessRuleRepository 构造 agent harness 规则仓储（不做缓存，每次直查小表）
func NewAgentHarnessRuleRepository(db *gorm.DB) agentharness.RuleRepository {
	return &agentHarnessRuleRepository{db: db}
}

// List 返回全部规则，按前缀长度降序——越具体的规则越靠前，与命中优先级同序。
func (r *agentHarnessRuleRepository) List(ctx context.Context) ([]agentharness.Rule, error) {
	var rows []dbmodel.AgentHarnessRule
	if err := r.db.WithContext(ctx).
		Where(constant.DBConditionDeletedAtZero).
		Order(constant.AgentHarnessRuleListOrder).
		Find(&rows).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list harness rules")
	}
	return lo.Map(rows, func(row dbmodel.AgentHarnessRule, _ int) agentharness.Rule {
		return toAgentHarnessRule(row)
	}), nil
}

// Get 按 ID 查询规则；未找到返回 (nil, nil)。
func (r *agentHarnessRuleRepository) Get(ctx context.Context, id uint) (*agentharness.Rule, error) {
	var row dbmodel.AgentHarnessRule
	err := r.db.WithContext(ctx).Where(constant.WhereIDEquals, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get harness rule")
	}
	rule := toAgentHarnessRule(row)
	return &rule, nil
}

// Create 规范化后落库；pattern/name 非法时返回 ierr.ErrValidation 且不触达数据库。
func (r *agentHarnessRuleRepository) Create(ctx context.Context, rule agentharness.Rule) (*agentharness.Rule, error) {
	row, err := BuildAgentHarnessRuleRow(rule)
	if err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBCreate, err, "create harness rule")
	}
	created := toAgentHarnessRule(*row)
	return &created, nil
}

// Update 规范化后覆盖规则。
func (r *agentHarnessRuleRepository) Update(ctx context.Context, rule agentharness.Rule) error {
	row, err := BuildAgentHarnessRuleRow(rule)
	if err != nil {
		return err
	}
	err = r.db.WithContext(ctx).
		Model(&dbmodel.AgentHarnessRule{}).
		Where(constant.WhereIDEquals, rule.ID).
		Updates(map[string]any{
			constant.FieldPattern: row.Pattern,
			constant.FieldName:    row.Name,
		}).Error
	if err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update harness rule")
	}
	return nil
}

// Delete 硬删除规则；删除后历史数据在下一次查询即回落为 Other。
func (r *agentHarnessRuleRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&dbmodel.AgentHarnessRule{}, id).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete harness rule")
	}
	return nil
}

// Fingerprint 返回规则集指纹（条数 + 最近更新时间），供走势图读缓存键使用。
//
// 规则变更后指纹随之变化，缓存键随之变化——历史数据立即按新规则分组，
// 不需要跨模块失效通道。
func (r *agentHarnessRuleRepository) Fingerprint(ctx context.Context) (string, error) {
	var rows []agentHarnessRuleFingerprint
	err := r.db.WithContext(ctx).
		Model(&dbmodel.AgentHarnessRule{}).
		Where(constant.DBConditionDeletedAtZero).
		Select(constant.AgentHarnessRuleFingerprintSelect).
		Find(&rows).Error
	if err != nil {
		return "", ierr.Wrap(ierr.ErrDBQuery, err, "fingerprint harness rules")
	}
	var total int64
	var updatedAt int64
	if len(rows) > 0 {
		total = rows[0].Total
		if rows[0].UpdatedAt != nil {
			updatedAt = rows[0].UpdatedAt.UTC().UnixNano()
		}
	}
	return fmt.Sprintf(constant.AgentHarnessRuleFingerprintFormat, total, updatedAt), nil
}

func toAgentHarnessRule(row dbmodel.AgentHarnessRule) agentharness.Rule {
	return agentharness.Rule{
		ID:      row.ID,
		Pattern: row.Pattern,
		Name:    row.Name,
	}
}

// agentHarnessRuleFingerprint 规则指纹聚合行的扫描目标。
type agentHarnessRuleFingerprint struct {
	Total     int64      `gorm:"column:total"`
	UpdatedAt *time.Time `gorm:"column:updated_at"`
}

// BuildAgentHarnessRuleRow 规范化规则并转成数据库行，供 Create/Update 共用。
//
// 导出是为了单测能在不起数据库的前提下钉住「写库前一定先规范化」这一契约。
func BuildAgentHarnessRuleRow(rule agentharness.Rule) (*dbmodel.AgentHarnessRule, error) {
	pattern, err := agentharness.NormalizePattern(rule.Pattern)
	if err != nil {
		return nil, err
	}
	name, err := agentharness.NormalizeName(rule.Name)
	if err != nil {
		return nil, err
	}
	return &dbmodel.AgentHarnessRule{BaseModel: dbmodel.BaseModel{ID: rule.ID}, Pattern: pattern, Name: name}, nil
}
