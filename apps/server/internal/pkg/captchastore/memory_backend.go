package captchastore

import (
	"context"
	"sync"
	"time"
)

const (
	// DefaultMemoryCollectEvery 定义内存后端默认清理频率（按写入次数触发）。
	DefaultMemoryCollectEvery = 256
)

// MemoryBackendOptions 描述内存后端参数。
type MemoryBackendOptions struct {
	CollectEvery int
	NowFunc      func() time.Time
}

// MemoryBackend 是进程内内存实现，适用于单节点部署或本地开发。
type MemoryBackend struct {
	mu           sync.RWMutex
	items        map[string]Record
	collectEvery int
	writeCount   int
	nowFn        func() time.Time
}

// NewMemoryBackend 创建进程内内存后端。
func NewMemoryBackend(options MemoryBackendOptions) *MemoryBackend {
	collectEvery := options.CollectEvery
	if collectEvery <= 0 {
		collectEvery = DefaultMemoryCollectEvery
	}
	nowFn := options.NowFunc
	if nowFn == nil {
		nowFn = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &MemoryBackend{
		items:        make(map[string]Record),
		collectEvery: collectEvery,
		nowFn:        nowFn,
	}
}

// Set 写入内存后端。
func (b *MemoryBackend) Set(_ context.Context, record Record) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	b.items[record.ID] = Record{
		ID:        record.ID,
		Value:     record.Value,
		ExpiresAt: record.ExpiresAt,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
		Metadata:  cloneMetadata(record.Metadata),
	}
	b.writeCount += 1
	if b.collectEvery > 0 && b.writeCount%b.collectEvery == 0 {
		b.collectExpiredLocked(b.currentTime())
	}
	b.mu.Unlock()
	return nil
}

// Get 读取内存后端。
func (b *MemoryBackend) Get(_ context.Context, id string) (Record, bool, error) {
	if b == nil {
		return Record{}, false, nil
	}
	b.mu.RLock()
	record, found := b.items[id]
	b.mu.RUnlock()
	if !found {
		return Record{}, false, nil
	}
	return Record{
		ID:        record.ID,
		Value:     record.Value,
		ExpiresAt: record.ExpiresAt,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
		Metadata:  cloneMetadata(record.Metadata),
	}, true, nil
}

// Delete 删除内存后端数据。
func (b *MemoryBackend) Delete(_ context.Context, id string) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	delete(b.items, id)
	b.mu.Unlock()
	return nil
}

func (b *MemoryBackend) collectExpiredLocked(now time.Time) {
	for id, record := range b.items {
		if record.ExpiresAt.IsZero() {
			continue
		}
		if !record.ExpiresAt.After(now) {
			delete(b.items, id)
		}
	}
}

func (b *MemoryBackend) currentTime() time.Time {
	if b == nil || b.nowFn == nil {
		return time.Now().UTC()
	}
	return b.nowFn().UTC()
}
