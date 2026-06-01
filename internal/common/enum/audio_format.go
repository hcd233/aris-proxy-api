// Package enum provides common enums for the application.
package enum

// AudioFormat 音频输出格式
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type AudioFormat = string

const (

	// AudioFormatWav WAV格式
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	AudioFormatWav AudioFormat = "wav"

	// AudioFormatAac AAC格式
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	AudioFormatAac AudioFormat = "aac"

	// AudioFormatMp3 MP3格式
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	AudioFormatMp3 AudioFormat = "mp3"

	// AudioFormatFlac FLAC格式
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	AudioFormatFlac AudioFormat = "flac"

	// AudioFormatOpus OPUS格式
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	AudioFormatOpus AudioFormat = "opus"

	// AudioFormatPcm16 PCM16格式
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	AudioFormatPcm16 AudioFormat = "pcm16"
)
