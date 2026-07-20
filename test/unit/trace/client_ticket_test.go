package trace

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/redis/go-redis/v9"
)

func TestTraceClientTicketStore_IssueHashesAndConsumesOnce(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := cache.NewTraceClientTicketStore(client)

	ticket, expiresAt, err := store.Issue(context.Background(), 7, 10*time.Minute)
	if err != nil || ticket == "" || !expiresAt.After(time.Now()) {
		t.Fatalf("issue = %q, %v, %v", ticket, expiresAt, err)
	}
	for _, key := range mr.Keys() {
		if strings.Contains(key, ticket) {
			t.Fatalf("redis key leaked plaintext ticket: %s", key)
		}
	}

	userID, found, err := store.Consume(context.Background(), ticket)
	if err != nil || !found || userID != 7 {
		t.Fatalf("first consume = %d, %v, %v", userID, found, err)
	}
	_, found, err = store.Consume(context.Background(), ticket)
	if err != nil || found {
		t.Fatalf("second consume found = %v, err = %v", found, err)
	}
}

func TestTraceClientTicketStore_ValidateDoesNotConsume(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := cache.NewTraceClientTicketStore(client)

	ticket, _, err := store.Issue(context.Background(), 7, 10*time.Minute)
	if err != nil || ticket == "" {
		t.Fatalf("issue = %q, %v", ticket, err)
	}

	userID, found, err := store.Validate(context.Background(), ticket)
	if err != nil || !found || userID != 7 {
		t.Fatalf("validate = %d, %v, %v", userID, found, err)
	}

	userID, found, err = store.Validate(context.Background(), ticket)
	if err != nil || !found || userID != 7 {
		t.Fatalf("second validate = %d, %v, %v", userID, found, err)
	}

	userID, found, err = store.Consume(context.Background(), ticket)
	if err != nil || !found || userID != 7 {
		t.Fatalf("consume after validate = %d, %v, %v", userID, found, err)
	}

	_, found, err = store.Consume(context.Background(), ticket)
	if err != nil || found {
		t.Fatalf("second consume found = %v, err = %v", found, err)
	}
}
