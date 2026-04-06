package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormAuditLogRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_audit_log_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"audit_logs AS al"`,
		`"users AS u"`,
		`"actor_user_id"`,
		`"target_type"`,
		`"target_id"`,
		`"detail_json"`,
		`"request_id"`,
		`gorm:"column:`,
		`func parseAuditLogRecordTime(`,
	}

	source := string(content)
	for _, literal := range forbiddenLiterals {
		if strings.Contains(source, literal) {
			t.Fatalf("repository source still contains hardcoded table metadata literal %s", literal)
		}
	}
}
