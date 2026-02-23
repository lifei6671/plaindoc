package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	gormmysql "gorm.io/driver/mysql"
	gormpostgres "gorm.io/driver/postgres"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	// 中文注释：注册 modernc sqlite 驱动，供 GORM SQLite 方言使用。
	_ "modernc.org/sqlite"
)

const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
	DriverMySQL    = "mysql"
)

// OpenConfig 定义数据库连接入参，独立于配置模块便于测试复用。
type OpenConfig struct {
	Driver string
	DSN    string
}

// Database 包含 GORM 连接和底层 sql.DB，便于迁移与仓储层复用。
type Database struct {
	ORM    *gorm.DB
	SQL    *sql.DB
	Driver string
}

// Close 关闭数据库底层连接池。
func (d *Database) Close() error {
	if d == nil || d.SQL == nil {
		return nil
	}
	return d.SQL.Close()
}

// OpenDatabase 打开数据库并完成基础连通性检查（ORM 使用 GORM）。
func OpenDatabase(cfg OpenConfig) (*Database, error) {
	normalizedDriver, err := NormalizeDriver(cfg.Driver)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("database dsn must not be empty")
	}

	dialector, err := resolveDialector(normalizedDriver, cfg.DSN)
	if err != nil {
		return nil, err
	}

	ormDB, err := gorm.Open(dialector, &gorm.Config{
		// 中文注释：SQL 输出统一交给现有日志中间件处理，关闭 GORM 默认日志输出。
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open gorm database failed: %w", err)
	}

	sqlDB, err := ormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql db failed: %w", err)
	}

	applyDefaultPoolConfig(sqlDB, normalizedDriver)

	// 中文注释：启动阶段先做一次 ping，尽早发现连接或认证错误。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping database failed: %w", err)
	}

	return &Database{
		ORM:    ormDB,
		SQL:    sqlDB,
		Driver: normalizedDriver,
	}, nil
}

// NormalizeDriver 统一数据库驱动标识，避免配置别名造成分支复杂化。
func NormalizeDriver(raw string) (string, error) {
	driver := strings.ToLower(strings.TrimSpace(raw))
	switch driver {
	case DriverSQLite:
		return DriverSQLite, nil
	case DriverPostgres, "postgresql":
		return DriverPostgres, nil
	case DriverMySQL:
		return DriverMySQL, nil
	default:
		return "", fmt.Errorf("unsupported database driver %q", raw)
	}
}

func resolveDialector(driver string, dsn string) (gorm.Dialector, error) {
	switch driver {
	case DriverSQLite:
		// 中文注释：显式指定 DriverName=sqlite，使 GORM 走 modernc.org/sqlite 驱动。
		return gormsqlite.New(gormsqlite.Config{
			DriverName: "sqlite",
			DSN:        dsn,
		}), nil
	case DriverPostgres:
		return gormpostgres.Open(dsn), nil
	case DriverMySQL:
		return gormmysql.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
}

func applyDefaultPoolConfig(db *sql.DB, driver string) {
	switch driver {
	case DriverSQLite:
		// 中文注释：编辑器存在“写后立即读”（PUT 后 GET）场景。
		// 在开启 WAL 的前提下使用小连接池可显著降低串行排队耗时。
		// 这里保持保守并发（4/2），避免过高并发触发锁竞争放大。
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(2)
		db.SetConnMaxLifetime(0)
		db.SetConnMaxIdleTime(5 * time.Minute)
	case DriverPostgres, DriverMySQL:
		db.SetMaxOpenConns(20)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(30 * time.Minute)
		db.SetConnMaxIdleTime(5 * time.Minute)
	default:
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(2)
	}
}
