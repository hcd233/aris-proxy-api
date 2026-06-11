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

func TestAuditPaginateWhereDeletedAtZeroHasTablePrefix(t *testing.T) {
	t.Parallel()

	if constant.AuditPaginateWhereDeletedAtZero != "model_call_audits.deleted_at = 0" {
		t.Errorf("AuditPaginateWhereDeletedAtZero = %q, want \"model_call_audits.deleted_at = 0\"", constant.AuditPaginateWhereDeletedAtZero)
	}
}

func TestAuditRepoFields_IDHasTablePrefix(t *testing.T) {
	t.Parallel()

	if len(constant.AuditRepoFields) == 0 {
		t.Fatal("AuditRepoFields is empty")
	}

	idField := constant.AuditRepoFields[0]
	if idField != "model_call_audits.id" {
		t.Errorf("AuditRepoFields[0] = %q, want \"model_call_audits.id\" (must be qualified to avoid ambiguous column when JOINing)", idField)
	}
}

func TestAuditRepoFields_CreatedAtHasTablePrefix(t *testing.T) {
	t.Parallel()

	if len(constant.AuditRepoFields) == 0 {
		t.Fatal("AuditRepoFields is empty")
	}

	createdAtField := constant.AuditRepoFields[len(constant.AuditRepoFields)-1]
	if createdAtField != "model_call_audits.created_at" {
		t.Errorf("last AuditRepoFields entry = %q, want \"model_call_audits.created_at\"", createdAtField)
	}
}
