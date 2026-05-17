package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStartupHandlerServesHTMLForPageRoutes(t *testing.T) {
	state := NewStartupState()
	state.SetMigrationProgress(MigrationStartupProgress{
		Phase:          "applying",
		TotalCount:     34,
		PendingCount:   4,
		AppliedCount:   31,
		CurrentVersion: 32,
		CurrentName:    "reader_slug",
	})
	handler := NewStartupHandler(state)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/spaces", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for startup page, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "PlainDoc 正在初始化") {
		t.Fatalf("expected startup title in html: %s", body)
	}
	if !strings.Contains(body, "/api/startup/status") {
		t.Fatalf("expected polling endpoint in html: %s", body)
	}
}

func TestStartupHandlerStatusEndpointReturnsSnapshot(t *testing.T) {
	state := NewStartupState()
	state.SetMigrationProgress(MigrationStartupProgress{
		Phase:          "applying",
		TotalCount:     34,
		PendingCount:   4,
		AppliedCount:   31,
		CurrentVersion: 32,
		CurrentName:    "reader_slug",
	})
	handler := NewStartupHandler(state)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/startup/status", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 status endpoint, got %d", recorder.Code)
	}
	var snapshot StartupSnapshot
	if err := json.Unmarshal(recorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode startup snapshot failed: %v", err)
	}
	if snapshot.Phase != StartupPhaseMigrating || snapshot.CurrentName != "reader_slug" {
		t.Fatalf("unexpected startup snapshot: %+v", snapshot)
	}
}

func TestStartupHandlerHealthzReflectsStartupState(t *testing.T) {
	state := NewStartupState()
	handler := NewStartupHandler(state)

	startingRecorder := httptest.NewRecorder()
	handler.ServeHTTP(startingRecorder, httptest.NewRequest(http.MethodGet, "/api/healthz", nil))
	if startingRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 while starting, got %d", startingRecorder.Code)
	}

	state.MarkFailed("服务初始化失败，请查看服务日志。", nil)
	failedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(failedRecorder, httptest.NewRequest(http.MethodGet, "/api/healthz", nil))
	if failedRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 after failed startup, got %d", failedRecorder.Code)
	}

	state.MarkReady()
	readyRecorder := httptest.NewRecorder()
	handler.ServeHTTP(readyRecorder, httptest.NewRequest(http.MethodGet, "/api/healthz", nil))
	if readyRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 after ready, got %d", readyRecorder.Code)
	}
}

func TestStartupHandlerReturnsServiceUnavailableForBusinessAPI(t *testing.T) {
	state := NewStartupState()
	handler := NewStartupHandler(state)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for business api while starting, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":50300`) || !strings.Contains(body, "服务正在初始化") {
		t.Fatalf("unexpected startup api response: %s", body)
	}
}
