package logit

import (
	"io"
	"log/slog"
	"os"
)

// NewLogger 创建统一结构化日志实例，供中间件与业务层复用。
func NewLogger(level slog.Level) *slog.Logger {
	return NewLoggerWithWriter(level, os.Stdout)
}

// NewLoggerWithWriter 支持将日志写入指定输出目标（控制台或文件）。
func NewLoggerWithWriter(level slog.Level, writer io.Writer) *slog.Logger {
	if writer == nil {
		writer = os.Stdout
	}

	handlerOptions := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewJSONHandler(writer, handlerOptions)
	return slog.New(handler)
}
