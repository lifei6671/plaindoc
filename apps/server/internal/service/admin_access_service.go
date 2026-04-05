package service

import (
	"context"
	"errors"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

// AdminAccessService 封装后台访问权限判定。
type AdminAccessService struct {
	adminRoleRepo       repository.AdminRoleRepository
	spaceAdminScopeRepo repository.SpaceAdminScopeRepository
	spaceRepo           repository.SpaceRepository
}

// NewAdminAccessService 创建后台权限服务。
func NewAdminAccessService(
	adminRoleRepo repository.AdminRoleRepository,
	spaceAdminScopeRepo repository.SpaceAdminScopeRepository,
	spaceRepo repository.SpaceRepository,
) *AdminAccessService {
	return &AdminAccessService{
		adminRoleRepo:       adminRoleRepo,
		spaceAdminScopeRepo: spaceAdminScopeRepo,
		spaceRepo:           spaceRepo,
	}
}

// IsPlatformAdmin 判断是否平台管理员。
func (s *AdminAccessService) IsPlatformAdmin(ctx context.Context, userID string) (bool, error) {
	if s == nil || s.adminRoleRepo == nil {
		return false, errors.New("admin access service dependencies are nil")
	}
	return s.adminRoleRepo.HasRole(ctx, userID, models.AdminRolePlatformAdmin)
}

// IsSpaceAdmin 判断是否具备空间管理员角色。
func (s *AdminAccessService) IsSpaceAdmin(ctx context.Context, userID string) (bool, error) {
	if s == nil || s.adminRoleRepo == nil {
		return false, errors.New("admin access service dependencies are nil")
	}
	return s.adminRoleRepo.HasRole(ctx, userID, models.AdminRoleSpaceAdmin)
}

// IsAdmin 判断是否任一管理员。
func (s *AdminAccessService) IsAdmin(ctx context.Context, userID string) (bool, error) {
	platformAdmin, err := s.IsPlatformAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	if platformAdmin {
		return true, nil
	}
	return s.IsSpaceAdmin(ctx, userID)
}

// CanManageSpace 判断用户是否可管理指定空间。
func (s *AdminAccessService) CanManageSpace(ctx context.Context, userID string, spaceID string) (bool, error) {
	if s == nil || s.adminRoleRepo == nil || s.spaceAdminScopeRepo == nil {
		return false, errors.New("admin access service dependencies are nil")
	}
	if userID == "" || spaceID == "" {
		return false, nil
	}

	platformAdmin, err := s.IsPlatformAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	if platformAdmin {
		return true, nil
	}

	spaceAdmin, err := s.IsSpaceAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	if !spaceAdmin {
		return false, nil
	}

	hasScope, err := s.spaceAdminScopeRepo.HasScope(ctx, userID, spaceID)
	if err != nil {
		return false, err
	}
	if hasScope {
		return true, nil
	}

	if s.spaceRepo == nil {
		return false, nil
	}
	space, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if space == nil {
		return false, nil
	}
	return space.OwnerUserID == userID, nil
}

// ListAdminRoles 返回用户管理员角色列表，供菜单与后台入口控制使用。
func (s *AdminAccessService) ListAdminRoles(ctx context.Context, userID string) ([]models.AdminRole, error) {
	if s == nil || s.adminRoleRepo == nil {
		return nil, errors.New("admin access service dependencies are nil")
	}
	return s.adminRoleRepo.ListByUserID(ctx, userID)
}

// HasSpaceMembership 判断用户是否至少加入了一个空间。
//
// 后台普通用户是否能看到“空间管理”入口，不再依赖管理员角色，而是依赖实际成员身份。
func (s *AdminAccessService) HasSpaceMembership(ctx context.Context, userID string) (bool, error) {
	if s == nil || s.spaceRepo == nil {
		return false, errors.New("admin access service dependencies are nil")
	}
	spaces, err := s.spaceRepo.ListByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	return len(spaces) > 0, nil
}
