package repository

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// modelRepository ModelRepository 的 GORM 实现
type modelRepository struct {
	dao *dao.ModelDAO
	db  *gorm.DB
}

// NewModelRepository 构造 ModelRepository
func NewModelRepository(db *gorm.DB) llmproxy.ModelRepository {
	return &modelRepository{dao: dao.GetModelDAO(), db: db}
}

// FindByAlias 按 alias 查询指定用户的所有关联模型记录（网关解析专用）
//
// userID 三态：非 nil 精确匹配 user_id（0=共享池），nil 不过滤（勿在网关路径使用）。
func (r *modelRepository) FindByAlias(ctx context.Context, alias vo.EndpointAlias, userID *uint) ([]*aggregate.Model, error) {
	db := scopedDB(r.db.WithContext(ctx), userID)
	query := &dbmodel.Model{Alias: alias.String(), Enabled: true}
	models, err := r.dao.BatchGet(db, query, constant.ModelRepoFieldsFull)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find models by alias")
	}
	return util.MapErr(models, func(m *dbmodel.Model, _ int) (*aggregate.Model, error) {
		return toModelAggregate(m)
	})
}

func toModelAggregate(m *dbmodel.Model) (*aggregate.Model, error) {
	model, err := aggregate.CreateModel(m.ID, vo.EndpointAlias(m.Alias), m.UpstreamModel, m.EndpointID, m.Enabled, m.ContextLength, m.MaxOutputTokens, m.Capabilities)
	if err != nil {
		return nil, err
	}
	model.SetUserID(m.UserID)
	model.SetModelID(m.ModelID)
	model.SetTimestamps(m.CreatedAt, m.UpdatedAt)
	return model, nil
}

func toModelDBModel(m *aggregate.Model) *dbmodel.Model {
	return &dbmodel.Model{
		ID:              m.AggregateID(),
		Alias:           m.Alias().String(),
		ModelID:         m.ModelID(),
		UpstreamModel:   m.UpstreamModel(),
		EndpointID:      m.EndpointID(),
		Enabled:         m.Enabled(),
		ContextLength:   m.ContextLength(),
		MaxOutputTokens: m.MaxOutputTokens(),
		Capabilities:    m.Capabilities(),
	}
}

// FindByID 按 ID 查询模型（scopeUserID 非 nil 时精确匹配 user_id）
func (r *modelRepository) FindByID(ctx context.Context, id uint, scopeUserID *uint) (*aggregate.Model, error) {
	db := scopedDB(r.db.WithContext(ctx), scopeUserID)
	m, err := r.dao.Get(db, &dbmodel.Model{ID: id}, constant.ModelRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find model by id")
	}
	return toModelAggregate(m)
}

// Create 创建模型（写入归属 ownerUserID）
func (r *modelRepository) Create(ctx context.Context, m *aggregate.Model, ownerUserID uint) (uint, error) {
	db := r.db.WithContext(ctx)
	mdl := toModelDBModel(m)
	mdl.UserID = ownerUserID
	if err := db.Create(mdl).Error; err != nil {
		return 0, ierr.Wrap(ierr.ErrDBCreate, err, "create model")
	}
	return mdl.ID, nil
}

// Update 更新模型（仅更新非零值字段）
func (r *modelRepository) Update(ctx context.Context, m *aggregate.Model) error {
	return updateModelTx(r.db.WithContext(ctx), m)
}

// updateModelTx 更新模型行（仅更新非零值字段），可在事务内复用。
//
// user_id 一并写入：model 归属始终跟随其 endpoint（命令层已校验 owner 一致），
// 换绑 endpoint 后同步归属，避免出现"endpoint 在 A 名下、model 记在 B 名下"的悬挂状态。
// GORM 的 Updates(map) 不经过 field serializer，capabilities 需手动序列化为 JSON 字符串。
func updateModelTx(tx *gorm.DB, m *aggregate.Model) error {
	capJSON, _ := sonic.Marshal(m.Capabilities()) //nolint:errcheck // []string 序列化不会失败，且值已经聚合校验
	updates := map[string]any{
		constant.FieldUserID:               m.UserID(),
		constant.FieldModelAlias:           m.Alias().String(),
		constant.FieldModelID:              m.ModelID(),
		constant.FieldModelUpstreamModel:   m.UpstreamModel(),
		constant.FieldModelEndpointID:      m.EndpointID(),
		constant.FieldModelEnabled:         m.Enabled(),
		constant.FieldModelContextLength:   m.ContextLength(),
		constant.FieldModelMaxOutputTokens: m.MaxOutputTokens(),
		constant.FieldModelCapabilities:    string(capJSON),
	}
	if err := tx.Model(&dbmodel.Model{}).Where(constant.WhereIDEquals, m.AggregateID()).Updates(updates).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update model")
	}
	return nil
}

// Delete 删除模型（软删除；scopeUserID 非 nil 时精确匹配 user_id）
func (r *modelRepository) Delete(ctx context.Context, id uint, scopeUserID *uint) error {
	db := scopedDB(r.db.WithContext(ctx), scopeUserID)
	if err := r.dao.Delete(db, &dbmodel.Model{ID: id}); err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete model")
	}
	return nil
}

// DeleteByEndpointID 按 endpointID 批量删除模型
func (r *modelRepository) DeleteByEndpointID(ctx context.Context, endpointID uint) error {
	db := r.db.WithContext(ctx)
	if err := r.dao.BatchDeleteByField(db, constant.FieldEndpointID, []uint{endpointID}); err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete models by endpoint id")
	}
	return nil
}

// List 列出所有模型
func (r *modelRepository) List(ctx context.Context) ([]*aggregate.Model, error) {
	db := r.db.WithContext(ctx)
	var models []*dbmodel.Model
	if err := db.Find(&models).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list models")
	}
	return util.MapErr(models, func(m *dbmodel.Model, _ int) (*aggregate.Model, error) {
		return toModelAggregate(m)
	})
}

// Paginate 分页查询模型列表
//
//	@author centonhuang
//	@update 2026-05-27 10:00:00
func (r *modelRepository) Paginate(ctx context.Context, param model.CommonParam, scopeUserID *uint) ([]*aggregate.Model, *model.PageInfo, error) {
	db := scopedDB(r.db.WithContext(ctx), scopeUserID)
	records, pageInfo, err := r.dao.Paginate(
		db,
		&dbmodel.Model{},
		constant.ModelRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldAlias, constant.FieldModelUpstreamModel}},
			SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate models")
	}
	out, convErr := util.MapErr(records, func(m *dbmodel.Model, _ int) (*aggregate.Model, error) {
		return toModelAggregate(m)
	})
	if convErr != nil {
		return nil, nil, convErr
	}
	return out, pageInfo, nil
}

// PaginateWithFilter 带筛选的模型分页查询（Web 平铺模型列表专用）
//
//	@receiver r *modelRepository
//	@param ctx context.Context
//	@param param model.CommonParam
//	@param filter llmproxy.ModelListFilter
//	@param scopeUserID *uint nil 表示 admin 全量视角，非 nil 精确匹配 user_id
//	@return []*aggregate.Model
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-08-28 10:00:00
//
// capabilities 是 text 列 + serializer:json（非 PG 原生 jsonb），且 enum.InputModalities
// 是封闭枚举，故 LIKE '%"image"%' 在 PG 与 sqlite 上语法与行为一致，无需分库写法。
// 代价是不走索引；models 表千行量级可接受。
//
// 排序列走显式白名单：不能用 util.SafeSortField 代替（它只校验字符集，
// api_key 之类敏感列同样放行），白名单外取值回退默认列而非报错。
func (r *modelRepository) PaginateWithFilter(ctx context.Context, param model.CommonParam, filter llmproxy.ModelListFilter, scopeUserID *uint) ([]*aggregate.Model, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	if scopeUserID != nil {
		db = db.Where(constant.FieldUserID+" = ?", *scopeUserID)
	}
	switch filter.Status {
	case constant.ModelStatusEnabled:
		db = db.Where(constant.WhereModelEnabledEquals, true)
	case constant.ModelStatusDisabled:
		db = db.Where(constant.WhereModelEnabledEquals, false)
	}
	if filter.EndpointID != 0 {
		db = db.Where(constant.WhereEndpointIDEquals, filter.EndpointID)
	}
	// 未知能力值视为不过滤，避免前端拼错参数导致整页空白
	if lo.Contains(enum.InputModalities, filter.Capability) {
		db = db.Where(constant.WhereCapabilitiesLike, `%"`+filter.Capability+`"%`)
	}
	// 白名单外回退默认列但保留调用方排序方向，不报错（避免前端拼错导致整页 500）
	if !lo.Contains(constant.ModelListSortFields, param.SortField) {
		param.SortField = constant.ModelListDefaultSortField
	}

	records, pageInfo, err := r.dao.Paginate(
		db,
		&dbmodel.Model{},
		constant.ModelRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldAlias, constant.FieldModelID, constant.FieldModelUpstreamModel}},
			SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate models with filter")
	}
	out, convErr := util.MapErr(records, func(m *dbmodel.Model, _ int) (*aggregate.Model, error) {
		return toModelAggregate(m)
	})
	if convErr != nil {
		return nil, nil, convErr
	}
	return out, pageInfo, nil
}

// ListByEndpointIDs 按 endpoint ID 集合批量拉取模型聚合（id 升序）
//
// 不做二次 scope 过滤，调用方传入的 endpointIDs 必须已经过 scope 解析。
func (r *modelRepository) ListByEndpointIDs(ctx context.Context, endpointIDs []uint) ([]*aggregate.Model, error) {
	ids := lo.Uniq(lo.Filter(endpointIDs, func(id uint, _ int) bool { return id != 0 }))
	if len(ids) == 0 {
		return []*aggregate.Model{}, nil
	}
	db := r.db.WithContext(ctx)
	records, err := r.dao.BatchGetByField(db, constant.FieldEndpointID, ids, constant.ModelRepoFieldsFull)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list models by endpoint ids")
	}
	out, convErr := util.MapErr(records, func(m *dbmodel.Model, _ int) (*aggregate.Model, error) {
		return toModelAggregate(m)
	})
	if convErr != nil {
		return nil, convErr
	}
	return out, nil
}

// UpdateWithHistorySync 在单事务内更新模型并把历史数据中的旧业务模型 ID 批量替换为当前值。
//
// 模型更新与历史替换必须原子（接口契约见 domain）：若分两步，替换失败时模型本体
// 已改名，同样的更新请求重试时新旧 ID 相等、同步条件不再触发，历史将永久停留旧 ID。
func (r *modelRepository) UpdateWithHistorySync(ctx context.Context, m *aggregate.Model, oldModelID string) (llmproxy.ModelIDSyncCounts, error) {
	var counts llmproxy.ModelIDSyncCounts
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		counts = llmproxy.ModelIDSyncCounts{}
		if err := updateModelTx(tx, m); err != nil {
			return err
		}
		return replaceHistoricalModelIDs(tx, m.UserID(), oldModelID, m.ModelID(), &counts)
	})
	if err != nil {
		return llmproxy.ModelIDSyncCounts{}, err
	}
	return counts, nil
}

// replaceHistoricalModelIDs 在传入事务内将归属 userID 的历史数据中业务模型 ID oldID
// 批量替换为 newID（由 UpdateWithHistorySync 的外层事务包裹）。
//
// 三步（spec 2026-09-04-model-id-history-sync §5.4）：
//  1. audit：api_key_id 关联 user 的全部 key（含已删 key）；
//  2. session：api_key_name 关联（跨用户同名冲突名整体跳过，见 ownedKeyNames），
//     model_ids 数组逐元素替换（按存储字节构造的 LIKE 预过滤 + 精确等值确认）；
//  3. message：scope 为第 2 步实际命中会话引用到的消息，分块更新。
//
// 纯 Go 实现而非 PG jsonb SQL：sqlite 测试基建可运行，且本操作为偶发管理操作。
func replaceHistoricalModelIDs(tx *gorm.DB, userID uint, oldID, newID string, counts *llmproxy.ModelIDSyncCounts) error {
	if err := replaceAuditModelIDs(tx, userID, oldID, newID, counts); err != nil {
		return err
	}
	var referencedMsgIDs []uint
	if err := replaceSessionModelIDs(tx, userID, oldID, newID, counts, &referencedMsgIDs); err != nil {
		return err
	}
	return replaceMessageModelIDs(tx, oldID, newID, counts, referencedMsgIDs)
}

// replaceAuditModelIDs 替换归属 user（含已删 key）的审计记录中的旧 model id。
func replaceAuditModelIDs(tx *gorm.DB, userID uint, oldID, newID string, counts *llmproxy.ModelIDSyncCounts) error {
	res := tx.Model(&dbmodel.ModelCallAudit{}).
		Where(constant.WhereModelIDEquals+" AND "+constant.WhereAPIKeyIDIn, oldID,
			tx.Model(&dbmodel.ProxyAPIKey{}).Select(constant.FieldID).Where(constant.WhereUserIDEquals, userID)).
		Update(constant.FieldModelID, newID)
	if res.Error != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, res.Error, "replace audit model id")
	}
	counts.AuditCount = res.RowsAffected
	return nil
}

// replaceSessionModelIDs 替换归属 user 的会话 model_ids 数组中的旧 model id，
// 并收集命中会话引用到的消息 ID（供 message scope 收紧）。
func replaceSessionModelIDs(tx *gorm.DB, userID uint, oldID, newID string, counts *llmproxy.ModelIDSyncCounts, referencedMsgIDs *[]uint) error {
	names, err := ownedKeyNames(tx, userID)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}
	var sessions []dbmodel.Session
	if err := tx.Select(constant.SessionRepoFieldsModelIDSync).
		Where(constant.WhereSessionKeyAndModel, names, likeJSONSubstring(oldID)).
		Find(&sessions).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBQuery, err, "find sessions with old model id")
	}
	for i := range sessions {
		if !slices.Contains(sessions[i].ModelIDs, oldID) {
			continue
		}
		newIDs := lo.Map(sessions[i].ModelIDs, func(id string, _ int) string {
			if id == oldID {
				return newID
			}
			return id
		})
		// 单列更新（Save 是全字段 UPDATE，会携带 questions/metadata 等大 JSON 列回写）。
		// Update(column, slice) 不走字段 serializer（多元素会被驱动解析成 row value），
		// 须先按标准 JSON 序列化为与 serializer:json 一致的文本。
		encoded, err := sonic.Marshal(newIDs)
		if err != nil {
			return ierr.Wrap(ierr.ErrDBUpdate, err, "encode session model_ids")
		}
		if err := tx.Model(&sessions[i]).Update(constant.FieldModelIDs, string(encoded)).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBUpdate, err, "update session model_ids")
		}
		counts.SessionCount++
		*referencedMsgIDs = append(*referencedMsgIDs, sessions[i].MessageIDs...)
	}
	return nil
}

// ownedKeyNames 返回归属 user 的全部 API Key 名（含已删 key），并剔除与其他用户同名的冲突名。
//
// session 仅按 api_key_name 归属，而 key 名仅 (user_id, name, deleted_at) 复合唯一、
// 跨用户可重名；同名冲突时无法区分 session 归属，必须宁漏勿越——冲突名下的 session
// 一律不替换（含 user 自己的），防止批量写越界改写他人会话。
func ownedKeyNames(tx *gorm.DB, userID uint) ([]string, error) {
	var names []string
	if err := tx.Model(&dbmodel.ProxyAPIKey{}).Where(constant.WhereUserIDEquals, userID).
		Distinct().Pluck(constant.FieldName, &names).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "pluck api key names")
	}
	if len(names) == 0 {
		return names, nil
	}
	var conflicted []string
	if err := tx.Model(&dbmodel.ProxyAPIKey{}).
		Where(constant.WhereNameInAndUserIDNotEquals, names, userID).
		Distinct().Pluck(constant.FieldName, &conflicted).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "pluck conflicted api key names")
	}
	if len(conflicted) == 0 {
		return names, nil
	}
	return lo.Reject(names, func(name string, _ int) bool {
		return slices.Contains(conflicted, name)
	}), nil
}

// likeJSONSubstring 构造与 sessions.model_ids 存储字节一致的 LIKE 子串模式。
//
// model_ids 经 GORM serializer:json（标准 JSON 转义）序列化存储：`gpt"4` 存储为
// `gpt\"4`、`a&b` 存储为 `a\u0026b`（HTML 转义）、`gpt\4` 存储为 `gpt\\4`——
// 直接用原始串构造 LIKE 会因转义字节差异漏匹配。先按标准 JSON 编码取转义形式
// （sonic.ConfigStd 与 encoding/json 转义行为兼容），再转义 LIKE 通配符，
// 配合 ESCAPE 子句按字面匹配。
func likeJSONSubstring(s string) string {
	inner := s
	if encoded, err := sonic.ConfigStd.Marshal(s); err == nil {
		inner = string(encoded[1 : len(encoded)-1])
	}
	return "%" + escapeLikeWildcards(inner) + "%"
}

// escapeLikeWildcards 转义 LIKE 通配符（% _ \），配合 ESCAPE '\' 子句按字面匹配。
func escapeLikeWildcards(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// replaceMessageModelIDs 替换命中会话引用到的消息中的旧 model id（分块 IN 更新）。
func replaceMessageModelIDs(tx *gorm.DB, oldID, newID string, counts *llmproxy.ModelIDSyncCounts, msgIDs []uint) error {
	if len(msgIDs) == 0 {
		return nil
	}
	for _, chunk := range lo.Chunk(lo.Uniq(msgIDs), constant.ModelIDSyncINChunkSize) {
		res := tx.Model(&dbmodel.Message{}).
			Where(constant.WhereMessageIDAndModel, oldID, chunk).
			Update(constant.FieldModelID, newID)
		if res.Error != nil {
			return ierr.Wrap(ierr.ErrDBUpdate, res.Error, "replace message model id")
		}
		counts.MessageCount += res.RowsAffected
	}
	return nil
}
