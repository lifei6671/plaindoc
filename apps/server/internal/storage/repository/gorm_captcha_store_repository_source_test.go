package repository

import (
	"os"
	"strings"
	"testing"
)

func TestGormCaptchaStoreRepository_AvoidsHardcodedTableStructure(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("gorm_captcha_store_repository.go")
	if err != nil {
		t.Fatalf("read repository source failed: %v", err)
	}

	forbiddenLiterals := []string{
		`"auth_captcha_challenges"`,
		`"captcha_id"`,
		`"answer_hash"`,
		`"expires_at"`,
		`"created_at"`,
		`"updated_at"`,
		`"scene"`,
		`"subject_hash"`,
		`"answer_salt"`,
		`"issued_ip_hash"`,
		`"consumed_at"`,
		`"failed_verify_count"`,
		`gorm:"column:`,
		`parseRecordTime(`,
	}

	source := string(content)
	for _, literal := range forbiddenLiterals {
		if strings.Contains(source, literal) {
			t.Fatalf("repository source still contains hardcoded table metadata literal %s", literal)
		}
	}
}
