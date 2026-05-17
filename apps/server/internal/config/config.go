package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 统一承载服务启动所需配置，避免业务层分散读取环境变量。
type Config struct {
	Env            string
	Addr           string
	WebOrigin      string
	WebDistDir     string
	LogLevel       slog.Level
	LogOutput      string
	LogFilePath    string
	RequestTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	Database       DatabaseConfig
	JWT            JWTConfig
	Auth           AuthConfig
	SSRWorker      SSRWorkerConfig
}

// DatabaseConfig 预留多数据库接入所需参数。
type DatabaseConfig struct {
	Driver      string
	DSN         string
	AutoMigrate bool
	LogSQL      bool
}

// JWTConfig 为后续认证阶段提供密钥与过期时间基线。
type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// AuthConfig 认证入口配置（当前用于 local/ldap provider 选择与 LDAP 连接参数）。
type AuthConfig struct {
	DefaultProvider string
	LDAP            LDAPConfig
}

// LDAPConfig LDAP provider 配置。
type LDAPConfig struct {
	Enabled            bool
	ProviderID         string
	Host               string
	Port               int
	TLSMode            string
	BaseDN             string
	BindDN             string
	BindPassword       string
	UserFilter         string
	IDAttribute        string
	EmailAttribute     string
	NameAttribute      string
	ConnectTimeout     time.Duration
	ReadTimeout        time.Duration
	InsecureSkipVerify bool
}

// SSRWorkerConfig 描述 Go 进程内管理的 Node SSR 子进程配置。
type SSRWorkerConfig struct {
	Enabled          bool
	Exec             string
	Entry            string
	Count            int
	RenderTimeout    time.Duration
	StartTimeout     time.Duration
	MaxPayloadBytes  int64
	MaxResponseBytes int64
	ProtocolVersion  string
}

// Load 解析并校验环境变量；配置非法时返回明确错误，避免服务带病启动。
func Load() (Config, error) {
	cfg := Config{
		Env:        getenv("APP_ENV", "development"),
		Addr:       getenv("APP_ADDR", ":8080"),
		WebOrigin:  getenv("WEB_ORIGIN", "http://localhost:3001"),
		WebDistDir: getenv("WEB_DIST_DIR", "apps/web/dist"),
		Database: DatabaseConfig{
			Driver: getenv("DB_DRIVER", "sqlite"),
			DSN:    getenv("DB_DSN", "file:plaindoc.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"),
		},
		JWT: JWTConfig{
			Secret: getenv("JWT_SECRET", "plaindoc-dev-secret"),
		},
		Auth: AuthConfig{
			DefaultProvider: strings.ToLower(getenv("AUTH_DEFAULT_PROVIDER", "local")),
			LDAP: LDAPConfig{
				ProviderID:     getenv("AUTH_LDAP_PROVIDER_ID", "ldap"),
				Host:           getenv("AUTH_LDAP_HOST", ""),
				Port:           636,
				TLSMode:        strings.ToLower(getenv("AUTH_LDAP_TLS_MODE", "ldaps")),
				BaseDN:         getenv("AUTH_LDAP_BASE_DN", ""),
				BindDN:         getenv("AUTH_LDAP_BIND_DN", ""),
				BindPassword:   os.Getenv("AUTH_LDAP_BIND_PASSWORD"),
				UserFilter:     getenv("AUTH_LDAP_USER_FILTER", "(mail=%s)"),
				IDAttribute:    getenv("AUTH_LDAP_ID_ATTRIBUTE", "entryUUID"),
				EmailAttribute: getenv("AUTH_LDAP_EMAIL_ATTRIBUTE", "mail"),
				NameAttribute:  getenv("AUTH_LDAP_NAME_ATTRIBUTE", "cn"),
			},
		},
		SSRWorker: SSRWorkerConfig{
			Exec:             getenv("SSR_WORKER_EXEC", "node"),
			Entry:            getenv("SSR_WORKER_ENTRY", ""),
			Count:            2,
			RenderTimeout:    1500 * time.Millisecond,
			StartTimeout:     5 * time.Second,
			MaxPayloadBytes:  1024 * 1024,
			MaxResponseBytes: 16 * 1024 * 1024,
			ProtocolVersion:  getenv("SSR_PROTOCOL_VERSION", "v1"),
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

	logSQL, err := parseBool("GORM_LOG_SQL", "false")
	if err != nil {
		return Config{}, err
	}
	cfg.Database.LogSQL = logSQL

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

	ldapEnabled, err := parseBool("AUTH_LDAP_ENABLED", "false")
	if err != nil {
		return Config{}, err
	}
	cfg.Auth.LDAP.Enabled = ldapEnabled

	defaultLDAPPort := "636"
	switch strings.ToLower(strings.TrimSpace(cfg.Auth.LDAP.TLSMode)) {
	case "starttls", "plain":
		defaultLDAPPort = "389"
	}
	ldapPort, err := parseInt("AUTH_LDAP_PORT", defaultLDAPPort)
	if err != nil {
		return Config{}, err
	}
	cfg.Auth.LDAP.Port = ldapPort
	cfg.Auth.LDAP.ProviderID = strings.TrimSpace(cfg.Auth.LDAP.ProviderID)
	cfg.Auth.LDAP.Host = strings.TrimSpace(cfg.Auth.LDAP.Host)
	cfg.Auth.LDAP.TLSMode = strings.ToLower(strings.TrimSpace(cfg.Auth.LDAP.TLSMode))
	cfg.Auth.LDAP.BaseDN = strings.TrimSpace(cfg.Auth.LDAP.BaseDN)
	cfg.Auth.LDAP.BindDN = strings.TrimSpace(cfg.Auth.LDAP.BindDN)
	cfg.Auth.LDAP.UserFilter = strings.TrimSpace(cfg.Auth.LDAP.UserFilter)
	cfg.Auth.LDAP.IDAttribute = strings.TrimSpace(cfg.Auth.LDAP.IDAttribute)
	cfg.Auth.LDAP.EmailAttribute = strings.TrimSpace(cfg.Auth.LDAP.EmailAttribute)
	cfg.Auth.LDAP.NameAttribute = strings.TrimSpace(cfg.Auth.LDAP.NameAttribute)

	ldapConnectTimeout, err := parseDuration("AUTH_LDAP_CONNECT_TIMEOUT", "3s")
	if err != nil {
		return Config{}, err
	}
	cfg.Auth.LDAP.ConnectTimeout = ldapConnectTimeout

	ldapReadTimeout, err := parseDuration("AUTH_LDAP_READ_TIMEOUT", "3s")
	if err != nil {
		return Config{}, err
	}
	cfg.Auth.LDAP.ReadTimeout = ldapReadTimeout

	ldapInsecureSkipVerify, err := parseBool("AUTH_LDAP_INSECURE_SKIP_VERIFY", "false")
	if err != nil {
		return Config{}, err
	}
	cfg.Auth.LDAP.InsecureSkipVerify = ldapInsecureSkipVerify
	cfg.Auth.DefaultProvider = strings.ToLower(strings.TrimSpace(cfg.Auth.DefaultProvider))

	ssrWorkerEnabled, err := parseBool("SSR_WORKER_ENABLED", "false")
	if err != nil {
		return Config{}, err
	}
	cfg.SSRWorker.Enabled = ssrWorkerEnabled

	ssrWorkerCount, err := parseInt("SSR_WORKER_COUNT", "2")
	if err != nil {
		return Config{}, err
	}
	cfg.SSRWorker.Count = ssrWorkerCount

	ssrRenderTimeout, err := parseDuration("SSR_RENDER_TIMEOUT", "1500ms")
	if err != nil {
		return Config{}, err
	}
	cfg.SSRWorker.RenderTimeout = ssrRenderTimeout

	ssrWorkerStartTimeout, err := parseDuration("SSR_WORKER_START_TIMEOUT", "5s")
	if err != nil {
		return Config{}, err
	}
	cfg.SSRWorker.StartTimeout = ssrWorkerStartTimeout

	ssrWorkerMaxPayloadBytes, err := parseByteSize("SSR_WORKER_MAX_PAYLOAD_BYTES", "1MB")
	if err != nil {
		return Config{}, err
	}
	cfg.SSRWorker.MaxPayloadBytes = ssrWorkerMaxPayloadBytes

	ssrWorkerMaxResponseBytes, err := parseByteSize("SSR_WORKER_MAX_RESPONSE_BYTES", "16MB")
	if err != nil {
		return Config{}, err
	}
	cfg.SSRWorker.MaxResponseBytes = ssrWorkerMaxResponseBytes

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
	if c.WebDistDir == "" {
		return errors.New("WEB_DIST_DIR must not be empty")
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
	switch c.Auth.DefaultProvider {
	case "local":
	case "ldap":
		if !c.Auth.LDAP.Enabled {
			return errors.New("AUTH_DEFAULT_PROVIDER=ldap requires AUTH_LDAP_ENABLED=true")
		}
	default:
		return fmt.Errorf("AUTH_DEFAULT_PROVIDER must be local/ldap, got %q", c.Auth.DefaultProvider)
	}
	if c.Auth.LDAP.Enabled {
		if c.Auth.LDAP.ProviderID == "" {
			return errors.New("AUTH_LDAP_PROVIDER_ID must not be empty when AUTH_LDAP_ENABLED is true")
		}
		if c.Auth.LDAP.Host == "" {
			return errors.New("AUTH_LDAP_HOST must not be empty when AUTH_LDAP_ENABLED is true")
		}
		if c.Auth.LDAP.Port <= 0 {
			return errors.New("AUTH_LDAP_PORT must be greater than 0 when AUTH_LDAP_ENABLED is true")
		}
		switch strings.ToLower(strings.TrimSpace(c.Auth.LDAP.TLSMode)) {
		case "ldaps", "starttls", "plain":
		default:
			return fmt.Errorf("AUTH_LDAP_TLS_MODE must be ldaps/starttls/plain, got %q", c.Auth.LDAP.TLSMode)
		}
		if c.Auth.LDAP.BaseDN == "" {
			return errors.New("AUTH_LDAP_BASE_DN must not be empty when AUTH_LDAP_ENABLED is true")
		}
		if !strings.Contains(c.Auth.LDAP.UserFilter, "%s") {
			return errors.New("AUTH_LDAP_USER_FILTER must include %s placeholder")
		}
		if c.Auth.LDAP.IDAttribute == "" || c.Auth.LDAP.EmailAttribute == "" || c.Auth.LDAP.NameAttribute == "" {
			return errors.New("AUTH_LDAP_*_ATTRIBUTE must not be empty when AUTH_LDAP_ENABLED is true")
		}
		if c.Auth.LDAP.ConnectTimeout <= 0 || c.Auth.LDAP.ReadTimeout <= 0 {
			return errors.New("AUTH_LDAP_*_TIMEOUT must be greater than 0 when AUTH_LDAP_ENABLED is true")
		}
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
	if c.SSRWorker.Enabled {
		if strings.TrimSpace(c.SSRWorker.Exec) == "" {
			return errors.New("SSR_WORKER_EXEC must not be empty when SSR_WORKER_ENABLED is true")
		}
		if strings.TrimSpace(c.SSRWorker.Entry) == "" {
			return errors.New("SSR_WORKER_ENTRY must not be empty when SSR_WORKER_ENABLED is true")
		}
		if c.SSRWorker.Count <= 0 {
			return errors.New("SSR_WORKER_COUNT must be greater than 0 when SSR_WORKER_ENABLED is true")
		}
		if c.SSRWorker.RenderTimeout <= 0 {
			return errors.New("SSR_RENDER_TIMEOUT must be greater than 0 when SSR_WORKER_ENABLED is true")
		}
		if c.SSRWorker.StartTimeout <= 0 {
			return errors.New("SSR_WORKER_START_TIMEOUT must be greater than 0 when SSR_WORKER_ENABLED is true")
		}
		if c.SSRWorker.MaxPayloadBytes <= 0 {
			return errors.New("SSR_WORKER_MAX_PAYLOAD_BYTES must be greater than 0 when SSR_WORKER_ENABLED is true")
		}
		if c.SSRWorker.MaxResponseBytes <= 0 {
			return errors.New("SSR_WORKER_MAX_RESPONSE_BYTES must be greater than 0 when SSR_WORKER_ENABLED is true")
		}
		if strings.TrimSpace(c.SSRWorker.ProtocolVersion) == "" {
			return errors.New("SSR_PROTOCOL_VERSION must not be empty when SSR_WORKER_ENABLED is true")
		}
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

func parseInt(key string, fallback string) (int, error) {
	raw := strings.TrimSpace(getenv(key, fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer value, got %q", key, raw)
	}
	return value, nil
}

func parseByteSize(key string, fallback string) (int64, error) {
	raw := strings.TrimSpace(getenv(key, fallback))
	normalized := strings.ToUpper(strings.ReplaceAll(raw, " ", ""))
	if normalized == "" {
		normalized = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(fallback), " ", ""))
	}

	multiplier := int64(1)
	numberPart := normalized
	for _, unit := range []struct {
		suffix     string
		multiplier int64
	}{
		{suffix: "GIB", multiplier: 1024 * 1024 * 1024},
		{suffix: "GB", multiplier: 1024 * 1024 * 1024},
		{suffix: "G", multiplier: 1024 * 1024 * 1024},
		{suffix: "MIB", multiplier: 1024 * 1024},
		{suffix: "MB", multiplier: 1024 * 1024},
		{suffix: "M", multiplier: 1024 * 1024},
		{suffix: "KIB", multiplier: 1024},
		{suffix: "KB", multiplier: 1024},
		{suffix: "K", multiplier: 1024},
		{suffix: "B", multiplier: 1},
	} {
		if strings.HasSuffix(normalized, unit.suffix) {
			multiplier = unit.multiplier
			numberPart = strings.TrimSuffix(normalized, unit.suffix)
			break
		}
	}

	value, err := strconv.ParseInt(numberPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid byte size, got %q", key, raw)
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	if value > maxInt64/multiplier {
		return 0, fmt.Errorf("%s byte size is too large, got %q", key, raw)
	}
	return value * multiplier, nil
}

func getenv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
