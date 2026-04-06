package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormDocumentImageAssetLifecycleRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_document_image_asset_lifecycle_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"document_image_assets"`,
		`"documents"`,
		`"file_blobs"`,
		`"id"`,
		`"blob_id"`,
		`"storage_provider"`,
		`"object_key"`,
		`"document_id"`,
		`"space_id"`,
		`"object_url"`,
		`"status"`,
		`"pending_cleanup_at"`,
		`"deleted_at"`,
		`"last_referenced_at"`,
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
