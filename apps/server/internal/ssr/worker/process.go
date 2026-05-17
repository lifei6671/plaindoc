package worker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/ssr/protocol"
)

var (
	ErrWorkerNotStarted       = errors.New("ssr worker process not started")
	ErrWorkerExited           = errors.New("ssr worker process exited")
	ErrWorkerProtocolMismatch = errors.New("ssr worker protocol version mismatch")
)

// Config 描述单个 SSR Worker 进程配置。
type Config struct {
	Name             string
	Exec             string
	Entry            string
	Args             []string
	Env              []string
	ProtocolVersion  string
	RenderTimeout    time.Duration
	MaxPayloadBytes  int64
	MaxResponseBytes int64
	Logger           *slog.Logger
}

// Process 封装一个 Node SSR Worker 子进程。
type Process struct {
	config Config

	mutex sync.Mutex

	command *exec.Cmd
	stdin   io.WriteCloser
	codec   *stdioCodec

	waitDoneCh chan error
	exited     bool
	exitErr    error
}

// NewProcess 创建一个 SSR Worker 进程实例。
func NewProcess(config Config) *Process {
	return &Process{config: config}
}

// Start 启动 Worker 进程。
func (process *Process) Start(ctx context.Context) error {
	if process == nil {
		return errors.New("ssr worker process is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	process.mutex.Lock()
	defer process.mutex.Unlock()

	if process.isRunningLocked() {
		return nil
	}

	if strings.TrimSpace(process.config.Exec) == "" {
		return errors.New("ssr worker exec is empty")
	}
	if strings.TrimSpace(process.config.Entry) == "" {
		return errors.New("ssr worker entry is empty")
	}

	commandArgs := buildSSRWorkerCommandArgs(process.config)

	process.logInfo("ssr worker starting", "exec", process.config.Exec, "args", commandArgs)
	command := exec.Command(process.config.Exec, commandArgs...)
	command.Env = append(os.Environ(), process.config.Env...)

	stdinWriter, err := command.StdinPipe()
	if err != nil {
		return fmt.Errorf("create ssr worker stdin pipe: %w", err)
	}
	stdoutReader, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create ssr worker stdout pipe: %w", err)
	}
	stderrReader, err := command.StderrPipe()
	if err != nil {
		return fmt.Errorf("create ssr worker stderr pipe: %w", err)
	}

	if err := command.Start(); err != nil {
		return fmt.Errorf("start ssr worker process: %w", err)
	}

	process.command = command
	process.stdin = stdinWriter
	// 请求 payload 与响应 HTML 的体积膨胀比例不同，stdout 读取使用独立上限，避免大 EPUB 阅读页 SSR 被 1MiB 请求上限误杀。
	// 兼容直接构造 worker.Config 的旧测试或调用方：未显式设置响应上限时，仍回退到原来的请求上限语义。
	maxResponseBytes := process.config.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = process.config.MaxPayloadBytes
	}
	process.codec = newStdioCodec(stdoutReader, stdinWriter, maxResponseBytes)
	process.waitDoneCh = make(chan error, 1)
	process.exited = false
	process.exitErr = nil

	go process.forwardStdErr(stderrReader)
	go func() {
		process.waitDoneCh <- command.Wait()
	}()

	if err := process.performHandshakeLocked(ctx); err != nil {
		_ = process.killLocked()
		process.logError("ssr worker handshake failed", err)
		return fmt.Errorf("ssr worker handshake failed: %w", err)
	}

	process.logInfo("ssr worker started", "pid", command.Process.Pid)
	return nil
}

func buildSSRWorkerCommandArgs(config Config) []string {
	commandArgs := make([]string, 0, 2+len(config.Args))
	if isNodeSSRWorkerExec(config.Exec) {
		// 当前 Node/V8 版本在大空间导出触发复杂 SSR 渲染时可能出现原生优化器崩溃；
		// 关闭 V8 优化比在 Worker 崩溃后重试更稳定，也和本仓库前端构建的既有规避方式一致。
		commandArgs = append(commandArgs, "--no-opt")
	}
	commandArgs = append(commandArgs, config.Entry)
	commandArgs = append(commandArgs, config.Args...)
	return commandArgs
}

func isNodeSSRWorkerExec(execPath string) bool {
	execName := strings.ToLower(strings.TrimSpace(filepath.Base(execPath)))
	return execName == "node" || execName == "node.exe"
}

// Stop 停止 Worker 进程。
func (process *Process) Stop(ctx context.Context) error {
	if process == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	process.mutex.Lock()
	if !process.isRunningLocked() {
		process.mutex.Unlock()
		return nil
	}

	command := process.command
	waitDoneCh := process.waitDoneCh
	process.command = nil
	process.stdin = nil
	process.codec = nil
	process.waitDoneCh = nil
	process.exited = true
	process.mutex.Unlock()

	if command != nil && command.Process != nil {
		_ = command.Process.Kill()
	}

	if waitDoneCh != nil {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait ssr worker stop: %w", ctx.Err())
		case <-waitDoneCh:
		}
	}

	process.logInfo("ssr worker stopped")
	return nil
}

// Restart 重启 Worker 进程。
func (process *Process) Restart(ctx context.Context) error {
	if process == nil {
		return errors.New("ssr worker process is nil")
	}

	if err := process.Stop(ctx); err != nil {
		return err
	}
	return process.Start(ctx)
}

// Render 请求 Worker 渲染并返回结果。
func (process *Process) Render(ctx context.Context, request protocol.RenderRequest) (protocol.RenderResponse, error) {
	if process == nil {
		return protocol.RenderResponse{}, errors.New("ssr worker process is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	process.mutex.Lock()
	defer process.mutex.Unlock()

	if !process.isRunningLocked() {
		return protocol.RenderResponse{}, ErrWorkerNotStarted
	}
	if process.hasExitedLocked() {
		return protocol.RenderResponse{}, ErrWorkerExited
	}
	if process.codec == nil {
		return protocol.RenderResponse{}, ErrWorkerNotStarted
	}

	requestPayloadSize := int64(len(request.Payload))
	if process.config.MaxPayloadBytes > 0 && requestPayloadSize > process.config.MaxPayloadBytes {
		return protocol.RenderResponse{}, fmt.Errorf(
			"ssr worker payload too large: %d > %d",
			requestPayloadSize,
			process.config.MaxPayloadBytes,
		)
	}

	normalizedRequest := request
	normalizedRequest.ID = strings.TrimSpace(normalizedRequest.ID)
	if normalizedRequest.ID == "" {
		return protocol.RenderResponse{}, errors.New("ssr worker request id is empty")
	}
	normalizedRequest.Route = strings.TrimSpace(normalizedRequest.Route)
	if normalizedRequest.Route == "" {
		return protocol.RenderResponse{}, errors.New("ssr worker request route is empty")
	}
	if normalizedRequest.Type == "" {
		normalizedRequest.Type = protocol.MessageTypeRender
	}
	if strings.TrimSpace(normalizedRequest.Version) == "" {
		normalizedRequest.Version = process.config.ProtocolVersion
	}
	if normalizedRequest.DeadlineMS <= 0 && process.config.RenderTimeout > 0 {
		normalizedRequest.DeadlineMS = process.config.RenderTimeout.Milliseconds()
	}

	if err := process.codec.WriteJSON(normalizedRequest); err != nil {
		process.logError("ssr worker write request failed", err)
		_ = process.killLocked()
		return protocol.RenderResponse{}, err
	}

	renderContext := ctx
	cancelRenderContext := func() {}
	if process.config.RenderTimeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			renderContext, cancelRenderContext = context.WithTimeout(ctx, process.config.RenderTimeout)
		}
	}
	defer cancelRenderContext()

	type readResult struct {
		response protocol.RenderResponse
		err      error
	}
	readResultChannel := make(chan readResult, 1)
	go func() {
		var response protocol.RenderResponse
		err := process.codec.ReadJSON(&response)
		readResultChannel <- readResult{response: response, err: err}
	}()

	select {
	case <-renderContext.Done():
		process.logError("ssr worker render timeout", renderContext.Err())
		_ = process.killLocked()
		return protocol.RenderResponse{}, renderContext.Err()
	case readResultValue := <-readResultChannel:
		if readResultValue.err != nil {
			process.logError("ssr worker read response failed", readResultValue.err)
			_ = process.killLocked()
			return protocol.RenderResponse{}, readResultValue.err
		}
		if strings.TrimSpace(readResultValue.response.ID) != normalizedRequest.ID {
			responseIDMismatchError := fmt.Errorf(
				"ssr worker response id mismatch: request=%s response=%s",
				normalizedRequest.ID,
				strings.TrimSpace(readResultValue.response.ID),
			)
			process.logError("ssr worker response id mismatch", responseIDMismatchError)
			_ = process.killLocked()
			return protocol.RenderResponse{}, responseIDMismatchError
		}
		return readResultValue.response, nil
	}
}

// IsUnavailableError 判断错误是否属于 Worker 不可用类错误。
func IsUnavailableError(err error) bool {
	return errors.Is(err, ErrWorkerNotStarted) || errors.Is(err, ErrWorkerExited)
}

func (process *Process) killLocked() error {
	if process.command == nil || process.command.Process == nil {
		return nil
	}
	return process.command.Process.Kill()
}

func (process *Process) isRunningLocked() bool {
	return process.command != nil && process.stdin != nil && process.codec != nil
}

func (process *Process) hasExitedLocked() bool {
	if process.waitDoneCh == nil {
		if process.exited {
			return true
		}
		return false
	}

	select {
	case waitErr := <-process.waitDoneCh:
		process.exited = true
		process.exitErr = waitErr
		process.waitDoneCh = nil
		process.command = nil
		process.stdin = nil
		process.codec = nil
		process.logInfo("ssr worker exited", "error", errorToString(waitErr))
		return true
	default:
		return false
	}
}

func (process *Process) forwardStdErr(stderrReader io.Reader) {
	if stderrReader == nil {
		return
	}

	scanner := bufio.NewScanner(stderrReader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		process.logInfo("ssr worker stderr", "line", line)
	}
}

func (process *Process) performHandshakeLocked(ctx context.Context) error {
	if process == nil || process.codec == nil {
		return ErrWorkerNotStarted
	}

	protocolVersion := strings.TrimSpace(process.config.ProtocolVersion)
	if protocolVersion == "" {
		return errors.New("ssr worker protocol version is empty")
	}

	handshakeRequest := protocol.HandshakeRequest{
		Type:    protocol.MessageTypeHandshake,
		Version: protocolVersion,
	}
	if err := process.codec.WriteJSON(handshakeRequest); err != nil {
		return err
	}

	type handshakeResult struct {
		response protocol.HandshakeResponse
		err      error
	}
	handshakeResultChannel := make(chan handshakeResult, 1)
	go func() {
		var response protocol.HandshakeResponse
		err := process.codec.ReadJSON(&response)
		handshakeResultChannel <- handshakeResult{response: response, err: err}
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("ssr worker handshake timeout: %w", ctx.Err())
	case result := <-handshakeResultChannel:
		if result.err != nil {
			return result.err
		}
		if !result.response.OK {
			if result.response.Error != nil && strings.TrimSpace(result.response.Error.Message) != "" {
				return errors.New(strings.TrimSpace(result.response.Error.Message))
			}
			return errors.New("ssr worker handshake rejected")
		}

		responseVersion := strings.TrimSpace(result.response.Version)
		if responseVersion == "" {
			return errors.New("ssr worker handshake response version is empty")
		}
		if responseVersion != protocolVersion {
			return fmt.Errorf(
				"%w: expected=%s got=%s",
				ErrWorkerProtocolMismatch,
				protocolVersion,
				responseVersion,
			)
		}
		return nil
	}
}

func (process *Process) logInfo(message string, attrs ...any) {
	if process == nil || process.config.Logger == nil {
		return
	}
	baseAttrs := []any{"worker_name", strings.TrimSpace(process.config.Name)}
	process.config.Logger.Info(message, append(baseAttrs, attrs...)...)
}

func (process *Process) logError(message string, err error) {
	if process == nil || process.config.Logger == nil {
		return
	}
	process.config.Logger.Error(
		message,
		"worker_name", strings.TrimSpace(process.config.Name),
		"error", errorToString(err),
	)
}

func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
