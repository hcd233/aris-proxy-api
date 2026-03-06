// Package enum provides common enums for the application.
package enum

// ContentPartType 内容部分类型
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type ContentPartType = string

const (

	// ContentPartTypeText 文本内容
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ContentPartTypeText ContentPartType = "text"

	// ContentPartTypeImageURL 图片URL内容
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ContentPartTypeImageURL ContentPartType = "image_url"

	// ContentPartTypeInputAudio 音频输入内容
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ContentPartTypeInputAudio ContentPartType = "input_audio"

	// ContentPartTypeFile 文件内容
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ContentPartTypeFile ContentPartType = "file"

	// ContentPartTypeRefusal 拒绝内容
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ContentPartTypeRefusal ContentPartType = "refusal"
)
