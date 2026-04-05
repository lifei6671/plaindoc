package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

func TestAdminDocumentShareService_ListShares_RespectsMineViewForNormalUsers(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		actorUserID     string
		rolesByUser     map[string][]models.AdminRole
		view            repository.DocumentShareAdminView
		expectErr       bool
		expectRestrict  bool
		expectListCalls int
	}

	cases := []testCase{
		{
			name:            "platform admin can list all shares without scope restriction",
			actorUserID:     "platform-user",
			rolesByUser:     map[string][]models.AdminRole{"platform-user": {models.AdminRolePlatformAdmin}},
			view:            repository.DocumentShareAdminViewAll,
			expectRestrict:  false,
			expectListCalls: 1,
		},
		{
			name:            "space admin can list all shares with scope restriction",
			actorUserID:     "space-user",
			rolesByUser:     map[string][]models.AdminRole{"space-user": {models.AdminRoleSpaceAdmin}},
			view:            repository.DocumentShareAdminViewAll,
			expectRestrict:  true,
			expectListCalls: 1,
		},
		{
			name:            "normal user can only list mine view",
			actorUserID:     "normal-user",
			rolesByUser:     map[string][]models.AdminRole{},
			view:            repository.DocumentShareAdminViewMine,
			expectRestrict:  false,
			expectListCalls: 1,
		},
		{
			name:        "normal user cannot list all view",
			actorUserID: "normal-user",
			rolesByUser: map[string][]models.AdminRole{},
			view:        repository.DocumentShareAdminViewAll,
			expectErr:   true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &recordingDocumentShareRepo{}
			docSvc := &DocumentShareService{shareRepo: repo}
			svc := &AdminDocumentShareService{
				shareRepo:          repo,
				documentShareSvc:   docSvc,
				adminAccessService: newTestAdminAccessService(tc.rolesByUser, map[string][]string{}),
			}

			_, err := svc.ListShares(context.Background(), ListAdminDocumentSharesInput{
				ActorUserID: tc.actorUserID,
				View:        tc.view,
				Page:        1,
				PageSize:    20,
			})

			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, ErrDocumentShareAccessDenied) {
					t.Fatalf("expected access denied, got: %v", err)
				}
				if len(repo.listParams) != 0 {
					t.Fatalf("expected no repository call, got %d", len(repo.listParams))
				}
				return
			}

			if err != nil {
				t.Fatalf("list shares failed: %v", err)
			}
			if got := len(repo.listParams); got != tc.expectListCalls {
				t.Fatalf("unexpected repository call count: got=%d want=%d", got, tc.expectListCalls)
			}
			gotParams := repo.listParams[0]
			if gotParams.View != tc.view {
				t.Fatalf("unexpected view: got=%s want=%s", gotParams.View, tc.view)
			}
			if gotParams.RestrictToScopes != tc.expectRestrict {
				t.Fatalf("unexpected restrict flag: got=%v want=%v", gotParams.RestrictToScopes, tc.expectRestrict)
			}
			if gotParams.ActorUserID != tc.actorUserID {
				t.Fatalf("unexpected actor user id: got=%s want=%s", gotParams.ActorUserID, tc.actorUserID)
			}
		})
	}
}

func TestAdminDocumentShareService_UpdateAndDisable_AllowShareCreators(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	creatorID := "creator-user"
	otherID := "other-user"
	ownShare := &models.DocumentShare{
		ShareID:         "share-own",
		DocumentID:      "doc-own",
		SpaceID:         "space-a",
		Mode:            models.DocumentShareModePublic,
		AccessVersion:   1,
		CreatedByUserID: &creatorID,
		UpdatedByUserID: &creatorID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	otherShare := &models.DocumentShare{
		ShareID:         "share-other",
		DocumentID:      "doc-other",
		SpaceID:         "space-a",
		Mode:            models.DocumentShareModePublic,
		AccessVersion:   1,
		CreatedByUserID: &otherID,
		UpdatedByUserID: &otherID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	t.Run("creator can update own share", func(t *testing.T) {
		t.Parallel()

		repo := &recordingDocumentShareRepo{
			sharesByShareID: map[string]*models.DocumentShare{
				ownShare.ShareID: ownShare,
			},
		}
		svc := &AdminDocumentShareService{
			shareRepo: repo,
			documentShareSvc: &DocumentShareService{
				shareRepo: repo,
				nowFn:     func() time.Time { return now },
			},
			adminAccessService: newTestAdminAccessService(map[string][]models.AdminRole{}, map[string][]string{}),
		}

		result, err := svc.UpdateShare(context.Background(), UpdateDocumentShareByIDInput{
			ShareID:      ownShare.ShareID,
			ActorUserID:  creatorID,
			ExpiresAtSet: true,
			ExpiresAt:    nil,
		})
		if err != nil {
			t.Fatalf("update own share failed: %v", err)
		}
		if result.ShareID != ownShare.ShareID {
			t.Fatalf("unexpected share id: got=%s want=%s", result.ShareID, ownShare.ShareID)
		}
		if len(repo.updateCalls) != 1 {
			t.Fatalf("expected one update call, got %d", len(repo.updateCalls))
		}
	})

	t.Run("creator can disable own share", func(t *testing.T) {
		t.Parallel()

		repo := &recordingDocumentShareRepo{
			sharesByShareID: map[string]*models.DocumentShare{
				ownShare.ShareID: ownShare,
			},
		}
		svc := &AdminDocumentShareService{
			shareRepo: repo,
			documentShareSvc: &DocumentShareService{
				shareRepo: repo,
				nowFn:     func() time.Time { return now },
			},
			adminAccessService: newTestAdminAccessService(map[string][]models.AdminRole{}, map[string][]string{}),
		}

		if err := svc.DisableShare(context.Background(), ownShare.ShareID, creatorID); err != nil {
			t.Fatalf("disable own share failed: %v", err)
		}
		if len(repo.updateCalls) != 1 {
			t.Fatalf("expected one update call, got %d", len(repo.updateCalls))
		}
	})

	t.Run("non creator cannot update other share", func(t *testing.T) {
		t.Parallel()

		repo := &recordingDocumentShareRepo{
			sharesByShareID: map[string]*models.DocumentShare{
				otherShare.ShareID: otherShare,
			},
		}
		svc := &AdminDocumentShareService{
			shareRepo: repo,
			documentShareSvc: &DocumentShareService{
				shareRepo: repo,
				nowFn:     func() time.Time { return now },
			},
			adminAccessService: newTestAdminAccessService(map[string][]models.AdminRole{}, map[string][]string{}),
		}

		_, err := svc.UpdateShare(context.Background(), UpdateDocumentShareByIDInput{
			ShareID:      otherShare.ShareID,
			ActorUserID:  creatorID,
			ExpiresAtSet: true,
			ExpiresAt:    nil,
		})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !errors.Is(err, ErrDocumentShareAccessDenied) {
			t.Fatalf("expected access denied, got: %v", err)
		}
		if len(repo.updateCalls) != 0 {
			t.Fatalf("expected no update call, got %d", len(repo.updateCalls))
		}
	})
}

type recordingDocumentShareRepo struct {
	listParams      []repository.ListAdminDocumentSharesParams
	sharesByShareID map[string]*models.DocumentShare
	updateCalls     []string
}

func (r *recordingDocumentShareRepo) Create(ctx context.Context, share *models.DocumentShare) error {
	return nil
}

func (r *recordingDocumentShareRepo) Update(ctx context.Context, share *models.DocumentShare) (bool, error) {
	if share != nil {
		r.updateCalls = append(r.updateCalls, share.ShareID)
		if r.sharesByShareID != nil {
			copyValue := *share
			r.sharesByShareID[share.ShareID] = &copyValue
		}
	}
	return true, nil
}

func (r *recordingDocumentShareRepo) GetByDocumentID(ctx context.Context, documentID string) (*models.DocumentShare, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r *recordingDocumentShareRepo) GetByShareID(ctx context.Context, shareID string) (*models.DocumentShare, error) {
	if share, ok := r.sharesByShareID[shareID]; ok && share != nil {
		copyValue := *share
		return &copyValue, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *recordingDocumentShareRepo) ResolveBySpaceAndDocKey(ctx context.Context, spaceID string, rawDocKey string) (*repository.DocumentShareAccessRecord, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r *recordingDocumentShareRepo) ListForAdmin(
	ctx context.Context,
	params repository.ListAdminDocumentSharesParams,
) ([]repository.AdminDocumentShareListRecord, int64, error) {
	r.listParams = append(r.listParams, params)
	return []repository.AdminDocumentShareListRecord{}, 0, nil
}

var _ repository.DocumentShareRepository = (*recordingDocumentShareRepo)(nil)
