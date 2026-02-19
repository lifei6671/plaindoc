package logit

import (
	"log/slog"
	"time"
)

// String 创建 string 类型日志属性。
func String(key string, value string) slog.Attr {
	return slog.String(key, value)
}

// Int 创建 int 类型日志属性。
func Int(key string, value int) slog.Attr {
	return slog.Int(key, value)
}

// Int64 创建 int64 类型日志属性。
func Int64(key string, value int64) slog.Attr {
	return slog.Int64(key, value)
}

// Uint64 创建 uint64 类型日志属性。
func Uint64(key string, value uint64) slog.Attr {
	return slog.Uint64(key, value)
}

// Float64 创建 float64 类型日志属性。
func Float64(key string, value float64) slog.Attr {
	return slog.Float64(key, value)
}

// Bool 创建 bool 类型日志属性。
func Bool(key string, value bool) slog.Attr {
	return slog.Bool(key, value)
}

// Duration 创建 duration 类型日志属性。
func Duration(key string, value time.Duration) slog.Attr {
	return slog.Duration(key, value)
}

// Time 创建 time.Time 类型日志属性。
func Time(key string, value time.Time) slog.Attr {
	return slog.Time(key, value)
}

// Any 使用泛型创建任意类型日志属性。
func Any[T any](key string, value T) slog.Attr {
	return slog.Any(key, value)
}

// Error 创建 error 类型日志属性。
func Error(key string, err error) slog.Attr {
	return slog.Any(key, err)
}
