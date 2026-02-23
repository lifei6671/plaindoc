package pool

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/ssr/protocol"
	"github.com/lifei6671/plaindoc/apps/server/internal/ssr/worker"
)

// Config 描述 SSR Worker 池配置。
type Config struct {
	WorkerCount int
	Worker      worker.Config
	Logger      *slog.Logger
}

// Pool 管理多个常驻 SSR Worker 子进程。
type Pool struct {
	config Config

	mutex    sync.RWMutex
	workers  []*worker.Process
	started  bool
	rrCursor atomic.Uint64
}

// New 创建 Worker 池实例。
func New(config Config) *Pool {
	return &Pool{
		config: config,
	}
}

// Start 拉起并初始化所有 Worker。
func (pool *Pool) Start(ctx context.Context) error {
	if pool == nil {
		return errors.New("ssr worker pool is nil")
	}

	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	if pool.started {
		return nil
	}

	workerCount := pool.config.WorkerCount
	if workerCount <= 0 {
		workerCount = 1
	}

	pool.workers = make([]*worker.Process, 0, workerCount)
	for workerIndex := 0; workerIndex < workerCount; workerIndex += 1 {
		workerConfig := pool.config.Worker
		workerConfig.Name = fmt.Sprintf("ssr-worker-%d", workerIndex+1)
		workerConfig.Logger = pool.config.Logger

		nextWorker := worker.NewProcess(workerConfig)
		if err := nextWorker.Start(ctx); err != nil {
			for _, initializedWorker := range pool.workers {
				_ = initializedWorker.Stop(context.Background())
			}
			return fmt.Errorf("start %s: %w", workerConfig.Name, err)
		}
		pool.workers = append(pool.workers, nextWorker)
	}

	pool.started = true
	return nil
}

// Close 停止池内所有 Worker。
func (pool *Pool) Close(ctx context.Context) error {
	if pool == nil {
		return nil
	}

	pool.mutex.Lock()
	workers := pool.workers
	pool.workers = nil
	pool.started = false
	pool.mutex.Unlock()

	var stopErrors []error
	for _, process := range workers {
		if process == nil {
			continue
		}
		if err := process.Stop(ctx); err != nil {
			stopErrors = append(stopErrors, err)
		}
	}
	if len(stopErrors) > 0 {
		return errors.Join(stopErrors...)
	}
	return nil
}

// Render 将渲染请求分发到某个 Worker 并返回结果。
func (pool *Pool) Render(ctx context.Context, request protocol.RenderRequest) (protocol.RenderResponse, error) {
	if pool == nil {
		return protocol.RenderResponse{}, errors.New("ssr worker pool is nil")
	}

	selectedWorker, err := pool.pickWorker()
	if err != nil {
		return protocol.RenderResponse{}, err
	}

	response, err := selectedWorker.Render(ctx, request)
	if err == nil {
		return response, nil
	}

	// 不可用错误先尝试一次重启再重试，减少短时抖动导致的降级概率。
	if worker.IsUnavailableError(err) {
		restartContext, cancelRestart := context.WithTimeout(context.Background(), 3*time.Second)
		restartErr := selectedWorker.Restart(restartContext)
		cancelRestart()
		if restartErr == nil {
			return selectedWorker.Render(ctx, request)
		}
	}

	return protocol.RenderResponse{}, err
}

func (pool *Pool) pickWorker() (*worker.Process, error) {
	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	if !pool.started || len(pool.workers) == 0 {
		return nil, errors.New("ssr worker pool not started")
	}

	index := int(pool.rrCursor.Add(1)-1) % len(pool.workers)
	return pool.workers[index], nil
}
