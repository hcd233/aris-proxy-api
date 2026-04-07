package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	objdao "github.com/hcd233/aris-proxy-api/internal/infrastructure/storage/obj_dao"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"go.uber.org/zap"
)

// ImageService 图片存储服务接口
//
//	@author centonhuang
//	@update 2026-04-07 10:00:00
type ImageService interface {
	ReplaceOpenAIBase64Images(ctx context.Context, userName string, messages []*dto.OpenAIChatCompletionMessageParam) error
	ReplaceAnthropicBase64Images(ctx context.Context, userName string, messages []*dto.AnthropicMessageParam) error
}

type imageService struct {
	imageDAO    *dao.ImageDAO
	imageObjDAO objdao.ObjDAO
}

// NewImageService 创建图片存储服务
//
//	@return ImageService
//	@author centonhuang
//	@update 2026-04-07 10:00:00
func NewImageService() ImageService {
	return &imageService{
		imageDAO:    dao.GetImageDAO(),
		imageObjDAO: objdao.GetImageObjDAO(),
	}
}

// ReplaceOpenAIBase64Images 遍历 OpenAI 消息列表，将 base64 图片上传到对象存储并替换为预签名 URL
//
//	@receiver s *imageService
//	@param ctx context.Context
//	@param userName string
//	@param messages []*dto.OpenAIChatCompletionMessageParam
//	@return error
//	@author centonhuang
//	@update 2026-04-07 10:00:00
func (s *imageService) ReplaceOpenAIBase64Images(ctx context.Context, userName string, messages []*dto.OpenAIChatCompletionMessageParam) error {
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	for _, msg := range messages {
		if msg.Content == nil || len(msg.Content.Parts) == 0 {
			continue
		}
		for _, part := range msg.Content.Parts {
			if part.Type != enum.ContentPartTypeImageURL || part.ImageURL == nil {
				continue
			}
			if !strings.HasPrefix(part.ImageURL.URL, "data:") {
				continue
			}
			presignedURL, err := s.processBase64Image(ctx, userID, userName, part.ImageURL.URL)
			if err != nil {
				return err
			}
			part.ImageURL.URL = presignedURL
		}
	}
	return nil
}

// ReplaceAnthropicBase64Images 遍历 Anthropic 消息列表，将 base64 图片上传到对象存储并替换为预签名 URL
//
//	@receiver s *imageService
//	@param ctx context.Context
//	@param userName string
//	@param messages []*dto.AnthropicMessageParam
//	@return error
//	@author centonhuang
//	@update 2026-04-07 10:00:00
func (s *imageService) ReplaceAnthropicBase64Images(ctx context.Context, userName string, messages []*dto.AnthropicMessageParam) error {
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	for _, msg := range messages {
		if msg.Content == nil || len(msg.Content.Blocks) == 0 {
			continue
		}
		if err := s.replaceAnthropicBlocks(ctx, userID, userName, msg.Content.Blocks); err != nil {
			return err
		}
	}
	return nil
}

// replaceAnthropicBlocks replaces base64 images in Anthropic content blocks (including nested tool_result content)
func (s *imageService) replaceAnthropicBlocks(ctx context.Context, userID uint, userName string, blocks []*dto.AnthropicContentBlock) error {
	for _, block := range blocks {
		switch {
		case block.Type == enum.AnthropicContentBlockTypeImage && block.Source != nil && block.Source.Type == "base64" && block.Source.Data != "":
			presignedURL, err := s.processBase64Image(ctx, userID, userName,
				fmt.Sprintf(constant.DataURLTemplate, block.Source.MediaType, block.Source.Data))
			if err != nil {
				return err
			}
			block.Source.Type = "url"
			block.Source.URL = presignedURL
			block.Source.Data = ""
			block.Source.MediaType = ""

		case block.Type == enum.AnthropicContentBlockTypeToolResult && block.Content != nil && len(block.Content.Blocks) > 0:
			if err := s.replaceAnthropicBlocks(ctx, userID, userName, block.Content.Blocks); err != nil {
				return err
			}
		}
	}
	return nil
}

// processBase64Image 处理单个 base64 图片：计算 checksum → 查 DB → 不存在则上传 → 生成预签名 URL
//
//	@receiver s *imageService
//	@param ctx context.Context
//	@param userID uint
//	@param userName string
//	@param dataURL string data:image/...;base64,...
//	@return string presigned URL
//	@return error
//	@author centonhuang
//	@update 2026-04-07 10:00:00
func (s *imageService) processBase64Image(ctx context.Context, userID uint, userName string, dataURL string) (string, error) {
	log := logger.WithCtx(ctx)

	mediaType, base64Data, err := util.ParseDataURL(dataURL)
	if err != nil {
		log.Warn("[ImageService] Failed to parse data URL", zap.Error(err))
		return dataURL, nil
	}

	rawBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		log.Warn("[ImageService] Failed to decode base64", zap.Error(err))
		return dataURL, nil
	}

	checksum := util.ComputeImageChecksum(rawBytes)
	db := database.GetDBInstance(ctx)

	existing, dbErr := s.imageDAO.Get(db, &dbmodel.Image{
		UserName: userName,
		CheckSum: checksum,
	}, []string{"object_key"})

	if dbErr == nil && existing != nil {
		return s.presignAndReturn(ctx, log, userID, existing.ObjectKey, dataURL)
	}

	ext := util.ImageMediaTypeExtensions[mediaType]
	if ext == "" {
		ext = ".bin"
	}
	objectKey := fmt.Sprintf(constant.ImageObjectKeyTemplate, checksum, ext)

	if err := s.imageObjDAO.UploadObject(ctx, userID, objectKey, int64(len(rawBytes)), bytes.NewReader(rawBytes)); err != nil {
		log.Error("[ImageService] Failed to upload image to object storage", zap.String("objectKey", objectKey), zap.Error(err))
		return dataURL, nil
	}

	image := &dbmodel.Image{
		UserName:  userName,
		CheckSum:  checksum,
		MediaType: mediaType,
		ObjectKey: objectKey,
	}
	if err := s.imageDAO.Create(db, image); err != nil {
		log.Error("[ImageService] Failed to create image record", zap.String("checksum", checksum), zap.Error(err))
		return dataURL, nil
	}

	return s.presignAndReturn(ctx, log, userID, objectKey, dataURL)
}

// presignAndReturn generates a presigned URL for the image object
func (s *imageService) presignAndReturn(ctx context.Context, log *zap.Logger, userID uint, objectKey string, fallback string) (string, error) {
	presignedURL, err := s.imageObjDAO.PresignObject(ctx, userID, objectKey)
	if err != nil {
		log.Error("[ImageService] Failed to presign image", zap.String("objectKey", objectKey), zap.Error(err))
		return fallback, nil
	}
	return presignedURL.String(), nil
}

