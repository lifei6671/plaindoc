package service

import (
	"context"
	"errors"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

// AdminDocumentShareService 封装后台分享中心业务。
type AdminDocumentShareService struct {
	shareRepo          repository.DocumentShareRepository
	documentShareSvc   *DocumentShareService
	adminAccessService *AdminAccessService
}

// NewAdminDocumentShareService 创建后台分享中心服务。
func NewAdminDocumentShareService(
	shareRepo repository.DocumentShareRepository,
	documentShareSvc *DocumentShareService,
	adminAccessService *AdminAccessService,
) *AdminDocumentShareService {
	return &AdminDocumentShareService{
		shareRepo:          shareRepo,
		documentShareSvc:   documentShareSvc,
		adminAccessService: adminAccessService,
	}
}

// ListShares 查询后台分享中心列表。
func (s *AdminDocumentShareService) ListShares(
	ctx context.Context,
	input ListAdminDocumentSharesInput,
) (ListAdminDocumentSharesResult, error) {
	if s == nil || s.documentShareSvc == nil || s.adminAccessService == nil {
		return ListAdminDocumentSharesResult{}, errors.New("admin document share service dependencies are nil")
	}
	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return ListAdminDocumentSharesResult{}, ErrDocumentShareAccessDenied
	}
	restrictToScopes, err := s.resolveScopeRestriction(ctx, actorUserID, input.View)
	if err != nil {
		return ListAdminDocumentSharesResult{}, err
	}
	return s.documentShareSvc.ListForAdmin(ctx, ListAdminDocumentSharesInput{
		ActorUserID:      actorUserID,
		RestrictToScopes: restrictToScopes,
		View:             input.View,
		Keyword:          strings.TrimSpace(input.Keyword),
		SpaceID:          strings.TrimSpace(input.SpaceID),
		Mode:             input.Mode,
		Expired:          strings.TrimSpace(input.Expired),
		Page:             input.Page,
		PageSize:         input.PageSize,
	})
}

// UpdateShare 更新后台分享配置。
func (s *AdminDocumentShareService) UpdateShare(
	ctx context.Context,
	input UpdateDocumentShareByIDInput,
) (DocumentShareConfig, error) {
	if s == nil || s.documentShareSvc == nil || s.shareRepo == nil || s.adminAccessService == nil {
		return DocumentShareConfig{}, errors.New("admin document share service dependencies are nil")
	}
	shareID := strings.TrimSpace(input.ShareID)
	actorUserID := strings.TrimSpace(input.ActorUserID)
	if shareID == "" || actorUserID == "" {
		return DocumentShareConfig{}, ErrDocumentShareNotFound
	}

	targetShare, err := s.shareRepo.GetByShareID(ctx, shareID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DocumentShareConfig{}, ErrDocumentShareNotFound
		}
		return DocumentShareConfig{}, err
	}
	ok, err := s.canManageShare(ctx, actorUserID, targetShare)
	if err != nil {
		return DocumentShareConfig{}, err
	}
	if !ok {
		return DocumentShareConfig{}, ErrDocumentShareAccessDenied
	}
	return s.documentShareSvc.UpdateByShareID(ctx, input)
}

// DisableShare 取消后台分享。
func (s *AdminDocumentShareService) DisableShare(
	ctx context.Context,
	shareID string,
	actorUserID string,
) error {
	if s == nil || s.documentShareSvc == nil || s.shareRepo == nil || s.adminAccessService == nil {
		return errors.New("admin document share service dependencies are nil")
	}
	normalizedShareID := strings.TrimSpace(shareID)
	normalizedActorUserID := strings.TrimSpace(actorUserID)
	if normalizedShareID == "" || normalizedActorUserID == "" {
		return ErrDocumentShareNotFound
	}
	targetShare, err := s.shareRepo.GetByShareID(ctx, normalizedShareID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDocumentShareNotFound
		}
		return err
	}
	ok, err := s.canManageShare(ctx, normalizedActorUserID, targetShare)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDocumentShareAccessDenied
	}
	return s.documentShareSvc.DisableByShareID(ctx, normalizedShareID, normalizedActorUserID)
}

// 普通登录用户只允许查看“我的分享”，查看全部分享仍然保留给管理员。
func (s *AdminDocumentShareService) resolveScopeRestriction(
	ctx context.Context,
	actorUserID string,
	view repository.DocumentShareAdminView,
) (bool, error) {
	isPlatformAdmin, err := s.adminAccessService.IsPlatformAdmin(ctx, actorUserID)
	if err != nil {
		return false, err
	}
	if isPlatformAdmin {
		return false, nil
	}
	isSpaceAdmin, err := s.adminAccessService.IsSpaceAdmin(ctx, actorUserID)
	if err != nil {
		return false, err
	}
	if !isSpaceAdmin {
		if view == repository.DocumentShareAdminViewMine {
			return false, nil
		}
		return false, ErrDocumentShareAccessDenied
	}
	return true, nil
}

func (s *AdminDocumentShareService) ensureCanManageSpace(ctx context.Context, actorUserID string, spaceID string) error {
	canManage, err := s.adminAccessService.CanManageSpace(ctx, actorUserID, spaceID)
	if err != nil {
		return err
	}
	if !canManage {
		return ErrDocumentShareAccessDenied
	}
	return nil
}

// 自助分享优先允许创建者本人操作；非创建者再走管理员的空间治理能力。
func (s *AdminDocumentShareService) canManageShare(
	ctx context.Context,
	actorUserID string,
	targetShare *models.DocumentShare,
) (bool, error) {
	if targetShare == nil {
		return false, ErrDocumentShareNotFound
	}
	if targetShare.CreatedByUserID == nil {
		return false, nil
	}
	if strings.TrimSpace(*targetShare.CreatedByUserID) == strings.TrimSpace(actorUserID) {
		return true, nil
	}
	spaceManaged, err := s.adminAccessService.CanManageSpace(ctx, actorUserID, strings.TrimSpace(targetShare.SpaceID))
	if err != nil {
		return false, err
	}
	return spaceManaged, nil
}
