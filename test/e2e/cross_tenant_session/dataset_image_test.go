// dataset 导出内联图片（共享 sqlite 内存库，串行执行）。
//
//nolint:paralleltest // 共享 sqlite 内存库，必须串行
package cross_tenant_session

import (
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/sonic"

	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
)

type exportedConversation struct {
	Images []string `json:"images"`
}

// exportImages 调用导出接口，返回含图片的会话的 images 列表与是否收到 done 事件
func (f *crossTenantSessionFixture) exportImages(t *testing.T) (images [][]string, done bool) {
	t.Helper()
	status, data := f.do(t, http.MethodGet, "/api/web/v1/dataset/export", f.tokenA, "")
	if status != http.StatusOK {
		t.Fatalf("export status=%d body=%s", status, data)
	}
	for frame := range strings.SplitSeq(string(data), "\n\n") {
		event, payload, ok := strings.Cut(frame, "\ndata: ")
		if !ok {
			continue
		}
		switch strings.TrimPrefix(event, "event: ") {
		case "done":
			done = true
		case "data":
			var line struct {
				JSON string `json:"json"`
			}
			var conv exportedConversation
			if err := sonic.UnmarshalString(payload, &line); err != nil {
				t.Fatalf("decode frame: %v", err)
			}
			if err := sonic.UnmarshalString(line.JSON, &conv); err != nil {
				t.Fatalf("decode conversation: %v", err)
			}
			if len(conv.Images) > 0 {
				images = append(images, conv.Images)
			}
		}
	}
	return images, done
}

func TestDatasetExportInlinesImages(t *testing.T) {
	f := newCrossTenantSessionFixture(t)
	f.seedImages(t)
	f.objectStore.objects["images/up.png"] = []byte("up")

	images, done := f.exportImages(t)
	if !done || len(images) != 1 || len(images[0]) != 2 {
		t.Fatalf("done=%v images=%v", done, images)
	}
	if images[0][0] != "data:image/png;base64,dXA=" || images[0][1] != "data:image/png;base64,cG5n" {
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
	if !done || len(images) != 1 || len(images[0]) != 1 {
		t.Fatalf("done=%v images=%v", done, images)
	}
}
