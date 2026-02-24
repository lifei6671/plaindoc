package service

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

var adminThemeIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)

// AdminThemeRecord 后台主题记录。
type AdminThemeRecord struct {
	ThemeID            string
	Name               string
	Description        string
	Variables          map[string]string
	SyntaxTheme        string
	CodeBlockStyle     map[string]any
	CodeBlockCodeStyle map[string]any
	InlineCodeStyle    map[string]any
	CustomCSS          string
	Builtin            bool
	Enabled            bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CreateAdminThemeInput 创建主题输入参数。
type CreateAdminThemeInput struct {
	ActorUserID        string
	RequestID          string
	ThemeID            string
	Name               string
	Description        string
	Variables          map[string]string
	SyntaxTheme        string
	CodeBlockStyle     map[string]any
	CodeBlockCodeStyle map[string]any
	InlineCodeStyle    map[string]any
	CustomCSS          string
	Enabled            bool
}

// UpdateAdminThemeInput 更新主题输入参数。
type UpdateAdminThemeInput struct {
	ActorUserID        string
	RequestID          string
	ThemeID            string
	Name               *string
	Description        *string
	Variables          *map[string]string
	SyntaxTheme        *string
	CodeBlockStyle     *map[string]any
	CodeBlockCodeStyle *map[string]any
	InlineCodeStyle    *map[string]any
	CustomCSS          *string
	Enabled            *bool
}

// AdminThemeService 封装后台主题管理业务。
type AdminThemeService struct {
	themeRepo          repository.ThemeRepository
	adminAccessService *AdminAccessService
	adminAuditService  *AdminAuditService
}

// NewAdminThemeService 创建后台主题管理服务。
func NewAdminThemeService(
	themeRepo repository.ThemeRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
) *AdminThemeService {
	return &AdminThemeService{
		themeRepo:          themeRepo,
		adminAccessService: adminAccessService,
		adminAuditService:  adminAuditService,
	}
}

// ListThemes 查询后台主题列表（包含 disabled 主题）。
func (s *AdminThemeService) ListThemes(
	ctx context.Context,
	actorUserID string,
) (result []AdminThemeRecord, err error) {
	defer func() {
		err = errcode.MapAdminThemeError(err)
	}()

	if s == nil || s.themeRepo == nil || s.adminAccessService == nil {
		return nil, errors.New("admin theme service dependencies are nil")
	}
	if err := s.ensureAdminActor(ctx, actorUserID); err != nil {
		return nil, err
	}

	themes, err := s.themeRepo.List(ctx, true)
	if err != nil {
		return nil, err
	}

	items := make([]AdminThemeRecord, 0, len(themes))
	for _, item := range themes {
		record, mapErr := mapThemeToAdminRecord(item)
		if mapErr != nil {
			return nil, mapErr
		}
		items = append(items, record)
	}
	return items, nil
}

// CreateTheme 创建自定义主题。
func (s *AdminThemeService) CreateTheme(
	ctx context.Context,
	input CreateAdminThemeInput,
) (result AdminThemeRecord, err error) {
	defer func() {
		err = errcode.MapAdminThemeError(err)
	}()

	if s == nil || s.themeRepo == nil || s.adminAccessService == nil {
		return AdminThemeRecord{}, errors.New("admin theme service dependencies are nil")
	}
	if err := s.ensureAdminActor(ctx, input.ActorUserID); err != nil {
		return AdminThemeRecord{}, err
	}

	themeID, err := normalizeAdminThemeID(input.ThemeID)
	if err != nil {
		return AdminThemeRecord{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return AdminThemeRecord{}, errcode.ErrAdminThemeInvalidName
	}

	syntaxTheme := strings.TrimSpace(input.SyntaxTheme)
	if syntaxTheme == "" {
		syntaxTheme = "one-light"
	}
	if syntaxTheme != "one-light" && syntaxTheme != "one-dark" {
		return AdminThemeRecord{}, errcode.ErrAdminThemeInvalidSyntax
	}

	existingTheme, err := s.themeRepo.GetByThemeID(ctx, themeID)
	if err == nil && existingTheme != nil {
		return AdminThemeRecord{}, errcode.ErrAdminThemeAlreadyExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return AdminThemeRecord{}, err
	}

	variablesJSON, err := marshalThemeVariables(input.Variables)
	if err != nil {
		return AdminThemeRecord{}, err
	}
	codeBlockStyleJSON, err := marshalThemeStyle(input.CodeBlockStyle)
	if err != nil {
		return AdminThemeRecord{}, err
	}
	codeBlockCodeStyleJSON, err := marshalThemeStyle(input.CodeBlockCodeStyle)
	if err != nil {
		return AdminThemeRecord{}, err
	}
	inlineCodeStyleJSON, err := marshalThemeStyle(input.InlineCodeStyle)
	if err != nil {
		return AdminThemeRecord{}, err
	}

	enabled := input.Enabled
	now := time.Now().UTC()
	theme := &models.Theme{
		ThemeID:                themeID,
		Name:                   name,
		Description:            strings.TrimSpace(input.Description),
		VariablesJSON:          variablesJSON,
		SyntaxTheme:            syntaxTheme,
		CodeBlockStyleJSON:     codeBlockStyleJSON,
		CodeBlockCodeStyleJSON: codeBlockCodeStyleJSON,
		InlineCodeStyleJSON:    inlineCodeStyleJSON,
		CustomCSS:              input.CustomCSS,
		IsBuiltin:              false,
		IsEnabled:              enabled,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := s.themeRepo.Create(ctx, theme); err != nil {
		return AdminThemeRecord{}, err
	}

	createdTheme, err := s.themeRepo.GetByThemeID(ctx, themeID)
	if err != nil {
		return AdminThemeRecord{}, err
	}
	record, err := mapThemeToAdminRecord(*createdTheme)
	if err != nil {
		return AdminThemeRecord{}, err
	}

	if err := s.recordThemeAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleTheme,
		Action:     AdminAuditActionCreate,
		TargetType: "theme",
		TargetID:   record.ThemeID,
		Summary:    "theme created: " + record.ThemeID,
		Detail: map[string]any{
			"name":        record.Name,
			"enabled":     record.Enabled,
			"syntaxTheme": record.SyntaxTheme,
		},
	}); err != nil {
		return AdminThemeRecord{}, err
	}

	return record, nil
}

// UpdateTheme 更新主题内容与启停状态。
func (s *AdminThemeService) UpdateTheme(
	ctx context.Context,
	input UpdateAdminThemeInput,
) (result AdminThemeRecord, err error) {
	defer func() {
		err = errcode.MapAdminThemeError(err)
	}()

	if s == nil || s.themeRepo == nil || s.adminAccessService == nil {
		return AdminThemeRecord{}, errors.New("admin theme service dependencies are nil")
	}
	if err := s.ensureAdminActor(ctx, input.ActorUserID); err != nil {
		return AdminThemeRecord{}, err
	}

	themeID, err := normalizeAdminThemeID(input.ThemeID)
	if err != nil {
		return AdminThemeRecord{}, err
	}

	targetTheme, err := s.themeRepo.GetByThemeID(ctx, themeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminThemeRecord{}, errcode.ErrAdminThemeNotFound
		}
		return AdminThemeRecord{}, err
	}
	if targetTheme.IsBuiltin {
		return AdminThemeRecord{}, errcode.ErrAdminThemeBuiltinImmutable
	}

	params := repository.UpdateThemeParams{
		ThemeID:   themeID,
		UpdatedAt: time.Now().UTC(),
	}

	changed := false
	changedFields := make([]string, 0, 8)
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return AdminThemeRecord{}, errcode.ErrAdminThemeInvalidName
		}
		params.Name = &name
		changed = true
		changedFields = append(changedFields, "name")
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		params.Description = &description
		changed = true
		changedFields = append(changedFields, "description")
	}
	if input.Variables != nil {
		variablesJSON, marshalErr := marshalThemeVariables(*input.Variables)
		if marshalErr != nil {
			return AdminThemeRecord{}, marshalErr
		}
		params.VariablesJSON = &variablesJSON
		changed = true
		changedFields = append(changedFields, "variables")
	}
	if input.SyntaxTheme != nil {
		syntaxTheme := strings.TrimSpace(*input.SyntaxTheme)
		if syntaxTheme != "one-light" && syntaxTheme != "one-dark" {
			return AdminThemeRecord{}, errcode.ErrAdminThemeInvalidSyntax
		}
		params.SyntaxTheme = &syntaxTheme
		changed = true
		changedFields = append(changedFields, "syntaxTheme")
	}
	if input.CodeBlockStyle != nil {
		value, marshalErr := marshalThemeStyle(*input.CodeBlockStyle)
		if marshalErr != nil {
			return AdminThemeRecord{}, marshalErr
		}
		params.CodeBlockStyleJSON = &value
		changed = true
		changedFields = append(changedFields, "codeBlockStyle")
	}
	if input.CodeBlockCodeStyle != nil {
		value, marshalErr := marshalThemeStyle(*input.CodeBlockCodeStyle)
		if marshalErr != nil {
			return AdminThemeRecord{}, marshalErr
		}
		params.CodeBlockCodeStyleJSON = &value
		changed = true
		changedFields = append(changedFields, "codeBlockCodeStyle")
	}
	if input.InlineCodeStyle != nil {
		value, marshalErr := marshalThemeStyle(*input.InlineCodeStyle)
		if marshalErr != nil {
			return AdminThemeRecord{}, marshalErr
		}
		params.InlineCodeStyleJSON = &value
		changed = true
		changedFields = append(changedFields, "inlineCodeStyle")
	}
	if input.CustomCSS != nil {
		customCSS := *input.CustomCSS
		params.CustomCSS = &customCSS
		changed = true
		changedFields = append(changedFields, "customCss")
	}
	if input.Enabled != nil {
		params.IsEnabled = input.Enabled
		changed = true
		changedFields = append(changedFields, "enabled")
	}

	if !changed {
		return AdminThemeRecord{}, errcode.ErrAdminThemeNoChanges
	}

	updated, err := s.themeRepo.Update(ctx, params)
	if err != nil {
		return AdminThemeRecord{}, err
	}
	if !updated {
		return AdminThemeRecord{}, errcode.ErrAdminThemeNotFound
	}

	latestTheme, err := s.themeRepo.GetByThemeID(ctx, themeID)
	if err != nil {
		return AdminThemeRecord{}, err
	}
	record, err := mapThemeToAdminRecord(*latestTheme)
	if err != nil {
		return AdminThemeRecord{}, err
	}

	if err := s.recordThemeAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleTheme,
		Action:     AdminAuditActionUpdate,
		TargetType: "theme",
		TargetID:   record.ThemeID,
		Summary:    "theme updated: " + record.ThemeID,
		Detail: map[string]any{
			"changedFields": changedFields,
		},
	}); err != nil {
		return AdminThemeRecord{}, err
	}

	return record, nil
}

// DeleteTheme 删除自定义主题（被文档引用时拒绝删除）。
func (s *AdminThemeService) DeleteTheme(
	ctx context.Context,
	actorUserID string,
	themeID string,
	requestID string,
) (err error) {
	defer func() {
		err = errcode.MapAdminThemeError(err)
	}()

	_ = requestID

	if s == nil || s.themeRepo == nil || s.adminAccessService == nil {
		return errors.New("admin theme service dependencies are nil")
	}
	if err := s.ensureAdminActor(ctx, actorUserID); err != nil {
		return err
	}

	normalizedThemeID, err := normalizeAdminThemeID(themeID)
	if err != nil {
		return err
	}

	targetTheme, err := s.themeRepo.GetByThemeID(ctx, normalizedThemeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrAdminThemeNotFound
		}
		return err
	}
	if targetTheme.IsBuiltin {
		return errcode.ErrAdminThemeBuiltinImmutable
	}

	references, err := s.themeRepo.CountDocumentReferences(ctx, normalizedThemeID)
	if err != nil {
		return err
	}
	if references > 0 {
		return errcode.ErrAdminThemeInUse
	}

	deleted, err := s.themeRepo.Delete(ctx, normalizedThemeID)
	if err != nil {
		return err
	}
	if !deleted {
		return errcode.ErrAdminThemeNotFound
	}

	if err := s.recordThemeAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleTheme,
		Action:     AdminAuditActionDelete,
		TargetType: "theme",
		TargetID:   normalizedThemeID,
		Summary:    "theme deleted: " + normalizedThemeID,
		Detail: map[string]any{
			"themeId": normalizedThemeID,
		},
	}); err != nil {
		return err
	}

	return nil
}

func (s *AdminThemeService) recordThemeAudit(ctx context.Context, input RecordAdminAuditInput) error {
	if s == nil || s.adminAuditService == nil {
		return nil
	}
	return s.adminAuditService.Record(ctx, input)
}

func (s *AdminThemeService) ensureAdminActor(ctx context.Context, actorUserID string) error {
	userID := strings.TrimSpace(actorUserID)
	if userID == "" {
		return errcode.ErrAdminForbidden
	}

	isAdmin, err := s.adminAccessService.IsAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return errcode.ErrAdminForbidden
	}
	return nil
}

func normalizeAdminThemeID(rawThemeID string) (string, error) {
	themeID := strings.ToLower(strings.TrimSpace(rawThemeID))
	if !adminThemeIDPattern.MatchString(themeID) {
		return "", errcode.ErrAdminThemeInvalidThemeID
	}
	return themeID, nil
}

func marshalThemeVariables(value map[string]string) (string, error) {
	if value == nil {
		value = map[string]string{}
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func marshalThemeStyle(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func mapThemeToAdminRecord(value models.Theme) (AdminThemeRecord, error) {
	var variables map[string]string
	if err := json.Unmarshal([]byte(value.VariablesJSON), &variables); err != nil {
		return AdminThemeRecord{}, err
	}
	if variables == nil {
		variables = map[string]string{}
	}

	codeBlockStyle, err := decodeThemeStyleJSON(value.CodeBlockStyleJSON)
	if err != nil {
		return AdminThemeRecord{}, err
	}
	codeBlockCodeStyle, err := decodeThemeStyleJSON(value.CodeBlockCodeStyleJSON)
	if err != nil {
		return AdminThemeRecord{}, err
	}
	inlineCodeStyle, err := decodeThemeStyleJSON(value.InlineCodeStyleJSON)
	if err != nil {
		return AdminThemeRecord{}, err
	}

	return AdminThemeRecord{
		ThemeID:            value.ThemeID,
		Name:               value.Name,
		Description:        value.Description,
		Variables:          variables,
		SyntaxTheme:        value.SyntaxTheme,
		CodeBlockStyle:     codeBlockStyle,
		CodeBlockCodeStyle: codeBlockCodeStyle,
		InlineCodeStyle:    inlineCodeStyle,
		CustomCSS:          value.CustomCSS,
		Builtin:            value.IsBuiltin,
		Enabled:            value.IsEnabled,
		CreatedAt:          value.CreatedAt,
		UpdatedAt:          value.UpdatedAt,
	}, nil
}

func decodeThemeStyleJSON(rawValue string) (map[string]any, error) {
	if strings.TrimSpace(rawValue) == "" {
		return map[string]any{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(rawValue), &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return map[string]any{}, nil
	}
	return payload, nil
}
