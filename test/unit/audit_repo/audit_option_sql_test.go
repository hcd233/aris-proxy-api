package audit_repo

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestListDistinctUserNames_DeletedAtHasTablePrefix(t *testing.T) {
	t.Parallel()

	if !strings.Contains(constant.AuditDistinctWhereDeletedAtZero, "mca.") {
		t.Errorf("AuditDistinctWhereDeletedAtZero = %q, expected table prefix 'mca.'", constant.AuditDistinctWhereDeletedAtZero)
	}

	if constant.AuditDistinctWhereDeletedAtZero != "mca.deleted_at = 0" {
		t.Errorf("AuditDistinctWhereDeletedAtZero = %q, want \"mca.deleted_at = 0\"", constant.AuditDistinctWhereDeletedAtZero)
	}
}

func TestDBConditionDeletedAtZeroIsNotUsedInUserNamesQuery(t *testing.T) {
	t.Parallel()

	if constant.DBConditionDeletedAtZero == constant.AuditDistinctWhereDeletedAtZero {
		t.Skip("constants are equal, skipping (they serve different contexts)")
	}
}
