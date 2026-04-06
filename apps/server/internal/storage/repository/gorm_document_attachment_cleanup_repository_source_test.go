package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormDocumentAttachmentCleanupRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_document_attachment_cleanup_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"document_attachments AS da"`,
		`"documents AS d"`,
		`"attachment_id"`,
		`"blob_id"`,
		`"document_id"`,
		`"status"`,
		`"deleted_at"`,
		`gorm:"column:`,
	}

	source := string(content)
	for _, literal := range forbiddenLiterals {
		if strings.Contains(source, literal) {
			t.Fatalf("repository source still contains hardcoded table metadata literal %s", literal)
		}
	}
}
