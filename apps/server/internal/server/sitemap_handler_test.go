package server

import (
	"context"
	"encoding/xml"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
)

type sitemapTestRecord struct {
	SpaceID            string
	NodeID             string
	DocumentID         string
	SpaceVisibility    string
	SpaceStatus        string
	SpaceUpdatedAt     string
	DocumentVisibility string
	DocumentStatus     string
	DocumentUpdatedAt  string
	ContentMD          string
}

func setupSitemapTestRouter(t *testing.T) (*storage.Database, func(*http.Request) *httptest.ResponseRecorder) {
	t.Helper()

	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-sitemap-handler?mode=memory&cache=shared",
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

func TestRouter_Sitemap_OnlyIncludesFullyPublicNonEmptyContent(t *testing.T) {
	database, serve := setupSitemapTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	ownerUserID := "01h3sitemapuser00000000000001"
	if err := database.ORM.Table("users").Create(map[string]any{
		"user_id":       ownerUserID,
		"email":         "sitemap-owner@example.com",
		"password_hash": "hashed",
		"name":          "sitemap-owner",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("insert sitemap owner failed: %v", err)
	}

	seedSitemapRecord(t, database, ownerUserID, now, sitemapTestRecord{
		SpaceID:            "space-public-visible",
		NodeID:             "node-public-visible",
		DocumentID:         "doc-public-visible",
		SpaceVisibility:    "public",
		DocumentVisibility: "public",
		ContentMD:          "# public content",
	})
	seedSitemapRecord(t, database, ownerUserID, now, sitemapTestRecord{
		SpaceID:            "space-public-empty",
		NodeID:             "node-public-empty",
		DocumentID:         "doc-public-empty",
		SpaceVisibility:    "public",
		DocumentVisibility: "public",
		ContentMD:          " \n\t ",
	})
	seedSitemapRecord(t, database, ownerUserID, now, sitemapTestRecord{
		SpaceID:            "space-member",
		NodeID:             "node-member",
		DocumentID:         "doc-member",
		SpaceVisibility:    "member",
		DocumentVisibility: "public",
		ContentMD:          "# hidden by space visibility",
	})
	seedSitemapRecord(t, database, ownerUserID, now, sitemapTestRecord{
		SpaceID:            "space-public-doc-member",
		NodeID:             "node-public-doc-member",
		DocumentID:         "doc-public-doc-member",
		SpaceVisibility:    "public",
		DocumentVisibility: "member",
		ContentMD:          "# hidden by doc visibility",
	})
	seedSitemapRecord(t, database, ownerUserID, now, sitemapTestRecord{
		SpaceID:            "space-public-doc-banned",
		NodeID:             "node-public-doc-banned",
		DocumentID:         "doc-public-doc-banned",
		SpaceVisibility:    "public",
		DocumentVisibility: "public",
		DocumentStatus:     "banned",
		ContentMD:          "# hidden by doc status",
	})
	seedSitemapRecord(t, database, ownerUserID, now, sitemapTestRecord{
		SpaceID:            "space-public-doc-deleted",
		NodeID:             "node-public-doc-deleted",
		DocumentID:         "doc-public-doc-deleted",
		SpaceVisibility:    "public",
		DocumentVisibility: "public",
		DocumentStatus:     "deleted",
		ContentMD:          "# hidden by deleted status",
	})

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	rec := serve(req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if gotType := rec.Header().Get("Content-Type"); !strings.Contains(gotType, "application/xml") {
		t.Fatalf("expected xml content-type, got %q", gotType)
	}

	var payload struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode sitemap xml failed: %v body=%s", err, rec.Body.String())
	}

	locSet := make(map[string]struct{}, len(payload.URLs))
	for _, item := range payload.URLs {
		locSet[strings.TrimSpace(item.Loc)] = struct{}{}
	}

	expectedLocs := []string{
		"http://example.com/",
		"http://example.com/r/space-public-visible",
		"http://example.com/r/space-public-visible/doc-public-visible",
	}
	if len(locSet) != len(expectedLocs) {
		t.Fatalf("expected %d sitemap urls, got %d: %v", len(expectedLocs), len(locSet), locSet)
	}
	for _, expected := range expectedLocs {
		if _, ok := locSet[expected]; !ok {
			t.Fatalf("expected sitemap contains %q, got %v", expected, locSet)
		}
	}

	unexpectedLocs := []string{
		"http://example.com/r/space-public-empty",
		"http://example.com/r/space-public-empty/doc-public-empty",
		"http://example.com/r/space-member",
		"http://example.com/r/space-member/doc-member",
		"http://example.com/r/space-public-doc-member/doc-public-doc-member",
		"http://example.com/r/space-public-doc-banned/doc-public-doc-banned",
		"http://example.com/r/space-public-doc-deleted/doc-public-doc-deleted",
	}
	for _, unexpected := range unexpectedLocs {
		if _, ok := locSet[unexpected]; ok {
			t.Fatalf("expected sitemap not contains %q, got %v", unexpected, locSet)
		}
	}
}

func TestRouter_Sitemap_RespectsUpdatedWithinDaysConfig(t *testing.T) {
	database, serve := setupSitemapTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	nowAt := time.Now().UTC()
	now := nowAt.Format(time.RFC3339Nano)
	oldAt := nowAt.AddDate(0, 0, -120).Format(time.RFC3339Nano)

	ownerUserID := "01h3sitemapcfguser000000000001"
	if err := database.ORM.Table("users").Create(map[string]any{
		"user_id":       ownerUserID,
		"email":         "sitemap-rule-owner@example.com",
		"password_hash": "hashed",
		"name":          "sitemap-rule-owner",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("insert sitemap owner failed: %v", err)
	}

	if err := database.ORM.Table("system_configs").Create(map[string]any{
		"config_key":         "sitemap",
		"config_value_json":  `{"generationMode":"updated_within_days","maxUpdatedWithinDays":30}`,
		"version":            1,
		"updated_by_user_id": ownerUserID,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert sitemap config failed: %v", err)
	}

	seedSitemapRecord(t, database, ownerUserID, now, sitemapTestRecord{
		SpaceID:            "space-recent",
		NodeID:             "node-recent",
		DocumentID:         "doc-recent",
		SpaceVisibility:    "public",
		DocumentVisibility: "public",
		DocumentUpdatedAt:  now,
		ContentMD:          "# recent content",
	})
	seedSitemapRecord(t, database, ownerUserID, now, sitemapTestRecord{
		SpaceID:            "space-old",
		NodeID:             "node-old",
		DocumentID:         "doc-old",
		SpaceVisibility:    "public",
		DocumentVisibility: "public",
		DocumentUpdatedAt:  oldAt,
		ContentMD:          "# old content",
	})

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	rec := serve(req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode sitemap xml failed: %v body=%s", err, rec.Body.String())
	}

	locSet := make(map[string]struct{}, len(payload.URLs))
	for _, item := range payload.URLs {
		locSet[strings.TrimSpace(item.Loc)] = struct{}{}
	}

	expectedLocs := []string{
		"http://example.com/",
		"http://example.com/r/space-recent",
		"http://example.com/r/space-recent/doc-recent",
	}
	for _, expected := range expectedLocs {
		if _, ok := locSet[expected]; !ok {
			t.Fatalf("expected sitemap contains %q, got %v", expected, locSet)
		}
	}

	unexpectedLocs := []string{
		"http://example.com/r/space-old",
		"http://example.com/r/space-old/doc-old",
	}
	for _, unexpected := range unexpectedLocs {
		if _, ok := locSet[unexpected]; ok {
			t.Fatalf("expected sitemap not contains %q, got %v", unexpected, locSet)
		}
	}
}

func TestRouter_RobotsTxt_ContainsSitemapDirective(t *testing.T) {
	logger := logit.NewLogger(slog.LevelError)
	router := NewRouter(testConfig(), logger, nil)

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Sitemap: /sitemap.xml") {
		t.Fatalf("expected robots.txt contains sitemap directive, body=%s", rec.Body.String())
	}
}

func seedSitemapRecord(
	t *testing.T,
	database *storage.Database,
	ownerUserID string,
	now string,
	record sitemapTestRecord,
) {
	t.Helper()

	spaceVisibility := strings.TrimSpace(record.SpaceVisibility)
	if spaceVisibility == "" {
		spaceVisibility = "public"
	}
	spaceStatus := strings.TrimSpace(record.SpaceStatus)
	if spaceStatus == "" {
		spaceStatus = "active"
	}
	documentVisibility := strings.TrimSpace(record.DocumentVisibility)
	if documentVisibility == "" {
		documentVisibility = "public"
	}
	documentStatus := strings.TrimSpace(record.DocumentStatus)
	if documentStatus == "" {
		documentStatus = "active"
	}

	spaceUpdatedAt := now
	if strings.TrimSpace(record.SpaceUpdatedAt) != "" {
		spaceUpdatedAt = strings.TrimSpace(record.SpaceUpdatedAt)
	}
	documentUpdatedAt := now
	if strings.TrimSpace(record.DocumentUpdatedAt) != "" {
		documentUpdatedAt = strings.TrimSpace(record.DocumentUpdatedAt)
	}

	if err := database.ORM.Table("spaces").Create(map[string]any{
		"space_id":      record.SpaceID,
		"name":          record.SpaceID,
		"owner_user_id": ownerUserID,
		"visibility":    spaceVisibility,
		"status":        spaceStatus,
		"created_at":    now,
		"updated_at":    spaceUpdatedAt,
	}).Error; err != nil {
		t.Fatalf("insert space failed: %v", err)
	}

	if err := database.ORM.Table("nodes").Create(map[string]any{
		"node_id":        record.NodeID,
		"space_id":       record.SpaceID,
		"parent_node_id": nil,
		"type":           "doc",
		"title":          record.DocumentID,
		"sort":           1,
		"created_at":     now,
		"updated_at":     now,
	}).Error; err != nil {
		t.Fatalf("insert node failed: %v", err)
	}

	if err := database.ORM.Table("documents").Create(map[string]any{
		"document_id":        record.DocumentID,
		"node_id":            record.NodeID,
		"theme_id":           "default",
		"title":              record.DocumentID,
		"content_md":         record.ContentMD,
		"version":            1,
		"visibility":         documentVisibility,
		"status":             documentStatus,
		"created_by_user_id": ownerUserID,
		"updated_by_user_id": ownerUserID,
		"created_at":         now,
		"updated_at":         documentUpdatedAt,
	}).Error; err != nil {
		t.Fatalf("insert document failed: %v", err)
	}
}
