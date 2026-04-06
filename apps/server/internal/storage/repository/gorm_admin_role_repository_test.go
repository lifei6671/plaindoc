package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormAdminRoleRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_admin_role_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"user_admin_roles"`,
		`"user_id"`,
		`"role"`,
		`Where(map[string]any{`,
	}

	source := string(content)
	for _, literal := range forbiddenLiterals {
		if strings.Contains(source, literal) {
			t.Fatalf("repository source still contains hardcoded table metadata literal %s", literal)
		}
	}
}
