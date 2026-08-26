// Package status 实现 aris status 状态面板：并发收集本地与网络检查结果并渲染。
package status

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/client/api"
	"github.com/hcd233/aris-proxy-api/internal/client/model"
	"github.com/hcd233/aris-proxy-api/internal/client/setup"
	"github.com/hcd233/aris-proxy-api/internal/client/trace"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// Report aris status 全部检查结果
type Report struct {
	ConfigFound      bool
	Host             string
	Agent            string
	ServerOK         bool
	ServerLatency    time.Duration
	ServerErr        string
	AuthOK           bool
	AuthMaskedKey    string
	AuthErr          string
	HooksFound       int
	HooksTotal       int
	HooksMissing     []string
	ProvidersFound   []string
	ProvidersMissing []string
	PendingCount     int
	PendingBytes     int64
	RejectedCount    int
	RecentErrors     int
	LogDir           string
}

// Collect 并发收集本地扫描与网络检查结果；无 config 时跳过网络请求
func Collect(ctx context.Context, paths trace.Paths, hc *http.Client) *Report {
	report := &Report{
		HooksTotal: len(constant.TraceClientCodexHookEvents),
		LogDir:     paths.LogDir(),
	}
	store := trace.NewConfigStore(paths)
	config, err := store.Load(ctx)
	if err == nil && config.Host != "" {
		report.ConfigFound = true
	}
	report.Host = config.Host
	report.AuthMaskedKey = maskAPIKey(config.APIKey)

	var wg sync.WaitGroup
	wg.Go(func() {
		collectLocal(paths, report)
	})
	if report.ConfigFound {
		client := api.New(config.Host, config.APIKey, hc)
		wg.Go(func() {
			latency, checkErr := client.CheckHealth(ctx)
			report.ServerLatency = latency
			if checkErr != nil {
				report.ServerErr = checkErr.Error()
			} else {
				report.ServerOK = true
			}
		})
		wg.Go(func() {
			if checkErr := client.CheckAPIKey(ctx); checkErr != nil {
				report.AuthErr = checkErr.Error()
			} else {
				report.AuthOK = true
			}
		})
	}
	wg.Wait()
	return report
}

// collectLocal 扫描本地目录：spool 积压、rejected、当日日志、hooks 注册状态
func collectLocal(paths trace.Paths, report *Report) {
	report.PendingCount, report.PendingBytes = scanRecordDir(paths.PendingDir())
	report.RejectedCount, _ = scanRecordDir(paths.RejectedDir())
	report.RecentErrors = countTodayLogEntries(paths)
	binPath, err := setup.ExecutablePath()
	if err != nil {
		report.HooksMissing = append([]string{}, constant.TraceClientCodexHookEvents...)
		return
	}
	report.HooksFound, report.HooksMissing = trace.InspectCodexHooks(paths, binPath)
	report.ProvidersFound, report.ProvidersMissing = scanProviderConfigs()
}

// scanProviderConfigs 遍历各 agent harness 默认配置路径，报告存在性
func scanProviderConfigs() (found, missing []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil
	}
	for _, tgt := range model.Targets() {
		if _, statErr := os.Stat(tgt.ConfigPath(home)); statErr == nil {
			found = append(found, tgt.Key())
		} else {
			missing = append(missing, tgt.Key())
		}
	}
	return found, missing
}

// scanRecordDir 统计目录中记录文件的数量与总字节数；目录缺失视为 0
func scanRecordDir(dir string) (count int, totalBytes int64) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || filepath.Ext(entry.Name()) != constant.TraceClientRecordFileSuffix {
			continue
		}
		count++
		totalBytes += info.Size()
	}
	return count, totalBytes
}

// countTodayLogEntries 统计当日客户端日志条目数
func countTodayLogEntries(paths trace.Paths) int {
	name := constant.TraceClientLogPrefix + time.Now().UTC().Format(constant.TraceClientLogDateFormat) + constant.TraceClientLogSuffix
	data, err := os.ReadFile(filepath.Join(paths.LogDir(), name))
	if err != nil {
		return 0
	}
	count := 0
	for line := range strings.SplitSeq(string(data), "\n") {
		if line != "" {
			count++
		}
	}
	return count
}

// maskAPIKey 脱敏 API Key：仅保留末 4 位
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return constant.ClientUIMaskPrefix
	}
	return constant.ClientUIMaskPrefix + key[len(key)-4:]
}
