package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	defaultAdminUserPage     = 1
	defaultAdminUserPageSize = 20
	maxAdminUserPageSize     = 100
)

var (
	ErrAdminUserInvalidStatusFilter  = errors.New("invalid admin user status filter")
	ErrAdminUserInvalidStatus        = errors.New("invalid admin user status")
	ErrAdminUserBanReasonRequired    = errors.New("admin user ban reason is required")
	ErrAdminUserNotFound             = errors.New("admin user target not found")
	ErrAdminUserInvalidUserID        = errors.New("admin user id is invalid")
	ErrAdminUserSelfOperationBlocked = errors.New("admin user self operation is blocked")
	ErrAdminUserAlreadyDeleted       = errors.New("admin user target already deleted")
)

// AdminUserStatusFilter 管理后台用户查询状态过滤条件。
type AdminUserStatusFilter string

const (
	AdminUserStatusFilterAll     AdminUserStatusFilter = "all"
	AdminUserStatusFilterActive  AdminUserStatusFilter = "active"
	AdminUserStatusFilterBanned  AdminUserStatusFilter = "banned"
	AdminUserStatusFilterDeleted AdminUserStatusFilter = "deleted"
)

// AdminUserRecord 管理后台用户信息视图。
type AdminUserRecord struct {
	UserID       string
	Email        string
	Name         string
	Status       models.EntityStatus
	BannedReason string
	BannedAt     *time.Time
	DeletedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ListAdminUsersInput 定义后台用户列表查询参数。
type ListAdminUsersInput struct {
	ActorUserID  string
	Keyword      string
	StatusFilter AdminUserStatusFilter
	Page         int
	PageSize     int
}

// ListAdminUsersResult 定义后台用户列表返回值。
type ListAdminUsersResult struct {
	Items    []AdminUserRecord
	Page     int
	PageSize int
	Total    int64
}

// UpdateAdminUserStatusInput 定义后台用户状态更新参数。
type UpdateAdminUserStatusInput struct {
	ActorUserID string
	RequestID   string
	UserID      string
	Status      models.EntityStatus
	Reason      string
}

// AdminUserService 封装后台用户管理业务。
type AdminUserService struct {
	userRepo           repository.UserRepository
	userSessionRepo    repository.UserSessionRepository
	adminAccessService *AdminAccessService
	adminAuditService  *AdminAuditService
}

// NewAdminUserService 创建后台用户管理服务。
func NewAdminUserService(
	userRepo repository.UserRepository,
	userSessionRepo repository.UserSessionRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
) *AdminUserService {
	return &AdminUserService{
		userRepo:           userRepo,
		userSessionRepo:    userSessionRepo,
		adminAccessService: adminAccessService,
		adminAuditService:  adminAuditService,
	}
}

// ListUsers 查询后台用户列表，默认不返回已删除用户。
func (s *AdminUserService) ListUsers(ctx context.Context, input ListAdminUsersInput) (ListAdminUsersResult, error) {
	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return ListAdminUsersResult{}, err
	}

	statuses, err := resolveUserStatuses(input.StatusFilter)
	if err != nil {
		return ListAdminUsersResult{}, err
	}

	page, pageSize := normalizePagination(input.Page, input.PageSize)
	users, total, err := s.userRepo.List(ctx, repository.ListUsersParams{
		Keyword:  strings.TrimSpace(input.Keyword),
		Statuses: statuses,
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
	})
	if err != nil {
		return ListAdminUsersResult{}, err
	}

	items := make([]AdminUserRecord, 0, len(users))
	for idx := range users {
		items = append(items, mapAdminUserRecord(&users[idx]))
	}

	return ListAdminUsersResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// UpdateUserStatus 更新后台目标用户状态（active/banned）。
func (s *AdminUserService) UpdateUserStatus(
	ctx context.Context,
	input UpdateAdminUserStatusInput,
) (AdminUserRecord, error) {
	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return AdminUserRecord{}, err
	}

	targetUserID := strings.TrimSpace(input.UserID)
	if targetUserID == "" {
		return AdminUserRecord{}, ErrAdminUserInvalidUserID
	}
	if strings.TrimSpace(input.ActorUserID) == targetUserID {
		return AdminUserRecord{}, ErrAdminUserSelfOperationBlocked
	}
	if input.Status != models.EntityStatusActive && input.Status != models.EntityStatusBanned {
		return AdminUserRecord{}, ErrAdminUserInvalidStatus
	}

	reason := strings.TrimSpace(input.Reason)
	if input.Status == models.EntityStatusBanned && reason == "" {
		return AdminUserRecord{}, ErrAdminUserBanReasonRequired
	}

	targetUser, err := s.userRepo.GetByUserID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminUserRecord{}, ErrAdminUserNotFound
		}
		return AdminUserRecord{}, err
	}
	if normalizeEntityStatus(targetUser.Status) == models.EntityStatusDeleted || targetUser.DeletedAt != nil {
		return AdminUserRecord{}, ErrAdminUserAlreadyDeleted
	}

	now := time.Now().UTC()
	params := repository.UpdateUserStatusParams{
		UserID:    targetUserID,
		Status:    input.Status,
		UpdatedAt: now,
	}
	if input.Status == models.EntityStatusBanned {
		params.BannedReason = reason
		params.BannedAt = &now
	}

	updated, err := s.userRepo.UpdateStatus(ctx, params)
	if err != nil {
		return AdminUserRecord{}, err
	}
	if !updated {
		return AdminUserRecord{}, ErrAdminUserNotFound
	}

	if input.Status == models.EntityStatusBanned {
		if err := s.userSessionRepo.RevokeAllByUserID(ctx, targetUserID, now); err != nil {
			return AdminUserRecord{}, err
		}
	}

	latestUser, err := s.userRepo.GetByUserID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminUserRecord{}, ErrAdminUserNotFound
		}
		return AdminUserRecord{}, err
	}

	record := mapAdminUserRecord(latestUser)
	if err := s.recordUserAudit(ctx, RecordAdminAuditInput{
		ActorUserID: input.ActorUserID,
		Module:      AdminAuditModuleUser,
		Action:      AdminAuditActionUpdate,
		TargetType:  "user",
		TargetID:    record.UserID,
		Summary:     "user status updated: " + record.UserID,
		RequestID:   input.RequestID,
		Detail: map[string]any{
			"statusBefore": targetUser.Status,
			"statusAfter":  record.Status,
			"reason":       strings.TrimSpace(input.Reason),
		},
	}); err != nil {
		return AdminUserRecord{}, err
	}

	return record, nil
}

// DeleteUser 执行软删除，并立即吊销目标用户所有在线会话。
func (s *AdminUserService) DeleteUser(
	ctx context.Context,
	actorUserID string,
	targetUserID string,
	requestID string,
) error {
	if err := s.ensurePlatformAdmin(ctx, actorUserID); err != nil {
		return err
	}

	normalizedTargetUserID := strings.TrimSpace(targetUserID)
	if normalizedTargetUserID == "" {
		return ErrAdminUserInvalidUserID
	}
	if strings.TrimSpace(actorUserID) == normalizedTargetUserID {
		return ErrAdminUserSelfOperationBlocked
	}

	targetUser, err := s.userRepo.GetByUserID(ctx, normalizedTargetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAdminUserNotFound
		}
		return err
	}
	if normalizeEntityStatus(targetUser.Status) == models.EntityStatusDeleted || targetUser.DeletedAt != nil {
		return nil
	}

	now := time.Now().UTC()
	deleted, err := s.userRepo.SoftDelete(ctx, normalizedTargetUserID, now)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrAdminUserNotFound
	}
	if err := s.userSessionRepo.RevokeAllByUserID(ctx, normalizedTargetUserID, now); err != nil {
		return err
	}

	if err := s.recordUserAudit(ctx, RecordAdminAuditInput{
		ActorUserID: actorUserID,
		Module:      AdminAuditModuleUser,
		Action:      AdminAuditActionDelete,
		TargetType:  "user",
		TargetID:    normalizedTargetUserID,
		Summary:     "user deleted: " + normalizedTargetUserID,
		RequestID:   requestID,
		Detail: map[string]any{
			"statusBefore": targetUser.Status,
			"statusAfter":  models.EntityStatusDeleted,
		},
	}); err != nil {
		return err
	}

	return nil
}

func (s *AdminUserService) ensurePlatformAdmin(ctx context.Context, actorUserID string) error {
	if s == nil || s.userRepo == nil || s.userSessionRepo == nil || s.adminAccessService == nil {
		return errors.New("admin user service dependencies are nil")
	}

	isPlatformAdmin, err := s.adminAccessService.IsPlatformAdmin(ctx, strings.TrimSpace(actorUserID))
	if err != nil {
		return err
	}
	if !isPlatformAdmin {
		return ErrAdminForbidden
	}
	return nil
}

func (s *AdminUserService) recordUserAudit(ctx context.Context, input RecordAdminAuditInput) error {
	if s == nil || s.adminAuditService == nil {
		return nil
	}
	return s.adminAuditService.Record(ctx, input)
}

func resolveUserStatuses(filter AdminUserStatusFilter) ([]models.EntityStatus, error) {
	switch normalizeAdminUserStatusFilter(filter) {
	case "":
		return []models.EntityStatus{
			models.EntityStatusActive,
			models.EntityStatusBanned,
		}, nil
	case AdminUserStatusFilterAll:
		return []models.EntityStatus{
			models.EntityStatusActive,
			models.EntityStatusBanned,
			models.EntityStatusDeleted,
		}, nil
	case AdminUserStatusFilterActive:
		return []models.EntityStatus{models.EntityStatusActive}, nil
	case AdminUserStatusFilterBanned:
		return []models.EntityStatus{models.EntityStatusBanned}, nil
	case AdminUserStatusFilterDeleted:
		return []models.EntityStatus{models.EntityStatusDeleted}, nil
	default:
		return nil, ErrAdminUserInvalidStatusFilter
	}
}

func normalizeAdminUserStatusFilter(filter AdminUserStatusFilter) AdminUserStatusFilter {
	value := strings.ToLower(strings.TrimSpace(string(filter)))
	if value == "" {
		return ""
	}
	return AdminUserStatusFilter(value)
}

func normalizePagination(page int, pageSize int) (int, int) {
	normalizedPage := page
	if normalizedPage <= 0 {
		normalizedPage = defaultAdminUserPage
	}

	normalizedPageSize := pageSize
	if normalizedPageSize <= 0 {
		normalizedPageSize = defaultAdminUserPageSize
	}
	if normalizedPageSize > maxAdminUserPageSize {
		normalizedPageSize = maxAdminUserPageSize
	}

	return normalizedPage, normalizedPageSize
}

func mapAdminUserRecord(user *models.User) AdminUserRecord {
	if user == nil {
		return AdminUserRecord{}
	}
	status := user.Status
	if !models.IsValidEntityStatus(status) {
		status = models.EntityStatusActive
	}
	return AdminUserRecord{
		UserID:       user.UserID,
		Email:        user.Email,
		Name:         user.Name,
		Status:       status,
		BannedReason: user.BannedReason,
		BannedAt:     user.BannedAt,
		DeletedAt:    user.DeletedAt,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}
