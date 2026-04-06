package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormDataRetentionRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_data_retention_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"audit_logs"`,
		`"auth_captcha_challenges"`,
		`"auth_risk_states"`,
		`"user_sessions"`,
		`"id"`,
		`"created_at"`,
		`"expires_at"`,
		`"updated_at"`,
		`"lock_until"`,
		`"revoked_at"`,
	}

	source := string(content)
	for _, literal := range forbiddenLiterals {
		if strings.Contains(source, literal) {
			t.Fatalf("repository source still contains hardcoded table metadata literal %s", literal)
		}
	}
}
