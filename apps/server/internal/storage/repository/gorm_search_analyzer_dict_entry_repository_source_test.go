package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormSearchAnalyzerDictEntryRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_search_analyzer_dict_entry_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"search_analyzer_dict_entries"`,
	}

	source := string(content)
	for _, literal := range forbiddenLiterals {
		if strings.Contains(source, literal) {
			t.Fatalf("repository source still contains hardcoded table metadata literal %s", literal)
		}
	}
}
