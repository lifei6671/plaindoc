package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormDocumentAttachmentRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_document_attachment_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"document_attachments"`,
		`"document_attachments AS da"`,
		`"file_blobs"`,
		`parseRecordTime(`,
		`parseNullableRecordTime(`,
	}

	source := string(content)
	for _, literal := range forbiddenLiterals {
		if strings.Contains(source, literal) {
			t.Fatalf("repository source still contains hardcoded table metadata literal %s", literal)
		}
	}
}
