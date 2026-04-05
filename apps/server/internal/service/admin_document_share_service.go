package service

import (
	"context"
	"errors"
	"strings"

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
	restrictToScopes, err := s.resolveScopeRestriction(ctx, actorUserID)
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
	if err := s.ensureCanManageSpace(ctx, actorUserID, strings.TrimSpace(targetShare.SpaceID)); err != nil {
		return DocumentShareConfig{}, err
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
	if err := s.ensureCanManageSpace(ctx, normalizedActorUserID, strings.TrimSpace(targetShare.SpaceID)); err != nil {
		return err
	}
	return s.documentShareSvc.DisableByShareID(ctx, normalizedShareID, normalizedActorUserID)
}

func (s *AdminDocumentShareService) resolveScopeRestriction(ctx context.Context, actorUserID string) (bool, error) {
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
