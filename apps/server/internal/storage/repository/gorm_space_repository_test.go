package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestGormSpaceRepository_HardDelete_DoesNotUseSelfReferencingDocumentDelete(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-space-hard-delete?mode=memory&cache=shared",
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

	if err := database.ORM.WithContext(ctx).Create(&models.User{
		UserID:       ownerUserID,
		Email:        "owner@example.com",
		PasswordHash: "hashed",
		Name:         "Owner",
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
		Title:           "空间文档",
		ContentMD:       "content",
		Version:         1,
		ContentVersion:  1,
		CreatedByUserID: &ownerUserID,
		UpdatedByUserID: &ownerUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed document failed: %v", err)
	}

	sqlLogger := &sqlCaptureLogger{Interface: gormlogger.Default.LogMode(gormlogger.Silent)}
	repo := NewGormSpaceRepository(database.ORM.Session(&gorm.Session{
		Logger: sqlLogger,
	})).(*gormSpaceRepository)

	deleted, err := repo.HardDelete(ctx, spaceID)
	if err != nil {
		t.Fatalf("hard delete failed: %v", err)
	}
	if !deleted {
		t.Fatalf("expected hard delete to remove the space")
	}

	for _, statement := range sqlLogger.statements {
		normalized := normalizeCapturedSQL(statement)
		if strings.Contains(
			normalized,
			"delete from documents where document_id in (select d.document_id from documents as d",
		) {
			t.Fatalf("unexpected self-referencing documents delete SQL: %s", statement)
		}
	}
}

func TestGormSpaceRepository_ListForAdmin_UsesBooleanCategoryDefaultFallback(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-space-list-for-admin?mode=memory&cache=shared",
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
	ownerUserID := "01kowner000000000000000001"
	spaceID := "01kspace000000000000000001"

	if err := database.ORM.WithContext(ctx).Create(&models.User{
		UserID:       ownerUserID,
		Email:        "owner-list@example.com",
		PasswordHash: "hashed",
		Name:         "Owner List",
		Status:       models.EntityStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Create(&models.Space{
		SpaceID:     spaceID,
		Name:        "列表空间",
		Description: "desc",
		CategoryID:  models.DefaultSpaceCategoryID,
		Category:    models.DefaultSpaceCategoryName,
		OwnerUserID: ownerUserID,
		Visibility:  models.VisibilityMember,
		Status:      models.EntityStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed space failed: %v", err)
	}

	sqlLogger := &sqlCaptureLogger{Interface: gormlogger.Default.LogMode(gormlogger.Silent)}
	repo := NewGormSpaceRepository(database.ORM.Session(&gorm.Session{
		Logger: sqlLogger,
	})).(*gormSpaceRepository)

	items, total, err := repo.ListForAdmin(ctx, ListAdminSpacesParams{
		ActorUserID:      ownerUserID,
		RestrictToScopes: false,
		Limit:            20,
		Offset:           0,
	})
	if err != nil {
		t.Fatalf("list for admin failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if !items[0].CategoryIsDef {
		t.Fatalf("expected category default flag to be true")
	}

	found := false
	for _, statement := range sqlLogger.statements {
		normalized := normalizeCapturedSQL(statement)
		if strings.Contains(normalized, "coalesce(sc.is_default, false) as category_is_default") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected admin list SQL to use boolean fallback, statements=%v", sqlLogger.statements)
	}
}

type sqlCaptureLogger struct {
	gormlogger.Interface
	statements []string
}

func (l *sqlCaptureLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	cloned := *l
	cloned.Interface = l.Interface.LogMode(level)
	return &cloned
}

func (l *sqlCaptureLogger) Trace(
	ctx context.Context,
	begin time.Time,
	fc func() (string, int64),
	err error,
) {
	sql, _ := fc()
	l.statements = append(l.statements, sql)
	l.Interface.Trace(ctx, begin, fc, err)
}

func normalizeCapturedSQL(raw string) string {
	lowered := strings.ToLower(strings.TrimSpace(raw))
	replacer := strings.NewReplacer(
		"`", "",
		`"`, "",
		"\n", " ",
		"\t", " ",
	)
	normalized := replacer.Replace(lowered)
	return strings.Join(strings.Fields(normalized), " ")
}
