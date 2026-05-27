package session_share

import (
	"testing"
)

func TestCreateShare(t *testing.T) {
	t.Run("normal_create", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})

	t.Run("session_not_found", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})

	t.Run("session_not_owned", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})
}

func TestGetShareContent(t *testing.T) {
	t.Run("normal_get", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})

	t.Run("share_not_found", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})

	t.Run("share_expired", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})
}

func TestListShares(t *testing.T) {
	t.Run("normal_list", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})

	t.Run("empty_list", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})
}

func TestDeleteShare(t *testing.T) {
	t.Run("normal_delete", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})

	t.Run("share_not_found", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})
}
