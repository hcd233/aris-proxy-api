package session_share

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	sessionquery "github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
)

type mockShareEntry struct {
	userID    uint
	sessionID uint
	createdAt time.Time
	expiresAt time.Time
}

type mockShareCache struct {
	shares     map[string]*mockShareEntry
	userShares map[uint][]string
	createErr  error
	getErr     error
	deleteErr  error
	listErr    error
}

func newMockShareCache() *mockShareCache {
	return &mockShareCache{
		shares:     make(map[string]*mockShareEntry),
		userShares: make(map[uint][]string),
	}
}

func (m *mockShareCache) CreateShare(_ context.Context, userID, sessionID uint) (string, time.Time, error) {
	if m.createErr != nil {
		return "", time.Time{}, m.createErr
	}
	shareID := "test-share-id-" + time.Now().Format("150405")
	now := time.Now()
	m.shares[shareID] = &mockShareEntry{
		userID:    userID,
		sessionID: sessionID,
		createdAt: now,
		expiresAt: now.Add(constant.ShareTTL),
	}
	m.userShares[userID] = append(m.userShares[userID], shareID)
	return shareID, now.Add(constant.ShareTTL), nil
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

func TestGetShareContent_Success(t *testing.T) {
	sc := newMockShareCache()
	shareID, _, _ := sc.CreateShare(context.Background(), 42, 1)
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{1: testSessionView(1)}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(0, enum.PermissionPending)

	rsp, err := h.HandleGetShareContent(ctx, &dto.GetShareContentReq{ShareID: shareID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rsp.Body.Session == nil {
		t.Fatal("expected non-nil session in response")
	}
	if rsp.Body.Session.ID != 1 {
		t.Errorf("session ID = %d, want 1", rsp.Body.Session.ID)
	}
}

func TestGetShareContent_ShareNotFound(t *testing.T) {
	sc := newMockShareCache()
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(0, enum.PermissionPending)

	rsp, _ := h.HandleGetShareContent(ctx, &dto.GetShareContentReq{ShareID: "nonexistent"})
	if rsp.Body.Error == nil {
		t.Error("expected error for share not found")
	}
}

func TestGetShareContent_NoAPIKeyNameLeaked(t *testing.T) {
	sc := newMockShareCache()
	shareID, _, _ := sc.CreateShare(context.Background(), 42, 1)
	view := testSessionView(1)
	view.APIKeyName = "sensitive-key-name"
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{1: view}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(0, enum.PermissionPending)

	rsp, _ := h.HandleGetShareContent(ctx, &dto.GetShareContentReq{ShareID: shareID})
	if rsp.Body.Session == nil {
		t.Fatal("expected non-nil session")
	}
	data, _ := sonic.Marshal(rsp.Body.Session)
	if containsJSONKey(data, "apiKeyName") {
		t.Error("ShareContentSessionDetail must not contain apiKeyName field")
	}
}

func TestListShares_Success(t *testing.T) {
	sc := newMockShareCache()
	sc.CreateShare(context.Background(), 42, 1)
	sc.CreateShare(context.Background(), 42, 2)
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
	shareID, _, _ := sc.CreateShare(context.Background(), 42, 1)
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
	shareID, _, _ := sc.CreateShare(context.Background(), 42, 1)
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

func containsJSONKey(data []byte, key string) bool {
	needle := []byte(`"` + key + `"`)
	for i := 0; i <= len(data)-len(needle); i++ {
		if data[i] == needle[0] {
			match := true
			for j := 1; j < len(needle); j++ {
				if data[i+j] != needle[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
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
}
