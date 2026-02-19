package logit

import (
	"context"
	"log/slog"
	"testing"
)

func TestRequestAttrs_OverrideByKey(t *testing.T) {
	ctx := WithRequestAttrsContainer(context.Background())
	SetRequestAttrs(ctx,
		String("stage", "start"),
		String("user_id", "u-1"),
	)
	SetRequestAttrs(ctx,
		String("stage", "done"),
	)

	attrs := SnapshotRequestAttrs(ctx)
	result := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		if attr.Value.Kind() == slog.KindString {
			result[attr.Key] = attr.Value.String()
		}
	}

	if result["stage"] != "done" {
		t.Fatalf("expected stage=done, got %q", result["stage"])
	}
	if result["user_id"] != "u-1" {
		t.Fatalf("expected user_id=u-1, got %q", result["user_id"])
	}
}

func TestSetRequestAttrs_NoContainer_NoPanic(t *testing.T) {
	// 中文注释：未初始化请求容器时写入属性应被安全忽略，避免业务代码额外判空。
	SetRequestAttrs(context.Background(), String("k", "v"))
}
