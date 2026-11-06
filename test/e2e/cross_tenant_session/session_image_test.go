// 会话图片接口的归属与引用校验（共享 sqlite 内存库，串行执行）。
//
//nolint:paralleltest // 共享 sqlite 内存库，必须串行
package cross_tenant_session

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/common/vo"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
)

// e2eObjectStore 只用于预签名的最小对象存储
type e2eObjectStore struct{ objects map[string][]byte }

func (s *e2eObjectStore) Put(_ context.Context, key, _ string, data []byte) error {
	s.objects[key] = data
	return nil
}

func (s *e2eObjectStore) Get(_ context.Context, key string) ([]byte, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, ierr.New(ierr.ErrDataNotExists, "object missing")
	}
	return data, nil
}

func (s *e2eObjectStore) Delete(_ context.Context, key string) error {
	delete(s.objects, key)
	return nil
}

func (s *e2eObjectStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://storage.example.com/" + key + "?X-Amz-Signature=e2e", nil
}

type imageFixture struct {
	session                 *dbmodel.Session
	uploadedSHA, pendingSHA string
	foreignSHA              string
	unprocessableSHA        string
}

// seedImages 为 userA 新建一个含两张已外置图片的会话，另有一张图只出现在 orphan 会话
func (f *crossTenantSessionFixture) seedImages(t *testing.T) *imageFixture {
	t.Helper()
	fx := &imageFixture{
		uploadedSHA:      vo.SHA256Hex([]byte("uploaded")),
		pendingSHA:       vo.SHA256Hex([]byte("pending")),
		foreignSHA:       vo.SHA256Hex([]byte("foreign")),
		unprocessableSHA: vo.SHA256Hex([]byte("unprocessable")),
	}
	part := func(sha string) *vo.UnifiedContentPart {
		return &vo.UnifiedContentPart{Type: enum.ContentPartTypeImageURL, ImageURL: vo.ImageRefOf(sha), ImageMime: "image/png", ImageSize: 3}
	}
	mine := &dbmodel.Message{CheckSum: "img-mine", Message: &vo.UnifiedMessage{Role: enum.RoleUser, Content: &vo.UnifiedContent{
		Parts: []*vo.UnifiedContentPart{
			part(fx.uploadedSHA), part(fx.pendingSHA), part(fx.unprocessableSHA),
			// 别人的图片引用以文本形式出现：模拟客户端伪造的引用在落库时被降级为文本
			{Type: enum.ContentPartTypeText, Text: vo.ImageRefOf(fx.foreignSHA)},
		},
	}}}
	foreign := &dbmodel.Message{CheckSum: "img-foreign", Message: &vo.UnifiedMessage{Role: enum.RoleUser, Content: &vo.UnifiedContent{
		Parts: []*vo.UnifiedContentPart{part(fx.foreignSHA)},
	}}}
	for _, m := range []*dbmodel.Message{mine, foreign} {
		if err := f.db.Create(m).Error; err != nil {
			t.Fatalf("create message: %v", err)
		}
	}
	fx.session = &dbmodel.Session{APIKeyName: sharedKeyName, APIKeyID: f.keyA.ID, MessageIDs: []uint{mine.ID}, ToolIDs: []uint{}, Questions: []uint{mine.ID}, ModelIDs: []string{}}
	other := &dbmodel.Session{APIKeyName: "", APIKeyID: 0, MessageIDs: []uint{foreign.ID}, ToolIDs: []uint{}, Questions: []uint{foreign.ID}, ModelIDs: []string{}}
	for _, s := range []*dbmodel.Session{fx.session, other} {
		if err := f.db.Create(s).Error; err != nil {
			t.Fatalf("create session: %v", err)
		}
	}
	next := time.Now().Add(time.Hour)
	blobs := []*dbmodel.Blob{
		{SHA256: fx.uploadedSHA, Status: enum.BlobStatusUploaded, StorageKey: "images/up.png", Mime: "image/png", Size: 3},
		{SHA256: fx.pendingSHA, Status: enum.BlobStatusPending, Data: []byte("png"), StorageKey: "images/pd.png", Mime: "image/png", Size: 3, NextRetryAt: &next},
		{SHA256: fx.foreignSHA, Status: enum.BlobStatusPending, Data: []byte("sec"), StorageKey: "images/fr.png", Mime: "image/png", Size: 3, NextRetryAt: &next},
		{SHA256: fx.unprocessableSHA, Status: enum.BlobStatusUnprocessable, Data: []byte("big"), StorageKey: "images/un.png", Mime: "image/png", Size: 3},
	}
	for _, b := range blobs {
		if err := f.db.Create(b).Error; err != nil {
			t.Fatalf("create blob: %v", err)
		}
	}
	return fx
}

type sessionImageBody struct {
	Image *struct {
		URL     string `json:"url"`
		DataURL string `json:"dataUrl"`
		Mime    string `json:"mime"`
	} `json:"image"`
}

func (f *crossTenantSessionFixture) getImage(t *testing.T, sessionID uint, sha, token string) (status int, data []byte) {
	t.Helper()
	return f.do(t, http.MethodGet, fmt.Sprintf("/api/web/v1/session/image?id=%d&sha256=%s", sessionID, sha), token, "")
}

func decodeImage(t *testing.T, data []byte) *sessionImageBody {
	t.Helper()
	var body sessionImageBody
	if err := sonic.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, data)
	}
	if body.Image == nil {
		t.Fatalf("no image in response: %s", data)
	}
	return &body
}

func TestSessionImageOwnerUploaded(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	fx := f.seedImages(t)
	status, data := f.getImage(t, fx.session.ID, fx.uploadedSHA, f.tokenA)
	body := decodeImage(t, data)
	if status != http.StatusOK || !strings.HasPrefix(body.Image.URL, "https://storage.example.com/images/up.png") || body.Image.DataURL != "" {
		t.Fatalf("status=%d body=%s", status, data)
	}
}

func TestSessionImageOwnerPending(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	fx := f.seedImages(t)
	_, data := f.getImage(t, fx.session.ID, fx.pendingSHA, f.tokenA)
	if body := decodeImage(t, data); body.Image.DataURL != "data:image/png;base64,cG5n" || body.Image.URL != "" {
		t.Fatalf("body=%s", data)
	}
}

func TestSessionImageOtherUserRejected(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	fx := f.seedImages(t)
	status, data := f.getImage(t, fx.session.ID, fx.uploadedSHA, f.tokenB)
	requireRejected(t, status, data, "user B reads user A image")
}

func TestSessionImageAdminAllowed(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	fx := f.seedImages(t)
	_, data := f.getImage(t, fx.session.ID, fx.pendingSHA, f.tokenAdmin)
	decodeImage(t, data)
}

func TestSessionImageUnreferencedRejected(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	fx := f.seedImages(t)
	status, data := f.getImage(t, fx.session.ID, fx.foreignSHA, f.tokenA)
	if status != http.StatusOK || bizErrorCode(t, data) != bizCodeDataNotExists {
		t.Fatalf("own session must not expose foreign image: status=%d body=%s", status, data)
	}
}

// TestSessionImageTextRefRejected 自己的消息文本里出现别人的图片引用（伪造引用降级后的形态）不能取图
func TestSessionImageTextRefRejected(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	fx := f.seedImages(t)
	status, data := f.getImage(t, fx.session.ID, fx.foreignSHA, f.tokenA)
	if status != http.StatusOK || bizErrorCode(t, data) != bizCodeDataNotExists {
		t.Fatalf("text ref must not grant access: status=%d body=%s", status, data)
	}
}

// TestSessionImageUnprocessableNotPreviewed unprocessable 的原图不作为预览返回
func TestSessionImageUnprocessableNotPreviewed(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	fx := f.seedImages(t)
	_, data := f.getImage(t, fx.session.ID, fx.unprocessableSHA, f.tokenA)
	if body := decodeImage(t, data); body.Image.URL != "" || body.Image.DataURL != "" || body.Image.Mime != "image/png" {
		t.Fatalf("body=%s", data)
	}
}

func TestSessionImageBadSHA256(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	fx := f.seedImages(t)
	for _, sha := range []string{strings.ToUpper(fx.uploadedSHA), fx.uploadedSHA[:10]} {
		if status, data := f.getImage(t, fx.session.ID, sha, f.tokenA); status != http.StatusUnprocessableEntity {
			t.Fatalf("sha=%s status=%d body=%s", sha, status, data)
		}
	}
}
