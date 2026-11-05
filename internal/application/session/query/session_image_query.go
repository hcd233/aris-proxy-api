package query

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"go.uber.org/zap"

	sessionport "github.com/hcd233/aris-mass/internal/application/session/port"
	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/domain/blob"
	"github.com/hcd233/aris-mass/internal/logger"
)

type getSessionImageHandler struct {
	metaQuery sessionport.GetSessionMetaByUserHandler
	reader    blob.ImageReader
}

// NewGetSessionImageHandler 构造会话图片查询处理器
//
//	@param metaQuery sessionport.GetSessionMetaByUserHandler 复用会话元数据查询完成归属校验
//	@param reader blob.ImageReader
//	@return sessionport.GetSessionImageHandler
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func NewGetSessionImageHandler(metaQuery sessionport.GetSessionMetaByUserHandler, reader blob.ImageReader) sessionport.GetSessionImageHandler {
	return &getSessionImageHandler{metaQuery: metaQuery, reader: reader}
}

// Handle 校验会话归属与图片引用后返回读取方式
//
//	权限与消息列表接口一致（owner / admin / demo 白名单）；图片必须被该会话的消息引用，
//	防止用自己的会话 ID 探测别人的图片。
func (h *getSessionImageHandler) Handle(ctx context.Context, q sessionport.GetSessionImageQuery) (*sessionport.SessionImageView, error) {
	if !vo.IsSHA256Hex(q.SHA256) {
		return nil, ierr.New(ierr.ErrValidation, "invalid image sha256")
	}
	meta, err := h.metaQuery.Handle(ctx, sessionport.GetSessionMetaByUserQuery{
		UserID:            q.UserID,
		IsAdmin:           q.IsAdmin,
		SessionID:         q.SessionID,
		IsDemo:            q.IsDemo,
		AllowedSessionIDs: q.AllowedSessionIDs,
	})
	if err != nil {
		return nil, err
	}
	referenced, err := h.reader.IsReferenced(ctx, meta.MessageIDs, q.SHA256)
	if err != nil {
		return nil, err
	}
	if !referenced {
		logger.WithCtx(ctx).Warn("[SessionQuery] Image not referenced by session",
			zap.Uint("sessionID", q.SessionID), zap.String("sha256", q.SHA256))
		return nil, ierr.New(ierr.ErrDataNotExists, "image not found")
	}
	view, err := h.reader.Locate(ctx, q.SHA256)
	if err != nil {
		return nil, err
	}
	if view == nil {
		return nil, ierr.New(ierr.ErrDataNotExists, "image not found")
	}
	out := &sessionport.SessionImageView{
		URL: view.URL, Mime: view.Mime, Size: view.Size,
		Width: view.Width, Height: view.Height, ExpiresAt: view.ExpiresAt,
	}
	if out.URL == "" && strings.HasPrefix(view.Mime, constant.MIMETypeImagePrefix) {
		out.DataURL = fmt.Sprintf(constant.DataURLTemplate, view.Mime, base64.StdEncoding.EncodeToString(view.Data))
	}
	return out, nil
}
