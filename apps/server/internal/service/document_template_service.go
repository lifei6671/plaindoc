package service

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	documentTemplateDefaultPage     = 1
	documentTemplateDefaultPageSize = 20
	documentTemplateMaxPageSize     = 100
	documentTemplateMaxKeywordLen   = 64
)

var (
	documentTemplateKeyPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)
	ErrDocumentTemplateInvalidPage   = errors.New("document template page is invalid")
	ErrDocumentTemplateInvalidSize   = errors.New("document template page size is invalid")
	ErrDocumentTemplateInvalidScene  = errors.New("document template scene key is invalid")
	ErrDocumentTemplateInvalidKey    = errors.New("document template key is invalid")
	ErrDocumentTemplateInvalidSearch = errors.New("document template keyword is invalid")
	ErrDocumentTemplateNotFound      = errors.New("document template not found")
)

// ListDocumentTemplatesInput 文档模板列表查询输入。
type ListDocumentTemplatesInput struct {
	SceneKey string
	Keyword  string
	Page     int
	PageSize int
}

// DocumentTemplateSummary 文档模板列表项。
type DocumentTemplateSummary struct {
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

// DocumentTemplateListPage 文档模板分页结果。
type DocumentTemplateListPage struct {
	Items    []DocumentTemplateSummary
	Page     int
	PageSize int
	Total    int64
}

// DocumentTemplateDetail 文档模板详情。
type DocumentTemplateDetail struct {
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

// DocumentTemplateService 提供文档模板读取能力。
type DocumentTemplateService struct {
	repo repository.DocumentTemplateRepository
}

// NewDocumentTemplateService 创建文档模板服务。
func NewDocumentTemplateService(repo repository.DocumentTemplateRepository) *DocumentTemplateService {
	return &DocumentTemplateService{repo: repo}
}

// ListEnabledTemplates 查询已启用模板列表（支持场景与关键词过滤）。
func (s *DocumentTemplateService) ListEnabledTemplates(
	ctx context.Context,
	input ListDocumentTemplatesInput,
) (DocumentTemplateListPage, error) {
	if s == nil || s.repo == nil {
		return DocumentTemplateListPage{}, errors.New("document template service dependencies are nil")
	}

	normalizedSceneKey := strings.ToLower(strings.TrimSpace(input.SceneKey))
	if normalizedSceneKey != "" && !documentTemplateKeyPattern.MatchString(normalizedSceneKey) {
		return DocumentTemplateListPage{}, ErrDocumentTemplateInvalidScene
	}

	normalizedKeyword := strings.TrimSpace(input.Keyword)
	if len([]rune(normalizedKeyword)) > documentTemplateMaxKeywordLen {
		return DocumentTemplateListPage{}, ErrDocumentTemplateInvalidSearch
	}

	page := input.Page
	if page <= 0 {
		if input.Page == 0 {
			page = documentTemplateDefaultPage
		} else {
			return DocumentTemplateListPage{}, ErrDocumentTemplateInvalidPage
		}
	}

	pageSize := input.PageSize
	if pageSize <= 0 {
		if input.PageSize == 0 {
			pageSize = documentTemplateDefaultPageSize
		} else {
			return DocumentTemplateListPage{}, ErrDocumentTemplateInvalidSize
		}
	}
	if pageSize > documentTemplateMaxPageSize {
		pageSize = documentTemplateMaxPageSize
	}

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	rows, total, err := s.repo.List(ctx, repository.ListDocumentTemplatesParams{
		SceneKey:    normalizedSceneKey,
		Keyword:     normalizedKeyword,
		EnabledOnly: true,
		Limit:       pageSize,
		Offset:      offset,
	})
	if err != nil {
		return DocumentTemplateListPage{}, err
	}

	items := make([]DocumentTemplateSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, DocumentTemplateSummary{
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

	return DocumentTemplateListPage{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// GetEnabledTemplateByID 按模板标识读取已启用模板详情。
func (s *DocumentTemplateService) GetEnabledTemplateByID(
	ctx context.Context,
	templateID string,
) (DocumentTemplateDetail, error) {
	if s == nil || s.repo == nil {
		return DocumentTemplateDetail{}, errors.New("document template service dependencies are nil")
	}

	normalizedTemplateID := strings.ToLower(strings.TrimSpace(templateID))
	if !documentTemplateKeyPattern.MatchString(normalizedTemplateID) {
		return DocumentTemplateDetail{}, ErrDocumentTemplateInvalidKey
	}

	row, err := s.repo.GetByTemplateID(ctx, normalizedTemplateID, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DocumentTemplateDetail{}, ErrDocumentTemplateNotFound
		}
		return DocumentTemplateDetail{}, err
	}

	return DocumentTemplateDetail{
		TemplateID:      strings.TrimSpace(row.TemplateID),
		SceneKey:        strings.TrimSpace(row.SceneKey),
		SceneName:       strings.TrimSpace(row.SceneName),
		Name:            strings.TrimSpace(row.Name),
		Description:     strings.TrimSpace(row.Description),
		DefaultTitle:    strings.TrimSpace(row.DefaultTitle),
		ContentMD:       row.ContentMD,
		Sort:            row.Sort,
		Builtin:         row.IsBuiltin,
		Enabled:         row.IsEnabled,
		CreatedByUserID: row.CreatedByUserID,
		UpdatedByUserID: row.UpdatedByUserID,
		CreatedAt:       strings.TrimSpace(row.CreatedAtRaw),
		UpdatedAt:       strings.TrimSpace(row.UpdatedAtRaw),
	}, nil
}
