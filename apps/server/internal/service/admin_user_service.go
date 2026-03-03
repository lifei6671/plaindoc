package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	defaultAdminUserPage     = 1
	defaultAdminUserPageSize = 20
	maxAdminUserPageSize     = 100
)

// AdminUserStatusFilter 管理后台用户查询状态过滤条件。
type AdminUserStatusFilter string

const (
	AdminUserStatusFilterAll     AdminUserStatusFilter = "all"
	AdminUserStatusFilterActive  AdminUserStatusFilter = "active"
	AdminUserStatusFilterBanned  AdminUserStatusFilter = "banned"
	AdminUserStatusFilterDeleted AdminUserStatusFilter = "deleted"
)

// AdminUserRole 后台用户角色（含普通用户）。
type AdminUserRole string

const (
	AdminUserRoleUser          AdminUserRole = "user"
	AdminUserRoleSpaceAdmin    AdminUserRole = "space_admin"
	AdminUserRolePlatformAdmin AdminUserRole = "platform_admin"
)

// AdminUserRecord 管理后台用户信息视图。
type AdminUserRecord struct {
	UserID       string
	Email        string
	Name         string
	Role         AdminUserRole
	CanEditRole  bool
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

// CreateAdminUserInput 定义后台新增用户参数。
type CreateAdminUserInput struct {
	ActorUserID string
	RequestID   string
	Email       string
	Name        string
	Password    string
	Role        AdminUserRole
}

// UpdateAdminUserStatusInput 定义后台用户状态更新参数。
type UpdateAdminUserStatusInput struct {
	ActorUserID string
	RequestID   string
	UserID      string
	Status      models.EntityStatus
	Reason      string
}

// UpdateAdminUserRoleInput 定义后台用户角色更新参数。
type UpdateAdminUserRoleInput struct {
	ActorUserID string
	RequestID   string
	UserID      string
	Role        AdminUserRole
}

// SendAdminUserPasswordResetEmailInput 定义后台发送密码重置邮件参数。
type SendAdminUserPasswordResetEmailInput struct {
	ActorUserID string
	RequestID   string
	UserID      string
	ClientIP    string
}

// AdminUserService 封装后台用户管理业务。
type AdminUserService struct {
	userRepo           repository.UserRepository
	userSessionRepo    repository.UserSessionRepository
	adminRoleRepo      repository.AdminRoleRepository
	adminAccessService *AdminAccessService
	adminAuditService  *AdminAuditService
	passwordResetService *PasswordResetService
}

// NewAdminUserService 创建后台用户管理服务。
func NewAdminUserService(
	userRepo repository.UserRepository,
	userSessionRepo repository.UserSessionRepository,
	adminRoleRepo repository.AdminRoleRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
	passwordResetServices ...*PasswordResetService,
) *AdminUserService {
	var passwordResetService *PasswordResetService
	if len(passwordResetServices) > 0 {
		passwordResetService = passwordResetServices[0]
	}
	return &AdminUserService{
		userRepo:           userRepo,
		userSessionRepo:    userSessionRepo,
		adminRoleRepo:      adminRoleRepo,
		adminAccessService: adminAccessService,
		adminAuditService:  adminAuditService,
		passwordResetService: passwordResetService,
	}
}

// ListUsers 查询后台用户列表，默认不返回已删除用户。
func (s *AdminUserService) ListUsers(
	ctx context.Context,
	input ListAdminUsersInput,
) (result ListAdminUsersResult, err error) {
	defer func() {
		err = errcode.MapAdminUserError(err)
	}()

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

	actorRole, err := s.resolveUserRole(ctx, strings.TrimSpace(input.ActorUserID))
	if err != nil {
		return ListAdminUsersResult{}, err
	}
	userIDs := make([]string, 0, len(users))
	for idx := range users {
		userIDs = append(userIDs, users[idx].UserID)
	}
	rolesByUserID, err := s.adminRoleRepo.ListByUserIDs(ctx, userIDs)
	if err != nil {
		return ListAdminUsersResult{}, err
	}

	items := make([]AdminUserRecord, 0, len(users))
	for idx := range users {
		targetRoles := rolesByUserID[users[idx].UserID]
		targetRole := resolveAdminUserRole(targetRoles)
		items = append(
			items,
			mapAdminUserRecord(
				&users[idx],
				targetRole,
				canEditRole(
					actorRole,
					targetRole,
					strings.TrimSpace(input.ActorUserID),
					users[idx].UserID,
				),
			),
		)
	}

	return ListAdminUsersResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// CreateUser 新增后台用户，默认创建为 active 状态。
func (s *AdminUserService) CreateUser(
	ctx context.Context,
	input CreateAdminUserInput,
) (result AdminUserRecord, err error) {
	defer func() {
		err = errcode.MapAdminUserError(err)
	}()

	_ = input.RequestID

	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return AdminUserRecord{}, err
	}
	actorUserID := strings.TrimSpace(input.ActorUserID)
	actorRole, err := s.resolveUserRole(ctx, actorUserID)
	if err != nil {
		return AdminUserRecord{}, err
	}

	email := normalizeEmail(input.Email)
	name := strings.TrimSpace(input.Name)
	password := input.Password
	role := normalizeAdminUserRole(input.Role)

	if email == "" || !strings.Contains(email, "@") {
		return AdminUserRecord{}, errcode.ErrAdminUserInvalidEmail
	}
	if name == "" {
		return AdminUserRecord{}, errcode.ErrAdminUserInvalidName
	}
	if len(password) < 6 {
		return AdminUserRecord{}, errcode.ErrAdminUserInvalidPassword
	}
	if !isValidAdminUserRole(role) {
		return AdminUserRecord{}, errcode.ErrAdminUserInvalidRole
	}
	if adminUserRoleRank(role) > adminUserRoleRank(actorRole) {
		return AdminUserRecord{}, errcode.ErrAdminUserRoleForbidden
	}

	_, err = s.userRepo.GetByEmail(ctx, email)
	if err == nil {
		return AdminUserRecord{}, errcode.ErrAdminUserEmailAlreadyExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return AdminUserRecord{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AdminUserRecord{}, err
	}

	now := time.Now().UTC()
	user := &models.User{
		UserID:       strings.ToLower(ulid.Make().String()),
		Email:        email,
		PasswordHash: string(passwordHash),
		Name:         name,
		Status:       models.EntityStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		if isUniqueConstraintError(err) {
			return AdminUserRecord{}, errcode.ErrAdminUserEmailAlreadyExists
		}
		return AdminUserRecord{}, err
	}
	if err := s.adminRoleRepo.ReplaceByUserID(ctx, user.UserID, adminRolesByUserRole(role)); err != nil {
		return AdminUserRecord{}, err
	}

	latestUser, err := s.userRepo.GetByUserID(ctx, user.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminUserRecord{}, errcode.ErrAdminUserNotFound
		}
		return AdminUserRecord{}, err
	}

	record := mapAdminUserRecord(
		latestUser,
		role,
		canEditRole(actorRole, role, actorUserID, latestUser.UserID),
	)
	if err := s.recordUserAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleUser,
		Action:     AdminAuditActionCreate,
		TargetType: "user",
		TargetID:   record.UserID,
		Summary:    "user created: " + record.UserID,
		Detail: map[string]any{
			"email":  record.Email,
			"name":   record.Name,
			"status": record.Status,
			"role":   record.Role,
		},
	}); err != nil {
		return AdminUserRecord{}, err
	}

	return record, nil
}

// UpdateUserStatus 更新后台目标用户状态（active/banned）。
func (s *AdminUserService) UpdateUserStatus(
	ctx context.Context,
	input UpdateAdminUserStatusInput,
) (result AdminUserRecord, err error) {
	defer func() {
		err = errcode.MapAdminUserError(err)
	}()

	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return AdminUserRecord{}, err
	}
	actorUserID := strings.TrimSpace(input.ActorUserID)
	actorRole, err := s.resolveUserRole(ctx, actorUserID)
	if err != nil {
		return AdminUserRecord{}, err
	}

	targetUserID := strings.TrimSpace(input.UserID)
	if targetUserID == "" {
		return AdminUserRecord{}, errcode.ErrAdminUserInvalidUserID
	}
	if strings.TrimSpace(input.ActorUserID) == targetUserID {
		return AdminUserRecord{}, errcode.ErrAdminUserSelfOperationBlocked
	}
	if input.Status != models.EntityStatusActive && input.Status != models.EntityStatusBanned {
		return AdminUserRecord{}, errcode.ErrAdminUserInvalidStatus
	}

	reason := strings.TrimSpace(input.Reason)
	if input.Status == models.EntityStatusBanned && reason == "" {
		return AdminUserRecord{}, errcode.ErrAdminUserBanReasonRequired
	}

	targetUser, err := s.userRepo.GetByUserID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminUserRecord{}, errcode.ErrAdminUserNotFound
		}
		return AdminUserRecord{}, err
	}
	if normalizeEntityStatus(targetUser.Status) == models.EntityStatusDeleted || targetUser.DeletedAt != nil {
		return AdminUserRecord{}, errcode.ErrAdminUserAlreadyDeleted
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
		return AdminUserRecord{}, errcode.ErrAdminUserNotFound
	}

	if input.Status == models.EntityStatusBanned {
		if err := s.userSessionRepo.RevokeAllByUserID(ctx, targetUserID, now); err != nil {
			return AdminUserRecord{}, err
		}
	}

	latestUser, err := s.userRepo.GetByUserID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminUserRecord{}, errcode.ErrAdminUserNotFound
		}
		return AdminUserRecord{}, err
	}
	targetRole, err := s.resolveUserRole(ctx, targetUserID)
	if err != nil {
		return AdminUserRecord{}, err
	}
	record := mapAdminUserRecord(
		latestUser,
		targetRole,
		canEditRole(actorRole, targetRole, actorUserID, targetUserID),
	)
	if err := s.recordUserAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleUser,
		Action:     AdminAuditActionUpdate,
		TargetType: "user",
		TargetID:   record.UserID,
		Summary:    "user status updated: " + record.UserID,
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

// UpdateUserRole 更新后台目标用户角色。
func (s *AdminUserService) UpdateUserRole(
	ctx context.Context,
	input UpdateAdminUserRoleInput,
) (result AdminUserRecord, err error) {
	defer func() {
		err = errcode.MapAdminUserError(err)
	}()

	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return AdminUserRecord{}, err
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	targetUserID := strings.TrimSpace(input.UserID)
	if targetUserID == "" {
		return AdminUserRecord{}, errcode.ErrAdminUserInvalidUserID
	}
	if actorUserID == targetUserID {
		return AdminUserRecord{}, errcode.ErrAdminUserSelfOperationBlocked
	}

	nextRole := normalizeAdminUserRole(input.Role)
	if !isValidAdminUserRole(nextRole) {
		return AdminUserRecord{}, errcode.ErrAdminUserInvalidRole
	}

	actorRole, err := s.resolveUserRole(ctx, actorUserID)
	if err != nil {
		return AdminUserRecord{}, err
	}
	if adminUserRoleRank(nextRole) > adminUserRoleRank(actorRole) {
		return AdminUserRecord{}, errcode.ErrAdminUserRoleForbidden
	}

	targetUser, err := s.userRepo.GetByUserID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminUserRecord{}, errcode.ErrAdminUserNotFound
		}
		return AdminUserRecord{}, err
	}
	if normalizeEntityStatus(targetUser.Status) == models.EntityStatusDeleted || targetUser.DeletedAt != nil {
		return AdminUserRecord{}, errcode.ErrAdminUserAlreadyDeleted
	}

	currentRole, err := s.resolveUserRole(ctx, targetUserID)
	if err != nil {
		return AdminUserRecord{}, err
	}
	if adminUserRoleRank(currentRole) > adminUserRoleRank(actorRole) {
		return AdminUserRecord{}, errcode.ErrAdminUserRoleForbidden
	}

	if err := s.adminRoleRepo.ReplaceByUserID(ctx, targetUserID, adminRolesByUserRole(nextRole)); err != nil {
		return AdminUserRecord{}, err
	}

	latestUser, err := s.userRepo.GetByUserID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminUserRecord{}, errcode.ErrAdminUserNotFound
		}
		return AdminUserRecord{}, err
	}

	record := mapAdminUserRecord(
		latestUser,
		nextRole,
		canEditRole(actorRole, nextRole, actorUserID, targetUserID),
	)
	if err := s.recordUserAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleUser,
		Action:     AdminAuditActionUpdate,
		TargetType: "user",
		TargetID:   record.UserID,
		Summary:    "user role updated: " + record.UserID,
		Detail: map[string]any{
			"roleBefore": currentRole,
			"roleAfter":  nextRole,
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
) (err error) {
	defer func() {
		err = errcode.MapAdminUserError(err)
	}()

	_ = requestID

	if err := s.ensurePlatformAdmin(ctx, actorUserID); err != nil {
		return err
	}

	normalizedTargetUserID := strings.TrimSpace(targetUserID)
	if normalizedTargetUserID == "" {
		return errcode.ErrAdminUserInvalidUserID
	}
	if strings.TrimSpace(actorUserID) == normalizedTargetUserID {
		return errcode.ErrAdminUserSelfOperationBlocked
	}

	targetUser, err := s.userRepo.GetByUserID(ctx, normalizedTargetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrAdminUserNotFound
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
		return errcode.ErrAdminUserNotFound
	}
	if err := s.userSessionRepo.RevokeAllByUserID(ctx, normalizedTargetUserID, now); err != nil {
		return err
	}

	if err := s.recordUserAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleUser,
		Action:     AdminAuditActionDelete,
		TargetType: "user",
		TargetID:   normalizedTargetUserID,
		Summary:    "user deleted: " + normalizedTargetUserID,
		Detail: map[string]any{
			"statusBefore": targetUser.Status,
			"statusAfter":  models.EntityStatusDeleted,
		},
	}); err != nil {
		return err
	}

	return nil
}

// SendPasswordResetEmail 后台向目标用户发送密码重置邮件。
func (s *AdminUserService) SendPasswordResetEmail(
	ctx context.Context,
	input SendAdminUserPasswordResetEmailInput,
) (err error) {
	defer func() {
		err = errcode.MapAdminUserError(err)
	}()

	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return err
	}
	if s == nil || s.passwordResetService == nil {
		return errcode.ErrAdminUserPasswordResetUnavailable
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	targetUserID := strings.TrimSpace(input.UserID)
	if targetUserID == "" {
		return errcode.ErrAdminUserInvalidUserID
	}
	if actorUserID == targetUserID {
		return errcode.ErrAdminUserSelfOperationBlocked
	}

	targetUser, err := s.userRepo.GetByUserID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrAdminUserNotFound
		}
		return err
	}
	if normalizeEntityStatus(targetUser.Status) == models.EntityStatusDeleted || targetUser.DeletedAt != nil {
		return errcode.ErrAdminUserAlreadyDeleted
	}

	_, err = s.passwordResetService.RequestByAdmin(ctx, RequestPasswordResetByAdminInput{
		TargetUserID:      targetUserID,
		RequestedByUserID: actorUserID,
		ClientIP:          strings.TrimSpace(input.ClientIP),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrPasswordResetEmailDisabled):
			return errcode.ErrAdminUserPasswordResetUnavailable
		case errors.Is(err, ErrPasswordResetRateLimited):
			return errcode.ErrAdminUserPasswordResetRateLimited
		case errors.Is(err, ErrPasswordResetUserNotSupported):
			return errcode.ErrAdminUserPasswordResetUnsupported
		case errors.Is(err, ErrPasswordResetEmailSendFailed):
			return errcode.ErrAdminUserPasswordResetEmailSendFailed
		default:
			return err
		}
	}

	return s.recordUserAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleUser,
		Action:     AdminAuditActionUpdate,
		TargetType: "user_password_reset_email",
		TargetID:   targetUserID,
		Summary:    "password reset email sent: " + targetUserID,
		Detail: map[string]any{
			"userId": targetUserID,
			"email":  targetUser.Email,
		},
		RequestID: input.RequestID,
	})
}

func (s *AdminUserService) ensurePlatformAdmin(ctx context.Context, actorUserID string) error {
	if s == nil || s.userRepo == nil || s.userSessionRepo == nil || s.adminRoleRepo == nil || s.adminAccessService == nil {
		return errors.New("admin user service dependencies are nil")
	}

	isPlatformAdmin, err := s.adminAccessService.IsPlatformAdmin(ctx, strings.TrimSpace(actorUserID))
	if err != nil {
		return err
	}
	if !isPlatformAdmin {
		return errcode.ErrAdminForbidden
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
		return nil, errcode.ErrAdminUserInvalidStatusFilter
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

func mapAdminUserRecord(user *models.User, role AdminUserRole, canEditRole bool) AdminUserRecord {
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
		Role:         role,
		CanEditRole:  canEditRole,
		Status:       status,
		BannedReason: user.BannedReason,
		BannedAt:     user.BannedAt,
		DeletedAt:    user.DeletedAt,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func normalizeAdminUserRole(value AdminUserRole) AdminUserRole {
	switch strings.ToLower(strings.TrimSpace(string(value))) {
	case string(AdminUserRoleSpaceAdmin):
		return AdminUserRoleSpaceAdmin
	case string(AdminUserRolePlatformAdmin):
		return AdminUserRolePlatformAdmin
	case "", string(AdminUserRoleUser):
		return AdminUserRoleUser
	default:
		return AdminUserRole(strings.ToLower(strings.TrimSpace(string(value))))
	}
}

func isValidAdminUserRole(value AdminUserRole) bool {
	switch normalizeAdminUserRole(value) {
	case AdminUserRoleUser, AdminUserRoleSpaceAdmin, AdminUserRolePlatformAdmin:
		return true
	default:
		return false
	}
}

func resolveAdminUserRole(roles []models.AdminRole) AdminUserRole {
	if len(roles) == 0 {
		return AdminUserRoleUser
	}
	for _, role := range roles {
		if role == models.AdminRolePlatformAdmin {
			return AdminUserRolePlatformAdmin
		}
	}
	for _, role := range roles {
		if role == models.AdminRoleSpaceAdmin {
			return AdminUserRoleSpaceAdmin
		}
	}
	return AdminUserRoleUser
}

func adminRolesByUserRole(role AdminUserRole) []models.AdminRole {
	switch normalizeAdminUserRole(role) {
	case AdminUserRolePlatformAdmin:
		return []models.AdminRole{models.AdminRolePlatformAdmin}
	case AdminUserRoleSpaceAdmin:
		return []models.AdminRole{models.AdminRoleSpaceAdmin}
	default:
		return []models.AdminRole{}
	}
}

func adminUserRoleRank(role AdminUserRole) int {
	switch normalizeAdminUserRole(role) {
	case AdminUserRolePlatformAdmin:
		return 2
	case AdminUserRoleSpaceAdmin:
		return 1
	default:
		return 0
	}
}

func canEditRole(actorRole AdminUserRole, targetRole AdminUserRole, actorUserID string, targetUserID string) bool {
	if strings.TrimSpace(actorUserID) == "" || strings.TrimSpace(targetUserID) == "" {
		return false
	}
	if strings.TrimSpace(actorUserID) == strings.TrimSpace(targetUserID) {
		return false
	}
	return adminUserRoleRank(actorRole) >= adminUserRoleRank(targetRole)
}

func (s *AdminUserService) resolveUserRole(ctx context.Context, userID string) (AdminUserRole, error) {
	roles, err := s.adminRoleRepo.ListByUserID(ctx, strings.TrimSpace(userID))
	if err != nil {
		return AdminUserRoleUser, err
	}
	return resolveAdminUserRole(roles), nil
}
