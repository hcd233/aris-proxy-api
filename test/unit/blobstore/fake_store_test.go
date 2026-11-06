package blobstore

import (
	"context"
	"sync"
	"time"

	"github.com/hcd233/aris-mass/internal/common/ierr"
)

// fakeStore 内存对象存储，可注入失败与阻塞
type fakeStore struct {
	mu         sync.Mutex
	objects    map[string][]byte
	puts       map[string]int
	putCalls   int
	failPut    bool
	blockPut   bool
	failDelete bool
	deletes    []string
	presignErr error
	// gate 非 nil 时 Put 先等它关闭再执行，用于让上传池的 worker 停在处理中；
	// entered 非 nil 时 Put 进入后先发一次信号，测试据此确认 worker 已开始处理
	gate    chan struct{}
	entered chan struct{}
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: map[string][]byte{}, puts: map[string]int{}}
}

func (f *fakeStore) Put(ctx context.Context, key, _ string, data []byte) error {
	f.mu.Lock()
	f.putCalls++
	block, fail, gate, entered := f.blockPut, f.failPut, f.gate, f.entered
	f.mu.Unlock()
	if entered != nil {
		entered <- struct{}{}
	}
	if gate != nil {
		<-gate
	}
	if block {
		<-ctx.Done()
		return ierr.Wrap(ierr.ErrInternal, ctx.Err(), "fake put blocked")
	}
	if fail {
		return ierr.New(ierr.ErrInternal, "fake put failure")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = append([]byte(nil), data...)
	f.puts[key]++
	return nil
}

func (f *fakeStore) Get(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.objects[key]
	if !ok {
		return nil, ierr.New(ierr.ErrDataNotExists, "fake object missing")
	}
	return data, nil
}

func (f *fakeStore) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failDelete {
		return ierr.New(ierr.ErrInternal, "fake delete failure")
	}
	f.deletes = append(f.deletes, key)
	delete(f.objects, key)
	return nil
}

func (f *fakeStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	if f.presignErr != nil {
		return "", f.presignErr
	}
	return "https://storage.example.com/" + key + "?X-Amz-Signature=fake", nil
}

func (f *fakeStore) totalPuts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.puts {
		n += c
	}
	return n
}
