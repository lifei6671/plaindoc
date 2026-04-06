package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormDocumentImageAssetRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_document_image_asset_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"document_image_assets"`,
		`"document_image_assets AS dia"`,
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
