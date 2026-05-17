package storage

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
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

func TestSplitSQLStatements_HandlesDollarQuotedBlocks(t *testing.T) {
	sqlText := `ALTER TABLE documents
	ADD COLUMN IF NOT EXISTS format VARCHAR(16) NOT NULL DEFAULT 'markdown';

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'chk_documents_format'
	) THEN
		ALTER TABLE documents
			ADD CONSTRAINT chk_documents_format
			CHECK (format IN ('markdown', 'docx', 'xlsx'));
	END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_documents_format
	ON documents(format);`

	statements := splitSQLStatements(sqlText)
	if len(statements) != 3 {
		t.Fatalf("expected 3 statements, got %d: %#v", len(statements), statements)
	}
	if !strings.HasPrefix(statements[1], "DO $$") || !strings.HasSuffix(statements[1], "END $$") {
		t.Fatalf("expected second statement to keep dollar-quoted block intact, got %#v", statements[1])
	}
	if !strings.Contains(statements[1], "CHECK (format IN ('markdown', 'docx', 'xlsx'))") {
		t.Fatalf("expected block content to be preserved, got %#v", statements[1])
	}
	if !strings.HasPrefix(statements[2], "CREATE INDEX IF NOT EXISTS idx_documents_format") {
		t.Fatalf("expected third statement to remain separate, got %#v", statements[2])
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

func TestMigrateUpWithOptions_ReportsProgress(t *testing.T) {
	database, err := OpenDatabase(OpenConfig{
		Driver: DriverSQLite,
		DSN:    "file:test-migrate-progress?mode=memory&cache=shared",
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

	var progressEvents []MigrationProgress
	if err := MigrateUpWithOptions(context.Background(), database.ORM, DriverSQLite, MigrateOptions{
		OnProgress: func(progress MigrationProgress) {
			progressEvents = append(progressEvents, progress)
		},
	}); err != nil {
		t.Fatalf("MigrateUpWithOptions failed: %v", err)
	}

	if len(progressEvents) < 3 {
		t.Fatalf("expected progress events, got %d", len(progressEvents))
	}
	firstEvent := progressEvents[0]
	if firstEvent.Phase != "loaded" || firstEvent.TotalCount != len(migrations) || firstEvent.PendingCount != len(migrations) {
		t.Fatalf("unexpected first progress event: %+v", firstEvent)
	}
	lastEvent := progressEvents[len(progressEvents)-1]
	if lastEvent.Phase != "complete" || lastEvent.AppliedCount != len(migrations) || lastEvent.TotalCount != len(migrations) {
		t.Fatalf("unexpected complete progress event: %+v", lastEvent)
	}
	var applyingFound bool
	for _, progressEvent := range progressEvents {
		if progressEvent.Phase == "applying" && progressEvent.CurrentVersion > 0 && progressEvent.CurrentName != "" {
			applyingFound = true
			break
		}
	}
	if !applyingFound {
		t.Fatalf("expected applying event with current migration, got %+v", progressEvents)
	}
}

func TestMigrateUpWithOptions_ReportsFailedProgress(t *testing.T) {
	database, err := OpenDatabase(OpenConfig{
		Driver: DriverSQLite,
		DSN:    "file:test-migrate-progress-failed?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	var progressEvents []MigrationProgress
	err = MigrateUpWithOptions(ctx, database.ORM, DriverSQLite, MigrateOptions{
		OnProgress: func(progress MigrationProgress) {
			progressEvents = append(progressEvents, progress)
			if progress.Phase == "applying" {
				cancel()
			}
		},
	})
	if err == nil {
		t.Fatal("expected migration failure after context cancellation")
	}

	lastEvent := progressEvents[len(progressEvents)-1]
	if lastEvent.Phase != "failed" || lastEvent.CurrentVersion <= 0 || lastEvent.CurrentName == "" {
		t.Fatalf("unexpected failed progress event: %+v", lastEvent)
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
		"admin_space_transfer_jobs",
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

func TestMigrateUp_SQLiteTimeColumnsUseTimestamp(t *testing.T) {
	database, err := OpenDatabase(OpenConfig{
		Driver: DriverSQLite,
		DSN:    "file:test-migrate-sqlite-time-columns?mode=memory&cache=shared",
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

	expectedColumns := map[string][]string{
		"users":                        {"banned_at", "deleted_at", "created_at", "updated_at"},
		"user_identities":              {"last_login_at", "created_at", "updated_at"},
		"user_sessions":                {"expires_at", "revoked_at", "created_at", "updated_at"},
		"user_admin_roles":             {"created_at", "updated_at"},
		"space_admin_scopes":           {"created_at", "updated_at"},
		"system_configs":               {"created_at", "updated_at"},
		"audit_logs":                   {"created_at"},
		"spaces":                       {"banned_at", "deleted_at", "created_at", "updated_at"},
		"space_categories":             {"created_at", "updated_at"},
		"space_cover_assets":           {"created_at", "updated_at"},
		"space_members":                {"created_at", "updated_at"},
		"nodes":                        {"created_at", "updated_at"},
		"themes":                       {"created_at", "updated_at"},
		"documents":                    {"banned_at", "deleted_at", "rendered_at", "created_at", "updated_at"},
		"document_revisions":           {"created_at"},
		"document_file_revisions":      {"created_at"},
		"node_permissions":             {"created_at", "updated_at"},
		"document_permissions":         {"created_at", "updated_at"},
		"auth_risk_states":             {"window_started_at", "lock_until", "created_at", "updated_at"},
		"auth_captcha_challenges":      {"expires_at", "consumed_at", "created_at", "updated_at"},
		"file_blobs":                   {"deleted_at", "created_at", "updated_at"},
		"document_attachments":         {"deleted_at", "created_at", "updated_at"},
		"document_image_assets":        {"pending_cleanup_at", "deleted_at", "last_referenced_at", "created_at", "updated_at"},
		"admin_space_transfer_jobs":    {"started_at", "completed_at", "created_at", "updated_at", "expires_at"},
		"search_analyzer_dict_entries": {"created_at", "updated_at"},
		"search_index_jobs":            {"next_run_at", "started_at", "created_at", "updated_at"},
		"password_reset_tokens":        {"expires_at", "consumed_at", "invalidated_at", "created_at", "updated_at"},
		"document_templates":           {"created_at", "updated_at"},
		"document_template_scenes":     {"created_at", "updated_at"},
		"document_shares":              {"expires_at", "disabled_at", "created_at", "updated_at"},
	}

	for tableName, columns := range expectedColumns {
		for _, columnName := range columns {
			columnType, err := sqliteColumnType(ctx, database.SQL, tableName, columnName)
			if err != nil {
				t.Fatalf("sqliteColumnType(%s.%s) failed: %v", tableName, columnName, err)
			}
			if columnType != "TIMESTAMP" {
				t.Fatalf("expected %s.%s to use TIMESTAMP, got %s", tableName, columnName, columnType)
			}
		}
	}
}

func TestMigrateUp_SQLiteLegacyTextTimeColumnsPreserveData(t *testing.T) {
	database, err := OpenDatabase(OpenConfig{
		Driver: DriverSQLite,
		DSN:    "file:test-migrate-sqlite-legacy-text-times?mode=memory&cache=shared",
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

	const (
		jobID            = "01legacytimecolsjob00000000001"
		captchaID        = "01legacytimecolscaptcha0000001"
		createdAtRaw     = "2026-04-01 08:09:10"
		updatedAtRaw     = "2026-04-01 11:12:13"
		nextRunAtRaw     = "2026-04-01 14:15:16"
		startedAtRaw     = "2026-04-01 17:18:19"
		windowStartedRaw = "2026-04-02 08:09:10"
		lockUntilRaw     = "2026-04-02 11:12:13"
		expiresAtRaw     = "2026-04-03 08:09:10"
		consumedAtRaw    = "2026-04-03 09:10:11"
	)

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO search_index_jobs (
	job_id,
	provider,
	job_type,
	dedupe_key,
	payload_json,
	status,
	priority,
	retry_count,
	next_run_at,
	started_at,
	last_error,
	created_at,
	updated_at
) VALUES (?, 'bleve', 'DOC_UPSERT', 'legacy-job', '{}', 'running', 10, 2, ?, ?, '', ?, ?)`,
		jobID,
		nextRunAtRaw,
		startedAtRaw,
		createdAtRaw,
		updatedAtRaw,
	); err != nil {
		t.Fatalf("insert search_index_jobs failed: %v", err)
	}

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO auth_risk_states (
	scene,
	subject_type,
	subject_hash,
	window_started_at,
	attempt_count,
	failed_count,
	captcha_failed_count,
	lock_until,
	created_at,
	updated_at
) VALUES ('login', 'email', 'legacy-subject', ?, 3, 1, 2, ?, ?, ?)`,
		windowStartedRaw,
		lockUntilRaw,
		createdAtRaw,
		updatedAtRaw,
	); err != nil {
		t.Fatalf("insert auth_risk_states failed: %v", err)
	}

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO auth_captcha_challenges (
	captcha_id,
	scene,
	subject_hash,
	level,
	answer_hash,
	answer_salt,
	issued_ip_hash,
	expires_at,
	consumed_at,
	failed_verify_count,
	created_at,
	updated_at
) VALUES (?, 'login', 'legacy-subject', 5, 'answer-hash', 'answer-salt', 'ip-hash', ?, ?, 1, ?, ?)`,
		captchaID,
		expiresAtRaw,
		consumedAtRaw,
		createdAtRaw,
		updatedAtRaw,
	); err != nil {
		t.Fatalf("insert auth_captcha_challenges failed: %v", err)
	}

	migrations, err := LoadMigrations(DriverSQLite)
	if err != nil {
		t.Fatalf("LoadMigrations failed: %v", err)
	}
	var migration33 Migration
	for _, migration := range migrations {
		if migration.Version == 33 {
			migration33 = migration
			break
		}
	}
	if migration33.Version != 33 {
		t.Fatalf("migration 33 not found")
	}

	if err := applyMigrationDown(ctx, database.SQL, DriverSQLite, migration33); err != nil {
		t.Fatalf("applyMigrationDown(33) failed: %v", err)
	}

	for _, item := range []struct {
		table  string
		column string
	}{
		{table: "auth_risk_states", column: "window_started_at"},
		{table: "auth_captcha_challenges", column: "expires_at"},
		{table: "search_index_jobs", column: "next_run_at"},
	} {
		columnType, err := sqliteColumnType(ctx, database.SQL, item.table, item.column)
		if err != nil {
			t.Fatalf("sqliteColumnType(%s.%s) failed: %v", item.table, item.column, err)
		}
		if columnType != "TEXT" {
			t.Fatalf("expected downgraded %s.%s to use TEXT, got %s", item.table, item.column, columnType)
		}
	}

	if err := applyMigrationUp(ctx, database.SQL, DriverSQLite, migration33); err != nil {
		t.Fatalf("applyMigrationUp(33) failed: %v", err)
	}

	for _, item := range []struct {
		table  string
		column string
	}{
		{table: "auth_risk_states", column: "window_started_at"},
		{table: "auth_risk_states", column: "lock_until"},
		{table: "auth_captcha_challenges", column: "expires_at"},
		{table: "auth_captcha_challenges", column: "consumed_at"},
		{table: "search_index_jobs", column: "next_run_at"},
		{table: "search_index_jobs", column: "started_at"},
	} {
		columnType, err := sqliteColumnType(ctx, database.SQL, item.table, item.column)
		if err != nil {
			t.Fatalf("sqliteColumnType(%s.%s) failed: %v", item.table, item.column, err)
		}
		if columnType != "TIMESTAMP" {
			t.Fatalf("expected upgraded %s.%s to use TIMESTAMP, got %s", item.table, item.column, columnType)
		}
	}

	var riskWindowStartedAt, riskLockUntilAt sql.NullString
	if err := database.SQL.QueryRowContext(ctx, `
SELECT window_started_at, lock_until
FROM auth_risk_states
WHERE scene = 'login' AND subject_type = 'email' AND subject_hash = 'legacy-subject'`,
	).Scan(&riskWindowStartedAt, &riskLockUntilAt); err != nil {
		t.Fatalf("query auth_risk_states failed: %v", err)
	}
	assertSQLiteTimeValueEqual(t, "auth_risk_states.window_started_at", windowStartedRaw, riskWindowStartedAt)
	assertSQLiteTimeValueEqual(t, "auth_risk_states.lock_until", lockUntilRaw, riskLockUntilAt)

	var challengeExpiresAt, challengeConsumedAt sql.NullString
	if err := database.SQL.QueryRowContext(ctx, `
SELECT expires_at, consumed_at
FROM auth_captcha_challenges
WHERE captcha_id = ?`,
		captchaID,
	).Scan(&challengeExpiresAt, &challengeConsumedAt); err != nil {
		t.Fatalf("query auth_captcha_challenges failed: %v", err)
	}
	assertSQLiteTimeValueEqual(t, "auth_captcha_challenges.expires_at", expiresAtRaw, challengeExpiresAt)
	assertSQLiteTimeValueEqual(t, "auth_captcha_challenges.consumed_at", consumedAtRaw, challengeConsumedAt)

	var jobNextRunAt, jobStartedAt, jobCreatedAt, jobUpdatedAt sql.NullString
	if err := database.SQL.QueryRowContext(ctx, `
SELECT next_run_at, started_at, created_at, updated_at
FROM search_index_jobs
WHERE job_id = ?`,
		jobID,
	).Scan(&jobNextRunAt, &jobStartedAt, &jobCreatedAt, &jobUpdatedAt); err != nil {
		t.Fatalf("query search_index_jobs failed: %v", err)
	}
	assertSQLiteTimeValueEqual(t, "search_index_jobs.next_run_at", nextRunAtRaw, jobNextRunAt)
	assertSQLiteTimeValueEqual(t, "search_index_jobs.started_at", startedAtRaw, jobStartedAt)
	assertSQLiteTimeValueEqual(t, "search_index_jobs.created_at", createdAtRaw, jobCreatedAt)
	assertSQLiteTimeValueEqual(t, "search_index_jobs.updated_at", updatedAtRaw, jobUpdatedAt)
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

func sqliteColumnType(ctx context.Context, db *sql.DB, tableName string, columnName string) (string, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, tableName))
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal any
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			return "", err
		}
		if name == columnName {
			return strings.ToUpper(strings.TrimSpace(columnType)), nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("column %s not found in table %s", columnName, tableName)
}

func assertSQLiteTimeValueEqual(t *testing.T, fieldName string, expected string, got sql.NullString) {
	t.Helper()
	if !got.Valid {
		t.Fatalf("expected %s=%q, got NULL", fieldName, expected)
	}
	expectedAt := recordtime.Parse(expected)
	gotAt := recordtime.Parse(got.String)
	if expectedAt.IsZero() || gotAt.IsZero() || !expectedAt.Equal(gotAt) {
		t.Fatalf("expected %s=%q, got raw=%q", fieldName, expected, got.String)
	}
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
