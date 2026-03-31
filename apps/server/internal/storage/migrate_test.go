package storage

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"testing"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestDriverSupportsMigrationTransactions(t *testing.T) {
	tests := []struct {
		name     string
		driver   string
		expected bool
	}{
		{name: "sqlite uses transaction", driver: DriverSQLite, expected: true},
		{name: "postgres uses transaction", driver: DriverPostgres, expected: true},
		{name: "mysql skips transaction", driver: DriverMySQL, expected: false},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			if got := driverSupportsMigrationTransactions(testCase.driver); got != testCase.expected {
				t.Fatalf("expected %v, got %v", testCase.expected, got)
			}
		})
	}
}

func TestMigrateUpWithOptions_LogsEachMigrationFile(t *testing.T) {
	database, err := OpenDatabase(OpenConfig{
		Driver: DriverSQLite,
		DSN:    "file:test-migrate-logging?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	migrations, err := LoadMigrations(DriverSQLite)
	if err != nil {
		t.Fatalf("LoadMigrations failed: %v", err)
	}

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{}))

	if err := MigrateUpWithOptions(context.Background(), database.ORM, DriverSQLite, MigrateOptions{
		Logger: logger,
	}); err != nil {
		t.Fatalf("MigrateUpWithOptions failed: %v", err)
	}

	logOutput := logBuffer.String()
	applyingCount := strings.Count(logOutput, "\"msg\":\"database migration applying\"")
	appliedCount := strings.Count(logOutput, "\"msg\":\"database migration applied\"")

	if applyingCount != len(migrations) {
		t.Fatalf("expected %d applying logs, got %d", len(migrations), applyingCount)
	}
	if appliedCount != len(migrations) {
		t.Fatalf("expected %d applied logs, got %d", len(migrations), appliedCount)
	}
}

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
		"user_identities",
		"user_sessions",
		"user_admin_roles",
		"system_configs",
		"audit_logs",
		"spaces",
		"space_categories",
		"space_cover_assets",
		"space_admin_scopes",
		"space_members",
		"nodes",
		"themes",
		"document_templates",
		"documents",
		"document_revisions",
		"document_file_revisions",
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

	// 中文注释：校验 0008 种子已创建默认管理员账号，且重复迁移不会产生重复记录。
	var seededAdminCount int64
	if err := database.ORM.WithContext(ctx).
		Table("users").
		Where("email = ?", "admin@iminho.me").
		Count(&seededAdminCount).Error; err != nil {
		t.Fatalf("count seeded admin user failed: %v", err)
	}
	if seededAdminCount != 1 {
		t.Fatalf("expected seeded admin user count 1, got %d", seededAdminCount)
	}

	var seededAdminRoleCount int64
	if err := database.ORM.WithContext(ctx).
		Table("user_admin_roles").
		Where("user_id = ? AND role = ?", "01k5aa0bb1cc2dd3ee4ff5gg6h", "platform_admin").
		Count(&seededAdminRoleCount).Error; err != nil {
		t.Fatalf("count seeded admin role failed: %v", err)
	}
	if seededAdminRoleCount != 1 {
		t.Fatalf("expected seeded admin role count 1, got %d", seededAdminRoleCount)
	}

	if err := smokeInsertGraph(ctx, database.ORM); err != nil {
		t.Fatalf("smokeInsertGraph failed: %v", err)
	}

	migrations, err := LoadMigrations(DriverSQLite)
	if err != nil {
		t.Fatalf("LoadMigrations failed: %v", err)
	}

	if err := MigrateDown(ctx, database.ORM, DriverSQLite, len(migrations)); err != nil {
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
	userIdentity := &models.UserIdentity{
		UserID:       userID,
		ProviderType: "local",
		ProviderID:   "local",
		ExternalID:   "tester@example.com",
		LoginName:    "tester@example.com",
	}
	if err := orm.WithContext(ctx).Create(userIdentity).Error; err != nil {
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

	var persisted struct {
		Format         string `gorm:"column:format"`
		ContentVersion int    `gorm:"column:content_version"`
	}
	if err := orm.WithContext(ctx).
		Table("documents").
		Select("format", "content_version").
		Where("document_id = ?", documentID).
		Take(&persisted).Error; err != nil {
		return err
	}
	if persisted.Format != string(models.DocumentFormatMarkdown) {
		return gorm.ErrInvalidData
	}
	if persisted.ContentVersion != 1 {
		return gorm.ErrInvalidData
	}

	return nil
}
