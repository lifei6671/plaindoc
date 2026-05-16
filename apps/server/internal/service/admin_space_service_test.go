package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func TestAdminSpaceServiceUpdateMetadataRejectsOwnerBindingOtherUsersCoverAsset(t *testing.T) {
	t.Parallel()

	coverAssetID := "cover-other"
	spaceRepo := &fakeAdminSpaceExportPermissionSpaceRepo{
		spaces: map[string]*models.Space{
			"space-a": {
				SpaceID:     "space-a",
				Name:        "空间 A",
				OwnerUserID: "owner-user",
				Visibility:  models.VisibilityMember,
				Status:      models.EntityStatusActive,
			},
		},
		coverAssets: map[string]*models.SpaceCoverAsset{
			coverAssetID: {
				AssetID:         coverAssetID,
				ObjectKey:       "space-covers/other.webp",
				ObjectURL:       "/uploads/space-covers/other.webp",
				Source:          string(AdminSpaceCoverSourceUserUpload),
				Width:           1600,
				Height:          900,
				CreatedByUserID: "other-user",
			},
		},
	}
	accessService := NewAdminAccessService(newStubAdminRoleRepo(nil), newStubSpaceAdminScopeRepo(nil), nil)
	svc := NewAdminSpaceService(spaceRepo, nil, nil, nil, nil, nil, accessService, nil)

	_, err := svc.UpdateMetadata(context.Background(), UpdateAdminSpaceMetadataInput{
		ActorUserID:  "owner-user",
		SpaceID:      "space-a",
		CoverAssetID: &coverAssetID,
	})

	if !errors.Is(err, errcode.ErrAdminForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
	if spaceRepo.updateMetadataCalls != 0 {
		t.Fatalf("expected metadata update to be skipped, got %d calls", spaceRepo.updateMetadataCalls)
	}
}
