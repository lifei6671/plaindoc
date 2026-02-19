package logit

import (
	"log/slog"
	"testing"
)

func TestAny_Generic(t *testing.T) {
	attr := Any("payload", struct {
		ID   string `json:"id"`
		Size int    `json:"size"`
	}{
		ID:   "doc-1",
		Size: 3,
	})

	if attr.Key != "payload" {
		t.Fatalf("expected key payload, got %s", attr.Key)
	}
	if attr.Value.Kind() != slog.KindAny {
		t.Fatalf("expected kind any, got %v", attr.Value.Kind())
	}
}

func TestTypedAttrHelpers(t *testing.T) {
	if String("k", "v").Value.Kind() != slog.KindString {
		t.Fatal("String helper should create string attr")
	}
	if Int("n", 1).Value.Kind() != slog.KindInt64 {
		t.Fatal("Int helper should create int attr")
	}
	if Bool("ok", true).Value.Kind() != slog.KindBool {
		t.Fatal("Bool helper should create bool attr")
	}
}
