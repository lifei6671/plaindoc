package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

//go:embed migrations/*/*.sql
var migrationFS embed.FS

// Migration 表示一条双向迁移脚本（up/down）。
type Migration struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

// MigrateOptions 描述迁移执行时的可选行为。
type MigrateOptions struct {
	Logger *slog.Logger
}

// MigrateUp 执行所有未应用的迁移。
func MigrateUp(ctx context.Context, db *gorm.DB, driver string) error {
	return MigrateUpWithOptions(ctx, db, driver, MigrateOptions{})
}

// MigrateUpWithOptions 执行所有未应用的迁移，并允许注入日志等可选能力。
func MigrateUpWithOptions(ctx context.Context, db *gorm.DB, driver string, options MigrateOptions) error {
	sqlDB, err := unwrapSQLDB(db)
	if err != nil {
		return err
	}

	normalizedDriver, err := NormalizeDriver(driver)
	if err != nil {
		return err
	}

	migrations, err := LoadMigrations(normalizedDriver)
	if err != nil {
		return err
	}
	if err := ensureMigrationsTable(ctx, sqlDB); err != nil {
		return err
	}

	appliedVersions, err := listAppliedMigrationVersions(ctx, sqlDB)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if appliedVersions[migration.Version] {
			continue
		}
		logMigrationEvent(ctx, options.Logger, "database migration applying", normalizedDriver, migration)
		if err := applyMigrationUp(ctx, sqlDB, normalizedDriver, migration); err != nil {
			logMigrationFailure(ctx, options.Logger, normalizedDriver, migration, err)
			return err
		}
		logMigrationEvent(ctx, options.Logger, "database migration applied", normalizedDriver, migration)
	}

	return nil
}

// MigrateDown 回滚指定步数（steps），steps<=0 时不执行回滚。
func MigrateDown(ctx context.Context, db *gorm.DB, driver string, steps int) error {
	if steps <= 0 {
		return nil
	}

	sqlDB, err := unwrapSQLDB(db)
	if err != nil {
		return err
	}

	normalizedDriver, err := NormalizeDriver(driver)
	if err != nil {
		return err
	}

	migrations, err := LoadMigrations(normalizedDriver)
	if err != nil {
		return err
	}
	if err := ensureMigrationsTable(ctx, sqlDB); err != nil {
		return err
	}

	migrationMap := make(map[int]Migration, len(migrations))
	for _, migration := range migrations {
		migrationMap[migration.Version] = migration
	}

	appliedOrdered, err := listAppliedMigrationVersionsDesc(ctx, sqlDB)
	if err != nil {
		return err
	}

	maxSteps := steps
	if len(appliedOrdered) < maxSteps {
		maxSteps = len(appliedOrdered)
	}

	for _, version := range appliedOrdered[:maxSteps] {
		migration, ok := migrationMap[version]
		if !ok {
			return fmt.Errorf("missing down migration for version %d", version)
		}
		if err := applyMigrationDown(ctx, sqlDB, normalizedDriver, migration); err != nil {
			return err
		}
	}

	return nil
}

func unwrapSQLDB(db *gorm.DB) (*sql.DB, error) {
	if db == nil {
		return nil, fmt.Errorf("gorm db must not be nil")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("unwrap sql db failed: %w", err)
	}
	return sqlDB, nil
}

// LoadMigrations 按数据库类型读取迁移文件并按版本排序。
func LoadMigrations(driver string) ([]Migration, error) {
	normalizedDriver, err := NormalizeDriver(driver)
	if err != nil {
		return nil, err
	}

	migrationDir := path.Join("migrations", normalizedDriver)
	entries, err := fs.ReadDir(migrationFS, migrationDir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory failed: %w", err)
	}

	type pair struct {
		version int
		name    string
		upSQL   string
		downSQL string
	}
	pairs := make(map[int]*pair, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		if !strings.HasSuffix(filename, ".sql") {
			continue
		}

		version, migrationName, direction, err := parseMigrationFilename(filename)
		if err != nil {
			return nil, fmt.Errorf("parse migration filename %q failed: %w", filename, err)
		}

		contentBytes, err := fs.ReadFile(migrationFS, path.Join(migrationDir, filename))
		if err != nil {
			return nil, fmt.Errorf("read migration file %q failed: %w", filename, err)
		}

		item, exists := pairs[version]
		if !exists {
			item = &pair{
				version: version,
				name:    migrationName,
			}
			pairs[version] = item
		}
		if item.name != migrationName {
			return nil, fmt.Errorf("migration version %d has inconsistent names: %q vs %q", version, item.name, migrationName)
		}

		switch direction {
		case "up":
			item.upSQL = string(contentBytes)
		case "down":
			item.downSQL = string(contentBytes)
		default:
			return nil, fmt.Errorf("unsupported migration direction %q", direction)
		}
	}

	versions := make([]int, 0, len(pairs))
	for version := range pairs {
		versions = append(versions, version)
	}
	sort.Ints(versions)

	migrations := make([]Migration, 0, len(versions))
	for _, version := range versions {
		item := pairs[version]
		if strings.TrimSpace(item.upSQL) == "" {
			return nil, fmt.Errorf("migration %d(%s) missing up sql", item.version, item.name)
		}
		if strings.TrimSpace(item.downSQL) == "" {
			return nil, fmt.Errorf("migration %d(%s) missing down sql", item.version, item.name)
		}
		migrations = append(migrations, Migration{
			Version: item.version,
			Name:    item.name,
			UpSQL:   item.upSQL,
			DownSQL: item.downSQL,
		})
	}

	return migrations, nil
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB) error {
	const query = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version BIGINT PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations table failed: %w", err)
	}
	return nil
}

func listAppliedMigrationVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	const query = `SELECT version FROM schema_migrations`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations failed: %w", err)
	}
	defer rows.Close()

	result := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan migration version failed: %w", err)
		}
		result[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration rows failed: %w", err)
	}
	return result, nil
}

func listAppliedMigrationVersionsDesc(ctx context.Context, db *sql.DB) ([]int, error) {
	const query = `SELECT version FROM schema_migrations ORDER BY version DESC`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations failed: %w", err)
	}
	defer rows.Close()

	result := make([]int, 0, 8)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan migration version failed: %w", err)
		}
		result = append(result, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration rows failed: %w", err)
	}
	return result, nil
}

func applyMigrationUp(ctx context.Context, db *sql.DB, driver string, migration Migration) error {
	if !driverSupportsMigrationTransactions(driver) {
		if err := executeSQLStatements(ctx, db, migration.UpSQL); err != nil {
			return fmt.Errorf("execute up migration %d(%s) failed: %w", migration.Version, migration.Name, err)
		}
		if err := insertAppliedMigration(ctx, db, driver, migration); err != nil {
			return err
		}
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction failed: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := executeSQLStatements(ctx, tx, migration.UpSQL); err != nil {
		return fmt.Errorf("execute up migration %d(%s) failed: %w", migration.Version, migration.Name, err)
	}

	if err := insertAppliedMigration(ctx, tx, driver, migration); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration transaction failed: %w", err)
	}
	return nil
}

func applyMigrationDown(ctx context.Context, db *sql.DB, driver string, migration Migration) error {
	if !driverSupportsMigrationTransactions(driver) {
		if err := executeSQLStatements(ctx, db, migration.DownSQL); err != nil {
			return fmt.Errorf("execute down migration %d(%s) failed: %w", migration.Version, migration.Name, err)
		}
		if err := deleteAppliedMigration(ctx, db, driver, migration.Version); err != nil {
			return err
		}
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rollback transaction failed: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := executeSQLStatements(ctx, tx, migration.DownSQL); err != nil {
		return fmt.Errorf("execute down migration %d(%s) failed: %w", migration.Version, migration.Name, err)
	}

	if err := deleteAppliedMigration(ctx, tx, driver, migration.Version); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rollback transaction failed: %w", err)
	}
	return nil
}

type sqlStatementExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertAppliedMigration(ctx context.Context, executor sqlStatementExecutor, driver string, migration Migration) error {
	insertQuery := fmt.Sprintf(
		"INSERT INTO schema_migrations(version, name, applied_at) VALUES(%s, %s, CURRENT_TIMESTAMP)",
		bindVar(driver, 1),
		bindVar(driver, 2),
	)
	if _, err := executor.ExecContext(ctx, insertQuery, migration.Version, migration.Name); err != nil {
		return fmt.Errorf("insert schema_migrations row failed: %w", err)
	}
	return nil
}

func deleteAppliedMigration(ctx context.Context, executor sqlStatementExecutor, driver string, version int) error {
	deleteQuery := fmt.Sprintf("DELETE FROM schema_migrations WHERE version=%s", bindVar(driver, 1))
	if _, err := executor.ExecContext(ctx, deleteQuery, version); err != nil {
		return fmt.Errorf("delete schema_migrations row failed: %w", err)
	}
	return nil
}

func driverSupportsMigrationTransactions(driver string) bool {
	return driver != DriverMySQL
}

func logMigrationEvent(ctx context.Context, logger *slog.Logger, message string, driver string, migration Migration) {
	if logger == nil {
		return
	}
	logger.InfoContext(
		ctx,
		message,
		slog.String("db_driver", driver),
		slog.Int("migration_version", migration.Version),
		slog.String("migration_name", migration.Name),
	)
}

func logMigrationFailure(ctx context.Context, logger *slog.Logger, driver string, migration Migration, err error) {
	if logger == nil {
		return
	}
	logger.ErrorContext(
		ctx,
		"database migration failed",
		slog.String("db_driver", driver),
		slog.Int("migration_version", migration.Version),
		slog.String("migration_name", migration.Name),
		slog.Any("error", err),
	)
}

func executeSQLStatements(ctx context.Context, executor sqlStatementExecutor, sqlText string) error {
	statements := splitSQLStatements(sqlText)
	for _, statement := range statements {
		if _, err := executor.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func splitSQLStatements(sqlText string) []string {
	statements := make([]string, 0, 16)
	var builder strings.Builder

	inSingleQuotes := false
	inDoubleQuotes := false
	inLineComment := false
	inBlockComment := false
	dollarQuoteDelimiter := ""

	for i := 0; i < len(sqlText); {
		current := sqlText[i]
		next := byte(0)
		if i+1 < len(sqlText) {
			next = sqlText[i+1]
		}

		if inLineComment {
			i++
			if current == '\n' {
				inLineComment = false
				builder.WriteByte(current)
			}
			continue
		}
		if inBlockComment {
			i++
			if current == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if dollarQuoteDelimiter != "" {
			if strings.HasPrefix(sqlText[i:], dollarQuoteDelimiter) {
				builder.WriteString(dollarQuoteDelimiter)
				i += len(dollarQuoteDelimiter)
				dollarQuoteDelimiter = ""
				continue
			}
			builder.WriteByte(current)
			i++
			continue
		}

		if !inSingleQuotes && !inDoubleQuotes {
			if current == '-' && next == '-' {
				inLineComment = true
				i += 2
				continue
			}
			if current == '/' && next == '*' {
				inBlockComment = true
				i += 2
				continue
			}
			if delimiter, consumed, ok := readDollarQuoteDelimiter(sqlText[i:]); ok {
				dollarQuoteDelimiter = delimiter
				builder.WriteString(delimiter)
				i += consumed
				continue
			}
		}

		if current == '\'' && !inDoubleQuotes {
			if inSingleQuotes && next == '\'' {
				builder.WriteByte(current)
				builder.WriteByte(next)
				i += 2
				continue
			}
			inSingleQuotes = !inSingleQuotes
		} else if current == '"' && !inSingleQuotes {
			inDoubleQuotes = !inDoubleQuotes
		}

		if current == ';' && !inSingleQuotes && !inDoubleQuotes {
			statement := strings.TrimSpace(builder.String())
			if statement != "" {
				statements = append(statements, statement)
			}
			builder.Reset()
			i++
			continue
		}

		builder.WriteByte(current)
		i++
	}

	last := strings.TrimSpace(builder.String())
	if last != "" {
		statements = append(statements, last)
	}

	return statements
}

func readDollarQuoteDelimiter(sqlText string) (delimiter string, consumed int, ok bool) {
	if len(sqlText) < 2 || sqlText[0] != '$' {
		return "", 0, false
	}

	end := strings.IndexByte(sqlText[1:], '$')
	if end < 0 {
		return "", 0, false
	}

	tag := sqlText[1 : 1+end]
	if tag != "" {
		for i := 0; i < len(tag); i++ {
			ch := tag[i]
			if !(ch == '_' || ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z') {
				return "", 0, false
			}
		}
		if tag[0] >= '0' && tag[0] <= '9' {
			return "", 0, false
		}
	}

	return "$" + tag + "$", len(tag) + 2, true
}

func parseMigrationFilename(filename string) (version int, name string, direction string, err error) {
	parts := strings.Split(filename, ".")
	if len(parts) != 3 || parts[2] != "sql" {
		return 0, "", "", fmt.Errorf("filename must match <version>_<name>.<up|down>.sql")
	}

	direction = parts[1]
	if direction != "up" && direction != "down" {
		return 0, "", "", fmt.Errorf("filename direction must be up/down")
	}

	head := parts[0]
	separatorIndex := strings.Index(head, "_")
	if separatorIndex <= 0 || separatorIndex == len(head)-1 {
		return 0, "", "", fmt.Errorf("filename head must match <version>_<name>")
	}

	versionText := head[:separatorIndex]
	name = head[separatorIndex+1:]

	version, err = strconv.Atoi(versionText)
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid version %q", versionText)
	}
	if version <= 0 {
		return 0, "", "", fmt.Errorf("version must be greater than 0")
	}

	return version, name, direction, nil
}

func bindVar(driver string, position int) string {
	if driver == DriverPostgres {
		return fmt.Sprintf("$%d", position)
	}
	return "?"
}
