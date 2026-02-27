package config

import "testing"

func TestLoad_Defaults(t *testing.T) {
	// 中文注释：显式清空关键环境变量，验证默认值是否可用。
	t.Setenv("APP_ENV", "")
	t.Setenv("APP_ADDR", "")
	t.Setenv("WEB_ORIGIN", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_DSN", "")
	t.Setenv("JWT_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Addr != ":8080" {
		t.Fatalf("expected default addr :8080, got %s", cfg.Addr)
	}
	if cfg.WebOrigin != "http://localhost:3001" {
		t.Fatalf("expected default origin http://localhost:3001, got %s", cfg.WebOrigin)
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid LOG_LEVEL, got nil")
	}
}

func TestLoad_ProductionRequiresJWTSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is default in production")
	}
}

func TestLoad_UnsupportedDBDriver(t *testing.T) {
	t.Setenv("DB_DRIVER", "oracle")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for unsupported DB_DRIVER")
	}
}

func TestLoad_UnsupportedLogOutput(t *testing.T) {
	t.Setenv("LOG_OUTPUT", "console")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for unsupported LOG_OUTPUT")
	}
}

func TestLoad_FileOutputRequiresPath(t *testing.T) {
	t.Setenv("LOG_OUTPUT", "file")
	t.Setenv("LOG_FILE_PATH", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when LOG_FILE_PATH is empty for file output")
	}
}

func TestLoad_InvalidAutoMigrateBool(t *testing.T) {
	t.Setenv("DB_AUTO_MIGRATE", "maybe")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid DB_AUTO_MIGRATE value")
	}
}

func TestLoad_LDAPDefaultProviderRequiresEnable(t *testing.T) {
	t.Setenv("AUTH_DEFAULT_PROVIDER", "ldap")
	t.Setenv("AUTH_LDAP_ENABLED", "false")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when ldap default provider is enabled but AUTH_LDAP_ENABLED is false")
	}
}

func TestLoad_LDAPEnabledRequiresBaseDN(t *testing.T) {
	t.Setenv("AUTH_LDAP_ENABLED", "true")
	t.Setenv("AUTH_LDAP_HOST", "ldap.example.com")
	t.Setenv("AUTH_LDAP_BASE_DN", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when AUTH_LDAP_BASE_DN is empty")
	}
}

func TestLoad_LDAPConfigValid(t *testing.T) {
	t.Setenv("AUTH_LDAP_ENABLED", "true")
	t.Setenv("AUTH_LDAP_HOST", "ldap.example.com")
	t.Setenv("AUTH_LDAP_BASE_DN", "dc=example,dc=com")
	t.Setenv("AUTH_LDAP_USER_FILTER", "(mail=%s)")
	t.Setenv("AUTH_LDAP_TLS_MODE", "ldaps")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid ldap config, got err=%v", err)
	}
	if !cfg.Auth.LDAP.Enabled {
		t.Fatal("expected ldap enabled")
	}
	if cfg.Auth.LDAP.Host != "ldap.example.com" {
		t.Fatalf("expected ldap host ldap.example.com, got %s", cfg.Auth.LDAP.Host)
	}
}
