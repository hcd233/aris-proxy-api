package ui

import (
	"bytes"
	"errors"
	"testing"

	clientui "github.com/hcd233/aris-proxy-api/internal/client/ui"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

func TestRunWithSpinnerNonTTYRunsAction(t *testing.T) {
	t.Parallel()
	ran := false
	var out bytes.Buffer
	err := clientui.RunWithSpinner(nil, &out, "working...", func() error {
		ran = true
		return nil
	})
	if err != nil {
		t.Fatalf("RunWithSpinner error: %v", err)
	}
	if !ran {
		t.Fatal("action was not executed")
	}
}

func TestRunWithSpinnerNonTTYPropagatesError(t *testing.T) {
	t.Parallel()
	wantErr := ierr.New(ierr.ErrValidation, "boom")
	var out bytes.Buffer
	err := clientui.RunWithSpinner(nil, &out, "working...", func() error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunWithSpinner error = %v, want %v", err, wantErr)
	}
}
