package enum

type Operator string

const (
	OpEqual    Operator = ":"   // 等于/包含
	OpNotEqual Operator = ":!"  // 不等于/不包含
	OpGreater  Operator = ":>"  // 大于
	OpLess     Operator = ":<"  // 小于
	OpGTE      Operator = ":>=" // 大于等于
	OpLTE      Operator = ":<=" // 小于等于
)
