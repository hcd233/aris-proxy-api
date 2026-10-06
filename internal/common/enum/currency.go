package enum

// Currency 计价币种；空值表示未计价。
type Currency string

const (
	CurrencyNone Currency = ""
	CurrencyCNY  Currency = "CNY"
	CurrencyUSD  Currency = "USD"
)

// Valid 币种是否合法（含空值=未计价）。
//
//	@receiver c Currency
//	@return bool
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func (c Currency) Valid() bool {
	switch c {
	case CurrencyNone, CurrencyCNY, CurrencyUSD:
		return true
	default:
		return false
	}
}
