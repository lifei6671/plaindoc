package service

import (
	"context"
	"encoding/json"
	"fmt"
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
		"enabled":                        true,
		"scheduleMinutes":                30,
		"cleanupBatchSize":               200,
		"auditLogRetentionDays":          30,
		"authCaptchaRetentionHours":      72,
		"authRiskStateRetentionDays":     30,
		"userSessionRetentionDays":       30,
		"documentRevisionRetentionCount": 30,
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

func TestDataRetentionCleanupService_RunOnce_CleansDocumentRevisionsByKeepCount(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-data-retention-cleanup-document-revisions?mode=memory&cache=shared",
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
	ownerUserID := "01hretentionrevisionuser000001"
	if err := database.ORM.WithContext(ctx).Table("users").Create(map[string]any{
		"user_id":       ownerUserID,
		"email":         "retention-revision@example.com",
		"password_hash": "hashed-password",
		"name":          "Retention Revision User",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Table("spaces").Create(map[string]any{
		"space_id":      "01hretentionrevisionspace001",
		"name":          "retention revision space",
		"owner_user_id": ownerUserID,
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("seed space failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Table("nodes").Create([]map[string]any{
		{
			"node_id":    "01hretentionrevisionnode001",
			"space_id":   "01hretentionrevisionspace001",
			"type":       "doc",
			"title":      "markdown history",
			"sort":       1,
			"created_at": now,
			"updated_at": now,
		},
		{
			"node_id":    "01hretentionrevisionnode002",
			"space_id":   "01hretentionrevisionspace001",
			"type":       "doc",
			"title":      "short markdown history",
			"sort":       2,
			"created_at": now,
			"updated_at": now,
		},
		{
			"node_id":    "01hretentionrevisionnode003",
			"space_id":   "01hretentionrevisionspace001",
			"type":       "doc",
			"title":      "office history",
			"sort":       3,
			"created_at": now,
			"updated_at": now,
		},
	}).Error; err != nil {
		t.Fatalf("seed nodes failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Table("documents").Create([]map[string]any{
		{
			"document_id":        "01hretentionrevisiondoc001",
			"node_id":            "01hretentionrevisionnode001",
			"theme_id":           "default",
			"title":              "markdown history",
			"format":             "markdown",
			"content_md":         "# current",
			"version":            35,
			"content_version":    35,
			"updated_by_user_id": ownerUserID,
			"created_at":         now,
			"updated_at":         now,
		},
		{
			"document_id":        "01hretentionrevisiondoc002",
			"node_id":            "01hretentionrevisionnode002",
			"theme_id":           "default",
			"title":              "short markdown history",
			"format":             "markdown",
			"content_md":         "# current",
			"version":            2,
			"content_version":    2,
			"updated_by_user_id": ownerUserID,
			"created_at":         now,
			"updated_at":         now,
		},
		{
			"document_id":        "01hretentionrevisiondoc003",
			"node_id":            "01hretentionrevisionnode003",
			"theme_id":           "default",
			"title":              "office history",
			"format":             "docx",
			"content_md":         "",
			"version":            33,
			"content_version":    33,
			"updated_by_user_id": ownerUserID,
			"created_at":         now,
			"updated_at":         now,
		},
	}).Error; err != nil {
		t.Fatalf("seed documents failed: %v", err)
	}

	markdownRevisions := make([]map[string]any, 0, 37)
	for version := 1; version <= 35; version++ {
		markdownRevisions = append(markdownRevisions, map[string]any{
			"document_revision_id": fmt.Sprintf("01hretentionrevisionm%03d", version),
			"document_id":          "01hretentionrevisiondoc001",
			"version":              version,
			"content_md":           fmt.Sprintf("# version %d", version),
			"base_version":         version - 1,
			"editor_user_id":       ownerUserID,
			"source":               "local",
			"created_at":           now.Add(time.Duration(version) * time.Minute),
		})
	}
	for version := 1; version <= 2; version++ {
		markdownRevisions = append(markdownRevisions, map[string]any{
			"document_revision_id": fmt.Sprintf("01hretentionrevisions%03d", version),
			"document_id":          "01hretentionrevisiondoc002",
			"version":              version,
			"content_md":           fmt.Sprintf("# short version %d", version),
			"base_version":         version - 1,
			"editor_user_id":       ownerUserID,
			"source":               "local",
			"created_at":           now.Add(time.Duration(version) * time.Minute),
		})
	}
	if err := database.ORM.WithContext(ctx).Table("document_revisions").Create(markdownRevisions).Error; err != nil {
		t.Fatalf("seed markdown revisions failed: %v", err)
	}

	fileBlobs := make([]map[string]any, 0, 33)
	fileRevisions := make([]map[string]any, 0, 33)
	for version := 1; version <= 33; version++ {
		blobID := fmt.Sprintf("01hretentionrevisionblob%03d", version)
		fileBlobs = append(fileBlobs, map[string]any{
			"blob_id":           blobID,
			"storage_provider":  "local",
			"object_key":        fmt.Sprintf("revisions/file-%03d.docx", version),
			"object_url":        fmt.Sprintf("/uploads/revisions/file-%03d.docx", version),
			"mime_type":         "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"size_bytes":        int64(1024 + version),
			"content_hash_algo": "sha256",
			"content_hash":      fmt.Sprintf("retention-revision-hash-%03d", version),
			"created_at":        now.Add(time.Duration(version) * time.Minute),
			"updated_at":        now.Add(time.Duration(version) * time.Minute),
		})
		fileRevisions = append(fileRevisions, map[string]any{
			"document_file_revision_id": fmt.Sprintf("01hretentionrevisionf%03d", version),
			"document_id":               "01hretentionrevisiondoc003",
			"blob_id":                   blobID,
			"file_name":                 fmt.Sprintf("file-%03d.docx", version),
			"mime_type":                 "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"version":                   version,
			"base_version":              version - 1,
			"editor_user_id":            ownerUserID,
			"source":                    "local",
			"created_at":                now.Add(time.Duration(version) * time.Minute),
		})
	}
	if err := database.ORM.WithContext(ctx).Table("file_blobs").Create(fileBlobs).Error; err != nil {
		t.Fatalf("seed file blobs failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Table("document_file_revisions").Create(fileRevisions).Error; err != nil {
		t.Fatalf("seed office revisions failed: %v", err)
	}

	retentionConfigJSON, err := json.Marshal(map[string]any{
		"enabled":                        true,
		"scheduleMinutes":                30,
		"cleanupBatchSize":               7,
		"cleanupTables":                  []string{DataRetentionCleanupTableDocumentRevisions},
		"auditLogRetentionDays":          30,
		"authCaptchaRetentionHours":      72,
		"authRiskStateRetentionDays":     30,
		"userSessionRetentionDays":       30,
		"documentRevisionRetentionCount": 30,
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
	if result.DeletedDocumentRevisions != 8 {
		t.Fatalf("expected deleted document revisions 8, got %d", result.DeletedDocumentRevisions)
	}

	assertRevisionCount := func(tableName string, documentID string, expectedCount int64) {
		t.Helper()
		var count int64
		if err := database.ORM.WithContext(ctx).Table(tableName).
			Where("document_id = ?", documentID).
			Count(&count).Error; err != nil {
			t.Fatalf("count %s for %s failed: %v", tableName, documentID, err)
		}
		if count != expectedCount {
			t.Fatalf("expected %s for %s count %d, got %d", tableName, documentID, expectedCount, count)
		}
	}
	assertRevisionCount("document_revisions", "01hretentionrevisiondoc001", 30)
	assertRevisionCount("document_revisions", "01hretentionrevisiondoc002", 2)
	assertRevisionCount("document_file_revisions", "01hretentionrevisiondoc003", 30)

	var oldestMarkdownCount int64
	if err := database.ORM.WithContext(ctx).Table("document_revisions").
		Where("document_id = ? AND version = ?", "01hretentionrevisiondoc001", 1).
		Count(&oldestMarkdownCount).Error; err != nil {
		t.Fatalf("count oldest markdown revision failed: %v", err)
	}
	if oldestMarkdownCount != 0 {
		t.Fatalf("expected oldest markdown revision deleted, got count=%d", oldestMarkdownCount)
	}
	var latestMarkdownCount int64
	if err := database.ORM.WithContext(ctx).Table("document_revisions").
		Where("document_id = ? AND version = ?", "01hretentionrevisiondoc001", 35).
		Count(&latestMarkdownCount).Error; err != nil {
		t.Fatalf("count latest markdown revision failed: %v", err)
	}
	if latestMarkdownCount != 1 {
		t.Fatalf("expected latest markdown revision kept, got count=%d", latestMarkdownCount)
	}
	var oldestOfficeCount int64
	if err := database.ORM.WithContext(ctx).Table("document_file_revisions").
		Where("document_id = ? AND version = ?", "01hretentionrevisiondoc003", 1).
		Count(&oldestOfficeCount).Error; err != nil {
		t.Fatalf("count oldest office revision failed: %v", err)
	}
	if oldestOfficeCount != 0 {
		t.Fatalf("expected oldest office revision deleted, got count=%d", oldestOfficeCount)
	}
}

func TestDataRetentionCleanupService_RunOnceForced(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-data-retention-cleanup-forced?mode=memory&cache=shared",
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

	if err := database.ORM.WithContext(ctx).Table("audit_logs").Create(map[string]any{
		"actor_user_id": nil,
		"module":        "system_config",
		"action":        "update",
		"target_type":   "system_config",
		"target_id":     "forced-cleanup-old-audit",
		"summary":       "forced cleanup old audit",
		"detail_json":   "{}",
		"request_id":    "req-forced-cleanup-old-audit",
		"created_at":    oldTime,
	}).Error; err != nil {
		t.Fatalf("seed audit logs failed: %v", err)
	}

	retentionConfigJSON, err := json.Marshal(map[string]any{
		"enabled":                        false,
		"scheduleMinutes":                30,
		"cleanupBatchSize":               200,
		"auditLogRetentionDays":          30,
		"authCaptchaRetentionHours":      72,
		"authRiskStateRetentionDays":     30,
		"userSessionRetentionDays":       30,
		"documentRevisionRetentionCount": 30,
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

	normalResult, err := cleanupService.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run data retention cleanup failed: %v", err)
	}
	if normalResult.DeletedAuditLogs != 0 {
		t.Fatalf("expected normal run delete 0 rows when disabled, got %d", normalResult.DeletedAuditLogs)
	}

	forcedResult, err := cleanupService.RunOnceForced(ctx)
	if err != nil {
		t.Fatalf("run forced data retention cleanup failed: %v", err)
	}
	if forcedResult.DeletedAuditLogs != 1 {
		t.Fatalf("expected forced run delete 1 row, got %d", forcedResult.DeletedAuditLogs)
	}

	var oldAuditCount int64
	if err := database.ORM.WithContext(ctx).Table("audit_logs").Where("target_id = ?", "forced-cleanup-old-audit").Count(&oldAuditCount).Error; err != nil {
		t.Fatalf("count old audit logs failed: %v", err)
	}
	if oldAuditCount != 0 {
		t.Fatalf("expected old audit log deleted by forced cleanup, got count=%d", oldAuditCount)
	}
}

func TestDataRetentionCleanupService_RunOnce_RespectsCleanupTables(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-data-retention-cleanup-selected-tables?mode=memory&cache=shared",
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

	if err := database.ORM.WithContext(ctx).Table("users").Create(map[string]any{
		"user_id":       "01hretentionuser000000000009",
		"email":         "retention-selected@example.com",
		"password_hash": "hashed-password",
		"name":          "Retention Selected User",
		"created_at":    oldTime,
		"updated_at":    oldTime,
	}).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Table("user_identities").Create(map[string]any{
		"user_id":       "01hretentionuser000000000009",
		"provider_type": "local",
		"provider_id":   "local",
		"external_id":   "retention-selected@example.com",
		"login_name":    "retention-selected@example.com",
		"created_at":    oldTime,
		"updated_at":    oldTime,
	}).Error; err != nil {
		t.Fatalf("seed user identity failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Table("audit_logs").Create(map[string]any{
		"actor_user_id": nil,
		"module":        "system_config",
		"action":        "update",
		"target_type":   "system_config",
		"target_id":     "selected-table-old-audit",
		"summary":       "selected table old audit",
		"detail_json":   "{}",
		"request_id":    "req-selected-table-old-audit",
		"created_at":    oldTime,
	}).Error; err != nil {
		t.Fatalf("seed audit logs failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Table("user_sessions").Create(map[string]any{
		"session_id":             "session-selected-old",
		"user_id":                "01hretentionuser000000000009",
		"refresh_token_hash":     "refresh-selected-old",
		"expires_at":             now.AddDate(0, 0, -40),
		"revoked_at":             now.AddDate(0, 0, -35),
		"replaced_by_session_id": nil,
		"created_at":             oldTime,
		"updated_at":             oldTime,
	}).Error; err != nil {
		t.Fatalf("seed user sessions failed: %v", err)
	}

	retentionConfigJSON, err := json.Marshal(map[string]any{
		"enabled":                        true,
		"scheduleMinutes":                30,
		"cleanupBatchSize":               200,
		"cleanupTables":                  []string{DataRetentionCleanupTableAuditLogs},
		"auditLogRetentionDays":          30,
		"authCaptchaRetentionHours":      72,
		"authRiskStateRetentionDays":     30,
		"userSessionRetentionDays":       30,
		"documentRevisionRetentionCount": 30,
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
	if result.DeletedUserSessions != 0 {
		t.Fatalf("expected deleted user sessions 0 when table not selected, got %d", result.DeletedUserSessions)
	}

	var oldAuditCount int64
	if err := database.ORM.WithContext(ctx).Table("audit_logs").Where("target_id = ?", "selected-table-old-audit").Count(&oldAuditCount).Error; err != nil {
		t.Fatalf("count old audit logs failed: %v", err)
	}
	if oldAuditCount != 0 {
		t.Fatalf("expected old audit log deleted, got count=%d", oldAuditCount)
	}

	var oldSessionCount int64
	if err := database.ORM.WithContext(ctx).Table("user_sessions").Where("session_id = ?", "session-selected-old").Count(&oldSessionCount).Error; err != nil {
		t.Fatalf("count old user sessions failed: %v", err)
	}
	if oldSessionCount != 1 {
		t.Fatalf("expected old user session kept when table not selected, got count=%d", oldSessionCount)
	}
}
