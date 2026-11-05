package model

import "time"

// Blob 消息图片存储记录（按原始字节 sha256 去重）
//
//	status=uploaded ⇔ 对象存储中已有 storage_key 且 data 为 NULL；其余状态 data 非空。
//	表只追加与改状态，不删行，故不嵌入 BaseModel（无软删）。
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type Blob struct {
	ID           uint       `json:"id" gorm:"column:id;primaryKey;autoIncrement;comment:Blob ID"`
	SHA256       string     `json:"sha256" gorm:"column:sha256;type:char(64);not null;uniqueIndex:idx_blob_sha256;comment:原始字节SHA256"`
	Status       string     `json:"status" gorm:"column:status;not null;default:'pending';index:idx_blob_status_retry,priority:1;comment:上传状态"`
	Data         []byte     `json:"-" gorm:"column:data;comment:未上传时暂存的图片字节"`
	StorageKey   string     `json:"storage_key" gorm:"column:storage_key;not null;default:'';comment:对象key"`
	Mime         string     `json:"mime" gorm:"column:mime;not null;default:'';comment:存储版本MIME"`
	Size         int64      `json:"size" gorm:"column:size;not null;default:0;comment:存储版本字节数"`
	Width        int        `json:"width" gorm:"column:width;not null;default:0;comment:存储版本宽"`
	Height       int        `json:"height" gorm:"column:height;not null;default:0;comment:存储版本高"`
	OriginalSize int64      `json:"original_size" gorm:"column:original_size;not null;default:0;comment:原始字节数"`
	Attempts     int        `json:"attempts" gorm:"column:attempts;not null;default:0;comment:上传尝试次数"`
	LastError    string     `json:"last_error" gorm:"column:last_error;type:varchar(512);not null;default:'';comment:最近一次上传失败原因"`
	NextRetryAt  *time.Time `json:"next_retry_at" gorm:"column:next_retry_at;index:idx_blob_status_retry,priority:2;comment:下次重试时间"`
	CreatedAt    time.Time  `json:"created_at" gorm:"comment:创建时间"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"comment:更新时间"`
}
