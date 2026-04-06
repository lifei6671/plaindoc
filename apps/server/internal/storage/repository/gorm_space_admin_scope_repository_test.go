package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormSpaceAdminScopeRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_space_admin_scope_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"space_admin_scopes"`,
		`"user_id"`,
		`"space_id"`,
		`+" = ? AND "+`,
	}

	source := string(content)
	for _, literal := range forbiddenLiterals {
		if strings.Contains(source, literal) {
			t.Fatalf("repository source still contains hardcoded table metadata literal %s", literal)
		}
	}
}
