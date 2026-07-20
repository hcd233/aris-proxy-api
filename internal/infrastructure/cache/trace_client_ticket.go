package cache

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/redis/go-redis/v9"
)

var consumeTraceClientTicketScript = redis.NewScript(`
local value = redis.call('GET', KEYS[1])
if not value then
  return ''
end
redis.call('DEL', KEYS[1])
return value
`)

type traceClientTicketStore struct {
	cache *redis.Client
}

func NewTraceClientTicketStore(cache *redis.Client) port.TraceClientTicketStore {
	return &traceClientTicketStore{cache: cache}
}

func (s *traceClientTicketStore) Issue(
	ctx context.Context,
	userID uint,
	ttl time.Duration,
) (string, time.Time, error) {
	if s.cache == nil {
		return "", time.Time{}, ierr.New(ierr.ErrInternal, constant.TraceClientTicketCacheNilMessage)
	}
	if userID == 0 || ttl <= 0 {
		return "", time.Time{}, ierr.New(ierr.ErrValidation, constant.TraceClientTicketInvalidMessage)
	}

	random := make([]byte, constant.TraceClientTicketRandomBytes)
	if _, err := rand.Read(random); err != nil {
		return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, err, constant.TraceClientTicketGenerateMessage)
	}
	ticket := base64.RawURLEncoding.EncodeToString(random)
	key := traceClientTicketKey(ticket)
	if err := s.cache.Set(ctx, key, strconv.FormatUint(uint64(userID), constant.DecimalBase), ttl).Err(); err != nil {
		return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, err, constant.TraceClientTicketStoreMessage)
	}
	return ticket, time.Now().UTC().Add(ttl), nil
}

func (s *traceClientTicketStore) Consume(
	ctx context.Context,
	ticket string,
) (userID uint, ok bool, err error) {
	if s.cache == nil {
		return 0, false, ierr.New(ierr.ErrInternal, constant.TraceClientTicketCacheNilMessage)
	}
	if ticket == "" {
		return 0, false, nil
	}

	value, runErr := consumeTraceClientTicketScript.Run(
		ctx,
		s.cache,
		[]string{traceClientTicketKey(ticket)},
	).Text()
	if runErr != nil {
		return 0, false, ierr.Wrap(ierr.ErrInternal, runErr, constant.TraceClientTicketConsumeMessage)
	}
	if value == "" {
		return 0, false, nil
	}
	parsed, parseErr := strconv.ParseUint(value, constant.DecimalBase, strconv.IntSize)
	if parseErr != nil {
		return 0, false, ierr.Wrap(ierr.ErrInternal, parseErr, constant.TraceClientTicketOwnerParseMessage)
	}
	return uint(parsed), true, nil
}

func (s *traceClientTicketStore) Validate(
	ctx context.Context,
	ticket string,
) (userID uint, ok bool, err error) {
	if s.cache == nil {
		return 0, false, ierr.New(ierr.ErrInternal, constant.TraceClientTicketCacheNilMessage)
	}
	if ticket == "" {
		return 0, false, nil
	}

	value, getErr := s.cache.Get(ctx, traceClientTicketKey(ticket)).Result()
	if getErr != nil {
		if getErr == redis.Nil {
			return 0, false, nil
		}
		return 0, false, ierr.Wrap(ierr.ErrInternal, getErr, constant.TraceClientTicketConsumeMessage)
	}
	if value == "" {
		return 0, false, nil
	}
	parsed, parseErr := strconv.ParseUint(value, constant.DecimalBase, strconv.IntSize)
	if parseErr != nil {
		return 0, false, ierr.Wrap(ierr.ErrInternal, parseErr, constant.TraceClientTicketOwnerParseMessage)
	}
	return uint(parsed), true, nil
}

func traceClientTicketKey(ticket string) string {
	digest := sha256.Sum256([]byte(ticket))
	return fmt.Sprintf(constant.TraceClientTicketKeyTemplate, hex.EncodeToString(digest[:]))
}
