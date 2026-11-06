// dataset 导出内联图片（共享 sqlite 内存库，串行执行）。
//
//nolint:paralleltest // 共享 sqlite 内存库，必须串行
package cross_tenant_session

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/bytedance/sonic"

	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
)

type exportedConversation struct {
	Images []string `json:"images"`
}

// exportImages 调用导出接口（NDJSON），返回含图片的会话的 images 列表与是否收齐 X-Total-Sessions 行
func (f *crossTenantSessionFixture) exportImages(t *testing.T) (images [][]string, done bool) {
	t.Helper()
	status, header, data := f.doWithHeader(t, http.MethodGet, "/api/web/v1/dataset/export", f.tokenA, "")
	if status != http.StatusOK || !strings.HasPrefix(header.Get("Content-Type"), "application/x-ndjson") {
		t.Fatalf("export status=%d content-type=%s body=%s", status, header.Get("Content-Type"), data)
	}
	total, err := strconv.Atoi(header.Get("X-Total-Sessions"))
	if err != nil || total == 0 {
		t.Fatalf("bad X-Total-Sessions %q", header.Get("X-Total-Sessions"))
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	for _, line := range lines {
		var conv exportedConversation
		if err := sonic.UnmarshalString(line, &conv); err != nil {
			t.Fatalf("decode line: %v (%s)", err, line)
		}
		if len(conv.Images) > 0 {
			images = append(images, conv.Images)
		}
	}
	return images, len(lines) == total
}

// TestDatasetExportNoMatchIsJSONError 无匹配会话时在开始流式输出前返回普通 JSON 业务错误
func TestDatasetExportNoMatchIsJSONError(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	status, header, data := f.doWithHeader(t, http.MethodGet, "/api/web/v1/dataset/export?startTime=2099-01-01T00:00:00Z", f.tokenA, "")
	if status != http.StatusOK || strings.HasPrefix(header.Get("Content-Type"), "application/x-ndjson") || bizErrorCode(t, data) != bizCodeDataNotExists {
		t.Fatalf("status=%d content-type=%s body=%s", status, header.Get("Content-Type"), data)
	}
}

func TestDatasetExportInlinesImages(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	f.seedImages(t)
	f.objectStore.objects["images/up.png"] = []byte("up")

	images, done := f.exportImages(t)
	if !done || len(images) != 1 || len(images[0]) != 3 {
		t.Fatalf("done=%v images=%v", done, images)
	}
	// unprocessable 的原图不作预览，但导出照常内联
	if images[0][0] != "data:image/png;base64,dXA=" || images[0][1] != "data:image/png;base64,cG5n" || images[0][2] != "data:image/png;base64,Ymln" {
		t.Fatalf("images=%v", images[0])
	}
}

func TestDatasetExportMissingBlobStillCompletes(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	fx := f.seedImages(t)
	if err := f.db.Where("sha256 = ?", fx.uploadedSHA).Delete(&dbmodel.Blob{}).Error; err != nil {
		t.Fatalf("delete blob: %v", err)
	}
	images, done := f.exportImages(t)
	if !done || len(images) != 1 || len(images[0]) != 2 {
		t.Fatalf("done=%v images=%v", done, images)
	}
}
