package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

func TestAdminDocumentImageAssetService_ListImageAssets_RestrictsByRole(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		actorUserID        string
		rolesByUser        map[string][]models.AdminRole
		expectError        bool
		expectRestrictFlag bool
	}

	cases := []testCase{
		{
			name:               "platform admin should not restrict scopes",
			actorUserID:        "platform-user",
			rolesByUser:        map[string][]models.AdminRole{"platform-user": {models.AdminRolePlatformAdmin}},
			expectError:        false,
			expectRestrictFlag: false,
		},
		{
			name:               "space admin should restrict scopes",
			actorUserID:        "space-admin-user",
			rolesByUser:        map[string][]models.AdminRole{"space-admin-user": {models.AdminRoleSpaceAdmin}},
			expectError:        false,
			expectRestrictFlag: true,
		},
		{
			name:        "non admin should be forbidden",
			actorUserID: "normal-user",
			rolesByUser: map[string][]models.AdminRole{},
			expectError: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			imageRepo := &stubDocumentImageAssetRepo{
				listRecords: []repository.AdminDocumentImageAssetListRecord{},
				listTotal:   0,
			}
			svc := NewAdminDocumentImageAssetService(
				imageRepo,
				newTestAdminAccessService(tc.rolesByUser, map[string][]string{}),
				nil,
				nil,
			)

			_, err := svc.ListImageAssets(context.Background(), ListAdminDocumentImageAssetsInput{
				ActorUserID: tc.actorUserID,
				Page:        1,
				PageSize:    20,
			})

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, errcode.ErrAdminForbidden) {
					t.Fatalf("expected admin forbidden error, got: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("list image assets failed: %v", err)
			}
			if len(imageRepo.listParams) != 1 {
				t.Fatalf("expected list called once, got %d", len(imageRepo.listParams))
			}
			got := imageRepo.listParams[0].RestrictToScopes
			if got != tc.expectRestrictFlag {
				t.Fatalf("unexpected restrict flag: got=%v want=%v", got, tc.expectRestrictFlag)
			}
		})
	}
}

func TestAdminDocumentImageAssetService_DeleteImageAsset_EnforcesSpaceScope(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	const imageAssetID = "img-1"
	const targetSpaceID = "space-a"

	newService := func(scopes map[string][]string) (*AdminDocumentImageAssetService, *stubDocumentImageAssetRepo) {
		repo := &stubDocumentImageAssetRepo{
			imageAssetsByID: map[string]*models.DocumentImageAsset{
				imageAssetID: {
					ImageAssetID:     imageAssetID,
					DocumentID:       "doc-1",
					SpaceID:          targetSpaceID,
					StorageProvider:  "local",
					ObjectKey:        "images/space-a/doc-1/example.png",
					ObjectURL:        "/uploads/images/space-a/doc-1/example.png",
					Status:           "active",
					LastReferencedAt: now,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
			},
		}

		svc := NewAdminDocumentImageAssetService(
			repo,
			newTestAdminAccessService(
				map[string][]models.AdminRole{"space-admin-user": {models.AdminRoleSpaceAdmin}},
				scopes,
			),
			nil,
			nil,
		)
		return svc, repo
	}

	t.Run("space admin without scope should be forbidden", func(t *testing.T) {
		t.Parallel()
		svc, repo := newService(map[string][]string{})

		_, err := svc.DeleteImageAsset(context.Background(), DeleteAdminDocumentImageAssetInput{
			ActorUserID:    "space-admin-user",
			ImageAssetID:   imageAssetID,
			PhysicalDelete: false,
		})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !errors.Is(err, errcode.ErrAdminForbidden) {
			t.Fatalf("expected admin forbidden error, got: %v", err)
		}
		if len(repo.softDeletedIDs) != 0 {
			t.Fatalf("soft delete should not be called for forbidden actor")
		}
	})

	t.Run("space admin with scope should allow soft delete", func(t *testing.T) {
		t.Parallel()
		svc, repo := newService(map[string][]string{
			"space-admin-user": {targetSpaceID},
		})

		result, err := svc.DeleteImageAsset(context.Background(), DeleteAdminDocumentImageAssetInput{
			ActorUserID:    "space-admin-user",
			ImageAssetID:   imageAssetID,
			PhysicalDelete: false,
		})
		if err != nil {
			t.Fatalf("delete image asset failed: %v", err)
		}
		if !result.SoftDeleted {
			t.Fatalf("expected soft deleted to be true")
		}
		if len(repo.softDeletedIDs) != 1 || repo.softDeletedIDs[0] != imageAssetID {
			t.Fatalf("unexpected soft delete calls: %+v", repo.softDeletedIDs)
		}
	})
}

func TestAdminDocumentAttachmentService_ListAttachments_RestrictsByRole(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		actorUserID        string
		rolesByUser        map[string][]models.AdminRole
		expectError        bool
		expectRestrictFlag bool
	}

	cases := []testCase{
		{
			name:               "platform admin should not restrict scopes",
			actorUserID:        "platform-user",
			rolesByUser:        map[string][]models.AdminRole{"platform-user": {models.AdminRolePlatformAdmin}},
			expectError:        false,
			expectRestrictFlag: false,
		},
		{
			name:               "space admin should restrict scopes",
			actorUserID:        "space-admin-user",
			rolesByUser:        map[string][]models.AdminRole{"space-admin-user": {models.AdminRoleSpaceAdmin}},
			expectError:        false,
			expectRestrictFlag: true,
		},
		{
			name:        "non admin should be forbidden",
			actorUserID: "normal-user",
			rolesByUser: map[string][]models.AdminRole{},
			expectError: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			attachmentRepo := &stubDocumentAttachmentRepo{
				listRecords: []repository.AdminDocumentAttachmentListRecord{},
				listTotal:   0,
			}
			svc := NewAdminDocumentAttachmentService(
				attachmentRepo,
				newTestAdminAccessService(tc.rolesByUser, map[string][]string{}),
				nil,
				nil,
			)

			_, err := svc.ListAttachments(context.Background(), ListAdminDocumentAttachmentsInput{
				ActorUserID: tc.actorUserID,
				Page:        1,
				PageSize:    20,
			})

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, errcode.ErrAdminForbidden) {
					t.Fatalf("expected admin forbidden error, got: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("list attachments failed: %v", err)
			}
			if len(attachmentRepo.listParams) != 1 {
				t.Fatalf("expected list called once, got %d", len(attachmentRepo.listParams))
			}
			got := attachmentRepo.listParams[0].RestrictToScopes
			if got != tc.expectRestrictFlag {
				t.Fatalf("unexpected restrict flag: got=%v want=%v", got, tc.expectRestrictFlag)
			}
		})
	}
}

func TestAdminDocumentAttachmentService_DeleteAttachment_EnforcesSpaceScope(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	const attachmentID = "att-1"
	const targetSpaceID = "space-a"

	newService := func(scopes map[string][]string) (*AdminDocumentAttachmentService, *stubDocumentAttachmentRepo) {
		repo := &stubDocumentAttachmentRepo{
			attachmentsByID: map[string]*models.DocumentAttachment{
				attachmentID: {
					AttachmentID:    attachmentID,
					DocumentID:      "doc-1",
					SpaceID:         targetSpaceID,
					FileName:        "manual.pdf",
					MimeType:        "application/pdf",
					SizeBytes:       1024,
					StorageProvider: "local",
					ObjectKey:       "attachments/space-a/doc-1/manual.pdf",
					ObjectURL:       "/uploads/attachments/space-a/doc-1/manual.pdf",
					Status:          models.EntityStatusActive,
					CreatedAt:       now,
					UpdatedAt:       now,
				},
			},
		}
		svc := NewAdminDocumentAttachmentService(
			repo,
			newTestAdminAccessService(
				map[string][]models.AdminRole{"space-admin-user": {models.AdminRoleSpaceAdmin}},
				scopes,
			),
			nil,
			nil,
		)
		return svc, repo
	}

	t.Run("space admin without scope should be forbidden", func(t *testing.T) {
		t.Parallel()
		svc, repo := newService(map[string][]string{})

		_, err := svc.DeleteAttachment(context.Background(), DeleteAdminDocumentAttachmentInput{
			ActorUserID:    "space-admin-user",
			AttachmentID:   attachmentID,
			PhysicalDelete: false,
		})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !errors.Is(err, errcode.ErrAdminForbidden) {
			t.Fatalf("expected admin forbidden error, got: %v", err)
		}
		if len(repo.softDeletedIDs) != 0 {
			t.Fatalf("soft delete should not be called for forbidden actor")
		}
	})

	t.Run("space admin with scope should allow soft delete", func(t *testing.T) {
		t.Parallel()
		svc, repo := newService(map[string][]string{
			"space-admin-user": {targetSpaceID},
		})

		result, err := svc.DeleteAttachment(context.Background(), DeleteAdminDocumentAttachmentInput{
			ActorUserID:    "space-admin-user",
			AttachmentID:   attachmentID,
			PhysicalDelete: false,
		})
		if err != nil {
			t.Fatalf("delete attachment failed: %v", err)
		}
		if !result.SoftDeleted {
			t.Fatalf("expected soft deleted to be true")
		}
		if len(repo.softDeletedIDs) != 1 || repo.softDeletedIDs[0] != attachmentID {
			t.Fatalf("unexpected soft delete calls: %+v", repo.softDeletedIDs)
		}
	})
}

type stubAdminRoleRepo struct {
	rolesByUser map[string]map[models.AdminRole]struct{}
}

func newStubAdminRoleRepo(input map[string][]models.AdminRole) *stubAdminRoleRepo {
	rolesByUser := make(map[string]map[models.AdminRole]struct{}, len(input))
	for userID, roles := range input {
		roleSet := make(map[models.AdminRole]struct{}, len(roles))
		for _, role := range roles {
			roleSet[role] = struct{}{}
		}
		rolesByUser[userID] = roleSet
	}
	return &stubAdminRoleRepo{rolesByUser: rolesByUser}
}

func (s *stubAdminRoleRepo) HasRole(_ context.Context, userID string, role models.AdminRole) (bool, error) {
	if s == nil {
		return false, nil
	}
	roleSet, ok := s.rolesByUser[userID]
	if !ok {
		return false, nil
	}
	_, exists := roleSet[role]
	return exists, nil
}

func (s *stubAdminRoleRepo) ListByUserID(_ context.Context, userID string) ([]models.AdminRole, error) {
	if s == nil {
		return nil, nil
	}
	roleSet, ok := s.rolesByUser[userID]
	if !ok {
		return []models.AdminRole{}, nil
	}
	result := make([]models.AdminRole, 0, len(roleSet))
	for role := range roleSet {
		result = append(result, role)
	}
	return result, nil
}

func (s *stubAdminRoleRepo) ListByUserIDs(_ context.Context, userIDs []string) (map[string][]models.AdminRole, error) {
	result := make(map[string][]models.AdminRole, len(userIDs))
	for _, userID := range userIDs {
		roles, err := s.ListByUserID(context.Background(), userID)
		if err != nil {
			return nil, err
		}
		result[userID] = roles
	}
	return result, nil
}

func (s *stubAdminRoleRepo) ReplaceByUserID(_ context.Context, userID string, roles []models.AdminRole) error {
	if s == nil {
		return nil
	}
	roleSet := make(map[models.AdminRole]struct{}, len(roles))
	for _, role := range roles {
		roleSet[role] = struct{}{}
	}
	s.rolesByUser[userID] = roleSet
	return nil
}

type stubSpaceAdminScopeRepo struct {
	scopesByUser map[string]map[string]struct{}
}

func newStubSpaceAdminScopeRepo(input map[string][]string) *stubSpaceAdminScopeRepo {
	scopesByUser := make(map[string]map[string]struct{}, len(input))
	for userID, scopes := range input {
		scopeSet := make(map[string]struct{}, len(scopes))
		for _, scope := range scopes {
			scopeSet[scope] = struct{}{}
		}
		scopesByUser[userID] = scopeSet
	}
	return &stubSpaceAdminScopeRepo{scopesByUser: scopesByUser}
}

func (s *stubSpaceAdminScopeRepo) HasScope(_ context.Context, userID string, spaceID string) (bool, error) {
	if s == nil {
		return false, nil
	}
	scopes, ok := s.scopesByUser[userID]
	if !ok {
		return false, nil
	}
	_, exists := scopes[spaceID]
	return exists, nil
}

func (s *stubSpaceAdminScopeRepo) UpsertScope(_ context.Context, userID string, spaceID string) error {
	if s == nil {
		return nil
	}
	scopes, ok := s.scopesByUser[userID]
	if !ok {
		scopes = make(map[string]struct{})
		s.scopesByUser[userID] = scopes
	}
	scopes[spaceID] = struct{}{}
	return nil
}

func (s *stubSpaceAdminScopeRepo) DeleteScope(_ context.Context, userID string, spaceID string) error {
	if s == nil {
		return nil
	}
	scopes, ok := s.scopesByUser[userID]
	if !ok {
		return nil
	}
	delete(scopes, spaceID)
	return nil
}

func (s *stubSpaceAdminScopeRepo) ListByUserID(_ context.Context, userID string) ([]string, error) {
	if s == nil {
		return []string{}, nil
	}
	scopes, ok := s.scopesByUser[userID]
	if !ok {
		return []string{}, nil
	}
	result := make([]string, 0, len(scopes))
	for scopeID := range scopes {
		result = append(result, scopeID)
	}
	return result, nil
}

func newTestAdminAccessService(
	rolesByUser map[string][]models.AdminRole,
	scopesByUser map[string][]string,
) *AdminAccessService {
	return NewAdminAccessService(
		newStubAdminRoleRepo(rolesByUser),
		newStubSpaceAdminScopeRepo(scopesByUser),
		nil,
	)
}

type stubDocumentImageAssetRepo struct {
	listParams      []repository.ListAdminDocumentImageAssetsParams
	listRecords     []repository.AdminDocumentImageAssetListRecord
	listTotal       int64
	imageAssetsByID map[string]*models.DocumentImageAsset
	softDeletedIDs  []string
}

func (s *stubDocumentImageAssetRepo) ListForAdmin(
	_ context.Context,
	params repository.ListAdminDocumentImageAssetsParams,
) ([]repository.AdminDocumentImageAssetListRecord, int64, error) {
	s.listParams = append(s.listParams, params)
	return s.listRecords, s.listTotal, nil
}

func (s *stubDocumentImageAssetRepo) GetByImageAssetID(
	_ context.Context,
	imageAssetID string,
) (*models.DocumentImageAsset, error) {
	asset, ok := s.imageAssetsByID[imageAssetID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copyValue := *asset
	return &copyValue, nil
}

func (s *stubDocumentImageAssetRepo) SoftDelete(
	_ context.Context,
	imageAssetID string,
	_ time.Time,
) (bool, error) {
	s.softDeletedIDs = append(s.softDeletedIDs, imageAssetID)
	return true, nil
}

func (s *stubDocumentImageAssetRepo) HardDelete(
	_ context.Context,
	_ string,
) (bool, error) {
	return true, nil
}

func (s *stubDocumentImageAssetRepo) CountActiveReferencesByObject(
	_ context.Context,
	_ string,
	_ string,
) (int64, error) {
	return 0, nil
}

func (s *stubDocumentImageAssetRepo) ListActiveReferencesByObject(
	_ context.Context,
	_ string,
	_ string,
	_ int,
) ([]repository.DocumentImageAssetReferenceRecord, error) {
	return []repository.DocumentImageAssetReferenceRecord{}, nil
}

type stubDocumentAttachmentRepo struct {
	listParams      []repository.ListAdminDocumentAttachmentsParams
	listRecords     []repository.AdminDocumentAttachmentListRecord
	listTotal       int64
	attachmentsByID map[string]*models.DocumentAttachment
	softDeletedIDs  []string
}

func (s *stubDocumentAttachmentRepo) Create(_ context.Context, _ *models.DocumentAttachment) error {
	return nil
}

func (s *stubDocumentAttachmentRepo) ListByDocumentID(
	_ context.Context,
	_ string,
	_ bool,
) ([]models.DocumentAttachment, error) {
	return []models.DocumentAttachment{}, nil
}

func (s *stubDocumentAttachmentRepo) ListForAdmin(
	_ context.Context,
	params repository.ListAdminDocumentAttachmentsParams,
) ([]repository.AdminDocumentAttachmentListRecord, int64, error) {
	s.listParams = append(s.listParams, params)
	return s.listRecords, s.listTotal, nil
}

func (s *stubDocumentAttachmentRepo) FindBlobByHash(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	_ int64,
) (*models.DocumentAttachmentBlob, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *stubDocumentAttachmentRepo) FindBlobByObject(
	_ context.Context,
	_ string,
	_ string,
) (*models.DocumentAttachmentBlob, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *stubDocumentAttachmentRepo) GetBlobByBlobID(
	_ context.Context,
	_ string,
) (*models.DocumentAttachmentBlob, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *stubDocumentAttachmentRepo) ListOrphanBlobs(
	_ context.Context,
	_ int,
) ([]models.DocumentAttachmentBlob, error) {
	return []models.DocumentAttachmentBlob{}, nil
}

func (s *stubDocumentAttachmentRepo) CreateBlob(_ context.Context, _ *models.DocumentAttachmentBlob) error {
	return nil
}

func (s *stubDocumentAttachmentRepo) HardDeleteBlobIfUnreferenced(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (s *stubDocumentAttachmentRepo) CountActiveReferencesByBlobID(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (s *stubDocumentAttachmentRepo) ListActiveReferencesByBlobID(
	_ context.Context,
	_ string,
	_ int,
) ([]repository.DocumentAttachmentReferenceRecord, error) {
	return []repository.DocumentAttachmentReferenceRecord{}, nil
}

func (s *stubDocumentAttachmentRepo) GetByAttachmentID(
	_ context.Context,
	attachmentID string,
) (*models.DocumentAttachment, error) {
	attachment, ok := s.attachmentsByID[attachmentID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copyValue := *attachment
	return &copyValue, nil
}

func (s *stubDocumentAttachmentRepo) SoftDelete(
	_ context.Context,
	attachmentID string,
	_ time.Time,
) (bool, error) {
	s.softDeletedIDs = append(s.softDeletedIDs, attachmentID)
	return true, nil
}

func (s *stubDocumentAttachmentRepo) HardDelete(_ context.Context, _ string) (bool, error) {
	return true, nil
}
