package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormSpaceCategoryRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_space_category_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"space_categories"`,
		`"category_id"`,
		`"is_default"`,
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
