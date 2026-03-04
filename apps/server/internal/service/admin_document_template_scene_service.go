package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	adminDocumentTemplateSceneDefaultPageSize = 20
	adminDocumentTemplateSceneMaxPageSize     = 100
	adminDocumentTemplateSceneMaxKeywordLen   = 64
	adminDocumentTemplateSceneNameMaxLen      = 80
	adminDocumentTemplateSceneDescMaxLen      = 255
)

// AdminDocumentTemplateSceneListInput 管理端文档模板场景列表查询输入。
type AdminDocumentTemplateSceneListInput struct {
	ActorUserID string
	Keyword     string
	Page        int
	PageSize    int
}

// AdminDocumentTemplateSceneListItem 管理端文档模板场景列表项。
type AdminDocumentTemplateSceneListItem struct {
	SceneKey      string
	SceneName     string
	Description   string
	Sort          int
	Builtin       bool
	TemplateCount int64
	UpdatedAt     string
}

// AdminDocumentTemplateSceneListResult 管理端文档模板场景分页结果。
type AdminDocumentTemplateSceneListResult struct {
	Items    []AdminDocumentTemplateSceneListItem
	Page     int
	PageSize int
	Total    int64
}

// AdminDocumentTemplateSceneRecord 管理端文档模板场景详情。
type AdminDocumentTemplateSceneRecord struct {
	SceneKey        string
	SceneName       string
	Description     string
	Sort            int
	Builtin         bool
	TemplateCount   int64
	CreatedByUserID *string
	UpdatedByUserID *string
	CreatedAt       string
	UpdatedAt       string
}

// CreateAdminDocumentTemplateSceneInput 创建文档模板场景输入参数。
type CreateAdminDocumentTemplateSceneInput struct {
	ActorUserID  string
	RequestID    string
	SceneKey     string
	SceneName    string
	Description  string
	Sort         int
}

// UpdateAdminDocumentTemplateSceneInput 更新文档模板场景输入参数。
type UpdateAdminDocumentTemplateSceneInput struct {
	ActorUserID  string
	RequestID    string
	SceneKey     string
	SceneName    *string
	Description  *string
	Sort         *int
}

// AdminDocumentTemplateSceneService 封装管理员文档模板场景治理能力。
type AdminDocumentTemplateSceneService struct {
	sceneRepo       repository.DocumentTemplateSceneRepository
	adminAccess     *AdminAccessService
	adminAudit      *AdminAuditService
}

// NewAdminDocumentTemplateSceneService 创建管理员文档模板场景治理服务。
func NewAdminDocumentTemplateSceneService(
	sceneRepo repository.DocumentTemplateSceneRepository,
	adminAccess *AdminAccessService,
	adminAudit *AdminAuditService,
) *AdminDocumentTemplateSceneService {
	return &AdminDocumentTemplateSceneService{
		sceneRepo:   sceneRepo,
		adminAccess: adminAccess,
		adminAudit:  adminAudit,
	}
}

// ListScenes 查询管理端文档模板场景列表。
func (s *AdminDocumentTemplateSceneService) ListScenes(
	ctx context.Context,
	input AdminDocumentTemplateSceneListInput,
) (result AdminDocumentTemplateSceneListResult, err error) {
	defer func() {
		err = errcode.MapAdminDocumentTemplateSceneError(err)
	}()

	if s == nil || s.sceneRepo == nil || s.adminAccess == nil {
		return AdminDocumentTemplateSceneListResult{}, errors.New("admin document template scene service dependencies are nil")
	}
	if err := s.ensurePlatformAdminActor(ctx, input.ActorUserID); err != nil {
		return AdminDocumentTemplateSceneListResult{}, err
	}

	keyword := strings.TrimSpace(input.Keyword)
	if len([]rune(keyword)) > adminDocumentTemplateSceneMaxKeywordLen {
		return AdminDocumentTemplateSceneListResult{}, errcode.ErrAdminDocumentTemplateSceneInvalidKeyword
	}

	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = adminDocumentTemplateSceneDefaultPageSize
	}
	if pageSize > adminDocumentTemplateSceneMaxPageSize {
		pageSize = adminDocumentTemplateSceneMaxPageSize
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	rows, total, err := s.sceneRepo.List(ctx, repository.ListDocumentTemplateScenesParams{
		Keyword: keyword,
		Limit:   pageSize,
		Offset:  offset,
	})
	if err != nil {
		return AdminDocumentTemplateSceneListResult{}, err
	}

	items := make([]AdminDocumentTemplateSceneListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapAdminDocumentTemplateSceneListItem(row))
	}

	return AdminDocumentTemplateSceneListResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// GetScene 按场景标识读取后台文档模板场景详情。
func (s *AdminDocumentTemplateSceneService) GetScene(
	ctx context.Context,
	actorUserID string,
	sceneKey string,
) (result AdminDocumentTemplateSceneRecord, err error) {
	defer func() {
		err = errcode.MapAdminDocumentTemplateSceneError(err)
	}()

	if s == nil || s.sceneRepo == nil || s.adminAccess == nil {
		return AdminDocumentTemplateSceneRecord{}, errors.New("admin document template scene service dependencies are nil")
	}
	if err := s.ensurePlatformAdminActor(ctx, actorUserID); err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}

	normalizedSceneKey, err := normalizeAdminDocumentTemplateSceneKey(sceneKey)
	if err != nil {
		return AdminDocumentTemplateSceneRecord{}, errcode.ErrAdminDocumentTemplateSceneInvalidSceneKey
	}

	row, err := s.sceneRepo.GetBySceneKey(ctx, normalizedSceneKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminDocumentTemplateSceneRecord{}, errcode.ErrAdminDocumentTemplateSceneNotFound
		}
		return AdminDocumentTemplateSceneRecord{}, err
	}
	templateCount, err := s.sceneRepo.CountTemplatesBySceneKey(ctx, normalizedSceneKey)
	if err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}

	return mapAdminDocumentTemplateSceneRecord(*row, templateCount), nil
}

// CreateScene 创建文档模板场景。
func (s *AdminDocumentTemplateSceneService) CreateScene(
	ctx context.Context,
	input CreateAdminDocumentTemplateSceneInput,
) (result AdminDocumentTemplateSceneRecord, err error) {
	defer func() {
		err = errcode.MapAdminDocumentTemplateSceneError(err)
	}()

	if s == nil || s.sceneRepo == nil || s.adminAccess == nil {
		return AdminDocumentTemplateSceneRecord{}, errors.New("admin document template scene service dependencies are nil")
	}
	if err := s.ensurePlatformAdminActor(ctx, input.ActorUserID); err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}

	sceneKey, err := normalizeAdminDocumentTemplateSceneKey(input.SceneKey)
	if err != nil {
		return AdminDocumentTemplateSceneRecord{}, errcode.ErrAdminDocumentTemplateSceneInvalidSceneKey
	}
	sceneName, err := normalizeAdminDocumentTemplateSceneName(input.SceneName)
	if err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}
	description, err := normalizeAdminDocumentTemplateSceneDescription(input.Description)
	if err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}
	if !isAdminDocumentTemplateSortValid(input.Sort) {
		return AdminDocumentTemplateSceneRecord{}, errcode.ErrAdminDocumentTemplateSceneInvalidSort
	}

	existing, err := s.sceneRepo.GetBySceneKey(ctx, sceneKey)
	if err == nil && existing != nil {
		return AdminDocumentTemplateSceneRecord{}, errcode.ErrAdminDocumentTemplateSceneAlreadyExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return AdminDocumentTemplateSceneRecord{}, err
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	now := time.Now().UTC()
	scene := &models.DocumentTemplateScene{
		SceneKey:        sceneKey,
		SceneName:       sceneName,
		Description:     description,
		Sort:            input.Sort,
		IsBuiltin:       false,
		CreatedByUserID: &actorUserID,
		UpdatedByUserID: &actorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.sceneRepo.Create(ctx, scene); err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}

	created, err := s.sceneRepo.GetBySceneKey(ctx, sceneKey)
	if err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}
	payload := mapAdminDocumentTemplateSceneRecord(*created, 0)

	if err := s.recordSceneAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleTemplateScene,
		Action:     AdminAuditActionCreate,
		TargetType: "document_template_scene",
		TargetID:   payload.SceneKey,
		Summary:    "document template scene created: " + payload.SceneKey,
		Detail: map[string]any{
			"sceneName": payload.SceneName,
			"sort":      payload.Sort,
		},
		RequestID: input.RequestID,
	}); err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}

	return payload, nil
}

// UpdateScene 更新文档模板场景。
func (s *AdminDocumentTemplateSceneService) UpdateScene(
	ctx context.Context,
	input UpdateAdminDocumentTemplateSceneInput,
) (result AdminDocumentTemplateSceneRecord, err error) {
	defer func() {
		err = errcode.MapAdminDocumentTemplateSceneError(err)
	}()

	if s == nil || s.sceneRepo == nil || s.adminAccess == nil {
		return AdminDocumentTemplateSceneRecord{}, errors.New("admin document template scene service dependencies are nil")
	}
	if err := s.ensurePlatformAdminActor(ctx, input.ActorUserID); err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}

	sceneKey, err := normalizeAdminDocumentTemplateSceneKey(input.SceneKey)
	if err != nil {
		return AdminDocumentTemplateSceneRecord{}, errcode.ErrAdminDocumentTemplateSceneInvalidSceneKey
	}

	target, err := s.sceneRepo.GetBySceneKey(ctx, sceneKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminDocumentTemplateSceneRecord{}, errcode.ErrAdminDocumentTemplateSceneNotFound
		}
		return AdminDocumentTemplateSceneRecord{}, err
	}
	if target.IsBuiltin {
		return AdminDocumentTemplateSceneRecord{}, errcode.ErrAdminDocumentTemplateSceneBuiltinImmutable
	}

	changedFields := make([]string, 0, 3)
	params := repository.UpdateDocumentTemplateSceneParams{
		SceneKey:        sceneKey,
		UpdatedAt:       time.Now().UTC(),
		UpdatedByUserID: stringPtr(strings.TrimSpace(input.ActorUserID)),
	}
	if input.SceneName != nil {
		sceneName, normalizeErr := normalizeAdminDocumentTemplateSceneName(*input.SceneName)
		if normalizeErr != nil {
			return AdminDocumentTemplateSceneRecord{}, normalizeErr
		}
		if sceneName != strings.TrimSpace(target.SceneName) {
			params.SceneName = &sceneName
			changedFields = append(changedFields, "sceneName")
		}
	}
	if input.Description != nil {
		description, normalizeErr := normalizeAdminDocumentTemplateSceneDescription(*input.Description)
		if normalizeErr != nil {
			return AdminDocumentTemplateSceneRecord{}, normalizeErr
		}
		if description != strings.TrimSpace(target.Description) {
			params.Description = &description
			changedFields = append(changedFields, "description")
		}
	}
	if input.Sort != nil {
		if !isAdminDocumentTemplateSortValid(*input.Sort) {
			return AdminDocumentTemplateSceneRecord{}, errcode.ErrAdminDocumentTemplateSceneInvalidSort
		}
		if *input.Sort != target.Sort {
			params.Sort = input.Sort
			changedFields = append(changedFields, "sort")
		}
	}
	if len(changedFields) == 0 {
		return AdminDocumentTemplateSceneRecord{}, errcode.ErrAdminDocumentTemplateSceneNoChanges
	}

	updated, err := s.sceneRepo.UpdateBySceneKey(ctx, params)
	if err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}
	if !updated {
		return AdminDocumentTemplateSceneRecord{}, errcode.ErrAdminDocumentTemplateSceneNotFound
	}

	next, err := s.sceneRepo.GetBySceneKey(ctx, sceneKey)
	if err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}
	templateCount, err := s.sceneRepo.CountTemplatesBySceneKey(ctx, sceneKey)
	if err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}
	payload := mapAdminDocumentTemplateSceneRecord(*next, templateCount)

	if err := s.recordSceneAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleTemplateScene,
		Action:     AdminAuditActionUpdate,
		TargetType: "document_template_scene",
		TargetID:   payload.SceneKey,
		Summary:    "document template scene updated: " + payload.SceneKey,
		Detail: map[string]any{
			"changedFields": changedFields,
		},
		RequestID: input.RequestID,
	}); err != nil {
		return AdminDocumentTemplateSceneRecord{}, err
	}

	return payload, nil
}

// DeleteScene 删除文档模板场景。
func (s *AdminDocumentTemplateSceneService) DeleteScene(
	ctx context.Context,
	actorUserID string,
	sceneKey string,
	requestID string,
) (err error) {
	defer func() {
		err = errcode.MapAdminDocumentTemplateSceneError(err)
	}()

	if s == nil || s.sceneRepo == nil || s.adminAccess == nil {
		return errors.New("admin document template scene service dependencies are nil")
	}
	if err := s.ensurePlatformAdminActor(ctx, actorUserID); err != nil {
		return err
	}

	normalizedSceneKey, err := normalizeAdminDocumentTemplateSceneKey(sceneKey)
	if err != nil {
		return errcode.ErrAdminDocumentTemplateSceneInvalidSceneKey
	}

	target, err := s.sceneRepo.GetBySceneKey(ctx, normalizedSceneKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrAdminDocumentTemplateSceneNotFound
		}
		return err
	}
	if target.IsBuiltin {
		return errcode.ErrAdminDocumentTemplateSceneBuiltinImmutable
	}

	templateCount, err := s.sceneRepo.CountTemplatesBySceneKey(ctx, normalizedSceneKey)
	if err != nil {
		return err
	}
	if templateCount > 0 {
		return errcode.ErrAdminDocumentTemplateSceneInUse
	}

	deleted, err := s.sceneRepo.DeleteBySceneKey(ctx, normalizedSceneKey)
	if err != nil {
		return err
	}
	if !deleted {
		return errcode.ErrAdminDocumentTemplateSceneNotFound
	}

	if err := s.recordSceneAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleTemplateScene,
		Action:     AdminAuditActionDelete,
		TargetType: "document_template_scene",
		TargetID:   normalizedSceneKey,
		Summary:    "document template scene deleted: " + normalizedSceneKey,
		Detail: map[string]any{
			"sceneKey": normalizedSceneKey,
		},
		RequestID: requestID,
	}); err != nil {
		return err
	}

	return nil
}

func (s *AdminDocumentTemplateSceneService) recordSceneAudit(ctx context.Context, input RecordAdminAuditInput) error {
	if s == nil || s.adminAudit == nil {
		return nil
	}
	return s.adminAudit.Record(ctx, input)
}

func (s *AdminDocumentTemplateSceneService) ensurePlatformAdminActor(ctx context.Context, actorUserID string) error {
	userID := strings.TrimSpace(actorUserID)
	if userID == "" {
		return errcode.ErrAdminForbidden
	}
	isPlatformAdmin, err := s.adminAccess.IsPlatformAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if !isPlatformAdmin {
		return errcode.ErrAdminForbidden
	}
	return nil
}

func normalizeAdminDocumentTemplateSceneKey(rawSceneKey string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(rawSceneKey))
	if !adminDocumentTemplateKeyPattern.MatchString(normalized) {
		return "", errcode.ErrAdminDocumentTemplateSceneInvalidSceneKey
	}
	return normalized, nil
}

func normalizeAdminDocumentTemplateSceneName(rawSceneName string) (string, error) {
	sceneName := strings.TrimSpace(rawSceneName)
	if sceneName == "" || len([]rune(sceneName)) > adminDocumentTemplateSceneNameMaxLen {
		return "", errcode.ErrAdminDocumentTemplateSceneInvalidSceneName
	}
	return sceneName, nil
}

func normalizeAdminDocumentTemplateSceneDescription(rawDescription string) (string, error) {
	description := strings.TrimSpace(rawDescription)
	if len([]rune(description)) > adminDocumentTemplateSceneDescMaxLen {
		return "", errcode.ErrAdminDocumentTemplateSceneInvalidDescription
	}
	return description, nil
}

func mapAdminDocumentTemplateSceneListItem(value repository.DocumentTemplateSceneSummaryRecord) AdminDocumentTemplateSceneListItem {
	return AdminDocumentTemplateSceneListItem{
		SceneKey:      strings.TrimSpace(value.SceneKey),
		SceneName:     strings.TrimSpace(value.SceneName),
		Description:   strings.TrimSpace(value.Description),
		Sort:          value.Sort,
		Builtin:       value.IsBuiltin,
		TemplateCount: value.TemplateCount,
		UpdatedAt:     strings.TrimSpace(value.UpdatedAtRaw),
	}
}

func mapAdminDocumentTemplateSceneRecord(value repository.DocumentTemplateSceneDetailRecord, templateCount int64) AdminDocumentTemplateSceneRecord {
	return AdminDocumentTemplateSceneRecord{
		SceneKey:        strings.TrimSpace(value.SceneKey),
		SceneName:       strings.TrimSpace(value.SceneName),
		Description:     strings.TrimSpace(value.Description),
		Sort:            value.Sort,
		Builtin:         value.IsBuiltin,
		TemplateCount:   templateCount,
		CreatedByUserID: value.CreatedByUserID,
		UpdatedByUserID: value.UpdatedByUserID,
		CreatedAt:       strings.TrimSpace(value.CreatedAtRaw),
		UpdatedAt:       strings.TrimSpace(value.UpdatedAtRaw),
	}
}
