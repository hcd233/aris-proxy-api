// Package cron_trigger 验证 cron 手动触发的全链路行为。
//
// 环境变量：
//   - BASE_URL   API 根地址（必填）
//   - JWT_TOKEN  管理员 JWT（必填）
package cron_trigger

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

const e2eHTTPTimeout = 30 * time.Second

func mustE2EEnv(t *testing.T) (baseURL, jwtToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	jwtToken = os.Getenv("JWT_TOKEN")
	if baseURL == "" || jwtToken == "" {
		t.Skip("BASE_URL and JWT_TOKEN are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), jwtToken
}

func newE2EClient() *http.Client {
	return &http.Client{Timeout: e2eHTTPTimeout}
}

// doJSON 发出请求并返回状态码、TraceID 与原始响应体。
func doJSON(t *testing.T, client *http.Client, method, url, jwtToken string) (statusCode int, traceID string, respBody []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, http.NoBody)
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+jwtToken)
	httpResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send %s %s failed: %v", method, url, err)
	}
	defer func() { _ = httpResp.Body.Close() }()
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		t.Fatalf("read response body failed: %v", err)
	}
	return httpResp.StatusCode, httpResp.Header.Get(constant.HTTPHeaderTraceID), bodyBytes
}

type cronJobItem struct {
	Name string `json:"name"`
}
type listCronJobsRsp struct {
	Jobs []cronJobItem `json:"jobs"`
}
type cronAuditItem struct {
	CronName      string `json:"cronName"`
	TriggerSource string `json:"triggerSource"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
}
type listCronAuditsRsp struct {
	Logs []cronAuditItem `json:"logs"`
}

func TestE2E_CronManualTrigger_ProducesManualAudit(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustE2EEnv(t)
	client := newE2EClient()

	// 1. 取第一个任务名
	status, traceID, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/cron/list?page=1&pageSize=20", jwtToken)
	if status != http.StatusOK {
		t.Fatalf("list cron jobs status=%d traceID=%s body=%s", status, traceID, string(body))
	}
	var listRsp listCronJobsRsp
	if err := sonic.Unmarshal(body, &listRsp); err != nil {
		t.Fatalf("unmarshal list rsp: %v", err)
	}
	if len(listRsp.Jobs) == 0 {
		t.Fatal("no cron jobs found")
	}
	name := listRsp.Jobs[0].Name
	t.Logf("triggering cron job: %s", name)

	// 2. 触发
	status, traceID, body = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/cron/trigger?name="+name, jwtToken)
	if status != http.StatusOK {
		t.Fatalf("trigger cron job status=%d traceID=%s body=%s", status, traceID, string(body))
	}

	// 3. 轮询执行日志，等待出现 triggerSource=manual 的新记录（最长 30s）
	start := time.Now().Add(-2 * time.Minute).Format("2006-01-02T15:04:05Z")
	deadline := time.Now().Add(e2eHTTPTimeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		status, traceID, body = doJSON(t, client, http.MethodGet,
			fmt.Sprintf("%s/api/v1/cron/log/list?page=1&pageSize=20&sort=desc&sortField=created_at&startTime=%s", baseURL, start),
			jwtToken)
		if status != http.StatusOK {
			t.Fatalf("list cron audits status=%d traceID=%s body=%s", status, traceID, string(body))
		}
		var auditRsp listCronAuditsRsp
		if err := sonic.Unmarshal(body, &auditRsp); err != nil {
			t.Fatalf("unmarshal audit rsp: %v", err)
		}
		for _, log := range auditRsp.Logs {
			if log.CronName == name && log.TriggerSource == "manual" && log.Status == "success" {
				t.Logf("found successful manual audit record: %s", log.CreatedAt)
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("successful manual trigger audit record not found within timeout")
		}
	}
}

func TestE2E_CronTrigger_NotFound(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustE2EEnv(t)
	client := newE2EClient()
	status, traceID, body := doJSON(t, client, http.MethodPost, baseURL+"/api/v1/cron/trigger?name=non-existent-job", jwtToken)
	if status != http.StatusOK {
		t.Fatalf("unified contract: expected 200 for unknown cron job, got status=%d traceID=%s body=%s", status, traceID, string(body))
	}
	var rsp struct {
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := sonic.Unmarshal(body, &rsp); err != nil || rsp.Error == nil || rsp.Error.Code != 10003 {
		t.Fatalf("expected error code 10003 for unknown cron job, body=%s", string(body))
	}
}
