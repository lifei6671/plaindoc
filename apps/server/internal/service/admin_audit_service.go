package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

const (
	defaultAdminAuditPageSize = 20
	maxAdminAuditPageSize     = 100
)

// AdminAuditModule 审计模块。
type AdminAuditModule string

const (
	AdminAuditModuleUser         AdminAuditModule = "user"
	AdminAuditModuleSpace        AdminAuditModule = "space"
	AdminAuditModuleDocument     AdminAuditModule = "document"
	AdminAuditModuleTemplate     AdminAuditModule = "document_template"
	AdminAuditModuleTheme        AdminAuditModule = "theme"
	AdminAuditModuleSystemConfig AdminAuditModule = "system_config"
)

// AdminAuditAction 审计动作。
type AdminAuditAction string

const (
	AdminAuditActionCreate AdminAuditAction = "create"
	AdminAuditActionUpdate AdminAuditAction = "update"
	AdminAuditActionDelete AdminAuditAction = "delete"
)

var validAdminAuditModules = map[string]struct{}{
	string(AdminAuditModuleUser):         {},
	string(AdminAuditModuleSpace):        {},
	string(AdminAuditModuleDocument):     {},
	string(AdminAuditModuleTemplate):     {},
	string(AdminAuditModuleTheme):        {},
	string(AdminAuditModuleSystemConfig): {},
}

var validAdminAuditActions = map[string]struct{}{
	string(AdminAuditActionCreate): {},
	string(AdminAuditActionUpdate): {},
	string(AdminAuditActionDelete): {},
}

// RecordAdminAuditInput 写入后台审计日志参数。
type RecordAdminAuditInput struct {
	ActorUserID string
	Module      AdminAuditModule
	Action      AdminAuditAction
	TargetType  string
	TargetID    string
	Summary     string
	Detail      any
	RequestID   string
}

// ListAdminAuditsInput 后台审计日志查询参数。
type ListAdminAuditsInput struct {
	ActorUserID       string
	Keyword           string
	ModuleFilter      string
	ActionFilter      string
	ActorUserIDFilter string
	TargetTypeFilter  string
	TargetIDFilter    string
	RequestIDFilter   string
	CreatedAtFrom     *time.Time
	CreatedAtTo       *time.Time
	Page              int
	PageSize          int
}

// AdminAuditRecord 后台审计日志项。
type AdminAuditRecord struct {
	ID          int64
	ActorUserID *string
	ActorName   string
	ActorEmail  string
	Module      string
	Action      string
	TargetType  string
	TargetID    string
	Summary     string
	Detail      map[string]any
	RequestID   string
	CreatedAt   time.Time
}

// ListAdminAuditsResult 后台审计日志分页结果。
type ListAdminAuditsResult struct {
	Items    []AdminAuditRecord
	Page     int
	PageSize int
	Total    int64
}

// AdminAuditService 封装后台审计日志写入与查询。
type AdminAuditService struct {
	auditLogRepo       repository.AuditLogRepository
	adminAccessService *AdminAccessService
}

// NewAdminAuditService 创建后台审计日志服务。
func NewAdminAuditService(
	auditLogRepo repository.AuditLogRepository,
	adminAccessService *AdminAccessService,
) *AdminAuditService {
	return &AdminAuditService{
		auditLogRepo:       auditLogRepo,
		adminAccessService: adminAccessService,
	}
}

// Record 写入审计日志。
func (s *AdminAuditService) Record(
	ctx context.Context,
	input RecordAdminAuditInput,
) (err error) {
	defer func() {
		err = errcode.MapAdminAuditError(err)
	}()

	if s == nil || s.auditLogRepo == nil || s.adminAccessService == nil {
		return errors.New("admin audit service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		actorUserID = adminAuditActorUserIDFromContext(ctx)
	}
	if actorUserID == "" {
		return errcode.ErrAdminForbidden
	}
	isAdmin, err := s.adminAccessService.IsAdmin(ctx, actorUserID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return errcode.ErrAdminForbidden
	}

	module, err := normalizeAdminAuditModule(string(input.Module))
	if err != nil {
		return err
	}
	action, err := normalizeAdminAuditAction(string(input.Action))
	if err != nil {
		return err
	}

	targetType := strings.ToLower(strings.TrimSpace(input.TargetType))
	if targetType == "" {
		return errcode.ErrAdminAuditInvalidTargetType
	}
	targetID := strings.TrimSpace(input.TargetID)
	if targetID == "" {
		return errcode.ErrAdminAuditInvalidTargetID
	}

	summary := strings.TrimSpace(input.Summary)
	if summary == "" {
		summary = action + " " + targetType + " " + targetID
	}

	detailJSON := "{}"
	if input.Detail != nil {
		payload, marshalErr := json.Marshal(input.Detail)
		if marshalErr != nil {
			return marshalErr
		}
		detailJSON = string(payload)
	}

	requestID := strings.TrimSpace(input.RequestID)
	if requestID == "" {
		requestID = adminAuditRequestIDFromContext(ctx)
	}
	auditLog := &models.AuditLog{
		ActorUserID: &actorUserID,
		Module:      module,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Summary:     summary,
		DetailJSON:  detailJSON,
		RequestID:   requestID,
		CreatedAt:   time.Now().UTC(),
	}
	return s.auditLogRepo.Create(ctx, auditLog)
}

// ListAudits 查询后台审计日志。
func (s *AdminAuditService) ListAudits(
	ctx context.Context,
	input ListAdminAuditsInput,
) (result ListAdminAuditsResult, err error) {
	defer func() {
		err = errcode.MapAdminAuditError(err)
	}()

	if s == nil || s.auditLogRepo == nil || s.adminAccessService == nil {
		return ListAdminAuditsResult{}, errors.New("admin audit service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return ListAdminAuditsResult{}, errcode.ErrAdminForbidden
	}
	isAdmin, err := s.adminAccessService.IsAdmin(ctx, actorUserID)
	if err != nil {
		return ListAdminAuditsResult{}, err
	}
	if !isAdmin {
		return ListAdminAuditsResult{}, errcode.ErrAdminForbidden
	}

	moduleFilter, err := normalizeAdminAuditModuleFilter(input.ModuleFilter)
	if err != nil {
		return ListAdminAuditsResult{}, err
	}
	actionFilter, err := normalizeAdminAuditActionFilter(input.ActionFilter)
	if err != nil {
		return ListAdminAuditsResult{}, err
	}
	if input.CreatedAtFrom != nil && input.CreatedAtTo != nil && input.CreatedAtFrom.After(*input.CreatedAtTo) {
		return ListAdminAuditsResult{}, errcode.ErrAdminAuditInvalidTimeRange
	}

	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = defaultAdminAuditPageSize
	}
	if pageSize > maxAdminAuditPageSize {
		pageSize = maxAdminAuditPageSize
	}

	actorUserIDFilter := strings.TrimSpace(input.ActorUserIDFilter)
	restrictModules := []string{}
	isPlatformAdmin, err := s.adminAccessService.IsPlatformAdmin(ctx, actorUserID)
	if err != nil {
		return ListAdminAuditsResult{}, err
	}
	if !isPlatformAdmin {
		restrictModules = []string{
			string(AdminAuditModuleSpace),
			string(AdminAuditModuleDocument),
			string(AdminAuditModuleTheme),
		}
		if moduleFilter != "" && moduleFilter != string(AdminAuditModuleSpace) &&
			moduleFilter != string(AdminAuditModuleDocument) &&
			moduleFilter != string(AdminAuditModuleTheme) {
			return ListAdminAuditsResult{}, errcode.ErrAdminForbidden
		}
		if actorUserIDFilter == "" {
			actorUserIDFilter = actorUserID
		}
	}

	items, total, err := s.auditLogRepo.List(ctx, repository.ListAuditLogsParams{
		ActorUserID:     actorUserIDFilter,
		Keyword:         strings.TrimSpace(input.Keyword),
		Module:          moduleFilter,
		Action:          actionFilter,
		TargetType:      strings.ToLower(strings.TrimSpace(input.TargetTypeFilter)),
		TargetID:        strings.TrimSpace(input.TargetIDFilter),
		RequestID:       strings.TrimSpace(input.RequestIDFilter),
		CreatedAtFrom:   input.CreatedAtFrom,
		CreatedAtTo:     input.CreatedAtTo,
		RestrictModules: restrictModules,
		Limit:           pageSize,
		Offset:          (page - 1) * pageSize,
	})
	if err != nil {
		return ListAdminAuditsResult{}, err
	}

	records := make([]AdminAuditRecord, 0, len(items))
	for _, item := range items {
		detail := decodeAdminAuditDetailJSON(item.AuditLog.DetailJSON)
		records = append(records, AdminAuditRecord{
			ID:          item.AuditLog.ID,
			ActorUserID: item.AuditLog.ActorUserID,
			ActorName:   item.ActorName,
			ActorEmail:  item.ActorEmail,
			Module:      item.AuditLog.Module,
			Action:      item.AuditLog.Action,
			TargetType:  item.AuditLog.TargetType,
			TargetID:    item.AuditLog.TargetID,
			Summary:     item.AuditLog.Summary,
			Detail:      detail,
			RequestID:   item.AuditLog.RequestID,
			CreatedAt:   item.AuditLog.CreatedAt,
		})
	}

	return ListAdminAuditsResult{
		Items:    records,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func normalizeAdminAuditModule(rawModule string) (string, error) {
	module := strings.ToLower(strings.TrimSpace(rawModule))
	if _, ok := validAdminAuditModules[module]; !ok {
		return "", errcode.ErrAdminAuditInvalidModule
	}
	return module, nil
}

func normalizeAdminAuditModuleFilter(rawModule string) (string, error) {
	module := strings.ToLower(strings.TrimSpace(rawModule))
	if module == "" {
		return "", nil
	}
	if _, ok := validAdminAuditModules[module]; !ok {
		return "", errcode.ErrAdminAuditInvalidModuleFilter
	}
	return module, nil
}

func normalizeAdminAuditAction(rawAction string) (string, error) {
	action := strings.ToLower(strings.TrimSpace(rawAction))
	if _, ok := validAdminAuditActions[action]; !ok {
		return "", errcode.ErrAdminAuditInvalidAction
	}
	return action, nil
}

func normalizeAdminAuditActionFilter(rawAction string) (string, error) {
	action := strings.ToLower(strings.TrimSpace(rawAction))
	if action == "" {
		return "", nil
	}
	if _, ok := validAdminAuditActions[action]; !ok {
		return "", errcode.ErrAdminAuditInvalidActionFilter
	}
	return action, nil
}

func decodeAdminAuditDetailJSON(rawJSON string) map[string]any {
	value := strings.TrimSpace(rawJSON)
	if value == "" {
		return map[string]any{}
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return map[string]any{
			"raw": value,
		}
	}
	if payload == nil {
		return map[string]any{}
	}
	return payload
}
