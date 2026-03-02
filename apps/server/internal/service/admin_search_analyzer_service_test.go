package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	searchcfg "github.com/lifei6671/plaindoc/apps/server/internal/search"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

type stubAdminRoleRepository struct {
	isPlatformAdmin bool
}

func (r *stubAdminRoleRepository) HasRole(ctx context.Context, userID string, role models.AdminRole) (bool, error) {
	if role == models.AdminRolePlatformAdmin {
		return r.isPlatformAdmin, nil
	}
	return false, nil
}

func (r *stubAdminRoleRepository) ListByUserID(ctx context.Context, userID string) ([]models.AdminRole, error) {
	if r.isPlatformAdmin {
		return []models.AdminRole{models.AdminRolePlatformAdmin}, nil
	}
	return []models.AdminRole{}, nil
}

func (r *stubAdminRoleRepository) ListByUserIDs(
	ctx context.Context,
	userIDs []string,
) (map[string][]models.AdminRole, error) {
	result := make(map[string][]models.AdminRole, len(userIDs))
	for _, userID := range userIDs {
		roles, _ := r.ListByUserID(ctx, userID)
		result[userID] = roles
	}
	return result, nil
}

func (r *stubAdminRoleRepository) ReplaceByUserID(ctx context.Context, userID string, roles []models.AdminRole) error {
	return nil
}

type inMemorySearchAnalyzerDictEntryRepo struct {
	nextID int64
	items  map[int64]models.SearchAnalyzerDictEntry
}

func newInMemorySearchAnalyzerDictEntryRepo() *inMemorySearchAnalyzerDictEntryRepo {
	return &inMemorySearchAnalyzerDictEntryRepo{
		nextID: 1,
		items:  map[int64]models.SearchAnalyzerDictEntry{},
	}
}

func (r *inMemorySearchAnalyzerDictEntryRepo) List(
	ctx context.Context,
	params repository.ListSearchAnalyzerDictEntriesParams,
) ([]models.SearchAnalyzerDictEntry, int64, error) {
	analyzer := strings.ToLower(strings.TrimSpace(params.Analyzer))
	statuses := make(map[string]struct{}, len(params.Statuses))
	for _, status := range params.Statuses {
		statuses[strings.ToLower(strings.TrimSpace(status))] = struct{}{}
	}
	rows := make([]models.SearchAnalyzerDictEntry, 0, len(r.items))
	for _, item := range r.items {
		if strings.ToLower(strings.TrimSpace(item.Analyzer)) != analyzer {
			continue
		}
		if len(statuses) > 0 {
			if _, exists := statuses[strings.ToLower(strings.TrimSpace(item.Status))]; !exists {
				continue
			}
		}
		rows = append(rows, item)
	}
	slices.SortFunc(rows, func(a, b models.SearchAnalyzerDictEntry) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	total := int64(len(rows))
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(rows) {
		return []models.SearchAnalyzerDictEntry{}, total, nil
	}
	limit := params.Limit
	if limit <= 0 {
		limit = len(rows)
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return append([]models.SearchAnalyzerDictEntry(nil), rows[offset:end]...), total, nil
}

func (r *inMemorySearchAnalyzerDictEntryRepo) ListActiveByAnalyzer(
	ctx context.Context,
	analyzer string,
) ([]models.SearchAnalyzerDictEntry, error) {
	rows, _, err := r.List(ctx, repository.ListSearchAnalyzerDictEntriesParams{
		Analyzer: analyzer,
		Statuses: []string{models.SearchAnalyzerDictEntryStatusActive},
	})
	return rows, err
}

func (r *inMemorySearchAnalyzerDictEntryRepo) GetByID(
	ctx context.Context,
	id int64,
) (*models.SearchAnalyzerDictEntry, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cloned := item
	return &cloned, nil
}

func (r *inMemorySearchAnalyzerDictEntryRepo) GetByAnalyzerAndTerm(
	ctx context.Context,
	analyzer string,
	term string,
) (*models.SearchAnalyzerDictEntry, error) {
	normalizedAnalyzer := strings.ToLower(strings.TrimSpace(analyzer))
	normalizedTerm := strings.TrimSpace(term)
	for _, item := range r.items {
		if strings.ToLower(strings.TrimSpace(item.Analyzer)) == normalizedAnalyzer &&
			strings.TrimSpace(item.Term) == normalizedTerm {
			cloned := item
			return &cloned, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *inMemorySearchAnalyzerDictEntryRepo) Create(
	ctx context.Context,
	entry *models.SearchAnalyzerDictEntry,
) error {
	if entry == nil {
		return nil
	}
	cloned := *entry
	cloned.ID = r.nextID
	r.nextID += 1
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = time.Now().UTC()
	}
	if cloned.UpdatedAt.IsZero() {
		cloned.UpdatedAt = cloned.CreatedAt
	}
	r.items[cloned.ID] = cloned
	entry.ID = cloned.ID
	return nil
}

func (r *inMemorySearchAnalyzerDictEntryRepo) UpdateByID(
	ctx context.Context,
	id int64,
	updates map[string]any,
) (bool, error) {
	current, ok := r.items[id]
	if !ok {
		return false, nil
	}
	if term, exists := updates["term"]; exists {
		if value, ok := term.(string); ok {
			current.Term = value
		}
	}
	if weight, exists := updates["weight"]; exists {
		if weight == nil {
			current.Weight = nil
		} else if value, ok := weight.(*int); ok {
			current.Weight = value
		}
	}
	if tag, exists := updates["tag"]; exists {
		if value, ok := tag.(string); ok {
			current.Tag = value
		}
	}
	if status, exists := updates["status"]; exists {
		if value, ok := status.(string); ok {
			current.Status = value
		}
	}
	if updatedBy, exists := updates["updated_by_user_id"]; exists {
		if updatedBy == nil {
			current.UpdatedByUserID = nil
		} else if value, ok := updatedBy.(*string); ok {
			current.UpdatedByUserID = value
		}
	}
	if updatedAt, exists := updates["updated_at"]; exists {
		if value, ok := updatedAt.(time.Time); ok {
			current.UpdatedAt = value
		}
	}
	r.items[id] = current
	return true, nil
}

func TestAdminSearchAnalyzerService_CreateAndDeleteDictEntry(t *testing.T) {
	ctx := context.Background()
	dictRepo := newInMemorySearchAnalyzerDictEntryRepo()
	adminAccessService := NewAdminAccessService(&stubAdminRoleRepository{isPlatformAdmin: true}, nil, nil)
	service := NewAdminSearchAnalyzerService(dictRepo, nil, adminAccessService, nil, nil)

	weight200 := 200
	created, err := service.CreateDictEntry(ctx, UpsertAdminSearchAnalyzerDictEntryInput{
		ActorUserID: "admin-user-id",
		Analyzer:    "jieba",
		Term:        "微服务架构",
		Weight:      &weight200,
		Tag:         "n",
	})
	if err != nil {
		t.Fatalf("create dict entry failed: %v", err)
	}
	if created.ID <= 0 {
		t.Fatalf("expected positive id, got %d", created.ID)
	}
	if created.Status != models.SearchAnalyzerDictEntryStatusActive {
		t.Fatalf("expected active status, got %q", created.Status)
	}

	if _, err := service.CreateDictEntry(ctx, UpsertAdminSearchAnalyzerDictEntryInput{
		ActorUserID: "admin-user-id",
		Analyzer:    "jieba",
		Term:        "微服务架构",
	}); err == nil {
		t.Fatal("expected duplicated term create error")
	}

	deleted, err := service.DeleteDictEntry(ctx, DeleteAdminSearchAnalyzerDictEntryInput{
		ActorUserID: "admin-user-id",
		Analyzer:    "jieba",
		EntryID:     created.ID,
	})
	if err != nil {
		t.Fatalf("delete dict entry failed: %v", err)
	}
	if deleted.Status != models.SearchAnalyzerDictEntryStatusDeleted {
		t.Fatalf("expected deleted status, got %q", deleted.Status)
	}
}

func TestAdminSearchAnalyzerService_CreateDictEntry_SyncsSearchDictVersion(t *testing.T) {
	ctx := context.Background()
	dictRepo := newInMemorySearchAnalyzerDictEntryRepo()
	systemConfigRepo := &stubSystemConfigRepository{
		recordByKey: map[string]*models.SystemConfig{
			searchcfg.SystemConfigKey: {
				ConfigKey: searchcfg.SystemConfigKey,
				ConfigValueJSON: `{
					"enabled":true,
					"activeProvider":"bleve",
					"fallbackPolicy":"degrade_to_database",
					"analysis":{
						"activeAnalyzer":"jieba",
						"analyzers":{
							"simple":{"enabled":true},
							"jieba":{
								"enabled":true,
								"mode":"search",
								"hmm":true,
								"stopwordsEnabled":false,
								"dictSource":"db",
								"dictVersion":"v2026-03-02-001"
							}
						}
					}
				}`,
				Version: 2,
			},
		},
		errByKey: map[string]error{},
	}
	searchConfigService := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})
	adminAccessService := NewAdminAccessService(&stubAdminRoleRepository{isPlatformAdmin: true}, nil, nil)
	service := NewAdminSearchAnalyzerService(dictRepo, systemConfigRepo, adminAccessService, nil, searchConfigService)

	if _, err := service.CreateDictEntry(ctx, UpsertAdminSearchAnalyzerDictEntryInput{
		ActorUserID: "admin-user-id",
		Analyzer:    "jieba",
		Term:        "可观察性平台",
		Tag:         "n",
	}); err != nil {
		t.Fatalf("create dict entry failed: %v", err)
	}

	record, err := systemConfigRepo.GetByConfigKey(ctx, searchcfg.SystemConfigKey)
	if err != nil {
		t.Fatalf("load search config failed: %v", err)
	}
	if record.Version != 3 {
		t.Fatalf("expected search config version 3, got %d", record.Version)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(record.ConfigValueJSON), &payload); err != nil {
		t.Fatalf("unmarshal search config failed: %v", err)
	}
	config := searchcfg.NormalizeConfig(payload)
	if config.Analysis.Analyzers.Jieba.DictVersion == "v2026-03-02-001" {
		t.Fatalf("expected dict version changed, got %q", config.Analysis.Analyzers.Jieba.DictVersion)
	}
	if !strings.HasPrefix(config.Analysis.Analyzers.Jieba.DictVersion, "v") {
		t.Fatalf("expected dict version prefix v, got %q", config.Analysis.Analyzers.Jieba.DictVersion)
	}

	analyzers, err := service.ListAnalyzers(ctx, "admin-user-id")
	if err != nil {
		t.Fatalf("list analyzers failed: %v", err)
	}
	var jiebaVersion string
	for _, item := range analyzers {
		if item.Name == "jieba" {
			jiebaVersion = item.DictVersion
			break
		}
	}
	if jiebaVersion == "" {
		t.Fatal("expected jieba analyzer record")
	}
	if jiebaVersion != config.Analysis.Analyzers.Jieba.DictVersion {
		t.Fatalf("expected analyzer dict version %q, got %q", config.Analysis.Analyzers.Jieba.DictVersion, jiebaVersion)
	}
}

func TestAdminSearchAnalyzerService_ReloadAnalyzer(t *testing.T) {
	ctx := context.Background()
	systemConfigRepo := &stubSystemConfigRepository{
		recordByKey: map[string]*models.SystemConfig{
			searchcfg.SystemConfigKey: {
				ConfigKey: searchcfg.SystemConfigKey,
				ConfigValueJSON: `{
					"enabled":true,"activeProvider":"bleve",
					"fallbackPolicy":"degrade_to_bleve",
					"analysis":{
						"activeAnalyzer":"jieba",
						"analyzers":{
							"simple":{"enabled":false},
							"jieba":{
								"enabled":true,
								"mode":"search",
								"hmm":true,
								"stopwordsEnabled":false,
								"dictSource":"db",
								"dictVersion":"v2026-03-02-001"
							}
						}
					}
				}`,
				Version: 2,
			},
		},
		errByKey: map[string]error{},
	}
	searchConfigService := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})
	adminAccessService := NewAdminAccessService(&stubAdminRoleRepository{isPlatformAdmin: true}, nil, nil)
	service := NewAdminSearchAnalyzerService(nil, nil, adminAccessService, nil, searchConfigService)

	result, err := service.ReloadAnalyzer(ctx, ReloadAdminSearchAnalyzerInput{
		ActorUserID: "admin-user-id",
		Analyzer:    "jieba",
	})
	if err != nil {
		t.Fatalf("reload analyzer failed: %v", err)
	}
	if result.Analyzer != "jieba" {
		t.Fatalf("expected analyzer jieba, got %q", result.Analyzer)
	}
	if result.ActiveAnalyzer != "jieba" {
		t.Fatalf("expected active analyzer jieba, got %q", result.ActiveAnalyzer)
	}
	if result.DictVersion != "v2026-03-02-001" {
		t.Fatalf("expected dict version v2026-03-02-001, got %q", result.DictVersion)
	}
}

func TestAdminSearchAnalyzerService_AnalyzePreview(t *testing.T) {
	ctx := context.Background()
	dictRepo := newInMemorySearchAnalyzerDictEntryRepo()
	weight := 200
	if err := dictRepo.Create(ctx, &models.SearchAnalyzerDictEntry{
		Analyzer: "jieba",
		Term:     "微服务架构",
		Weight:   &weight,
		Status:   models.SearchAnalyzerDictEntryStatusActive,
	}); err != nil {
		t.Fatalf("create dict entry failed: %v", err)
	}

	systemConfigRepo := &stubSystemConfigRepository{
		recordByKey: map[string]*models.SystemConfig{
			searchcfg.SystemConfigKey: {
				ConfigKey: searchcfg.SystemConfigKey,
				ConfigValueJSON: `{
					"enabled":true,"activeProvider":"bleve",
					"fallbackPolicy":"degrade_to_bleve",
					"analysis":{
						"activeAnalyzer":"jieba",
						"analyzers":{
							"simple":{"enabled":false},
							"jieba":{
								"enabled":true,
								"mode":"search",
								"hmm":true,
								"stopwordsEnabled":false,
								"dictSource":"db",
								"dictVersion":"v2026-03-02-005"
							}
						}
					}
				}`,
				Version: 5,
			},
		},
		errByKey: map[string]error{},
	}
	searchConfigService := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})
	adminAccessService := NewAdminAccessService(&stubAdminRoleRepository{isPlatformAdmin: true}, nil, nil)
	service := NewAdminSearchAnalyzerService(dictRepo, nil, adminAccessService, nil, searchConfigService)

	result, err := service.AnalyzePreview(ctx, AnalyzePreviewAdminSearchAnalyzerInput{
		ActorUserID: "admin-user-id",
		Analyzer:    "jieba",
		Mode:        "query",
		Text: "```go\nfmt.Println(\"hello\")\n```\n" +
			"微服务架构设计",
	})
	if err != nil {
		t.Fatalf("analyze preview failed: %v", err)
	}
	if result.Analyzer != "jieba" {
		t.Fatalf("expected analyzer jieba, got %q", result.Analyzer)
	}
	if result.Mode != "query" {
		t.Fatalf("expected mode query, got %q", result.Mode)
	}
	if !slices.Contains(result.Tokens, "微服务架构") {
		t.Fatalf("expected token list to contain custom term, got %v", result.Tokens)
	}
	if strings.Contains(result.NormalizedText, "fmt.Println") {
		t.Fatalf("expected normalized text to drop code block content, got %q", result.NormalizedText)
	}
	if result.DictVersion != "v2026-03-02-005" {
		t.Fatalf("expected dict version v2026-03-02-005, got %q", result.DictVersion)
	}
}

func TestAdminSearchAnalyzerService_AnalyzePreviewInvalidMode(t *testing.T) {
	ctx := context.Background()
	dictRepo := newInMemorySearchAnalyzerDictEntryRepo()
	systemConfigRepo := &stubSystemConfigRepository{
		recordByKey: map[string]*models.SystemConfig{
			searchcfg.SystemConfigKey: {
				ConfigKey: searchcfg.SystemConfigKey,
				ConfigValueJSON: `{
					"enabled":true,"activeProvider":"bleve",
					"fallbackPolicy":"degrade_to_bleve",
					"analysis":{
						"activeAnalyzer":"jieba",
						"analyzers":{
							"simple":{"enabled":false},
							"jieba":{
								"enabled":true,
								"mode":"search",
								"hmm":true,
								"stopwordsEnabled":false,
								"dictSource":"db",
								"dictVersion":"v2026-03-02-005"
							}
						}
					}
				}`,
				Version: 5,
			},
		},
		errByKey: map[string]error{},
	}

	searchConfigService := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})
	adminAccessService := NewAdminAccessService(&stubAdminRoleRepository{isPlatformAdmin: true}, nil, nil)
	service := NewAdminSearchAnalyzerService(dictRepo, nil, adminAccessService, nil, searchConfigService)

	_, err := service.AnalyzePreview(ctx, AnalyzePreviewAdminSearchAnalyzerInput{
		ActorUserID: "admin-user-id",
		Analyzer:    "jieba",
		Mode:        "invalid",
		Text:        "微服务架构",
	})
	if !errors.Is(err, errcode.ErrAdminSearchAnalyzerInvalidMode) {
		t.Fatalf("expected invalid mode error, got %v", err)
	}
}

func TestAdminSearchAnalyzerService_AnalyzePreviewAnalyzerDisabled(t *testing.T) {
	ctx := context.Background()
	dictRepo := newInMemorySearchAnalyzerDictEntryRepo()
	weight := 180
	if err := dictRepo.Create(ctx, &models.SearchAnalyzerDictEntry{
		Analyzer: "jieba",
		Term:     "微服务架构",
		Weight:   &weight,
		Status:   models.SearchAnalyzerDictEntryStatusActive,
	}); err != nil {
		t.Fatalf("create dict entry failed: %v", err)
	}
	systemConfigRepo := &stubSystemConfigRepository{
		recordByKey: map[string]*models.SystemConfig{
			searchcfg.SystemConfigKey: {
				ConfigKey: searchcfg.SystemConfigKey,
				ConfigValueJSON: `{
					"enabled":true,"activeProvider":"bleve",
					"fallbackPolicy":"degrade_to_bleve",
					"analysis":{
						"activeAnalyzer":"simple",
						"analyzers":{
							"simple":{"enabled":true},
							"jieba":{
								"enabled":false,
								"mode":"search",
								"hmm":true,
								"stopwordsEnabled":false,
								"dictSource":"db",
								"dictVersion":"v2026-03-02-005"
							}
						}
					}
				}`,
				Version: 5,
			},
		},
		errByKey: map[string]error{},
	}
	searchConfigService := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})
	adminAccessService := NewAdminAccessService(&stubAdminRoleRepository{isPlatformAdmin: true}, nil, nil)
	service := NewAdminSearchAnalyzerService(dictRepo, nil, adminAccessService, nil, searchConfigService)

	result, err := service.AnalyzePreview(ctx, AnalyzePreviewAdminSearchAnalyzerInput{
		ActorUserID: "admin-user-id",
		Analyzer:    "jieba",
		Text:        "微服务架构",
	})
	if err != nil {
		t.Fatalf("analyze preview failed when jieba disabled: %v", err)
	}
	if result.Analyzer != "jieba" {
		t.Fatalf("expected analyzer jieba, got %q", result.Analyzer)
	}
	if !slices.Contains(result.Tokens, "微服务架构") {
		t.Fatalf("expected token list to contain custom term, got %v", result.Tokens)
	}
}
