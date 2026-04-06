package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormUserSessionRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_user_session_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"session_id"`,
		`"user_id"`,
		`"refresh_token_hash"`,
		`"expires_at"`,
		`"revoked_at"`,
		`"replaced_by_session_id"`,
		`"updated_at"`,
	}

	source := string(content)
	for _, literal := range forbiddenLiterals {
		if strings.Contains(source, literal) {
			t.Fatalf("repository source still contains hardcoded table metadata literal %s", literal)
		}
	}
}
