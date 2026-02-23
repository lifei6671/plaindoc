package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

var (
	ErrInvalidVisibilityValue = errors.New("invalid visibility value")
	ErrViewerLoginRequired    = errors.New("viewer login required")
	ErrSpaceAccessDenied      = errors.New("space access denied")
	ErrDocumentAccessDenied   = errors.New("document access denied")
	ErrSpaceNotFound          = errors.New("space not found")
	ErrDocumentNotFound       = errors.New("document not found")
)

// VisibilityService 编排空间/文档公开级别的读取与变更权限判断。
type VisibilityService struct {
	spaceRepo    repository.SpaceRepository
	documentRepo repository.DocumentRepository
}

// NewVisibilityService 创建可见性业务服务。
func NewVisibilityService(
	spaceRepo repository.SpaceRepository,
	documentRepo repository.DocumentRepository,
) *VisibilityService {
	return &VisibilityService{
		spaceRepo:    spaceRepo,
		documentRepo: documentRepo,
	}
}

// GetSpace 按空间可见性规则返回空间信息。
func (s *VisibilityService) GetSpace(
	ctx context.Context,
	spaceID string,
	viewerUserID string,
) (*models.Space, error) {
	if s == nil || s.spaceRepo == nil {
		return nil, errors.New("visibility service dependencies are nil")
	}

	space, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSpaceNotFound
		}
		return nil, err
	}
	if err := EnsureEntityActive(space.Status, space.BannedAt, space.DeletedAt); err != nil {
		return nil, ErrSpaceAccessDenied
	}

	spaceVisibility := normalizeVisibility(space.Visibility)
	if err := s.authorizeRead(ctx, space.SpaceID, spaceVisibility, viewerUserID); err != nil {
		switch {
		case errors.Is(err, ErrViewerLoginRequired):
			return nil, ErrViewerLoginRequired
		case errors.Is(err, ErrSpaceAccessDenied):
			return nil, ErrSpaceAccessDenied
		default:
			return nil, err
		}
	}

	space.Visibility = spaceVisibility
	return space, nil
}

// UpdateSpaceVisibility 仅允许空间 owner 修改空间可见性。
func (s *VisibilityService) UpdateSpaceVisibility(
	ctx context.Context,
	spaceID string,
	actorUserID string,
	visibility models.Visibility,
) (*models.Space, error) {
	if s == nil || s.spaceRepo == nil {
		return nil, errors.New("visibility service dependencies are nil")
	}
	if actorUserID == "" {
		return nil, ErrViewerLoginRequired
	}
	if !models.IsValidVisibility(visibility) {
		return nil, ErrInvalidVisibilityValue
	}

	space, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSpaceNotFound
		}
		return nil, err
	}
	if space.OwnerUserID != actorUserID {
		return nil, ErrSpaceAccessDenied
	}
	if err := EnsureEntityActive(space.Status, space.BannedAt, space.DeletedAt); err != nil {
		return nil, ErrSpaceAccessDenied
	}

	updated, err := s.spaceRepo.UpdateVisibility(ctx, spaceID, visibility)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSpaceNotFound
		}
		return nil, err
	}
	updated.Visibility = normalizeVisibility(updated.Visibility)
	return updated, nil
}

// GetDocument 按空间与文档综合可见性规则返回文档信息。
func (s *VisibilityService) GetDocument(
	ctx context.Context,
	documentID string,
	viewerUserID string,
) (*models.Document, error) {
	if s == nil || s.spaceRepo == nil || s.documentRepo == nil {
		return nil, errors.New("visibility service dependencies are nil")
	}

	documentAccess, err := s.documentRepo.GetAccessByDocumentID(ctx, documentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}
	if err := EnsureEntityActive(
		documentAccess.SpaceStatus,
		documentAccess.SpaceBannedAt,
		documentAccess.SpaceDeletedAt,
	); err != nil {
		return nil, ErrDocumentAccessDenied
	}
	if err := EnsureEntityActive(
		documentAccess.Document.Status,
		documentAccess.Document.BannedAt,
		documentAccess.Document.DeletedAt,
	); err != nil {
		return nil, ErrDocumentAccessDenied
	}

	// 空间 owner 恒具备读权限：直接短路返回，避免 member 可见性下的额外成员查询。
	trimmedViewerUserID := strings.TrimSpace(viewerUserID)
	if trimmedViewerUserID != "" && trimmedViewerUserID == strings.TrimSpace(documentAccess.SpaceOwnerUserID) {
		documentAccess.Document.Visibility = normalizeVisibility(documentAccess.Document.Visibility)
		return &documentAccess.Document, nil
	}

	effectiveVisibility := stricterVisibility(
		normalizeVisibility(documentAccess.SpaceVisibility),
		normalizeVisibility(documentAccess.Document.Visibility),
	)
	if err := s.authorizeRead(ctx, documentAccess.SpaceID, effectiveVisibility, trimmedViewerUserID); err != nil {
		switch {
		case errors.Is(err, ErrViewerLoginRequired):
			return nil, ErrViewerLoginRequired
		case errors.Is(err, ErrSpaceAccessDenied):
			return nil, ErrDocumentAccessDenied
		default:
			return nil, err
		}
	}

	documentAccess.Document.Visibility = normalizeVisibility(documentAccess.Document.Visibility)
	return &documentAccess.Document, nil
}

// UpdateDocumentVisibility 允许空间 owner / collaborator 修改文档可见性。
func (s *VisibilityService) UpdateDocumentVisibility(
	ctx context.Context,
	documentID string,
	actorUserID string,
	visibility models.Visibility,
) (*models.Document, error) {
	if s == nil || s.spaceRepo == nil || s.documentRepo == nil {
		return nil, errors.New("visibility service dependencies are nil")
	}
	if actorUserID == "" {
		return nil, ErrViewerLoginRequired
	}
	if !models.IsValidVisibility(visibility) {
		return nil, ErrInvalidVisibilityValue
	}

	documentAccess, err := s.documentRepo.GetAccessByDocumentID(ctx, documentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}
	if err := EnsureEntityActive(
		documentAccess.SpaceStatus,
		documentAccess.SpaceBannedAt,
		documentAccess.SpaceDeletedAt,
	); err != nil {
		return nil, ErrDocumentAccessDenied
	}
	if strings.TrimSpace(documentAccess.SpaceOwnerUserID) != strings.TrimSpace(actorUserID) {
		hasWriterAccess, accessErr := s.spaceRepo.HasWriterAccess(ctx, documentAccess.SpaceID, actorUserID)
		if accessErr != nil {
			return nil, accessErr
		}
		if !hasWriterAccess {
			return nil, ErrDocumentAccessDenied
		}
	}

	updated, err := s.documentRepo.UpdateVisibility(ctx, documentID, visibility)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}
	updated.Visibility = normalizeVisibility(updated.Visibility)
	return updated, nil
}

func (s *VisibilityService) authorizeRead(
	ctx context.Context,
	spaceID string,
	visibility models.Visibility,
	viewerUserID string,
) error {
	switch visibility {
	case models.VisibilityPublic:
		return nil
	case models.VisibilityAuthenticated:
		if viewerUserID == "" {
			return ErrViewerLoginRequired
		}
		return nil
	case models.VisibilityMember:
		if viewerUserID == "" {
			return ErrViewerLoginRequired
		}
		member, err := s.spaceRepo.HasReaderAccess(ctx, spaceID, viewerUserID)
		if err != nil {
			return err
		}
		if !member {
			return ErrSpaceAccessDenied
		}
		return nil
	default:
		return fmt.Errorf("unsupported visibility %q", visibility)
	}
}

func normalizeVisibility(value models.Visibility) models.Visibility {
	if models.IsValidVisibility(value) {
		return value
	}
	return models.VisibilityMember
}

func stricterVisibility(left models.Visibility, right models.Visibility) models.Visibility {
	if visibilityRank(left) >= visibilityRank(right) {
		return left
	}
	return right
}

func visibilityRank(value models.Visibility) int {
	switch value {
	case models.VisibilityPublic:
		return 1
	case models.VisibilityAuthenticated:
		return 2
	case models.VisibilityMember:
		return 3
	default:
		return 3
	}
}
