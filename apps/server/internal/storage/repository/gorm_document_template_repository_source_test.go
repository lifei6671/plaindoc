package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormDocumentTemplateRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_document_template_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"document_templates AS t"`,
		`"document_template_scenes AS s"`,
		`"template_id"`,
		`"scene_key"`,
		`"scene_name"`,
		`"default_title"`,
		`"content_md"`,
		`"created_by_user_id"`,
		`"updated_by_user_id"`,
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
