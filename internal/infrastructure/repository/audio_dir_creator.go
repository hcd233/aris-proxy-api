// Package repository 基础设施仓储实现
package repository

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	objdao "github.com/hcd233/aris-proxy-api/internal/infrastructure/storage/obj_dao"
)

// AudioDirCreator 将 objdao.ObjDAO 适配为只暴露创建目录语义的小接口，
// 用于 application/oauth2 跨域消费对象存储能力，避免 application 层直接
// 依赖 objdao 的全量接口。
//
//	@author centonhuang
//	@update 2026-04-24 14:00:00
type AudioDirCreator struct {
	dao objdao.ObjDAO
}

// NewAudioDirCreator 构造音频目录创建器（audio 用户独立目录）
//
//	@return *AudioDirCreator
//	@author centonhuang
//	@update 2026-04-22 20:30:00
func NewAudioDirCreator() *AudioDirCreator {
	return &AudioDirCreator{dao: objdao.GetAudioObjDAO()}
}

// CreateDir 为指定用户创建独立的音频存储目录，隐藏底层 ObjectInfo 细节
//
//	@receiver a *AudioDirCreator
//	@param ctx context.Context
//	@param userID uint
//	@return error
//	@author centonhuang
//	@update 2026-04-24 14:00:00
func (a *AudioDirCreator) CreateDir(ctx context.Context, userID uint) error {
	if _, err := a.dao.CreateDir(ctx, userID); err != nil {
		return ierr.Wrap(ierr.ErrObjStorage, err, "create audio dir")
	}
	return nil
}
