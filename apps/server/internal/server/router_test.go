package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/config"
	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

func testConfig() config.Config {
	// 中文注释：测试仅关注路由与中间件行为，使用最小可运行配置。
	return config.Config{
		Env:            "test",
		Addr:           ":0",
		WebOrigin:      "http://localhost:5173",
		LogLevel:       slog.LevelError,
		RequestTimeout: 0,
		ReadTimeout:    0,
		WriteTimeout:   0,
		IdleTimeout:    0,
		Database: config.DatabaseConfig{
			Driver: "sqlite",
			DSN:    "file::memory:",
		},
		JWT: config.JWTConfig{
			Secret:          "test-secret",
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
		},
	}
}

func TestRouter_Healthz(t *testing.T) {
	logger := logit.NewLogger(slog.LevelError)
	router := NewRouter(testConfig(), logger, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	payload := decodeJSONResultData[map[string]any](t, rec.Body.Bytes())
	if payload["ok"] != true {
		t.Fatalf("expected ok=true, got %v", payload["ok"])
	}
}

func TestRouter_NoRouteUsesUnifiedErrorShape(t *testing.T) {
	logger := logit.NewLogger(slog.LevelError)
	router := NewRouter(testConfig(), logger, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/not-exists", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatal("expected X-Request-Id header to be set")
	}

	var payload struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Code != response.ResolveErrorCode("ROUTE_NOT_FOUND") {
		t.Fatalf("expected code %d, got %d", response.ResolveErrorCode("ROUTE_NOT_FOUND"), payload.Code)
	}
	if payload.RequestID == "" {
		t.Fatal("expected requestId in response body")
	}
}

func TestRouter_NoMethodUsesUnifiedErrorShape(t *testing.T) {
	logger := logit.NewLogger(slog.LevelError)
	router := NewRouter(testConfig(), logger, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatal("expected X-Request-Id header to be set")
	}

	var payload struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Code != response.ResolveErrorCode("METHOD_NOT_ALLOWED") {
		t.Fatalf("expected code %d, got %d", response.ResolveErrorCode("METHOD_NOT_ALLOWED"), payload.Code)
	}
	if payload.RequestID == "" {
		t.Fatal("expected requestId in response body")
	}
}

func TestRouter_RequestAttrsAreFlushedAtRequestEnd(t *testing.T) {
	var buffer bytes.Buffer
	logger := logit.NewLoggerWithWriter(slog.LevelInfo, &buffer)
	router := NewRouter(testConfig(), logger, nil)
	router.GET("/api/test-attrs", func(c *gin.Context) {
		logit.SetRequestAttrs(c.Request.Context(),
			logit.String("phase", "prepare"),
			logit.String("biz_id", "first"),
		)
		logit.SetRequestAttrs(c.Request.Context(),
			logit.String("phase", "finish"),
			logit.String("biz_id", "second"),
		)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test-attrs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	lines := bytes.Split(bytes.TrimSpace(buffer.Bytes()), []byte("\n"))
	if len(lines) == 0 {
		t.Fatal("expected aggregated access log line")
	}

	var logPayload map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &logPayload); err != nil {
		t.Fatalf("failed to decode aggregated log: %v", err)
	}
	if logPayload["phase"] != "finish" {
		t.Fatalf("expected phase=finish, got %v", logPayload["phase"])
	}
	if logPayload["biz_id"] != "second" {
		t.Fatalf("expected biz_id=second, got %v", logPayload["biz_id"])
	}
}
