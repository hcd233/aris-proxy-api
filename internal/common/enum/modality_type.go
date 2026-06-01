// Package enum provides common enums for the application.
package enum

// ModalityType 模态类型（输出类型）
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ModalityType = string

const (

	// ModalityTypeText 文本输出
	//
	//	@author centonhuang
	//	@update 2026-03-10 10:00:00
	ModalityTypeText ModalityType = "text"

	// ModalityTypeAudio 音频输出
	//
	//	@author centonhuang
	//	@update 2026-03-10 10:00:00
	ModalityTypeAudio ModalityType = "audio"
)
