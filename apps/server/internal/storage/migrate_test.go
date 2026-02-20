package storage

import (
	"context"
	"database/sql"
	"testing"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestMigrateUpAndDown_SQLite(t *testing.T) {
	database, err := OpenDatabase(OpenConfig{
		Driver: DriverSQLite,
		DSN:    "file:test-migrate-up-down?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	ctx := context.Background()
	if err := MigrateUp(ctx, database.ORM, DriverSQLite); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}
	// 中文注释：重复执行 up 迁移应保持幂等，不应报错。
	if err := MigrateUp(ctx, database.ORM, DriverSQLite); err != nil {
		t.Fatalf("MigrateUp (idempotent) failed: %v", err)
	}

	requiredTables := []string{
		"users",
		"user_sessions",
		"user_admin_roles",
		"system_configs",
		"audit_logs",
		"spaces",
		"space_admin_scopes",
		"space_members",
		"nodes",
		"themes",
		"documents",
		"document_revisions",
		"node_permissions",
		"document_permissions",
	}
	for _, table := range requiredTables {
		exists, err := sqliteTableExists(ctx, database.SQL, table)
		if err != nil {
			t.Fatalf("check table %s failed: %v", table, err)
		}
		if !exists {
			t.Fatalf("expected table %s to exist", table)
		}
	}

	if err := smokeInsertGraph(ctx, database.ORM); err != nil {
		t.Fatalf("smokeInsertGraph failed: %v", err)
	}

	if err := MigrateDown(ctx, database.ORM, DriverSQLite, 10); err != nil {
		t.Fatalf("MigrateDown failed: %v", err)
	}

	for _, table := range requiredTables {
		exists, err := sqliteTableExists(ctx, database.SQL, table)
		if err != nil {
			t.Fatalf("check table %s after down failed: %v", table, err)
		}
		if exists {
			t.Fatalf("expected table %s to be dropped", table)
		}
	}
}

func sqliteTableExists(ctx context.Context, db *sql.DB, tableName string) (bool, error) {
	const query = `SELECT 1 FROM sqlite_master WHERE type='table' AND name=? LIMIT 1`
	var value int
	err := db.QueryRowContext(ctx, query, tableName).Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func smokeInsertGraph(ctx context.Context, orm *gorm.DB) error {
	userID := "01h0m1gr4t10n0000000000001"
	spaceID := "01h0m1gr4t10n0000000000002"
	nodeID := "01h0m1gr4t10n0000000000003"
	themeID := "default"
	documentID := "01h0m1gr4t10n0000000000004"

	user := &models.User{
		UserID:       userID,
		Email:        "tester@example.com",
		PasswordHash: "hashed",
		Name:         "tester",
		Status:       models.EntityStatusActive,
	}
	if err := orm.WithContext(ctx).Create(user).Error; err != nil {
		return err
	}

	space := &models.Space{
		SpaceID:     spaceID,
		Name:        "default",
		OwnerUserID: userID,
		Visibility:  models.VisibilityMember,
		Status:      models.EntityStatusActive,
	}
	if err := orm.WithContext(ctx).Create(space).Error; err != nil {
		return err
	}

	node := &models.Node{
		NodeID:  nodeID,
		SpaceID: spaceID,
		Type:    models.NodeTypeDoc,
		Title:   "hello",
		Sort:    1,
	}
	if err := orm.WithContext(ctx).Create(node).Error; err != nil {
		return err
	}

	theme := &models.Theme{
		ThemeID:                themeID,
		Name:                   "内置默认",
		Description:            "通用文档风格",
		VariablesJSON:          "{}",
		SyntaxTheme:            "one-light",
		CodeBlockStyleJSON:     "{}",
		CodeBlockCodeStyleJSON: "{}",
		InlineCodeStyleJSON:    "{}",
		IsBuiltin:              true,
	}
	if err := orm.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(theme).Error; err != nil {
		return err
	}

	document := &models.Document{
		DocumentID:      documentID,
		NodeID:          nodeID,
		ThemeID:         themeID,
		Visibility:      models.VisibilityMember,
		Status:          models.EntityStatusActive,
		Title:           "hello",
		ContentMD:       "# hello",
		Version:         1,
		UpdatedByUserID: &userID,
	}
	if err := orm.WithContext(ctx).Create(document).Error; err != nil {
		return err
	}

	return nil
}
