package dto

import (
	"math"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// PricingDTO 模型定价（wire 展示单位：货币单位每 1M tokens）。
// currency=="" ⇔ 未计价；currency=="USD" 时 rules 须恰好含一条无条件默认规则
// （time_windows 空 + context_min=0 + context_max=0），数组顺序=匹配优先级。
type PricingDTO struct {
	Currency enum.Currency    `json:"currency,omitempty" enum:",USD" doc:"计价币种；空=未计价，USD=已计价"`
	Rules    []PricingRuleDTO `json:"rules,omitempty" maxItems:"32" doc:"定价规则（数组顺序=匹配优先级）"`
}

// TimeWindowDTO 时段窗口（循环生效）
type TimeWindowDTO struct {
	Days     []int  `json:"days,omitempty" maxItems:"7" minimum:"1" maximum:"7" doc:"周几（1=周一…7=周日），空=每天"`
	Start    string `json:"start" minLength:"5" maxLength:"5" doc:"起始时刻 HH:MM（含）"`
	End      string `json:"end" minLength:"5" maxLength:"5" doc:"结束时刻 HH:MM（不含）；小于 start 表示跨午夜"`
	Timezone string `json:"timezone,omitempty" maxLength:"64" doc:"IANA 时区，缺省 UTC"`
}

// PricingRuleDTO 定价规则（单价：货币单位/1M tokens）
type PricingRuleDTO struct {
	TimeWindows        []TimeWindowDTO `json:"time_windows,omitempty" maxItems:"8" doc:"时段窗口（空=全时段，多窗口 OR）"`
	ContextMin         int64           `json:"context_min,omitempty" minimum:"0" doc:"prompt token 下界（含）"`
	ContextMax         int64           `json:"context_max,omitempty" minimum:"0" doc:"上界（不含），0=无上限"`
	InputPrice         float64         `json:"input_price" minimum:"0" maximum:"1000000" doc:"输入单价"`
	OutputPrice        float64         `json:"output_price" minimum:"0" maximum:"1000000" doc:"输出单价"`
	CacheCreationPrice float64         `json:"cache_creation_price" minimum:"0" maximum:"1000000" doc:"缓存创建单价"`
	CacheReadPrice     float64         `json:"cache_read_price" minimum:"0" maximum:"1000000" doc:"缓存读取单价"`
}

// PriceMicroFromDisplay 展示单位 → 微单位（1e-6 货币单位），半入。
//
//	@param v float64 展示单位价格
//	@return int64 微单位价格
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func PriceMicroFromDisplay(v float64) int64 {
	return int64(math.Round(v * 1e6))
}

// PriceDisplayFromMicro 微单位 → 展示单位。
//
//	@param m int64 微单位价格
//	@return float64 展示单位价格
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func PriceDisplayFromMicro(m int64) float64 {
	return float64(m) / 1e6
}
