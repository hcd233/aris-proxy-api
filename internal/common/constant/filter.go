package constant

// ── Filter (parser) error format strings ──
const (
	FilterErrEmptyFieldName = "empty field name in filter expression: %s"
	FilterErrInvalidExpr    = "invalid filter expression: %s"
	FilterErrUnknownField   = "unknown filter field: %s"
	FilterErrNullValueOp    = "operator %s not supported for NULL value"
	FilterErrUnsupportedOp  = "unsupported operator: %s"

	// ── Filter SQL fragments ──
	FilterSQLAND       = " AND "
	FilterSQLISNULL    = " IS NULL"
	FilterSQLISNOTNULL = " IS NOT NULL"
	FilterSQLLIKE      = " LIKE ?"
	FilterSQLNOTLIKE   = " NOT LIKE ?"
	FilterSQLEQ        = " = ?"
	FilterSQLNEQ       = " != ?"
	FilterSQLGT        = " > ?"
	FilterSQLLT        = " < ?"
	FilterSQLGTE       = " >= ?"
	FilterSQLLTE       = " <= ?"

	// ── Multi-value SQL fragments ──
	FilterSQLIN    = " IN (?)"
	FilterSQLNOTIN = " NOT IN (?)"
	FilterSQLOR    = " OR "

	// ── Multi-value parser errors ──
	FilterErrMultiValueWithComparison = "multi-value not supported with comparison operator: %s"
)
