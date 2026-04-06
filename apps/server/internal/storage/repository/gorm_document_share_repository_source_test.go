package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormDocumentShareRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_document_share_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"document_shares"`,
		`"document_shares AS ds"`,
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
