package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	searchcfg "github.com/lifei6671/plaindoc/apps/server/internal/search"
	searchanalyzer "github.com/lifei6671/plaindoc/apps/server/internal/search/analyzer"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	defaultAdminSearchAnalyzerPage     = 1
	defaultAdminSearchAnalyzerPageSize = 20
	maxAdminSearchAnalyzerPageSize     = 100
	adminSearchAnalyzerModeQuery       = "query"
	adminSearchAnalyzerModeIndex       = "index"
)

// AdminSearchAnalyzerDictStatusFilter 后台词典状态过滤。
type AdminSearchAnalyzerDictStatusFilter string

const (
	AdminSearchAnalyzerDictStatusFilterAll     AdminSearchAnalyzerDictStatusFilter = "all"
	AdminSearchAnalyzerDictStatusFilterActive  AdminSearchAnalyzerDictStatusFilter = models.SearchAnalyzerDictEntryStatusActive
	AdminSearchAnalyzerDictStatusFilterDeleted AdminSearchAnalyzerDictStatusFilter = models.SearchAnalyzerDictEntryStatusDeleted
)

// AdminSearchAnalyzerRecord 后台分词器列表项。
type AdminSearchAnalyzerRecord struct {
	Name               string
	Enabled            bool
	Active             bool
	DictVersion        string
	SupportsUserDict   bool
	SupportsHotReload  bool
	SupportsPhraseHint bool
	SupportsStopwords  bool
	SupportsSynonyms   bool
}

// AdminSearchAnalyzerDictEntryRecord 后台词典词条项。
type AdminSearchAnalyzerDictEntryRecord struct {
	ID              int64
	Analyzer        string
	Term            string
	Weight          *int
	Tag             string
	Status          string
	CreatedByUserID *string
	UpdatedByUserID *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ListAdminSearchAnalyzerDictEntriesInput 后台词典列表查询参数。
type ListAdminSearchAnalyzerDictEntriesInput struct {
	ActorUserID  string
	Analyzer     string
	StatusFilter AdminSearchAnalyzerDictStatusFilter
	Page         int
	PageSize     int
}

// ListAdminSearchAnalyzerDictEntriesResult 后台词典列表结果。
type ListAdminSearchAnalyzerDictEntriesResult struct {
	Items    []AdminSearchAnalyzerDictEntryRecord
	Page     int
	PageSize int
	Total    int64
}

// UpsertAdminSearchAnalyzerDictEntryInput 后台词典写入参数。
type UpsertAdminSearchAnalyzerDictEntryInput struct {
	ActorUserID string
	RequestID   string
	Analyzer    string
	Term        string
	Weight      *int
	Tag         string
}

// UpdateAdminSearchAnalyzerDictEntryInput 后台词典更新参数。
type UpdateAdminSearchAnalyzerDictEntryInput struct {
	ActorUserID string
	RequestID   string
	Analyzer    string
	EntryID     int64
	Term        string
	Weight      *int
	Tag         string
}

// DeleteAdminSearchAnalyzerDictEntryInput 后台词典删除参数。
type DeleteAdminSearchAnalyzerDictEntryInput struct {
	ActorUserID string
	RequestID   string
	Analyzer    string
	EntryID     int64
}

// ReloadAdminSearchAnalyzerInput 后台分词器重载参数。
type ReloadAdminSearchAnalyzerInput struct {
	ActorUserID string
	RequestID   string
	Analyzer    string
}

// ReloadAdminSearchAnalyzerResult 后台分词器重载结果。
type ReloadAdminSearchAnalyzerResult struct {
	Analyzer       string
	DictVersion    string
	SourceVersion  int
	LoadedAt       time.Time
	ActiveAnalyzer string
}

// AnalyzePreviewAdminSearchAnalyzerInput 后台分词预览参数。
type AnalyzePreviewAdminSearchAnalyzerInput struct {
	ActorUserID string
	Analyzer    string
	Mode        string
	Text        string
	Language    string
	SpaceID     string
}

// AnalyzePreviewAdminSearchAnalyzerResult 后台分词预览结果。
type AnalyzePreviewAdminSearchAnalyzerResult struct {
	Analyzer       string
	Mode           string
	Tokens         []string
	NormalizedText string
	TokenCount     int
	DictVersion    string
}

// AdminSearchAnalyzerService 封装后台分词器与词典治理。
type AdminSearchAnalyzerService struct {
	dictEntryRepo       repository.SearchAnalyzerDictEntryRepository
	adminAccessService  *AdminAccessService
	adminAuditService   *AdminAuditService
	searchConfigService *SearchConfigService
}

// NewAdminSearchAnalyzerService 创建后台分词器治理服务。
func NewAdminSearchAnalyzerService(
	dictEntryRepo repository.SearchAnalyzerDictEntryRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
	searchConfigService *SearchConfigService,
) *AdminSearchAnalyzerService {
	return &AdminSearchAnalyzerService{
		dictEntryRepo:       dictEntryRepo,
		adminAccessService:  adminAccessService,
		adminAuditService:   adminAuditService,
		searchConfigService: searchConfigService,
	}
}

// ListAnalyzers 查询后台分词器状态。
func (s *AdminSearchAnalyzerService) ListAnalyzers(
	ctx context.Context,
	actorUserID string,
) (result []AdminSearchAnalyzerRecord, err error) {
	defer func() {
		err = errcode.MapAdminSearchAnalyzerError(err)
	}()

	if s == nil || s.adminAccessService == nil || s.searchConfigService == nil {
		return nil, errors.New("admin search analyzer service dependencies are nil")
	}
	if err := s.ensurePlatformAdmin(ctx, actorUserID); err != nil {
		return nil, err
	}

	// 后台分词治理页需要实时反映 system_configs.search 最新值，
	// 这里显式刷新而非复用内存快照，避免展示滞后。
	snapshot, err := s.searchConfigService.Refresh(ctx)
	if err != nil {
		return nil, err
	}

	names := []searchcfg.AnalyzerName{searchcfg.AnalyzerSimple, searchcfg.AnalyzerJieba}
	items := make([]AdminSearchAnalyzerRecord, 0, len(names))
	for _, name := range names {
		enabled := snapshot.Config.IsAnalyzerEnabled(name)
		active := snapshot.Config.Analysis.ActiveAnalyzer == name

		dictVersion := ""
		if name == searchcfg.AnalyzerJieba {
			dictVersion = strings.TrimSpace(snapshot.Config.Analysis.Analyzers.Jieba.DictVersion)
		}
		if dictVersion == "" {
			dictVersion = searchcfg.DefaultDictVersion
		}

		capabilities := searchanalyzer.Capabilities{}
		if snapshot.Registry != nil {
			if provider, getErr := snapshot.Registry.Get(string(name)); getErr == nil {
				capabilities = provider.Capabilities()
			}
		}

		items = append(items, AdminSearchAnalyzerRecord{
			Name:               string(name),
			Enabled:            enabled,
			Active:             active,
			DictVersion:        dictVersion,
			SupportsUserDict:   capabilities.SupportsUserDict,
			SupportsHotReload:  capabilities.SupportsHotReload,
			SupportsPhraseHint: capabilities.SupportsPhraseHint,
			SupportsStopwords:  capabilities.SupportsStopwords,
			SupportsSynonyms:   capabilities.SupportsSynonyms,
		})
	}

	return items, nil
}

// ListDictEntries 查询后台词典词条列表。
func (s *AdminSearchAnalyzerService) ListDictEntries(
	ctx context.Context,
	input ListAdminSearchAnalyzerDictEntriesInput,
) (result ListAdminSearchAnalyzerDictEntriesResult, err error) {
	defer func() {
		err = errcode.MapAdminSearchAnalyzerError(err)
	}()

	if s == nil || s.dictEntryRepo == nil || s.adminAccessService == nil {
		return ListAdminSearchAnalyzerDictEntriesResult{}, errors.New("admin search analyzer service dependencies are nil")
	}
	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return ListAdminSearchAnalyzerDictEntriesResult{}, err
	}

	analyzerName, err := normalizeAdminSearchAnalyzerName(input.Analyzer)
	if err != nil {
		return ListAdminSearchAnalyzerDictEntriesResult{}, err
	}
	statuses, err := resolveAdminSearchAnalyzerStatuses(input.StatusFilter)
	if err != nil {
		return ListAdminSearchAnalyzerDictEntriesResult{}, err
	}

	page, pageSize := normalizeAdminSearchAnalyzerPagination(input.Page, input.PageSize)
	rows, total, err := s.dictEntryRepo.List(ctx, repository.ListSearchAnalyzerDictEntriesParams{
		Analyzer: analyzerName,
		Statuses: statuses,
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
	})
	if err != nil {
		return ListAdminSearchAnalyzerDictEntriesResult{}, err
	}

	items := make([]AdminSearchAnalyzerDictEntryRecord, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapAdminSearchAnalyzerDictEntryRecord(row))
	}
	return ListAdminSearchAnalyzerDictEntriesResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// CreateDictEntry 创建或恢复后台词典词条。
func (s *AdminSearchAnalyzerService) CreateDictEntry(
	ctx context.Context,
	input UpsertAdminSearchAnalyzerDictEntryInput,
) (result AdminSearchAnalyzerDictEntryRecord, err error) {
	defer func() {
		err = errcode.MapAdminSearchAnalyzerError(err)
	}()

	if s == nil || s.dictEntryRepo == nil || s.adminAccessService == nil {
		return AdminSearchAnalyzerDictEntryRecord{}, errors.New("admin search analyzer service dependencies are nil")
	}
	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}

	analyzerName, err := normalizeAdminSearchAnalyzerName(input.Analyzer)
	if err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	term := strings.TrimSpace(input.Term)
	if term == "" {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerInvalidTerm
	}
	weight, err := normalizeAdminSearchAnalyzerWeight(input.Weight)
	if err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	tag := strings.TrimSpace(input.Tag)
	if _, err := searchanalyzer.FormatJiebaDictEntry(term, derefAdminSearchAnalyzerWeight(weight), tag); err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerInvalidTerm
	}

	existing, err := s.dictEntryRepo.GetByAnalyzerAndTerm(ctx, analyzerName, term)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}

	now := time.Now().UTC()
	actorUserID := strings.TrimSpace(input.ActorUserID)
	var actorPtr *string
	if actorUserID != "" {
		actorPtr = &actorUserID
	}

	if existing == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		entry := &models.SearchAnalyzerDictEntry{
			Analyzer:        analyzerName,
			Term:            term,
			Weight:          weight,
			Tag:             tag,
			Status:          models.SearchAnalyzerDictEntryStatusActive,
			CreatedByUserID: actorPtr,
			UpdatedByUserID: actorPtr,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := s.dictEntryRepo.Create(ctx, entry); err != nil {
			return AdminSearchAnalyzerDictEntryRecord{}, err
		}
		if err := s.recordAudit(ctx, RecordAdminAuditInput{
			Module:     AdminAuditModuleSystemConfig,
			Action:     AdminAuditActionCreate,
			TargetType: "search_analyzer_dict",
			TargetID:   strconv.FormatInt(entry.ID, 10),
			Summary:    "search analyzer dict entry created: " + term,
			Detail: map[string]any{
				"analyzer": analyzerName,
				"term":     term,
				"weight":   weight,
				"tag":      tag,
				"status":   entry.Status,
			},
			RequestID: strings.TrimSpace(input.RequestID),
		}); err != nil {
			return AdminSearchAnalyzerDictEntryRecord{}, err
		}
		return mapAdminSearchAnalyzerDictEntryRecord(*entry), nil
	}

	if strings.EqualFold(strings.TrimSpace(existing.Status), models.SearchAnalyzerDictEntryStatusActive) {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerDictEntryExists
	}

	updated, err := s.dictEntryRepo.UpdateByID(ctx, existing.ID, map[string]any{
		"weight":             weight,
		"tag":                tag,
		"status":             models.SearchAnalyzerDictEntryStatusActive,
		"updated_by_user_id": actorPtr,
		"updated_at":         now,
	})
	if err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	if !updated {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerDictEntryNotFound
	}

	latest, err := s.dictEntryRepo.GetByID(ctx, existing.ID)
	if err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	if latest == nil {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerDictEntryNotFound
	}
	if err := s.recordAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSystemConfig,
		Action:     AdminAuditActionUpdate,
		TargetType: "search_analyzer_dict",
		TargetID:   strconv.FormatInt(latest.ID, 10),
		Summary:    "search analyzer dict entry revived: " + latest.Term,
		Detail: map[string]any{
			"analyzer": latest.Analyzer,
			"term":     latest.Term,
			"weight":   latest.Weight,
			"tag":      latest.Tag,
			"status":   latest.Status,
		},
		RequestID: strings.TrimSpace(input.RequestID),
	}); err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	return mapAdminSearchAnalyzerDictEntryRecord(*latest), nil
}

// UpdateDictEntry 更新后台词典词条。
func (s *AdminSearchAnalyzerService) UpdateDictEntry(
	ctx context.Context,
	input UpdateAdminSearchAnalyzerDictEntryInput,
) (result AdminSearchAnalyzerDictEntryRecord, err error) {
	defer func() {
		err = errcode.MapAdminSearchAnalyzerError(err)
	}()

	if s == nil || s.dictEntryRepo == nil || s.adminAccessService == nil {
		return AdminSearchAnalyzerDictEntryRecord{}, errors.New("admin search analyzer service dependencies are nil")
	}
	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	if input.EntryID <= 0 {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerInvalidDictEntry
	}

	analyzerName, err := normalizeAdminSearchAnalyzerName(input.Analyzer)
	if err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}

	current, err := s.dictEntryRepo.GetByID(ctx, input.EntryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerDictEntryNotFound
		}
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	if current == nil || !strings.EqualFold(strings.TrimSpace(current.Analyzer), analyzerName) {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerDictEntryNotFound
	}

	term := strings.TrimSpace(input.Term)
	if term == "" {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerInvalidTerm
	}
	weight, err := normalizeAdminSearchAnalyzerWeight(input.Weight)
	if err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	tag := strings.TrimSpace(input.Tag)
	if _, err := searchanalyzer.FormatJiebaDictEntry(term, derefAdminSearchAnalyzerWeight(weight), tag); err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerInvalidTerm
	}

	duplicated, err := s.dictEntryRepo.GetByAnalyzerAndTerm(ctx, analyzerName, term)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	if duplicated != nil && duplicated.ID != current.ID {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerDictEntryExists
	}

	now := time.Now().UTC()
	actorUserID := strings.TrimSpace(input.ActorUserID)
	var actorPtr *string
	if actorUserID != "" {
		actorPtr = &actorUserID
	}
	updated, err := s.dictEntryRepo.UpdateByID(ctx, current.ID, map[string]any{
		"term":               term,
		"weight":             weight,
		"tag":                tag,
		"updated_by_user_id": actorPtr,
		"updated_at":         now,
	})
	if err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	if !updated {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerDictEntryNotFound
	}

	latest, err := s.dictEntryRepo.GetByID(ctx, current.ID)
	if err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	if latest == nil {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerDictEntryNotFound
	}
	if err := s.recordAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSystemConfig,
		Action:     AdminAuditActionUpdate,
		TargetType: "search_analyzer_dict",
		TargetID:   strconv.FormatInt(latest.ID, 10),
		Summary:    "search analyzer dict entry updated: " + latest.Term,
		Detail: map[string]any{
			"analyzer": analyzerName,
			"term":     latest.Term,
			"weight":   latest.Weight,
			"tag":      latest.Tag,
			"status":   latest.Status,
		},
		RequestID: strings.TrimSpace(input.RequestID),
	}); err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	return mapAdminSearchAnalyzerDictEntryRecord(*latest), nil
}

// DeleteDictEntry 删除后台词典词条（逻辑删除）。
func (s *AdminSearchAnalyzerService) DeleteDictEntry(
	ctx context.Context,
	input DeleteAdminSearchAnalyzerDictEntryInput,
) (result AdminSearchAnalyzerDictEntryRecord, err error) {
	defer func() {
		err = errcode.MapAdminSearchAnalyzerError(err)
	}()

	if s == nil || s.dictEntryRepo == nil || s.adminAccessService == nil {
		return AdminSearchAnalyzerDictEntryRecord{}, errors.New("admin search analyzer service dependencies are nil")
	}
	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	if input.EntryID <= 0 {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerInvalidDictEntry
	}
	analyzerName, err := normalizeAdminSearchAnalyzerName(input.Analyzer)
	if err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}

	current, err := s.dictEntryRepo.GetByID(ctx, input.EntryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerDictEntryNotFound
		}
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	if current == nil || !strings.EqualFold(strings.TrimSpace(current.Analyzer), analyzerName) {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerDictEntryNotFound
	}

	now := time.Now().UTC()
	actorUserID := strings.TrimSpace(input.ActorUserID)
	var actorPtr *string
	if actorUserID != "" {
		actorPtr = &actorUserID
	}
	updated, err := s.dictEntryRepo.UpdateByID(ctx, current.ID, map[string]any{
		"status":             models.SearchAnalyzerDictEntryStatusDeleted,
		"updated_by_user_id": actorPtr,
		"updated_at":         now,
	})
	if err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	if !updated {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerDictEntryNotFound
	}

	latest, err := s.dictEntryRepo.GetByID(ctx, current.ID)
	if err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	if latest == nil {
		return AdminSearchAnalyzerDictEntryRecord{}, errcode.ErrAdminSearchAnalyzerDictEntryNotFound
	}
	if err := s.recordAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSystemConfig,
		Action:     AdminAuditActionDelete,
		TargetType: "search_analyzer_dict",
		TargetID:   strconv.FormatInt(latest.ID, 10),
		Summary:    "search analyzer dict entry deleted: " + latest.Term,
		Detail: map[string]any{
			"analyzer": analyzerName,
			"term":     latest.Term,
			"weight":   latest.Weight,
			"tag":      latest.Tag,
			"status":   latest.Status,
		},
		RequestID: strings.TrimSpace(input.RequestID),
	}); err != nil {
		return AdminSearchAnalyzerDictEntryRecord{}, err
	}
	return mapAdminSearchAnalyzerDictEntryRecord(*latest), nil
}

// ReloadAnalyzer 重载后台分词器配置与运行时状态。
func (s *AdminSearchAnalyzerService) ReloadAnalyzer(
	ctx context.Context,
	input ReloadAdminSearchAnalyzerInput,
) (result ReloadAdminSearchAnalyzerResult, err error) {
	defer func() {
		err = errcode.MapAdminSearchAnalyzerError(err)
	}()

	if s == nil || s.adminAccessService == nil || s.searchConfigService == nil {
		return ReloadAdminSearchAnalyzerResult{}, errors.New("admin search analyzer service dependencies are nil")
	}
	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return ReloadAdminSearchAnalyzerResult{}, err
	}

	analyzerName, err := normalizeAdminSearchAnalyzerName(input.Analyzer)
	if err != nil {
		return ReloadAdminSearchAnalyzerResult{}, err
	}

	snapshot, err := s.searchConfigService.Refresh(ctx)
	if err != nil {
		return ReloadAdminSearchAnalyzerResult{}, err
	}
	if !snapshot.Config.IsAnalyzerEnabled(searchcfg.AnalyzerName(analyzerName)) ||
		!strings.EqualFold(string(snapshot.Config.Analysis.ActiveAnalyzer), analyzerName) {
		return ReloadAdminSearchAnalyzerResult{}, errcode.ErrAdminSearchAnalyzerNotActive
	}

	dictVersion := searchcfg.DefaultDictVersion
	if strings.EqualFold(analyzerName, string(searchcfg.AnalyzerJieba)) {
		dictVersion = strings.TrimSpace(snapshot.Config.Analysis.Analyzers.Jieba.DictVersion)
		if dictVersion == "" {
			dictVersion = searchcfg.DefaultDictVersion
		}
	}

	if err := s.recordAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSystemConfig,
		Action:     AdminAuditActionUpdate,
		TargetType: "search_analyzer",
		TargetID:   analyzerName,
		Summary:    "search analyzer reloaded: " + analyzerName,
		Detail: map[string]any{
			"analyzer":       analyzerName,
			"dictVersion":    dictVersion,
			"sourceVersion":  snapshot.SourceVersion,
			"loadedAt":       snapshot.LoadedAt,
			"activeAnalyzer": snapshot.Config.Analysis.ActiveAnalyzer,
		},
		RequestID: strings.TrimSpace(input.RequestID),
	}); err != nil {
		return ReloadAdminSearchAnalyzerResult{}, err
	}

	return ReloadAdminSearchAnalyzerResult{
		Analyzer:       analyzerName,
		DictVersion:    dictVersion,
		SourceVersion:  snapshot.SourceVersion,
		LoadedAt:       snapshot.LoadedAt,
		ActiveAnalyzer: string(snapshot.Config.Analysis.ActiveAnalyzer),
	}, nil
}

// AnalyzePreview 预览当前配置与词典下的分词结果。
func (s *AdminSearchAnalyzerService) AnalyzePreview(
	ctx context.Context,
	input AnalyzePreviewAdminSearchAnalyzerInput,
) (result AnalyzePreviewAdminSearchAnalyzerResult, err error) {
	defer func() {
		err = errcode.MapAdminSearchAnalyzerError(err)
	}()

	if s == nil || s.dictEntryRepo == nil || s.adminAccessService == nil || s.searchConfigService == nil {
		return AnalyzePreviewAdminSearchAnalyzerResult{}, errors.New("admin search analyzer service dependencies are nil")
	}
	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return AnalyzePreviewAdminSearchAnalyzerResult{}, err
	}

	analyzerName, err := normalizeAdminSearchAnalyzerName(input.Analyzer)
	if err != nil {
		return AnalyzePreviewAdminSearchAnalyzerResult{}, err
	}
	mode, err := normalizeAdminSearchAnalyzerMode(input.Mode)
	if err != nil {
		return AnalyzePreviewAdminSearchAnalyzerResult{}, err
	}

	// 预览结果应与当前系统配置一致，避免使用过期快照导致行为偏差。
	snapshot, err := s.searchConfigService.Refresh(ctx)
	if err != nil {
		return AnalyzePreviewAdminSearchAnalyzerResult{}, err
	}

	userDictEntries, err := s.loadJiebaPreviewEntries(ctx, snapshot.Config)
	if err != nil {
		return AnalyzePreviewAdminSearchAnalyzerResult{}, err
	}

	dictVersion := strings.TrimSpace(snapshot.Config.Analysis.Analyzers.Jieba.DictVersion)
	if dictVersion == "" {
		dictVersion = searchcfg.DefaultDictVersion
	}
	provider, err := searchanalyzer.NewJiebaAnalyzer(searchanalyzer.JiebaOptions{
		DictVersion:     dictVersion,
		UserDictEntries: userDictEntries,
		EnableHMM:       snapshot.Config.Analysis.Analyzers.Jieba.HMM,
	})
	if err != nil {
		return AnalyzePreviewAdminSearchAnalyzerResult{}, err
	}

	analyzeInput := searchanalyzer.AnalyzeInput{
		Text:     input.Text,
		Mode:     mode,
		Language: strings.TrimSpace(input.Language),
		SpaceID:  strings.TrimSpace(input.SpaceID),
	}

	var output searchanalyzer.AnalyzeOutput
	switch mode {
	case searchanalyzer.ModeIndex:
		output, err = provider.AnalyzeForIndex(ctx, analyzeInput)
	default:
		output, err = provider.AnalyzeForQuery(ctx, analyzeInput)
	}
	if err != nil {
		return AnalyzePreviewAdminSearchAnalyzerResult{}, err
	}

	return AnalyzePreviewAdminSearchAnalyzerResult{
		Analyzer:       analyzerName,
		Mode:           string(mode),
		Tokens:         output.Tokens,
		NormalizedText: output.NormalizedText,
		TokenCount:     output.TokenCount,
		DictVersion:    output.DictVersion,
	}, nil
}

func (s *AdminSearchAnalyzerService) loadJiebaPreviewEntries(
	ctx context.Context,
	config searchcfg.Config,
) ([]string, error) {
	if s == nil || s.dictEntryRepo == nil {
		return []string{}, nil
	}
	// 预览允许在 jieba 未启用时执行，用于上线前校验词典效果；
	// 但词典来源仍遵循 search 配置，避免与运行时语义偏离。
	if config.Analysis.Analyzers.Jieba.DictSource != searchcfg.JiebaDictSourceDB {
		return []string{}, nil
	}

	entries, err := s.dictEntryRepo.ListActiveByAnalyzer(ctx, string(searchcfg.AnalyzerJieba))
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		weight := 0
		if entry.Weight != nil && *entry.Weight > 0 {
			weight = *entry.Weight
		}
		line, err := searchanalyzer.FormatJiebaDictEntry(entry.Term, weight, entry.Tag)
		if err != nil {
			return nil, fmt.Errorf("invalid jieba dict entry id=%d: %w", entry.ID, err)
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func (s *AdminSearchAnalyzerService) ensurePlatformAdmin(ctx context.Context, actorUserID string) error {
	userID := strings.TrimSpace(actorUserID)
	if userID == "" {
		return errcode.ErrAdminForbidden
	}
	isPlatformAdmin, err := s.adminAccessService.IsPlatformAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if !isPlatformAdmin {
		return errcode.ErrAdminForbidden
	}
	return nil
}

func (s *AdminSearchAnalyzerService) recordAudit(ctx context.Context, input RecordAdminAuditInput) error {
	if s == nil || s.adminAuditService == nil {
		return nil
	}
	return s.adminAuditService.Record(ctx, input)
}

func normalizeAdminSearchAnalyzerName(rawValue string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(rawValue))
	if normalized != string(searchcfg.AnalyzerJieba) {
		return "", errcode.ErrAdminSearchAnalyzerInvalidAnalyzer
	}
	return normalized, nil
}

func normalizeAdminSearchAnalyzerMode(rawValue string) (searchanalyzer.Mode, error) {
	switch strings.ToLower(strings.TrimSpace(rawValue)) {
	case "", adminSearchAnalyzerModeQuery:
		return searchanalyzer.ModeQuery, nil
	case adminSearchAnalyzerModeIndex:
		return searchanalyzer.ModeIndex, nil
	default:
		return "", errcode.ErrAdminSearchAnalyzerInvalidMode
	}
}

func resolveAdminSearchAnalyzerStatuses(filter AdminSearchAnalyzerDictStatusFilter) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(string(filter))) {
	case "", string(AdminSearchAnalyzerDictStatusFilterAll):
		return nil, nil
	case string(AdminSearchAnalyzerDictStatusFilterActive):
		return []string{models.SearchAnalyzerDictEntryStatusActive}, nil
	case string(AdminSearchAnalyzerDictStatusFilterDeleted):
		return []string{models.SearchAnalyzerDictEntryStatusDeleted}, nil
	default:
		return nil, errcode.ErrAdminSearchAnalyzerInvalidStatus
	}
}

func normalizeAdminSearchAnalyzerPagination(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = defaultAdminSearchAnalyzerPage
	}
	if pageSize <= 0 {
		pageSize = defaultAdminSearchAnalyzerPageSize
	}
	if pageSize > maxAdminSearchAnalyzerPageSize {
		pageSize = maxAdminSearchAnalyzerPageSize
	}
	return page, pageSize
}

func normalizeAdminSearchAnalyzerWeight(weight *int) (*int, error) {
	if weight == nil {
		return nil, nil
	}
	if *weight <= 0 {
		return nil, errcode.ErrAdminSearchAnalyzerInvalidWeight
	}
	value := *weight
	return &value, nil
}

func derefAdminSearchAnalyzerWeight(weight *int) int {
	if weight == nil {
		return 0
	}
	return *weight
}

func mapAdminSearchAnalyzerDictEntryRecord(
	value models.SearchAnalyzerDictEntry,
) AdminSearchAnalyzerDictEntryRecord {
	return AdminSearchAnalyzerDictEntryRecord{
		ID:              value.ID,
		Analyzer:        strings.ToLower(strings.TrimSpace(value.Analyzer)),
		Term:            strings.TrimSpace(value.Term),
		Weight:          value.Weight,
		Tag:             strings.TrimSpace(value.Tag),
		Status:          strings.ToLower(strings.TrimSpace(value.Status)),
		CreatedByUserID: value.CreatedByUserID,
		UpdatedByUserID: value.UpdatedByUserID,
		CreatedAt:       value.CreatedAt,
		UpdatedAt:       value.UpdatedAt,
	}
}
