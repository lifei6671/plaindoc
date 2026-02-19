package logit

import (
	"context"
	"log/slog"
	"sort"
	"sync"
)

type requestAttrsContextKey struct{}

// RequestAttrsContainer 是请求级日志属性容器，按 key 覆盖最新值。
type RequestAttrsContainer struct {
	mu    sync.RWMutex
	attrs map[string]slog.Attr
}

// WithRequestAttrsContainer 在 context 中注入请求级属性容器。
func WithRequestAttrsContainer(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := requestAttrsContainerFromContext(ctx); ok {
		return ctx
	}

	container := &RequestAttrsContainer{
		attrs: make(map[string]slog.Attr, 8),
	}
	return context.WithValue(ctx, requestAttrsContextKey{}, container)
}

// SetRequestAttrs 向当前请求容器写入属性；同名 key 会被最新值覆盖。
func SetRequestAttrs(ctx context.Context, attrs ...slog.Attr) {
	container, ok := requestAttrsContainerFromContext(ctx)
	if !ok {
		return
	}
	container.set(attrs...)
}

// SnapshotRequestAttrs 获取当前请求容器中的属性快照，按 key 排序便于稳定输出。
func SnapshotRequestAttrs(ctx context.Context) []slog.Attr {
	container, ok := requestAttrsContainerFromContext(ctx)
	if !ok {
		return nil
	}
	return container.snapshot()
}

func requestAttrsContainerFromContext(ctx context.Context) (*RequestAttrsContainer, bool) {
	if ctx == nil {
		return nil, false
	}

	value := ctx.Value(requestAttrsContextKey{})
	if value == nil {
		return nil, false
	}

	container, ok := value.(*RequestAttrsContainer)
	if !ok {
		return nil, false
	}
	return container, true
}

func (c *RequestAttrsContainer) set(attrs ...slog.Attr) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, attr := range attrs {
		if attr.Key == "" {
			continue
		}
		c.attrs[attr.Key] = attr
	}
}

func (c *RequestAttrsContainer) snapshot() []slog.Attr {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.attrs))
	for key := range c.attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]slog.Attr, 0, len(keys))
	for _, key := range keys {
		result = append(result, c.attrs[key])
	}
	return result
}
