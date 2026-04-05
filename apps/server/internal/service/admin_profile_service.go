package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	maxAdminProfileNameLength      = 120
	maxAdminProfileAvatarURLLength = 1024
	minAdminProfilePasswordLength  = 6
)

// AdminProfileRecord 管理后台个人信息视图。
type AdminProfileRecord struct {
	UserID    string
	Email     string
	Name      string
	AvatarURL string
	Roles     []models.AdminRole
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UpdateAdminProfileInput 更新个人信息入参。
type UpdateAdminProfileInput struct {
	ActorUserID string
	RequestID   string
	Name        *string
	AvatarURL   *string
}

// UpdateAdminPasswordInput 更新个人密码入参。
type UpdateAdminPasswordInput struct {
	ActorUserID     string
	RequestID       string
	CurrentPassword string
	NewPassword     string
	ConfirmPassword string
}

// AdminProfileService 处理管理后台个人资料相关业务。
type AdminProfileService struct {
	userRepo           repository.UserRepository
	adminAccessService *AdminAccessService
	adminAuditService  *AdminAuditService
}

// NewAdminProfileService 创建个人资料服务。
func NewAdminProfileService(
	userRepo repository.UserRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
) *AdminProfileService {
	return &AdminProfileService{
		userRepo:           userRepo,
		adminAccessService: adminAccessService,
		adminAuditService:  adminAuditService,
	}
}

// GetProfile 返回当前后台用户个人资料。
func (s *AdminProfileService) GetProfile(
	ctx context.Context,
	actorUserID string,
) (result AdminProfileRecord, err error) {
	defer func() {
		err = errcode.MapAdminProfileError(err)
	}()

	actorUserID = strings.TrimSpace(actorUserID)
	user, roles, err := s.loadActorUser(ctx, actorUserID)
	if err != nil {
		return AdminProfileRecord{}, err
	}
	return mapAdminProfileRecord(user, roles), nil
}

// UpdateProfile 更新当前后台用户昵称和头像地址。
func (s *AdminProfileService) UpdateProfile(
	ctx context.Context,
	input UpdateAdminProfileInput,
) (result AdminProfileRecord, err error) {
	defer func() {
		err = errcode.MapAdminProfileError(err)
	}()

	actorUserID := strings.TrimSpace(input.ActorUserID)
	user, roles, err := s.loadActorUser(ctx, actorUserID)
	if err != nil {
		return AdminProfileRecord{}, err
	}

	updateName := (*string)(nil)
	updateAvatarURL := (*string)(nil)
	auditDetail := map[string]any{}

	if input.Name != nil {
		normalizedName := strings.TrimSpace(*input.Name)
		if normalizedName == "" || len(normalizedName) > maxAdminProfileNameLength {
			return AdminProfileRecord{}, errcode.ErrAdminProfileInvalidName
		}
		if normalizedName != strings.TrimSpace(user.Name) {
			updateName = &normalizedName
			auditDetail["name"] = normalizedName
		}
	}

	if input.AvatarURL != nil {
		normalizedAvatarURL, normalizeErr := normalizeAdminProfileAvatarURL(*input.AvatarURL)
		if normalizeErr != nil {
			return AdminProfileRecord{}, normalizeErr
		}
		if normalizedAvatarURL != strings.TrimSpace(user.AvatarURL) {
			updateAvatarURL = &normalizedAvatarURL
			auditDetail["avatarUrl"] = normalizedAvatarURL
		}
	}

	if updateName == nil && updateAvatarURL == nil {
		return mapAdminProfileRecord(user, roles), nil
	}

	now := time.Now().UTC()
	updated, err := s.userRepo.UpdateProfile(ctx, repository.UpdateUserProfileParams{
		UserID:    actorUserID,
		Name:      updateName,
		AvatarURL: updateAvatarURL,
		UpdatedAt: now,
	})
	if err != nil {
		return AdminProfileRecord{}, err
	}
	if !updated {
		return AdminProfileRecord{}, errcode.ErrAdminProfileNotFound
	}

	latestUser, err := s.userRepo.GetByUserID(ctx, actorUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminProfileRecord{}, errcode.ErrAdminProfileNotFound
		}
		return AdminProfileRecord{}, err
	}

	if err := s.recordProfileAudit(ctx, RecordAdminAuditInput{
		ActorUserID: actorUserID,
		RequestID:   input.RequestID,
		Module:      AdminAuditModuleUser,
		Action:      AdminAuditActionUpdate,
		TargetType:  "profile",
		TargetID:    actorUserID,
		Summary:     "profile updated: " + actorUserID,
		Detail:      auditDetail,
	}); err != nil {
		return AdminProfileRecord{}, err
	}

	return mapAdminProfileRecord(latestUser, roles), nil
}

// UpdatePassword 更新当前后台用户密码。
func (s *AdminProfileService) UpdatePassword(
	ctx context.Context,
	input UpdateAdminPasswordInput,
) (err error) {
	defer func() {
		err = errcode.MapAdminProfileError(err)
	}()

	actorUserID := strings.TrimSpace(input.ActorUserID)
	user, _, err := s.loadActorUser(ctx, actorUserID)
	if err != nil {
		return err
	}

	currentPassword := input.CurrentPassword
	newPassword := input.NewPassword
	if strings.TrimSpace(currentPassword) == "" {
		return errcode.ErrAdminProfileCurrentPasswordInvalid
	}
	if len(newPassword) < minAdminProfilePasswordLength {
		return errcode.ErrAdminProfilePasswordTooShort
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(newPassword)) == nil {
		return errcode.ErrAdminProfilePasswordUnchanged
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		return errcode.ErrAdminProfileCurrentPasswordInvalid
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	updated, err := s.userRepo.UpdatePassword(ctx, actorUserID, string(passwordHash), time.Now().UTC())
	if err != nil {
		return err
	}
	if !updated {
		return errcode.ErrAdminProfileNotFound
	}

	return s.recordProfileAudit(ctx, RecordAdminAuditInput{
		ActorUserID: actorUserID,
		RequestID:   input.RequestID,
		Module:      AdminAuditModuleUser,
		Action:      AdminAuditActionUpdate,
		TargetType:  "profile_password",
		TargetID:    actorUserID,
		Summary:     "profile password updated: " + actorUserID,
		Detail: map[string]any{
			"passwordUpdated": true,
		},
	})
}

func (s *AdminProfileService) loadActorUser(
	ctx context.Context,
	actorUserID string,
) (*models.User, []models.AdminRole, error) {
	if s == nil || s.userRepo == nil || s.adminAccessService == nil {
		return nil, nil, errors.New("admin profile service dependencies are nil")
	}
	if actorUserID == "" {
		return nil, nil, errcode.ErrAdminForbidden
	}

	user, err := s.userRepo.GetByUserID(ctx, actorUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errcode.ErrAdminProfileNotFound
		}
		return nil, nil, err
	}

	roles, err := s.adminAccessService.ListAdminRoles(ctx, actorUserID)
	if err != nil {
		return nil, nil, err
	}
	return user, roles, nil
}

func (s *AdminProfileService) recordProfileAudit(ctx context.Context, input RecordAdminAuditInput) error {
	if s == nil || s.adminAuditService == nil {
		return nil
	}
	return s.adminAuditService.Record(ctx, input)
}

func normalizeAdminProfileAvatarURL(rawValue string) (string, error) {
	normalizedValue := strings.TrimSpace(rawValue)
	if normalizedValue == "" {
		return "", nil
	}
	if len(normalizedValue) > maxAdminProfileAvatarURLLength {
		return "", errcode.ErrAdminProfileInvalidAvatarURL
	}
	if strings.HasPrefix(normalizedValue, "/") {
		if strings.HasPrefix(normalizedValue, "/api/uploads/") {
			return strings.TrimPrefix(normalizedValue, "/api"), nil
		}
		if strings.HasPrefix(normalizedValue, "/uploads/") {
			return normalizedValue, nil
		}
		return "", errcode.ErrAdminProfileInvalidAvatarURL
	}

	parsedURL, err := url.Parse(normalizedValue)
	if err != nil {
		return "", errcode.ErrAdminProfileInvalidAvatarURL
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", errcode.ErrAdminProfileInvalidAvatarURL
	}
	if strings.TrimSpace(parsedURL.Host) == "" {
		return "", errcode.ErrAdminProfileInvalidAvatarURL
	}
	return normalizedValue, nil
}

func mapAdminProfileRecord(user *models.User, roles []models.AdminRole) AdminProfileRecord {
	if user == nil {
		return AdminProfileRecord{}
	}
	return AdminProfileRecord{
		UserID:    strings.TrimSpace(user.UserID),
		Email:     strings.TrimSpace(user.Email),
		Name:      strings.TrimSpace(user.Name),
		AvatarURL: strings.TrimSpace(user.AvatarURL),
		Roles:     append([]models.AdminRole(nil), roles...),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
