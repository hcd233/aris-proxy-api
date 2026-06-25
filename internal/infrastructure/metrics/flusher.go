package metrics

import (
	"context"
	"os"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Flusher 周期性把本 instance 的运行时快照写入共享存储（Redis）。
//
// 每个 pod 各跑一个 Flusher（不走 cron 分布式锁、不进协程池），
// 由 fx 生命周期 OnStart/OnStop 驱动。
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type Flusher struct {
	gatherer   prometheus.Gatherer
	store      SnapshotStore
	instanceID string
	interval   time.Duration
	retention  time.Duration
	cancel     context.CancelFunc
	done       chan struct{}
}

// NewFlusher 创建 Flusher，instanceID 取自 hostname（K8s 注入的 pod 名）。
//
//	@param gatherer prometheus.Gatherer
//	@param store SnapshotStore
//	@param interval time.Duration
//	@param retention time.Duration
//	@return *Flusher
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func NewFlusher(gatherer prometheus.Gatherer, store SnapshotStore, interval, retention time.Duration) *Flusher {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = constant.RuntimeMetricsUnknownInstance
	}
	return &Flusher{
		gatherer:   gatherer,
		store:      store,
		instanceID: hostname,
		interval:   interval,
		retention:  retention,
		done:       make(chan struct{}),
	}
}

// Start 启动后台采集循环。
//
//	@receiver f *Flusher
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func (f *Flusher) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	go f.loop(ctx)
	logger.Logger().Info("[MetricsFlusher] Started", zap.String("instance", f.instanceID), zap.Duration("interval", f.interval))
}

// Stop 停止后台采集循环（不做最终 flush）。
//
//	@receiver f *Flusher
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func (f *Flusher) Stop() {
	if f.cancel != nil {
		f.cancel()
		<-f.done
	}
}

func (f *Flusher) loop(ctx context.Context) {
	defer close(f.done)
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			f.flush(now)
		}
	}
}

func (f *Flusher) flush(now time.Time) {
	snap, err := BuildSnapshot(f.gatherer, now)
	if err != nil {
		logger.Logger().Warn("[MetricsFlusher] Build snapshot failed", zap.Error(err))
		return
	}
	payload, err := sonic.Marshal(snap)
	if err != nil {
		logger.Logger().Warn("[MetricsFlusher] Marshal snapshot failed", zap.Error(err))
		return
	}
	cutoff := now.Add(-f.retention).Unix()
	if err := f.store.WriteSnapshot(f.instanceID, snap.TS, payload, cutoff); err != nil {
		logger.Logger().Warn("[MetricsFlusher] Write snapshot failed", zap.Error(err))
	}
}
