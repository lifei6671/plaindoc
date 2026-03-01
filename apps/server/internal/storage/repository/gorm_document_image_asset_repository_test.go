package repository

import (
	"context"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func TestGormDocumentImageAssetRepository_ListForAdmin_MapsAssetFields(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-document-image-asset-repository-list?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	ctx := context.Background()
	if err := storage.MigrateUp(ctx, database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}

	now := time.Now().UTC().Round(time.Second)
	ownerUserID := "01kowner000000000000000000"
	spaceID := "01kspace000000000000000000"
	nodeID := "01knode0000000000000000000"
	documentID := "01kdoc00000000000000000000"
	imageAssetID := "01kimg00000000000000000000"

	if err := database.ORM.WithContext(ctx).Create(&models.User{
		UserID:       ownerUserID,
		Email:        "owner@example.com",
		PasswordHash: "hashed",
		Name:         "Owner",
		AvatarURL:    "",
		Status:       models.EntityStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Create(&models.Space{
		SpaceID:     spaceID,
		Name:        "测试空间",
		Description: "desc",
		CategoryID:  "01jmf4v2x7m7f1m6qv5kh0t2mn",
		Category:    "未分类",
		OwnerUserID: ownerUserID,
		Visibility:  models.VisibilityMember,
		Status:      models.EntityStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed space failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Create(&models.Node{
		NodeID:          nodeID,
		SpaceID:         spaceID,
		Type:            models.NodeTypeDoc,
		Title:           "节点标题",
		Sort:            1,
		CreatedByUserID: &ownerUserID,
		UpdatedByUserID: &ownerUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed node failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Create(&models.Document{
		DocumentID:      documentID,
		NodeID:          nodeID,
		ThemeID:         "default",
		Visibility:      models.VisibilityMember,
		Status:          models.EntityStatusActive,
		Title:           "图片文档",
		ContentMD:       "content",
		Version:         1,
		CreatedByUserID: &ownerUserID,
		UpdatedByUserID: &ownerUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed document failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Create(&models.DocumentImageAsset{
		ImageAssetID:     imageAssetID,
		DocumentID:       documentID,
		SpaceID:          spaceID,
		StorageProvider:  "local",
		ObjectKey:        "images/" + spaceID + "/" + documentID + "/example.png",
		ObjectURL:        "/uploads/images/example.png",
		Status:           "active",
		LastReferencedAt: now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("seed image asset failed: %v", err)
	}

	repo := NewGormDocumentImageAssetRepository(database.ORM)
	records, total, err := repo.ListForAdmin(ctx, ListAdminDocumentImageAssetsParams{
		RestrictToScopes: false,
		Limit:            20,
		Offset:           0,
	})
	if err != nil {
		t.Fatalf("list image assets failed: %v", err)
	}

	if total != 1 {
		t.Fatalf("unexpected total: got=%d want=1", total)
	}
	if len(records) != 1 {
		t.Fatalf("unexpected records length: got=%d want=1", len(records))
	}

	record := records[0]
	if record.ImageAsset.ImageAssetID != imageAssetID {
		t.Fatalf("unexpected image_asset_id: got=%q want=%q", record.ImageAsset.ImageAssetID, imageAssetID)
	}
	if record.ImageAsset.DocumentID != documentID {
		t.Fatalf("unexpected document_id: got=%q want=%q", record.ImageAsset.DocumentID, documentID)
	}
	if record.ImageAsset.SpaceID != spaceID {
		t.Fatalf("unexpected space_id: got=%q want=%q", record.ImageAsset.SpaceID, spaceID)
	}
	if record.ImageAsset.ObjectKey == "" || record.ImageAsset.ObjectURL == "" {
		t.Fatalf("expected non-empty object key/url, got key=%q url=%q", record.ImageAsset.ObjectKey, record.ImageAsset.ObjectURL)
	}
	if record.ImageAsset.CreatedAt.IsZero() || record.ImageAsset.UpdatedAt.IsZero() || record.ImageAsset.LastReferencedAt.IsZero() {
		t.Fatalf(
			"expected non-zero timestamps, got created=%v updated=%v lastReferenced=%v",
			record.ImageAsset.CreatedAt,
			record.ImageAsset.UpdatedAt,
			record.ImageAsset.LastReferencedAt,
		)
	}
	if record.DocumentTitle != "图片文档" {
		t.Fatalf("unexpected document title: got=%q", record.DocumentTitle)
	}
	if record.SpaceName != "测试空间" {
		t.Fatalf("unexpected space name: got=%q", record.SpaceName)
	}
}
