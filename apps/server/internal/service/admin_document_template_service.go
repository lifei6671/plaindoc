package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

const (
	adminDocumentTemplateDefaultPageSize = 20
	adminDocumentTemplateMaxPageSize     = 100
	adminDocumentTemplateMaxKeywordLen   = 64
	adminDocumentTemplateNameMaxLen      = 120
	adminDocumentTemplateDescMaxLen      = 255
	adminDocumentTemplateTitleMaxLen     = 120
	adminDocumentTemplateContentMaxBytes = 200 * 1024
)

var adminDocumentTemplateKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)

// AdminDocumentTemplateListInput 管理端文档模板列表查询输入。
type AdminDocumentTemplateListInput struct {
	ActorUserID string
	SceneKey    string
	Keyword     string
	Page        int
	PageSize    int
}

// AdminDocumentTemplateListItem 管理端文档模板列表项。
type AdminDocumentTemplateListItem struct {
	TemplateID   string
	SceneKey     string
	SceneName    string
	Name         string
	Description  string
	DefaultTitle string
	Sort         int
	Builtin      bool
	Enabled      bool
	UpdatedAt    string
}

// AdminDocumentTemplateListResult 管理端文档模板分页结果。
type AdminDocumentTemplateListResult struct {
	Items    []AdminDocumentTemplateListItem
	Page     int
	PageSize int
	Total    int64
}

// AdminDocumentTemplateRecord 管理端文档模板详情。
type AdminDocumentTemplateRecord struct {
	TemplateID      string
	SceneKey        string
	SceneName       string
	Name            string
	Description     string
	DefaultTitle    string
	ContentMD       string
	Sort            int
	Builtin         bool
	Enabled         bool
	CreatedByUserID *string
	UpdatedByUserID *string
	CreatedAt       string
	UpdatedAt       string
}

// CreateAdminDocumentTemplateInput 创建文档模板输入参数。
type CreateAdminDocumentTemplateInput struct {
	ActorUserID  string
	RequestID    string
	TemplateID   string
	SceneKey     string
	Name         string
	Description  string
	DefaultTitle string
	ContentMD    string
	Sort         int
	Enabled      bool
}

// UpdateAdminDocumentTemplateInput 更新文档模板输入参数。
type UpdateAdminDocumentTemplateInput struct {
	ActorUserID  string
	RequestID    string
	TemplateID   string
	Name         *string
	Description  *string
	DefaultTitle *string
	ContentMD    *string
	Sort         *int
	Enabled      *bool
}

// AdminDocumentTemplateService 封装管理员文档模板治理能力。
type AdminDocumentTemplateService struct {
	documentTemplateRepo      repository.DocumentTemplateRepository
	documentTemplateSceneRepo repository.DocumentTemplateSceneRepository
	adminAccessService        *AdminAccessService
	adminAuditService         *AdminAuditService
}

// NewAdminDocumentTemplateService 创建管理员文档模板治理服务。
func NewAdminDocumentTemplateService(
	documentTemplateRepo repository.DocumentTemplateRepository,
	documentTemplateSceneRepo repository.DocumentTemplateSceneRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
) *AdminDocumentTemplateService {
	return &AdminDocumentTemplateService{
		documentTemplateRepo:      documentTemplateRepo,
		documentTemplateSceneRepo: documentTemplateSceneRepo,
		adminAccessService:        adminAccessService,
		adminAuditService:         adminAuditService,
	}
}

// ListTemplates 查询管理端文档模板列表（返回启用与停用模板）。
func (s *AdminDocumentTemplateService) ListTemplates(
	ctx context.Context,
	input AdminDocumentTemplateListInput,
) (result AdminDocumentTemplateListResult, err error) {
	defer func() {
		err = errcode.MapAdminDocumentTemplateError(err)
	}()

	if s == nil || s.documentTemplateRepo == nil || s.adminAccessService == nil {
		return AdminDocumentTemplateListResult{}, errors.New("admin document template service dependencies are nil")
	}
	if err := s.ensurePlatformAdminActor(ctx, input.ActorUserID); err != nil {
		return AdminDocumentTemplateListResult{}, err
	}

	sceneKey := strings.TrimSpace(input.SceneKey)
	if sceneKey != "" {
		normalizedSceneKey, normalizeErr := normalizeAdminDocumentTemplateKey(sceneKey)
		if normalizeErr != nil {
			return AdminDocumentTemplateListResult{}, errcode.ErrAdminDocumentTemplateInvalidSceneKey
		}
		sceneKey = normalizedSceneKey
	}

	keyword := strings.TrimSpace(input.Keyword)
	if len([]rune(keyword)) > adminDocumentTemplateMaxKeywordLen {
		return AdminDocumentTemplateListResult{}, errcode.ErrAdminDocumentTemplateInvalidKeyword
	}

	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = adminDocumentTemplateDefaultPageSize
	}
	if pageSize > adminDocumentTemplateMaxPageSize {
		pageSize = adminDocumentTemplateMaxPageSize
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	rows, total, err := s.documentTemplateRepo.List(ctx, repository.ListDocumentTemplatesParams{
		SceneKey:    sceneKey,
		Keyword:     keyword,
		EnabledOnly: false,
		Limit:       pageSize,
		Offset:      offset,
	})
	if err != nil {
		return AdminDocumentTemplateListResult{}, err
	}

	items := make([]AdminDocumentTemplateListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, AdminDocumentTemplateListItem{
			TemplateID:   strings.TrimSpace(row.TemplateID),
			SceneKey:     strings.TrimSpace(row.SceneKey),
			SceneName:    strings.TrimSpace(row.SceneName),
			Name:         strings.TrimSpace(row.Name),
			Description:  strings.TrimSpace(row.Description),
			DefaultTitle: strings.TrimSpace(row.DefaultTitle),
			Sort:         row.Sort,
			Builtin:      row.IsBuiltin,
			Enabled:      row.IsEnabled,
			UpdatedAt:    strings.TrimSpace(row.UpdatedAtRaw),
		})
	}

	return AdminDocumentTemplateListResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// GetTemplate 按模板标识读取后台文档模板详情。
func (s *AdminDocumentTemplateService) GetTemplate(
	ctx context.Context,
	actorUserID string,
	templateID string,
) (result AdminDocumentTemplateRecord, err error) {
	defer func() {
		err = errcode.MapAdminDocumentTemplateError(err)
	}()

	if s == nil || s.documentTemplateRepo == nil || s.adminAccessService == nil {
		return AdminDocumentTemplateRecord{}, errors.New("admin document template service dependencies are nil")
	}
	if err := s.ensurePlatformAdminActor(ctx, actorUserID); err != nil {
		return AdminDocumentTemplateRecord{}, err
	}

	normalizedTemplateID, err := normalizeAdminDocumentTemplateKey(templateID)
	if err != nil {
		return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateInvalidTemplateID
	}

	row, err := s.documentTemplateRepo.GetByTemplateID(ctx, normalizedTemplateID, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateNotFound
		}
		return AdminDocumentTemplateRecord{}, err
	}
	return mapAdminDocumentTemplateRecord(*row), nil
}

// CreateTemplate 创建文档模板。
func (s *AdminDocumentTemplateService) CreateTemplate(
	ctx context.Context,
	input CreateAdminDocumentTemplateInput,
) (result AdminDocumentTemplateRecord, err error) {
	defer func() {
		err = errcode.MapAdminDocumentTemplateError(err)
	}()

	if s == nil || s.documentTemplateRepo == nil || s.documentTemplateSceneRepo == nil || s.adminAccessService == nil {
		return AdminDocumentTemplateRecord{}, errors.New("admin document template service dependencies are nil")
	}
	if err := s.ensurePlatformAdminActor(ctx, input.ActorUserID); err != nil {
		return AdminDocumentTemplateRecord{}, err
	}

	templateID, err := normalizeAdminDocumentTemplateKey(input.TemplateID)
	if err != nil {
		return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateInvalidTemplateID
	}
	sceneKey, err := normalizeAdminDocumentTemplateKey(input.SceneKey)
	if err != nil {
		return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateInvalidSceneKey
	}
	_, err = s.documentTemplateSceneRepo.GetBySceneKey(ctx, sceneKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateSceneNotFound
		}
		return AdminDocumentTemplateRecord{}, err
	}
	name, err := normalizeAdminDocumentTemplateName(input.Name)
	if err != nil {
		return AdminDocumentTemplateRecord{}, err
	}
	description, err := normalizeAdminDocumentTemplateDescription(input.Description)
	if err != nil {
		return AdminDocumentTemplateRecord{}, err
	}
	defaultTitle, err := normalizeAdminDocumentTemplateDefaultTitle(input.DefaultTitle)
	if err != nil {
		return AdminDocumentTemplateRecord{}, err
	}
	contentMD, err := normalizeAdminDocumentTemplateContent(input.ContentMD)
	if err != nil {
		return AdminDocumentTemplateRecord{}, err
	}
	if !isAdminDocumentTemplateSortValid(input.Sort) {
		return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateInvalidSort
	}

	existing, err := s.documentTemplateRepo.GetByTemplateID(ctx, templateID, false)
	if err == nil && existing != nil {
		return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateAlreadyExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return AdminDocumentTemplateRecord{}, err
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	now := time.Now().UTC()
	template := &models.DocumentTemplate{
		TemplateID:      templateID,
		SceneKey:        sceneKey,
		Name:            name,
		Description:     description,
		DefaultTitle:    defaultTitle,
		ContentMD:       contentMD,
		Sort:            input.Sort,
		IsBuiltin:       false,
		IsEnabled:       input.Enabled,
		CreatedByUserID: &actorUserID,
		UpdatedByUserID: &actorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.documentTemplateRepo.Create(ctx, template); err != nil {
		return AdminDocumentTemplateRecord{}, err
	}

	created, err := s.documentTemplateRepo.GetByTemplateID(ctx, templateID, false)
	if err != nil {
		return AdminDocumentTemplateRecord{}, err
	}
	payload := mapAdminDocumentTemplateRecord(*created)

	if err := s.recordTemplateAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleTemplate,
		Action:     AdminAuditActionCreate,
		TargetType: "document_template",
		TargetID:   payload.TemplateID,
		Summary:    "document template created: " + payload.TemplateID,
		Detail: map[string]any{
			"sceneKey":  payload.SceneKey,
			"sceneName": payload.SceneName,
			"name":      payload.Name,
			"enabled":   payload.Enabled,
		},
		RequestID: input.RequestID,
	}); err != nil {
		return AdminDocumentTemplateRecord{}, err
	}

	return payload, nil
}

// UpdateTemplate 更新文档模板。
func (s *AdminDocumentTemplateService) UpdateTemplate(
	ctx context.Context,
	input UpdateAdminDocumentTemplateInput,
) (result AdminDocumentTemplateRecord, err error) {
	defer func() {
		err = errcode.MapAdminDocumentTemplateError(err)
	}()

	if s == nil || s.documentTemplateRepo == nil || s.adminAccessService == nil {
		return AdminDocumentTemplateRecord{}, errors.New("admin document template service dependencies are nil")
	}
	if err := s.ensurePlatformAdminActor(ctx, input.ActorUserID); err != nil {
		return AdminDocumentTemplateRecord{}, err
	}

	templateID, err := normalizeAdminDocumentTemplateKey(input.TemplateID)
	if err != nil {
		return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateInvalidTemplateID
	}

	target, err := s.documentTemplateRepo.GetByTemplateID(ctx, templateID, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateNotFound
		}
		return AdminDocumentTemplateRecord{}, err
	}
	if target.IsBuiltin {
		return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateBuiltinImmutable
	}

	changedFields := make([]string, 0, 8)
	params := repository.UpdateDocumentTemplateParams{
		TemplateID:      templateID,
		UpdatedAt:       time.Now().UTC(),
		UpdatedByUserID: stringPtr(strings.TrimSpace(input.ActorUserID)),
	}

	if input.Name != nil {
		name, normalizeErr := normalizeAdminDocumentTemplateName(*input.Name)
		if normalizeErr != nil {
			return AdminDocumentTemplateRecord{}, normalizeErr
		}
		if name != strings.TrimSpace(target.Name) {
			params.Name = &name
			changedFields = append(changedFields, "name")
		}
	}
	if input.Description != nil {
		description, normalizeErr := normalizeAdminDocumentTemplateDescription(*input.Description)
		if normalizeErr != nil {
			return AdminDocumentTemplateRecord{}, normalizeErr
		}
		if description != strings.TrimSpace(target.Description) {
			params.Description = &description
			changedFields = append(changedFields, "description")
		}
	}
	if input.DefaultTitle != nil {
		defaultTitle, normalizeErr := normalizeAdminDocumentTemplateDefaultTitle(*input.DefaultTitle)
		if normalizeErr != nil {
			return AdminDocumentTemplateRecord{}, normalizeErr
		}
		if defaultTitle != strings.TrimSpace(target.DefaultTitle) {
			params.DefaultTitle = &defaultTitle
			changedFields = append(changedFields, "defaultTitle")
		}
	}
	if input.ContentMD != nil {
		contentMD, normalizeErr := normalizeAdminDocumentTemplateContent(*input.ContentMD)
		if normalizeErr != nil {
			return AdminDocumentTemplateRecord{}, normalizeErr
		}
		if contentMD != target.ContentMD {
			params.ContentMD = &contentMD
			changedFields = append(changedFields, "contentMd")
		}
	}
	if input.Sort != nil {
		if !isAdminDocumentTemplateSortValid(*input.Sort) {
			return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateInvalidSort
		}
		if *input.Sort != target.Sort {
			params.Sort = input.Sort
			changedFields = append(changedFields, "sort")
		}
	}
	if input.Enabled != nil && *input.Enabled != target.IsEnabled {
		params.IsEnabled = input.Enabled
		changedFields = append(changedFields, "enabled")
	}

	if len(changedFields) == 0 {
		return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateNoChanges
	}

	updated, err := s.documentTemplateRepo.UpdateByTemplateID(ctx, params)
	if err != nil {
		return AdminDocumentTemplateRecord{}, err
	}
	if !updated {
		return AdminDocumentTemplateRecord{}, errcode.ErrAdminDocumentTemplateNotFound
	}

	next, err := s.documentTemplateRepo.GetByTemplateID(ctx, templateID, false)
	if err != nil {
		return AdminDocumentTemplateRecord{}, err
	}
	payload := mapAdminDocumentTemplateRecord(*next)

	if err := s.recordTemplateAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleTemplate,
		Action:     AdminAuditActionUpdate,
		TargetType: "document_template",
		TargetID:   payload.TemplateID,
		Summary:    "document template updated: " + payload.TemplateID,
		Detail: map[string]any{
			"changedFields": changedFields,
			"enabled":       payload.Enabled,
			"sceneKey":      payload.SceneKey,
		},
		RequestID: input.RequestID,
	}); err != nil {
		return AdminDocumentTemplateRecord{}, err
	}

	return payload, nil
}

// DeleteTemplate 删除文档模板。
func (s *AdminDocumentTemplateService) DeleteTemplate(
	ctx context.Context,
	actorUserID string,
	templateID string,
	requestID string,
) (err error) {
	defer func() {
		err = errcode.MapAdminDocumentTemplateError(err)
	}()

	if s == nil || s.documentTemplateRepo == nil || s.adminAccessService == nil {
		return errors.New("admin document template service dependencies are nil")
	}
	if err := s.ensurePlatformAdminActor(ctx, actorUserID); err != nil {
		return err
	}

	normalizedTemplateID, err := normalizeAdminDocumentTemplateKey(templateID)
	if err != nil {
		return errcode.ErrAdminDocumentTemplateInvalidTemplateID
	}

	target, err := s.documentTemplateRepo.GetByTemplateID(ctx, normalizedTemplateID, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrAdminDocumentTemplateNotFound
		}
		return err
	}
	if target.IsBuiltin {
		return errcode.ErrAdminDocumentTemplateBuiltinImmutable
	}

	deleted, err := s.documentTemplateRepo.DeleteByTemplateID(ctx, normalizedTemplateID)
	if err != nil {
		return err
	}
	if !deleted {
		return errcode.ErrAdminDocumentTemplateNotFound
	}

	if err := s.recordTemplateAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleTemplate,
		Action:     AdminAuditActionDelete,
		TargetType: "document_template",
		TargetID:   normalizedTemplateID,
		Summary:    "document template deleted: " + normalizedTemplateID,
		Detail: map[string]any{
			"templateId": normalizedTemplateID,
		},
		RequestID: requestID,
	}); err != nil {
		return err
	}
	return nil
}

func (s *AdminDocumentTemplateService) recordTemplateAudit(ctx context.Context, input RecordAdminAuditInput) error {
	if s == nil || s.adminAuditService == nil {
		return nil
	}
	return s.adminAuditService.Record(ctx, input)
}

func (s *AdminDocumentTemplateService) ensurePlatformAdminActor(ctx context.Context, actorUserID string) error {
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

func normalizeAdminDocumentTemplateKey(rawKey string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(rawKey))
	if !adminDocumentTemplateKeyPattern.MatchString(normalized) {
		return "", errors.New("invalid key")
	}
	return normalized, nil
}

func normalizeAdminDocumentTemplateName(rawName string) (string, error) {
	name := strings.TrimSpace(rawName)
	if name == "" || len([]rune(name)) > adminDocumentTemplateNameMaxLen {
		return "", errcode.ErrAdminDocumentTemplateInvalidName
	}
	return name, nil
}

func normalizeAdminDocumentTemplateDescription(rawDescription string) (string, error) {
	description := strings.TrimSpace(rawDescription)
	if len([]rune(description)) > adminDocumentTemplateDescMaxLen {
		return "", errcode.ErrAdminDocumentTemplateInvalidDescription
	}
	return description, nil
}

func normalizeAdminDocumentTemplateDefaultTitle(rawDefaultTitle string) (string, error) {
	defaultTitle := strings.TrimSpace(rawDefaultTitle)
	if len([]rune(defaultTitle)) > adminDocumentTemplateTitleMaxLen {
		return "", errcode.ErrAdminDocumentTemplateInvalidDefaultTitle
	}
	return defaultTitle, nil
}

func normalizeAdminDocumentTemplateContent(rawContent string) (string, error) {
	if len([]byte(rawContent)) > adminDocumentTemplateContentMaxBytes {
		return "", errcode.ErrAdminDocumentTemplateInvalidContent
	}
	return rawContent, nil
}

func isAdminDocumentTemplateSortValid(sort int) bool {
	return sort >= -1000000 && sort <= 1000000
}

func mapAdminDocumentTemplateRecord(value repository.DocumentTemplateDetailRecord) AdminDocumentTemplateRecord {
	return AdminDocumentTemplateRecord{
		TemplateID:      strings.TrimSpace(value.TemplateID),
		SceneKey:        strings.TrimSpace(value.SceneKey),
		SceneName:       strings.TrimSpace(value.SceneName),
		Name:            strings.TrimSpace(value.Name),
		Description:     strings.TrimSpace(value.Description),
		DefaultTitle:    strings.TrimSpace(value.DefaultTitle),
		ContentMD:       value.ContentMD,
		Sort:            value.Sort,
		Builtin:         value.IsBuiltin,
		Enabled:         value.IsEnabled,
		CreatedByUserID: value.CreatedByUserID,
		UpdatedByUserID: value.UpdatedByUserID,
		CreatedAt:       strings.TrimSpace(value.CreatedAtRaw),
		UpdatedAt:       strings.TrimSpace(value.UpdatedAtRaw),
	}
}

func stringPtr(value string) *string {
	return &value
}
