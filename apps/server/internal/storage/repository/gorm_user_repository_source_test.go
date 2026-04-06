package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormUserRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_user_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"id, user_id, email, password_hash, name, avatar_url, status, banned_reason, banned_at, deleted_at"`,
		`"created_at DESC"`,
		`"user_id"`,
		`"email"`,
		`"password_hash"`,
		`"name"`,
		`"avatar_url"`,
		`"status"`,
		`"banned_reason"`,
		`"banned_at"`,
		`"deleted_at"`,
		`"created_at"`,
		`"updated_at"`,
		`gorm:"column:`,
	}

	source := string(content)
	for _, literal := range forbiddenLiterals {
		if strings.Contains(source, literal) {
			t.Fatalf("repository source still contains hardcoded table metadata literal %s", literal)
		}
	}
}
