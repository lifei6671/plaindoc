package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

func TestDataRetentionCleanupService_RunOnce(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-data-retention-cleanup?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	ctx := context.Background()
	if err := storage.MigrateUp(ctx, database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}

	now := time.Now().UTC()
	oldTime := now.AddDate(0, 0, -45)
	recentTime := now.AddDate(0, 0, -2)

	if err := database.ORM.WithContext(ctx).Table("users").Create(map[string]any{
		"user_id":       "01hretentionuser000000000001",
		"email":         "retention-user@example.com",
		"password_hash": "hashed-password",
		"name":          "Retention User",
		"created_at":    oldTime,
		"updated_at":    oldTime,
	}).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Table("user_identities").Create(map[string]any{
		"user_id":       "01hretentionuser000000000001",
		"provider_type": "local",
		"provider_id":   "local",
		"external_id":   "retention-user@example.com",
		"login_name":    "retention-user@example.com",
		"created_at":    oldTime,
		"updated_at":    oldTime,
	}).Error; err != nil {
		t.Fatalf("seed user identity failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Table("audit_logs").Create([]map[string]any{
		{
			"actor_user_id": nil,
			"module":        "system_config",
			"action":        "update",
			"target_type":   "system_config",
			"target_id":     "old-record",
			"summary":       "old audit log",
			"detail_json":   "{}",
			"request_id":    "req-old-audit",
			"created_at":    oldTime,
		},
		{
			"actor_user_id": nil,
			"module":        "system_config",
			"action":        "update",
			"target_type":   "system_config",
			"target_id":     "new-record",
			"summary":       "new audit log",
			"detail_json":   "{}",
			"request_id":    "req-new-audit",
			"created_at":    now,
		},
	}).Error; err != nil {
		t.Fatalf("seed audit logs failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Table("auth_captcha_challenges").Create([]map[string]any{
		{
			"captcha_id":          "captcha-old",
			"scene":               "login",
			"subject_hash":        "subject-hash-old",
			"level":               6,
			"answer_hash":         "hash-old",
			"answer_salt":         "salt-old",
			"issued_ip_hash":      "ip-old",
			"expires_at":          now.Add(-96 * time.Hour),
			"consumed_at":         nil,
			"failed_verify_count": 0,
			"created_at":          oldTime,
			"updated_at":          oldTime,
		},
		{
			"captcha_id":          "captcha-new",
			"scene":               "login",
			"subject_hash":        "subject-hash-new",
			"level":               6,
			"answer_hash":         "hash-new",
			"answer_salt":         "salt-new",
			"issued_ip_hash":      "ip-new",
			"expires_at":          now.Add(24 * time.Hour),
			"consumed_at":         nil,
			"failed_verify_count": 0,
			"created_at":          recentTime,
			"updated_at":          recentTime,
		},
	}).Error; err != nil {
		t.Fatalf("seed auth captcha challenges failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Table("auth_risk_states").Create([]map[string]any{
		{
			"scene":                "login",
			"subject_type":         "ip",
			"subject_hash":         "risk-old",
			"window_started_at":    oldTime,
			"attempt_count":        1,
			"failed_count":         1,
			"captcha_failed_count": 0,
			"lock_until":           nil,
			"created_at":           oldTime,
			"updated_at":           oldTime,
		},
		{
			"scene":                "login",
			"subject_type":         "ip",
			"subject_hash":         "risk-new",
			"window_started_at":    recentTime,
			"attempt_count":        1,
			"failed_count":         1,
			"captcha_failed_count": 0,
			"lock_until":           nil,
			"created_at":           recentTime,
			"updated_at":           recentTime,
		},
	}).Error; err != nil {
		t.Fatalf("seed auth risk states failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Table("user_sessions").Create([]map[string]any{
		{
			"session_id":             "session-old",
			"user_id":                "01hretentionuser000000000001",
			"refresh_token_hash":     "refresh-old",
			"expires_at":             now.AddDate(0, 0, -40),
			"revoked_at":             now.AddDate(0, 0, -35),
			"replaced_by_session_id": nil,
			"created_at":             oldTime,
			"updated_at":             oldTime,
		},
		{
			"session_id":             "session-new",
			"user_id":                "01hretentionuser000000000001",
			"refresh_token_hash":     "refresh-new",
			"expires_at":             now.AddDate(0, 0, 7),
			"revoked_at":             nil,
			"replaced_by_session_id": nil,
			"created_at":             recentTime,
			"updated_at":             recentTime,
		},
	}).Error; err != nil {
		t.Fatalf("seed user sessions failed: %v", err)
	}

	retentionConfigJSON, err := json.Marshal(map[string]any{
		"enabled":                    true,
		"scheduleMinutes":            30,
		"cleanupBatchSize":           200,
		"auditLogRetentionDays":      30,
		"authCaptchaRetentionHours":  72,
		"authRiskStateRetentionDays": 30,
		"userSessionRetentionDays":   30,
	})
	if err != nil {
		t.Fatalf("marshal retention config failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Table("system_configs").Create(map[string]any{
		"config_key":         SystemConfigKeyDataRetention,
		"config_value_json":  string(retentionConfigJSON),
		"version":            1,
		"updated_by_user_id": nil,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("seed system config failed: %v", err)
	}

	cleanupService := NewDataRetentionCleanupService(database.ORM, repository.NewGormSystemConfigRepository(database.ORM))
	result, err := cleanupService.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run data retention cleanup failed: %v", err)
	}
	if result.DeletedAuditLogs != 1 {
		t.Fatalf("expected deleted audit logs 1, got %d", result.DeletedAuditLogs)
	}
	if result.DeletedAuthCaptchaChallenges != 1 {
		t.Fatalf("expected deleted auth captcha challenges 1, got %d", result.DeletedAuthCaptchaChallenges)
	}
	if result.DeletedAuthRiskStates != 1 {
		t.Fatalf("expected deleted auth risk states 1, got %d", result.DeletedAuthRiskStates)
	}
	if result.DeletedUserSessions != 1 {
		t.Fatalf("expected deleted user sessions 1, got %d", result.DeletedUserSessions)
	}

	var oldAuditCount int64
	if err := database.ORM.WithContext(ctx).Table("audit_logs").Where("target_id = ?", "old-record").Count(&oldAuditCount).Error; err != nil {
		t.Fatalf("count old audit logs failed: %v", err)
	}
	if oldAuditCount != 0 {
		t.Fatalf("expected old audit log deleted, got count=%d", oldAuditCount)
	}
	var newAuditCount int64
	if err := database.ORM.WithContext(ctx).Table("audit_logs").Where("target_id = ?", "new-record").Count(&newAuditCount).Error; err != nil {
		t.Fatalf("count new audit logs failed: %v", err)
	}
	if newAuditCount != 1 {
		t.Fatalf("expected new audit log kept, got count=%d", newAuditCount)
	}

	var oldCaptchaCount int64
	if err := database.ORM.WithContext(ctx).Table("auth_captcha_challenges").Where("captcha_id = ?", "captcha-old").Count(&oldCaptchaCount).Error; err != nil {
		t.Fatalf("count old captcha challenge failed: %v", err)
	}
	if oldCaptchaCount != 0 {
		t.Fatalf("expected old captcha challenge deleted, got count=%d", oldCaptchaCount)
	}
	var newCaptchaCount int64
	if err := database.ORM.WithContext(ctx).Table("auth_captcha_challenges").Where("captcha_id = ?", "captcha-new").Count(&newCaptchaCount).Error; err != nil {
		t.Fatalf("count new captcha challenge failed: %v", err)
	}
	if newCaptchaCount != 1 {
		t.Fatalf("expected new captcha challenge kept, got count=%d", newCaptchaCount)
	}

	var oldRiskCount int64
	if err := database.ORM.WithContext(ctx).Table("auth_risk_states").Where("subject_hash = ?", "risk-old").Count(&oldRiskCount).Error; err != nil {
		t.Fatalf("count old auth risk state failed: %v", err)
	}
	if oldRiskCount != 0 {
		t.Fatalf("expected old auth risk state deleted, got count=%d", oldRiskCount)
	}
	var newRiskCount int64
	if err := database.ORM.WithContext(ctx).Table("auth_risk_states").Where("subject_hash = ?", "risk-new").Count(&newRiskCount).Error; err != nil {
		t.Fatalf("count new auth risk state failed: %v", err)
	}
	if newRiskCount != 1 {
		t.Fatalf("expected new auth risk state kept, got count=%d", newRiskCount)
	}

	var oldSessionCount int64
	if err := database.ORM.WithContext(ctx).Table("user_sessions").Where("session_id = ?", "session-old").Count(&oldSessionCount).Error; err != nil {
		t.Fatalf("count old user session failed: %v", err)
	}
	if oldSessionCount != 0 {
		t.Fatalf("expected old user session deleted, got count=%d", oldSessionCount)
	}
	var newSessionCount int64
	if err := database.ORM.WithContext(ctx).Table("user_sessions").Where("session_id = ?", "session-new").Count(&newSessionCount).Error; err != nil {
		t.Fatalf("count new user session failed: %v", err)
	}
	if newSessionCount != 1 {
		t.Fatalf("expected new user session kept, got count=%d", newSessionCount)
	}
}
