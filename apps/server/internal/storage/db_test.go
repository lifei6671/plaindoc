package storage

import (
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
