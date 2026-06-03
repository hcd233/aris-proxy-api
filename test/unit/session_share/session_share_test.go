package session_share

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	sessionquery "github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/samber/lo"
)

type mockShareEntry struct {
	userID    uint
	sessionID uint
	createdAt time.Time
	expiresAt time.Time
}

type mockShareCache struct {
	shares         map[string]*mockShareEntry
	userShares     map[uint][]string
	sharedSessions map[uint]bool
	createErr      error
	getErr         error
	deleteErr      error
	listErr        error
}

func newMockShareCache() *mockShareCache {
	return &mockShareCache{
		shares:         make(map[string]*mockShareEntry),
		userShares:     make(map[uint][]string),
		sharedSessions: make(map[uint]bool),
	}
}

func (m *mockShareCache) CreateShare(_ context.Context, userID, sessionID uint, ttl time.Duration) (string, time.Time, error) {
	if m.createErr != nil {
		return "", time.Time{}, m.createErr
	}
	if m.sharedSessions[sessionID] {
		return "", time.Time{}, ierr.New(ierr.ErrDataExists, "session already has an active share")
	}
	shareID := "test-share-id-" + time.Now().Format("150405")
	now := time.Now()
	m.shares[shareID] = &mockShareEntry{
		userID:    userID,
		sessionID: sessionID,
		createdAt: now,
		expiresAt: now.Add(ttl),
	}
	m.userShares[userID] = append(m.userShares[userID], shareID)
	m.sharedSessions[sessionID] = true
	return shareID, now.Add(ttl), nil
}

func (m *mockShareCache) GetShareSessionID(_ context.Context, shareID string) (uint, error) {
	if m.getErr != nil {
		return 0, m.getErr
	}
	entry, ok := m.shares[shareID]
	if !ok {
		return 0, ierr.New(ierr.ErrDataNotExists, "share link not found or expired")
	}
	return entry.sessionID, nil
}

func (m *mockShareCache) DeleteShare(_ context.Context, userID uint, shareID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	entry, ok := m.shares[shareID]
	if !ok || entry.userID != userID {
		return ierr.New(ierr.ErrDataNotExists, "share link not found or not owned by user")
	}
	delete(m.shares, shareID)
	filtered := make([]string, 0, len(m.userShares[userID]))
	for _, id := range m.userShares[userID] {
		if id != shareID {
			filtered = append(filtered, id)
		}
	}
	m.userShares[userID] = filtered

	remainingForSession := false
	for _, otherID := range m.userShares[userID] {
		if e, exists := m.shares[otherID]; exists && e.sessionID == entry.sessionID {
			remainingForSession = true
			break
		}
	}
	if !remainingForSession {
		delete(m.sharedSessions, entry.sessionID)
	}
	return nil
}

func (m *mockShareCache) ListUserShares(_ context.Context, userID uint, page, pageSize int) ([]*dto.ShareItem, *model.PageInfo, error) {
	if m.listErr != nil {
		return nil, nil, m.listErr
	}
	ids := m.userShares[userID]
	items := make([]*dto.ShareItem, 0, len(ids))
	for _, id := range ids {
		entry := m.shares[id]
		if entry == nil {
			continue
		}
		items = append(items, &dto.ShareItem{
			ShareID:   id,
			SessionID: entry.sessionID,
			CreatedAt: entry.createdAt,
			ExpiresAt: entry.expiresAt,
		})
	}
	return items, &model.PageInfo{Page: page, PageSize: pageSize, Total: int64(len(items))}, nil
}

func (m *mockShareCache) GetSessionShareID(_ context.Context, sessionID uint) (string, error) {
	if !m.sharedSessions[sessionID] {
		return "", nil
	}
	for shareID, entry := range m.shares {
		if entry.sessionID == sessionID {
			return shareID, nil
		}
	}
	return "mock-share-id", nil
}

type mockGetSessionByUserHandler struct {
	view map[uint]*sessionquery.SessionDetailView
	err  error
}

func (m *mockGetSessionByUserHandler) Handle(_ context.Context, q sessionquery.GetSessionByUserQuery) (*sessionquery.SessionDetailView, error) {
	if m.err != nil {
		return nil, m.err
	}
	v, ok := m.view[q.SessionID]
	if !ok {
		return nil, ierr.New(ierr.ErrDataNotExists, "session not found")
	}
	return v, nil
}

type mockListSessionsByUserHandler struct{}

func (m *mockListSessionsByUserHandler) Handle(_ context.Context, _ sessionquery.ListSessionsByUserQuery) ([]*sessionquery.SessionSummaryView, *model.PageInfo, error) {
	return nil, nil, nil
}

func ctxWithUser(userID uint, permission enum.Permission) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, constant.CtxKeyUserID, userID)
	ctx = context.WithValue(ctx, constant.CtxKeyPermission, permission)
	return ctx
}

func testSessionView(sessionID uint) *sessionquery.SessionDetailView {
	return &sessionquery.SessionDetailView{
		ID:         sessionID,
		APIKeyName: "test-key",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Metadata:   map[string]string{"k": "v"},
		Messages: []*sessionquery.MessageView{
			{ID: 1, Model: "gpt-4", Message: &vo.UnifiedMessage{}, CreatedAt: time.Now()},
		},
		Tools: []*sessionquery.ToolView{
			{ID: 2, Tool: &vo.UnifiedTool{}, CreatedAt: time.Now()},
		},
	}
}

func newTestHandler(sc cache.ShareCache, getByUser sessionquery.GetSessionByUserHandler) handler.SessionHandler {
	return handler.NewSessionHandler(handler.SessionDependencies{
		ListByUser: &mockListSessionsByUserHandler{},
		GetByUser:  getByUser,
		ShareCache: sc,
	})
}

func TestCreateShare_Success(t *testing.T) {
	sc := newMockShareCache()
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{1: testSessionView(1)}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, err := h.HandleCreateShare(ctx, &dto.CreateShareReq{Body: &dto.CreateShareReqBody{SessionID: 1}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rsp.Body.ShareID == "" {
		t.Error("expected non-empty shareID")
	}
	if rsp.Body.ExpiresAt.IsZero() {
		t.Error("expected non-zero expiresAt")
	}
	if rsp.Body.Error != nil {
		t.Errorf("expected no error in response, got code=%d", rsp.Body.Error.Code)
	}
}

func TestCreateShare_SessionNotFound(t *testing.T) {
	sc := newMockShareCache()
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, _ := h.HandleCreateShare(ctx, &dto.CreateShareReq{Body: &dto.CreateShareReqBody{SessionID: 999}})
	if rsp.Body.Error == nil {
		t.Error("expected error in response for non-existent session")
	}
	if rsp.Body.Error.Code != ierr.ErrDataNotExists.BizError().Code {
		t.Errorf("error code = %d, want %d (DataNotExists)", rsp.Body.Error.Code, ierr.ErrDataNotExists.BizError().Code)
	}
}

func TestCreateShare_CacheError(t *testing.T) {
	sc := newMockShareCache()
	sc.createErr = ierr.Wrap(ierr.ErrInternal, nil, "redis down")
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{1: testSessionView(1)}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, _ := h.HandleCreateShare(ctx, &dto.CreateShareReq{Body: &dto.CreateShareReqBody{SessionID: 1}})
	if rsp.Body.Error == nil {
		t.Error("expected error in response for cache failure")
	}
}

// TestCreateShare_NilBodyRejected 回归用例：huma 在 body 缺失时会传入空 Body 或 nil Body，
// handler 必须显式校验，而不是当作 SessionID=0 处理。
//
//	@author centonhuang
//	@update 2026-05-28 14:35:00
func TestCreateShare_NilBodyRejected(t *testing.T) {
	sc := newMockShareCache()
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{1: testSessionView(1)}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, _ := h.HandleCreateShare(ctx, &dto.CreateShareReq{Body: nil})
	if rsp.Body.Error == nil {
		t.Fatal("expected validation error when body is nil")
	}
	if rsp.Body.Error.Code != ierr.ErrValidation.BizError().Code {
		t.Errorf("error code = %d, want Validation (%d)", rsp.Body.Error.Code, ierr.ErrValidation.BizError().Code)
	}
	// 必须没有写入任何 share 记录
	if len(sc.shares) != 0 {
		t.Errorf("expected no shares to be created on nil body, got %d", len(sc.shares))
	}
}

func TestListShares_Success(t *testing.T) {
	sc := newMockShareCache()
	sc.CreateShare(context.Background(), 42, 1, constant.ShareTTLDefault)
	sc.CreateShare(context.Background(), 42, 2, constant.ShareTTLDefault)
	getByUser := &mockGetSessionByUserHandler{}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, err := h.HandleListShares(ctx, &dto.ListSharesReq{PageParam: model.PageParam{Page: 1, PageSize: 10}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rsp.Body.Shares) != 2 {
		t.Errorf("shares count = %d, want 2", len(rsp.Body.Shares))
	}
}

func TestListShares_Empty(t *testing.T) {
	sc := newMockShareCache()
	getByUser := &mockGetSessionByUserHandler{}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, err := h.HandleListShares(ctx, &dto.ListSharesReq{PageParam: model.PageParam{Page: 1, PageSize: 10}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rsp.Body.Shares) != 0 {
		t.Errorf("shares count = %d, want 0", len(rsp.Body.Shares))
	}
}

func TestDeleteShare_Success(t *testing.T) {
	sc := newMockShareCache()
	shareID, _, _ := sc.CreateShare(context.Background(), 42, 1, constant.ShareTTLDefault)
	getByUser := &mockGetSessionByUserHandler{}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, err := h.HandleDeleteShare(ctx, &dto.DeleteShareReq{ShareID: shareID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rsp.Body.Error != nil {
		t.Errorf("expected no error, got code=%d", rsp.Body.Error.Code)
	}
	if _, ok := sc.shares[shareID]; ok {
		t.Error("share should be deleted from cache")
	}
}

func TestDeleteShare_NotOwner(t *testing.T) {
	sc := newMockShareCache()
	shareID, _, _ := sc.CreateShare(context.Background(), 42, 1, constant.ShareTTLDefault)
	getByUser := &mockGetSessionByUserHandler{}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(99, enum.PermissionUser)

	rsp, _ := h.HandleDeleteShare(ctx, &dto.DeleteShareReq{ShareID: shareID})
	if rsp.Body.Error == nil {
		t.Error("expected error when deleting another user's share")
	}
	if rsp.Body.Error.Code != ierr.ErrDataNotExists.BizError().Code {
		t.Errorf("error code = %d, want DataNotExists (%d)", rsp.Body.Error.Code, ierr.ErrDataNotExists.BizError().Code)
	}
	if _, ok := sc.shares[shareID]; !ok {
		t.Error("share should still exist after failed delete by non-owner")
	}
}

func TestDeleteShare_Nonexistent(t *testing.T) {
	sc := newMockShareCache()
	getByUser := &mockGetSessionByUserHandler{}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, _ := h.HandleDeleteShare(ctx, &dto.DeleteShareReq{ShareID: "nonexistent"})
	if rsp.Body.Error == nil {
		t.Error("expected error when deleting non-existent share")
	}
}

func TestCreateShare_AlreadyShared(t *testing.T) {
	sc := newMockShareCache()
	sc.sharedSessions[1] = true
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{1: testSessionView(1)}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, _ := h.HandleCreateShare(ctx, &dto.CreateShareReq{Body: &dto.CreateShareReqBody{SessionID: 1}})
	if rsp.Body.Error == nil {
		t.Error("expected DataExists error for already-shared session")
	}
	if rsp.Body.Error.Code != ierr.ErrDataExists.BizError().Code {
		t.Errorf("error code = %d, want %d (DataExists)", rsp.Body.Error.Code, ierr.ErrDataExists.BizError().Code)
	}
}

func TestHandleGetSessionByUser_IsShared(t *testing.T) {
	sc := newMockShareCache()
	sc.sharedSessions[1] = true
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{1: testSessionView(1)}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, _ := h.HandleGetSessionByUser(ctx, &dto.GetSessionByUserReq{SessionID: 1})
	if rsp.Body.Session == nil {
		t.Fatal("expected non-nil session")
	}
	if rsp.Body.Session.ShareID == "" {
		t.Error("expected non-empty ShareID for shared session")
	}
}

func TestHandleGetSessionByUser_NotShared(t *testing.T) {
	sc := newMockShareCache()
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{1: testSessionView(1)}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, _ := h.HandleGetSessionByUser(ctx, &dto.GetSessionByUserReq{SessionID: 1})
	if rsp.Body.Session == nil {
		t.Fatal("expected non-nil session")
	}
	if rsp.Body.Session.ShareID != "" {
		t.Error("expected empty ShareID for non-shared session")
	}
}

func TestParseExpiresIn(t *testing.T) {
	past := time.Now().Add(-time.Hour).Unix()

	tests := []struct {
		name      string
		expiresIn string
		customAt  *int64
		want      time.Duration
		wantErr   bool
	}{
		{"default (empty)", "", nil, constant.ShareTTL1Day, false},
		{"1 day", "1d", nil, constant.ShareTTL1Day, false},
		{"1 week", "7d", nil, constant.ShareTTL1Week, false},
		{"1 week alt", "1w", nil, constant.ShareTTL1Week, false},
		{"1 month", "30d", nil, constant.ShareTTL1Month, false},
		{"1 month alt", "1M", nil, constant.ShareTTL1Month, false},
		{"never", "never", nil, constant.ShareTTLNeverExpire, false},
		{"unknown defaults", "something", nil, constant.ShareTTLDefault, false},
		{"custom missing at", "custom", nil, 0, true},
		{"custom past time", "custom", &past, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := handler.ParseExpiresIn(tt.expiresIn, tt.customAt)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseExpiresIn(%q) = %v, want %v", tt.expiresIn, got, tt.want)
			}
		})
	}

	// custom valid: use approximate match
	t.Run("custom valid", func(t *testing.T) {
		now := time.Now()
		target := now.Add(48 * time.Hour)
		customAt := lo.ToPtr(target.Unix())
		got, err := handler.ParseExpiresIn("custom", customAt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedTTL := time.Until(target)
		diff := got - expectedTTL
		if diff < 0 {
			diff = -diff
		}
		if diff > time.Second {
			t.Errorf("ttl = %v, want ~%v (diff %v)", got, expectedTTL, diff)
		}
	})
}

// TestCreateShareReq_DTOFollowsHumaBodyConvention 防回归：CreateShareReq 必须按 huma 框架的
// "包装结构 + Body 字段" 模式定义，否则 POST body 反序列化会被 huma 忽略，导致 SessionID
// 始终是零值（线上曾因此返回错位 session）。
//
//	@author centonhuang
//	@update 2026-05-28 14:35:00
func TestCreateShareReq_DTOFollowsHumaBodyConvention(t *testing.T) {
	reqType := reflect.TypeOf(dto.CreateShareReq{})
	bodyField, ok := reqType.FieldByName("Body")
	if !ok {
		t.Fatal("CreateShareReq must have a Body field for huma JSON body binding")
	}
	if bodyField.Tag.Get("json") != "body" {
		t.Errorf(`CreateShareReq.Body json tag = %q, want "body"`, bodyField.Tag.Get("json"))
	}
	// SessionID 必须落在 Body 子结构里，不能直接挂在顶层
	if _, exists := reqType.FieldByName("SessionID"); exists {
		t.Error("CreateShareReq must NOT have top-level SessionID field; it belongs in CreateShareReqBody")
	}

	bodyType := reflect.TypeOf(dto.CreateShareReqBody{})
	sessionIDField, ok := bodyType.FieldByName("SessionID")
	if !ok {
		t.Fatal("CreateShareReqBody must have SessionID field")
	}
	if sessionIDField.Tag.Get("json") != "sessionId" {
		t.Errorf(`CreateShareReqBody.SessionID json tag = %q, want "sessionId"`, sessionIDField.Tag.Get("json"))
	}
	if sessionIDField.Tag.Get("minimum") != "1" {
		t.Errorf(`CreateShareReqBody.SessionID minimum tag = %q, want "1" (reject zero values)`, sessionIDField.Tag.Get("minimum"))
	}

	expiresInField, ok := bodyType.FieldByName("ExpiresIn")
	if !ok {
		t.Fatal("CreateShareReqBody must have ExpiresIn field")
	}
	if expiresInField.Tag.Get("json") != "expiresIn" {
		t.Errorf(`CreateShareReqBody.ExpiresIn json tag = %q, want "expiresIn"`, expiresInField.Tag.Get("json"))
	}

	expiresAtField, ok := bodyType.FieldByName("ExpiresAt")
	if !ok {
		t.Fatal("CreateShareReqBody must have ExpiresAt field")
	}
	if expiresAtField.Tag.Get("json") != "expiresAt,omitempty" {
		t.Errorf(`CreateShareReqBody.ExpiresAt json tag = %q, want "expiresAt,omitempty"`, expiresAtField.Tag.Get("json"))
	}
}

func TestRedisShareCacheListUserShares_IncludesRecentlyExpiredOnly(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = rdb.Close() }()

	shareCache := cache.NewShareCache(rdb)
	now := time.Now()
	userID := uint(42)

	seedRedisShare(t, rdb, userID, "active-share", 1, now.Add(-time.Hour), true, constant.ShareTTLDefault)
	seedRedisShare(t, rdb, userID, "recent-expired-share", 2, now.Add(-constant.ShareTTLDefault-time.Hour), false, constant.ShareTTLDefault)
	seedRedisShare(t, rdb, userID, "old-expired-share", 3, now.Add(-constant.ShareTTLDefault-72*time.Hour-time.Hour), false, constant.ShareTTLDefault)

	shares, pageInfo, err := shareCache.ListUserShares(ctx, userID, 1, 10)
	if err != nil {
		t.Fatalf("ListUserShares returned error: %v", err)
	}
	if pageInfo.Total != 2 {
		t.Fatalf("pageInfo.Total = %d, want 2", pageInfo.Total)
	}
	if len(shares) != 2 {
		t.Fatalf("shares count = %d, want 2", len(shares))
	}
	if shares[0].ShareID != "active-share" || shares[1].ShareID != "recent-expired-share" {
		t.Fatalf("share order = [%s, %s], want [active-share, recent-expired-share]", shares[0].ShareID, shares[1].ShareID)
	}
	if shares[0].ExpiresAt.Before(now) {
		t.Fatalf("active share expiresAt=%s should be in the future", shares[0].ExpiresAt)
	}
	if !shares[1].ExpiresAt.Before(now) {
		t.Fatalf("recent expired share expiresAt=%s should be in the past", shares[1].ExpiresAt)
	}
}

func TestRedisShareCache_CreateShare_WithCustomTTL(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = rdb.Close() }()

	shareCache := cache.NewShareCache(rdb)
	userID := uint(42)
	sessionID := uint(1)

	shareID, expiresAt, err := shareCache.CreateShare(ctx, userID, sessionID, constant.ShareTTL1Week)
	if err != nil {
		t.Fatalf("CreateShare failed: %v", err)
	}
	if shareID == "" {
		t.Fatal("expected non-empty shareID")
	}

	expectedExpiry := time.Now().Add(constant.ShareTTL1Week)
	diff := expiresAt.Sub(expectedExpiry)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("expiresAt = %v, want ~%v", expiresAt, expectedExpiry)
	}

	gotSessionID, err := shareCache.GetShareSessionID(ctx, shareID)
	if err != nil {
		t.Fatalf("GetShareSessionID failed: %v", err)
	}
	if gotSessionID != sessionID {
		t.Errorf("GetShareSessionID = %d, want %d", gotSessionID, sessionID)
	}

	items, _, listErr := shareCache.ListUserShares(ctx, userID, 1, 10)
	if listErr != nil {
		t.Fatalf("ListUserShares failed: %v", listErr)
	}
	if len(items) != 1 {
		t.Fatalf("shares count = %d, want 1", len(items))
	}
	if items[0].ShareID != shareID {
		t.Errorf("shareID = %s, want %s", items[0].ShareID, shareID)
	}
	if items[0].ExpiresAt.Before(time.Now()) {
		t.Error("expiresAt should be in the future for 1-week share")
	}
}

func seedRedisShare(t *testing.T, rdb *redis.Client, userID uint, shareID string, sessionID uint, createdAt time.Time, active bool, ttl time.Duration) {
	t.Helper()
	ctx := context.Background()
	record := struct {
		ShareID   string `json:"shareId"`
		SessionID uint   `json:"sessionId"`
		CreatedAt int64  `json:"createdAt"`
		TTL       int64  `json:"ttl"`
	}{
		ShareID:   shareID,
		SessionID: sessionID,
		CreatedAt: createdAt.Unix(),
		TTL:       int64(ttl.Seconds()),
	}
	recordJSON, err := sonic.Marshal(record)
	if err != nil {
		t.Fatalf("marshal share record failed: %v", err)
	}
	if err := rdb.ZAdd(ctx, fmt.Sprintf(constant.UserSharesKeyTemplate, userID), redis.Z{
		Score:  float64(record.CreatedAt),
		Member: string(recordJSON),
	}).Err(); err != nil {
		t.Fatalf("seed user share failed: %v", err)
	}
	if !active {
		return
	}
	remaining := time.Until(createdAt.Add(ttl))
	if remaining <= 0 {
		t.Fatalf("active share %s has non-positive ttl %s", shareID, remaining)
	}
	if err := rdb.Set(ctx, fmt.Sprintf(constant.ShareKeyTemplate, shareID), sessionID, remaining).Err(); err != nil {
		t.Fatalf("seed active share key failed: %v", err)
	}
}
