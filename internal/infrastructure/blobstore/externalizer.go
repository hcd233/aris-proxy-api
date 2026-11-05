package blobstore

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/domain/blob"
	"github.com/hcd233/aris-mass/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
	"github.com/hcd233/aris-mass/internal/logger"
)

// ExternalizeStats 一次外置的统计
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type ExternalizeStats struct {
	Uploaded, Pending, Unprocessable, Reused, Failed int
	// SavedBytes message 中被移走的 data URI 字节数
	SavedBytes int64
}

// Externalizer 把 message 中的 base64 图片写入 blobs 表与对象存储，并替换为引用
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type Externalizer struct {
	db    *gorm.DB
	store blob.ObjectStore
	// TaskTimeout 单次外置中对象存储 PUT 的总时长上限；到期后剩余图片直接写 pending
	TaskTimeout time.Duration
}

// NewExternalizer 构造图片外置器
//
//	@param db *gorm.DB
//	@param store blob.ObjectStore 为 nil 时只写 pending，由重试任务在配置后补传
//	@return *Externalizer
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func NewExternalizer(db *gorm.DB, store blob.ObjectStore) *Externalizer {
	return &Externalizer{db: db, store: store, TaskTimeout: constant.BlobTaskTimeout}
}

// imageSite 一处待外置的图片部分
type imageSite struct {
	msg, part int
	sha       string
	urlLen    int
}

// Externalize 外置 msgs 中的 data URI 图片
//
//	返回新切片：有图片被替换的消息是副本，其余保持原指针；不修改入参。
//	blobs 行写入失败的图片保留 data URI（checksum 与引用形态一致），之后由存量迁移补外置。
//
//	@receiver e *Externalizer
//	@param ctx context.Context
//	@param msgs []*vo.UnifiedMessage
//	@return []*vo.UnifiedMessage
//	@return ExternalizeStats
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func (e *Externalizer) Externalize(ctx context.Context, msgs []*vo.UnifiedMessage) ([]*vo.UnifiedMessage, ExternalizeStats) {
	var stats ExternalizeStats
	sites, raws, order := e.collect(ctx, msgs, &stats)
	if len(sites) == 0 {
		return msgs, stats
	}

	db := e.db.WithContext(ctx)
	meta, err := e.loadMeta(db, order)
	if err != nil {
		logger.WithCtx(ctx).Error("[BlobExternalizer] Failed to load blob meta", zap.Error(err))
		stats.Failed += len(order)
		return msgs, stats
	}
	stats.Reused = len(meta)

	putCtx, cancel := context.WithTimeout(ctx, e.TaskTimeout)
	defer cancel()
	var created []string
	for _, sha := range order {
		if _, ok := meta[sha]; ok {
			continue
		}
		if e.persistNew(ctx, putCtx, db, sha, raws[sha], &stats) {
			created = append(created, sha)
		}
	}
	if len(created) > 0 {
		fresh, err := e.loadMeta(db, created)
		if err != nil {
			logger.WithCtx(ctx).Error("[BlobExternalizer] Failed to reload blob meta", zap.Error(err))
		}
		maps.Copy(meta, fresh)
		stats.Failed += len(created) - len(fresh)
	}
	out := replaceSites(msgs, sites, meta, &stats)
	return out, stats
}

// collect 找出所有 data URI 图片，按 sha256 去重保留首次出现顺序
func (e *Externalizer) collect(ctx context.Context, msgs []*vo.UnifiedMessage, stats *ExternalizeStats) (sites []imageSite, raws map[string][]byte, order []string) {
	raws = map[string][]byte{}
	for mi, m := range msgs {
		if m == nil || m.Content == nil {
			continue
		}
		for pi, p := range m.Content.Parts {
			if p == nil || p.Type != enum.ContentPartTypeImageURL || !strings.HasPrefix(p.ImageURL, constant.DataURLPrefix) {
				continue
			}
			_, raw, ok := vo.ParseImageDataURI(p.ImageURL)
			if !ok {
				logger.WithCtx(ctx).Warn("[BlobExternalizer] Invalid image data uri, kept as is", zap.Int("length", len(p.ImageURL)))
				stats.Failed++
				continue
			}
			sha := vo.SHA256Hex(raw)
			if _, seen := raws[sha]; !seen {
				raws[sha] = raw
				order = append(order, sha)
			}
			sites = append(sites, imageSite{msg: mi, part: pi, sha: sha, urlLen: len(p.ImageURL)})
		}
	}
	return sites, raws, order
}

func (e *Externalizer) loadMeta(db *gorm.DB, shas []string) (map[string]*dbmodel.Blob, error) {
	rows, err := dao.GetBlobDAO().FindMetaBySHA256s(db, shas)
	if err != nil {
		return nil, err
	}
	meta := make(map[string]*dbmodel.Blob, len(rows))
	for _, row := range rows {
		meta[row.SHA256] = row
	}
	return meta, nil
}

// persistNew 压缩、尽力上传并写入一张新图的 blobs 行；行写入失败返回 false
func (e *Externalizer) persistNew(ctx, putCtx context.Context, db *gorm.DB, sha string, raw []byte, stats *ExternalizeStats) bool {
	res := Compress(raw)
	row := &dbmodel.Blob{
		SHA256:       sha,
		StorageKey:   StorageKey(sha, res.Mime),
		Mime:         res.Mime,
		Size:         int64(len(res.Data)),
		Width:        res.Width,
		Height:       res.Height,
		OriginalSize: int64(len(raw)),
	}
	switch {
	case res.Unprocessable:
		row.Status, row.Data = enum.BlobStatusUnprocessable, res.Data
	case e.tryPut(putCtx, row.StorageKey, res, row):
		row.Status = enum.BlobStatusUploaded
	default:
		next := time.Now().UTC().Add(constant.BlobRetryBackoff[0])
		row.Status, row.Data, row.Attempts, row.NextRetryAt = enum.BlobStatusPending, res.Data, 1, &next
	}
	if err := dao.GetBlobDAO().InsertIgnoreConflict(db, row); err != nil {
		logger.WithCtx(ctx).Error("[BlobExternalizer] Failed to insert blob", zap.String("sha256", sha), zap.Error(err))
		return false
	}
	switch row.Status {
	case enum.BlobStatusUploaded:
		stats.Uploaded++
	case enum.BlobStatusPending:
		stats.Pending++
	default:
		stats.Unprocessable++
	}
	return true
}

// tryPut 对象存储已配置且总时长未耗尽时上传一次；失败原因记入 row.LastError
func (e *Externalizer) tryPut(putCtx context.Context, key string, res CompressResult, row *dbmodel.Blob) bool {
	if e.store == nil {
		return false
	}
	if err := putCtx.Err(); err != nil {
		row.LastError = TruncateError(err)
		return false
	}
	ctx, cancel := context.WithTimeout(putCtx, constant.BlobPutTimeout)
	defer cancel()
	if err := e.store.Put(ctx, key, res.Mime, res.Data); err != nil {
		logger.WithCtx(putCtx).Warn("[BlobExternalizer] Upload failed, kept pending", zap.String("key", key), zap.Error(err))
		row.LastError = TruncateError(err)
		return false
	}
	return true
}

// replaceSites 把已有 blobs 行的图片部分替换为引用（写在副本上）
func replaceSites(msgs []*vo.UnifiedMessage, sites []imageSite, meta map[string]*dbmodel.Blob, stats *ExternalizeStats) []*vo.UnifiedMessage {
	out := slices.Clone(msgs)
	copied := map[int]bool{}
	for _, s := range sites {
		row, ok := meta[s.sha]
		if !ok {
			continue
		}
		if !copied[s.msg] {
			cp := *msgs[s.msg]
			cp.Content = &vo.UnifiedContent{Text: cp.Content.Text, Parts: slices.Clone(cp.Content.Parts)}
			out[s.msg] = &cp
			copied[s.msg] = true
		}
		part := *out[s.msg].Content.Parts[s.part]
		part.ImageURL, part.ImageMime, part.ImageSize = vo.ImageRefOf(s.sha), row.Mime, row.Size
		out[s.msg].Content.Parts[s.part] = &part
		stats.SavedBytes += int64(s.urlLen)
	}
	return out
}

// TruncateError 错误信息截断到 last_error 列长度
//
//	@param err error
//	@return string
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func TruncateError(err error) string {
	msg := err.Error()
	if len(msg) > constant.BlobLastErrorMaxLen {
		return msg[:constant.BlobLastErrorMaxLen]
	}
	return msg
}
