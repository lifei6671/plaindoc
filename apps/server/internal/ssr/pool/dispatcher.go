package pool

import (
	"context"
	"errors"

	"github.com/lifei6671/plaindoc/apps/server/internal/ssr/protocol"
)

// Dispatcher 为上层 Handler 提供稳定的渲染入口抽象。
type Dispatcher struct {
	pool *Pool
}

// NewDispatcher 创建渲染分发器。
func NewDispatcher(pool *Pool) *Dispatcher {
	return &Dispatcher{pool: pool}
}

// Render 将请求分发给 WorkerPool。
func (dispatcher *Dispatcher) Render(
	ctx context.Context,
	request protocol.RenderRequest,
) (protocol.RenderResponse, error) {
	if dispatcher == nil || dispatcher.pool == nil {
		return protocol.RenderResponse{}, errors.New("ssr dispatcher not initialized")
	}
	return dispatcher.pool.Render(ctx, request)
}
