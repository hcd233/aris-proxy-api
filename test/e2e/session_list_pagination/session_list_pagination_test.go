// Package session_list_pagination 验证 session/list 拆分 COUNT 后的分页语义：
// page 1 与 page 2 的 total 必须一致（旧窗口实现 page>1 空结果时会退化成 0），
// 且每页条数不超过 pageSize。
//
// 默认离线 skip，需显式提供 BASE_URL + WEB_TOKEN（admin JWT）。
package session_list_pagination

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

const pageSize = 20

type listSessionsRsp struct {
	Sessions []struct {
		ID uint `json:"id"`
	} `json:"sessions"`
	PageInfo struct {
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Total    int64 `json:"total"`
	} `json:"pageInfo"`
}

func mustWebEnv(t *testing.T) (baseURL, token string) {
	t.Helper()
	baseURL, token = os.Getenv("BASE_URL"), os.Getenv("WEB_TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("BASE_URL and WEB_TOKEN are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), token
}

func fetchPage(t *testing.T, baseURL, token string, page int) listSessionsRsp {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	url := baseURL + "/api/web/v1/session/list?page=" + strconv.Itoa(page) + "&pageSize=" + strconv.Itoa(pageSize)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("session/list page %d status = %d, want 200", page, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read page %d body: %v", page, err)
	}
	var rsp listSessionsRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("decode page %d: %v", page, err)
	}
	return rsp
}

func TestSessionList_PageTotalConsistent(t *testing.T) {
	t.Parallel()
	baseURL, token := mustWebEnv(t)

	first := fetchPage(t, baseURL, token, 1)
	second := fetchPage(t, baseURL, token, 2)

	if first.PageInfo.Total != second.PageInfo.Total {
		t.Fatalf("total changed across pages: page1=%d page2=%d", first.PageInfo.Total, second.PageInfo.Total)
	}
	if first.PageInfo.Total < 0 {
		t.Fatalf("total must not be negative, got %d", first.PageInfo.Total)
	}
	if got := len(first.Sessions); got > pageSize {
		t.Errorf("page1 returned %d sessions, want <= %d", got, pageSize)
	}
	if got := len(second.Sessions); got > pageSize {
		t.Errorf("page2 returned %d sessions, want <= %d", got, pageSize)
	}
	if first.PageInfo.Total <= pageSize && len(second.Sessions) != 0 {
		t.Errorf("total=%d <= pageSize, page2 must be empty, got %d rows", first.PageInfo.Total, len(second.Sessions))
	}
}
