// Package status 实现 aris status 状态面板：并发收集本地与网络检查结果并渲染。
package status

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/client/api"
	clientconfig "github.com/hcd233/aris-proxy-api/internal/client/config"
	"github.com/hcd233/aris-proxy-api/internal/client/model"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// Report aris status 全部检查结果
type Report struct {
	ConfigFound      bool
	Host             string
	ServerOK         bool
	ServerLatency    time.Duration
	ServerErr        string
	AuthOK           bool
	AuthMaskedKey    string
	AuthErr          string
	ProvidersFound   []string
	ProvidersMissing []string
}

// Collect 并发收集本地扫描与网络检查结果；无 config 时跳过网络请求
func Collect(ctx context.Context, paths clientconfig.Paths, hc *http.Client) *Report {
	report := &Report{}
	store := clientconfig.NewStore(paths)
	config, err := store.Load(ctx)
	if err == nil && config.Host != "" {
		report.ConfigFound = true
	}
	report.Host = config.Host
	report.AuthMaskedKey = maskAPIKey(config.APIKey)

	var wg sync.WaitGroup
	wg.Go(func() {
		report.ProvidersFound, report.ProvidersMissing = scanProviderConfigs()
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
