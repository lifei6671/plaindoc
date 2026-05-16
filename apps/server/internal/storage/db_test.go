package storage

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNormalizeDriver(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{name: "sqlite", input: "sqlite", expected: DriverSQLite},
		{name: "postgres alias", input: "postgresql", expected: DriverPostgres},
		{name: "mysql", input: "mysql", expected: DriverMySQL},
		{name: "unsupported", input: "oracle", wantErr: true},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			got, err := NormalizeDriver(testCase.input)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != testCase.expected {
				t.Fatalf("expected %q, got %q", testCase.expected, got)
			}
		})
	}
}

func TestOpenDatabase_SQLiteMemory(t *testing.T) {
	database, err := OpenDatabase(OpenConfig{
		Driver: DriverSQLite,
		DSN:    "file:test-open-database?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	if database.Driver != DriverSQLite {
		t.Fatalf("expected driver %q, got %q", DriverSQLite, database.Driver)
	}
	if database.ORM == nil || database.SQL == nil {
		t.Fatal("expected gorm/sql db both initialized")
	}
}

func TestOpenDatabase_GormSQLLoggingDisabledByDefault(t *testing.T) {
	var buffer bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(previousLogger)

	database, err := OpenDatabase(OpenConfig{
		Driver: DriverSQLite,
		DSN:    "file:test-gorm-sql-log-disabled?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	if err := database.ORM.Exec("SELECT 1").Error; err != nil {
		t.Fatalf("exec query failed: %v", err)
	}
	if strings.Contains(buffer.String(), "SQL executed") {
		t.Fatalf("expected gorm sql log disabled by default, got logs:\n%s", buffer.String())
	}
}

func TestOpenDatabase_GormSQLLoggingEnabled(t *testing.T) {
	var buffer bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(previousLogger)

	database, err := OpenDatabase(OpenConfig{
		Driver: DriverSQLite,
		DSN:    "file:test-gorm-sql-log-enabled?mode=memory&cache=shared",
		LogSQL: true,
	})
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	if err := database.ORM.Exec("SELECT 1").Error; err != nil {
		t.Fatalf("exec query failed: %v", err)
	}
	if !strings.Contains(buffer.String(), "SQL executed") {
		t.Fatalf("expected gorm sql log when enabled, got logs:\n%s", buffer.String())
	}
}
