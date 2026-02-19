package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
)

func setupThemeTestRouter(t *testing.T) (*storage.Database, func(*http.Request) *httptest.ResponseRecorder) {
	t.Helper()

	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-theme-handler?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}

	if err := storage.MigrateUp(context.Background(), database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}

	logger := logit.NewLogger(slog.LevelError)
	router := NewRouter(testConfig(), logger, database.ORM)

	serve := func(req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	return database, serve
}

func TestRouter_ListThemes(t *testing.T) {
	database, serve := setupThemeTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	req := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
	rec := serve(req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var payload []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("expected at least one theme")
	}
	if payload[0].ID != "default" {
		t.Fatalf("expected first theme id default, got %s", payload[0].ID)
	}
}

func TestRouter_UpdateDocumentTheme(t *testing.T) {
	database, serve := setupThemeTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	userID := "01h0themeuser000000000000001"
	spaceID := "01h0themespace00000000000002"
	nodeID := "01h0themenode000000000000003"
	documentID := "01h0themedoc0000000000000004"
	customThemeID := "custom-tech"

	if err := database.ORM.Table("users").Create(map[string]any{
		"user_id":       userID,
		"email":         "theme@example.com",
		"password_hash": "hashed",
		"name":          "tester",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("insert user failed: %v", err)
	}
	if err := database.ORM.Table("spaces").Create(map[string]any{
		"space_id":      spaceID,
		"name":          "default",
		"owner_user_id": userID,
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("insert space failed: %v", err)
	}
	if err := database.ORM.Table("nodes").Create(map[string]any{
		"node_id":        nodeID,
		"space_id":       spaceID,
		"parent_node_id": nil,
		"type":           "doc",
		"title":          "hello",
		"sort":           1,
		"created_at":     now,
		"updated_at":     now,
	}).Error; err != nil {
		t.Fatalf("insert node failed: %v", err)
	}
	if err := database.ORM.Table("themes").Create(map[string]any{
		"theme_id":                   customThemeID,
		"name":                       "自定义主题",
		"description":                "for test",
		"variables_json":             "{}",
		"syntax_theme":               "one-light",
		"code_block_style_json":      "{}",
		"code_block_code_style_json": "{}",
		"inline_code_style_json":     "{}",
		"custom_css":                 "",
		"is_builtin":                 0,
		"created_at":                 now,
		"updated_at":                 now,
	}).Error; err != nil {
		t.Fatalf("insert custom theme failed: %v", err)
	}
	if err := database.ORM.Table("documents").Create(map[string]any{
		"document_id":        documentID,
		"node_id":            nodeID,
		"theme_id":           "default",
		"title":              "hello",
		"content_md":         "# hello",
		"version":            1,
		"updated_by_user_id": userID,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert document failed: %v", err)
	}

	requestBody := []byte(`{"themeId":"custom-tech"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/docs/"+documentID+"/theme", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		ID      string `json:"id"`
		ThemeID string `json:"themeId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload.ID != documentID {
		t.Fatalf("expected id %s, got %s", documentID, payload.ID)
	}
	if payload.ThemeID != customThemeID {
		t.Fatalf("expected theme id %s, got %s", customThemeID, payload.ThemeID)
	}

	var persisted struct {
		ThemeID string `gorm:"column:theme_id"`
	}
	if err := database.ORM.Table("documents").
		Select("theme_id").
		Where("document_id = ?", documentID).
		Scan(&persisted).Error; err != nil {
		t.Fatalf("query document theme failed: %v", err)
	}
	if persisted.ThemeID != customThemeID {
		t.Fatalf("expected persisted theme id %s, got %s", customThemeID, persisted.ThemeID)
	}
}
