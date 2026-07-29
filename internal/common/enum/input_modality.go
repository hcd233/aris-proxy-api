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
)

// InputModalities 全部已知输入模态（新增模态时仅在此扩展 + 前端加开关）
//
//	@author centonhuang
//	@update 2026-07-29 18:00:00
var InputModalities = []InputModality{InputModalityText, InputModalityImage}
