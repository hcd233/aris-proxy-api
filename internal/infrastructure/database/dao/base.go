// Package dao DAO
//
//	update 2024-10-17 02:31:49
package dao

import (
	"reflect"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// baseDAO 基础DAO
//
//	author centonhuang
//	update 2024-10-17 02:32:22
type baseDAO[ModelT interface{}] struct{}

// Create 创建数据
//
//	param dao *BaseDAO[T]
//	return Create
//	author centonhuang
//	update 2024-10-17 02:51:49
func (dao *baseDAO[ModelT]) Create(db *gorm.DB, data *ModelT) (err error) {
	err = db.Create(&data).Error
	return
}

// BatchCreate 批量创建数据
//
//	@param dao *baseDAO[ModelT]
//	@return BatchCreate
//	@author centonhuang
//	@update 2025-11-07 01:57:42
func (dao *baseDAO[ModelT]) BatchCreate(db *gorm.DB, data []*ModelT) (err error) {
	err = db.Create(&data).Error
	return
}

// Update 使用ID更新数据
//
//	param dao *BaseDAO[T]
//	return Update
//	author centonhuang
//	update 2024-10-17 02:52:18
func (dao *baseDAO[ModelT]) Update(db *gorm.DB, data *ModelT, info map[string]interface{}) (err error) {
	updateAtField := "updated_at"
	info[updateAtField] = time.Now().UTC()

	sql := db.Model(data)
	selectFields := lo.Filter(lo.Keys(info), func(item string, _ int) bool {
		return !reflect.ValueOf(info[item]).IsZero()
	})
	sql = sql.Select(selectFields)
	err = sql.Updates(info).Error
	return
}

// Delete 删除
//
//	param dao *BaseDAO[T]
//	return Delete
//	author centonhuang
//	update 2024-10-17 02:52:33
func (dao *baseDAO[ModelT]) Delete(db *gorm.DB, data *ModelT) (err error) {
	err = db.Model(data).Update("deleted_at", time.Now().UTC().Unix()).Error
	return
}

func (dao *baseDAO[ModelT]) BatchDelete(db *gorm.DB, data *[]ModelT) (err error) {
	err = db.Model(data).Update("deleted_at", time.Now().UTC().Unix()).Error
	return
}

// func GetByID 使用ID查询指定数据
//
//	param dao *BaseDAO[T]
//	return GetByID
//	author centonhuang
//	update 2024-10-17 03:06:57
//	@param dao
//	@return Get
//	@author centonhuang
//	@update 2025-11-14 16:05:03
func (dao *baseDAO[ModelT]) Get(db *gorm.DB, where *ModelT, fields []string) (data *ModelT, err error) {
	err = db.Select(fields).Where(where).Where("deleted_at = 0").First(&data).Error
	return
}

// BatchGetByField 根据指定字段的多个值批量查询数据
//
//	@param db *gorm.DB
//	@param whereField string 字段名
//	@param values any 字段值列表（切片类型，如 []string、[]uint 等）
//	@param selectFields []string 查询字段
//	@return data []*ModelT
//	@return err error
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func (dao *baseDAO[ModelT]) BatchGetByField(db *gorm.DB, whereField string, values any, selectFields []string) (data []*ModelT, err error) {
	if values == nil {
		return []*ModelT{}, nil
	}
	err = db.Select(selectFields).Where(whereField+" IN ?", values).Where("deleted_at = 0").Find(&data).Error
	return
}

// Paginate 分页查询
//
//	param dao *BaseDAO[T]
//	return Paginate
//	author centonhuang
//	update 2024-10-17 03:09:11
func (dao *baseDAO[ModelT]) Paginate(db *gorm.DB, where *ModelT, fields []string, param *CommonParam) (data []*ModelT, pageInfo *model.PageInfo, err error) {
	limit, offset := param.PageSize, (param.Page-1)*param.PageSize

	sql := db.Model(where).Select(fields).Where(where).Where("deleted_at = 0")

	if param.Query != "" && len(param.QueryFields) > 0 {
		like := "%" + param.Query + "%"
		expressions := make([]clause.Expression, 0, len(param.QueryFields))
		for _, field := range param.QueryFields {
			if field == "" {
				continue
			}
			expressions = append(expressions, clause.Like{Column: clause.Column{Name: field}, Value: like})
		}

		if len(expressions) > 0 {
			sql = sql.Where(expressions[0])
			for _, expr := range expressions[1:] {
				sql = sql.Or(expr)
			}
		}
	}

	if param.Sort != "" && param.SortField != "" {
		sql = sql.Order(clause.OrderByColumn{Column: clause.Column{Name: param.SortField}, Desc: param.Sort == enum.SortDesc})
	}

	pageInfo = &model.PageInfo{
		Page:     param.Page,
		PageSize: param.PageSize,
	}

	err = sql.Count(&pageInfo.Total).Error
	if err != nil {
		return
	}

	err = sql.Limit(limit).Offset(offset).Find(&data).Error

	return
}
