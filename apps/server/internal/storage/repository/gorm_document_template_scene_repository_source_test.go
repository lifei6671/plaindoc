package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormDocumentTemplateSceneRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_document_template_scene_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"document_template_scenes AS s"`,
		`"document_templates AS t"`,
		`"scene_key"`,
		`"scene_name"`,
		`"created_by_user_id"`,
		`"updated_by_user_id"`,
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
