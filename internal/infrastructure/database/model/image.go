// Package model defines the database schema for the model.
package model

// Image 图片数据库模型，存储用户请求中 base64 图片的对象存储信息
//
//	@author centonhuang
//	@update 2026-04-07 10:00:00
type Image struct {
	BaseModel
	ID        uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:图片ID"`
	UserName  string `json:"user_name" gorm:"column:user_name;not null;default:'';index:idx_image_checksum,priority:1;comment:用户名(API Key Name)"`
	CheckSum  string `json:"check_sum" gorm:"column:check_sum;not null;default:'';index:idx_image_checksum,priority:2;comment:图片内容SHA256校验和"`
	MediaType string `json:"media_type" gorm:"column:media_type;not null;default:'';comment:媒体类型(image/jpeg, image/png等)"`
	ObjectKey string `json:"object_key" gorm:"column:object_key;not null;default:'';comment:对象存储中的文件名"`
}
