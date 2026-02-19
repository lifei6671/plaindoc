package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Config 统一承载服务启动所需配置，避免业务层分散读取环境变量。
type Config struct {
	Env            string
	Addr           string
	WebOrigin      string
	LogLevel       slog.Level
	LogOutput      string
	LogFilePath    string
	RequestTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	Database       DatabaseConfig
	JWT            JWTConfig
}

// DatabaseConfig 预留多数据库接入所需参数。
type DatabaseConfig struct {
	Driver      string
	DSN         string
	AutoMigrate bool
}

// JWTConfig 为后续认证阶段提供密钥与过期时间基线。
type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// Load 解析并校验环境变量；配置非法时返回明确错误，避免服务带病启动。
func Load() (Config, error) {
	cfg := Config{
		Env:       getenv("APP_ENV", "development"),
		Addr:      getenv("APP_ADDR", ":8080"),
		WebOrigin: getenv("WEB_ORIGIN", "http://localhost:5173"),
		Database: DatabaseConfig{
			Driver: getenv("DB_DRIVER", "sqlite"),
			DSN:    getenv("DB_DSN", "file:plaindoc.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"),
		},
		JWT: JWTConfig{
			Secret: getenv("JWT_SECRET", "plaindoc-dev-secret"),
		},
	}

	logLevel, err := parseLogLevel(getenv("LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = logLevel
	cfg.LogOutput = strings.ToLower(getenv("LOG_OUTPUT", "stdout"))
	cfg.LogFilePath = getenv("LOG_FILE_PATH", "")

	autoMigrate, err := parseBool("DB_AUTO_MIGRATE", "true")
	if err != nil {
		return Config{}, err
	}
	cfg.Database.AutoMigrate = autoMigrate

	requestTimeout, err := parseDuration("REQUEST_TIMEOUT", "10s")
	if err != nil {
		return Config{}, err
	}
	cfg.RequestTimeout = requestTimeout

	readTimeout, err := parseDuration("HTTP_READ_TIMEOUT", "15s")
	if err != nil {
		return Config{}, err
	}
	cfg.ReadTimeout = readTimeout

	writeTimeout, err := parseDuration("HTTP_WRITE_TIMEOUT", "15s")
	if err != nil {
		return Config{}, err
	}
	cfg.WriteTimeout = writeTimeout

	idleTimeout, err := parseDuration("HTTP_IDLE_TIMEOUT", "60s")
	if err != nil {
		return Config{}, err
	}
	cfg.IdleTimeout = idleTimeout

	accessTokenTTL, err := parseDuration("JWT_ACCESS_TOKEN_TTL", "15m")
	if err != nil {
		return Config{}, err
	}
	cfg.JWT.AccessTokenTTL = accessTokenTTL

	refreshTokenTTL, err := parseDuration("JWT_REFRESH_TOKEN_TTL", "168h")
	if err != nil {
		return Config{}, err
	}
	cfg.JWT.RefreshTokenTTL = refreshTokenTTL

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate 对关键字段做兜底校验，尽早暴露配置问题。
func (c Config) Validate() error {
	if c.Addr == "" {
		return errors.New("APP_ADDR must not be empty")
	}
	if c.WebOrigin == "" {
		return errors.New("WEB_ORIGIN must not be empty")
	}
	if c.Database.DSN == "" {
		return errors.New("DB_DSN must not be empty")
	}

	driver := strings.ToLower(c.Database.Driver)
	switch driver {
	case "sqlite", "postgres", "mysql":
	default:
		return fmt.Errorf("unsupported DB_DRIVER %q, expected sqlite/postgres/mysql", c.Database.Driver)
	}

	if c.RequestTimeout <= 0 {
		return errors.New("REQUEST_TIMEOUT must be greater than 0")
	}
	if c.ReadTimeout <= 0 || c.WriteTimeout <= 0 || c.IdleTimeout <= 0 {
		return errors.New("HTTP timeouts must be greater than 0")
	}
	if c.JWT.AccessTokenTTL <= 0 || c.JWT.RefreshTokenTTL <= 0 {
		return errors.New("JWT token TTL must be greater than 0")
	}
	switch c.LogOutput {
	case "stdout", "file", "both":
	default:
		return fmt.Errorf("LOG_OUTPUT must be one of stdout/file/both, got %q", c.LogOutput)
	}
	if (c.LogOutput == "file" || c.LogOutput == "both") && c.LogFilePath == "" {
		return errors.New("LOG_FILE_PATH must not be empty when LOG_OUTPUT is file/both")
	}
	if c.Env == "production" && c.JWT.Secret == "plaindoc-dev-secret" {
		return errors.New("JWT_SECRET must be explicitly set in production")
	}

	return nil
}

func parseDuration(key string, fallback string) (time.Duration, error) {
	raw := getenv(key, fallback)
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration, got %q", key, raw)
	}
	return value, nil
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL must be one of debug/info/warn/error, got %q", value)
	}
}

func parseBool(key string, fallback string) (bool, error) {
	raw := strings.ToLower(strings.TrimSpace(getenv(key, fallback)))
	switch raw {
	case "1", "t", "true", "y", "yes", "on":
		return true, nil
	case "0", "f", "false", "n", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be a valid boolean value, got %q", key, raw)
	}
}

func getenv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
