package enum

// InputModality 模型支持的输入模态（模型能力集合成员）
//
//	@author centonhuang
//	@update 2026-07-29 18:00:00
type InputModality = string

const (

	// InputModalityText 文本输入
	//
	//	@author centonhuang
	//	@update 2026-07-29 18:00:00
	InputModalityText InputModality = "text"

	// InputModalityImage 图片输入
	//
	//	@author centonhuang
	//	@update 2026-07-29 18:00:00
	InputModalityImage InputModality = "image"

	// InputModalityPDF PDF 文件输入
	//
	//	@author centonhuang
	//	@update 2026-10-07 16:00:00
	InputModalityPDF InputModality = "pdf"

	// InputModalityVideo 视频输入
	//
	//	@author centonhuang
	//	@update 2026-10-07 16:00:00
	InputModalityVideo InputModality = "video"

	// InputModalityAudio 音频输入
	//
	//	@author centonhuang
	//	@update 2026-10-07 16:00:00
	InputModalityAudio InputModality = "audio"
)

// InputModalities 全部已知输入模态（枚举序即规范输出序；新增模态时在此扩展 + 前端加 chips）
//
//	@author centonhuang
//	@update 2026-10-07 16:00:00
var InputModalities = []InputModality{InputModalityText, InputModalityImage, InputModalityPDF, InputModalityVideo, InputModalityAudio}
