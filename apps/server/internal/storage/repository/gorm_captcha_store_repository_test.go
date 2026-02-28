package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/captchastore"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func TestGormCaptchaStoreRepository_UpsertGetDelete(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-captcha-store-repository-upsert?mode=memory&cache=shared",
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

	repo := NewGormCaptchaStoreRepository(database.ORM)
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)

	input := captchastore.Record{
		ID:        "captcha-store-1",
		Value:     "A1B2",
		ExpiresAt: now.Add(10 * time.Minute),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.UpsertCaptchaRecord(ctx, input); err != nil {
		t.Fatalf("upsert captcha record failed: %v", err)
	}

	got, found, err := repo.GetCaptchaRecordByID(ctx, input.ID)
	if err != nil {
		t.Fatalf("get captcha record failed: %v", err)
	}
	if !found {
		t.Fatal("expected captcha record found")
	}
	if got.ID != input.ID || got.Value != input.Value {
		t.Fatalf("unexpected captcha record: %+v", got)
	}
	if got.ExpiresAt.IsZero() {
		t.Fatalf("expected non-zero expires_at: %+v", got)
	}

	updated := input
	updated.Value = "QWER"
	updated.ExpiresAt = now.Add(20 * time.Minute)
	updated.UpdatedAt = now.Add(1 * time.Minute)
	if err := repo.UpsertCaptchaRecord(ctx, updated); err != nil {
		t.Fatalf("upsert existing captcha record failed: %v", err)
	}

	got, found, err = repo.GetCaptchaRecordByID(ctx, input.ID)
	if err != nil {
		t.Fatalf("get updated captcha record failed: %v", err)
	}
	if !found {
		t.Fatal("expected updated captcha record found")
	}
	if got.Value != "QWER" {
		t.Fatalf("expected updated value QWER, got %q", got.Value)
	}

	storedID := mapCaptchaStoreID(input.ID)
	var count int64
	if err := database.ORM.WithContext(ctx).
		Model(&models.AuthCaptchaChallenge{}).
		Where("captcha_id = ? AND scene = ?", storedID, captchaStoreScene).
		Count(&count).Error; err != nil {
		t.Fatalf("count store row failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 store row, got %d", count)
	}

	if err := repo.DeleteCaptchaRecordByID(ctx, input.ID); err != nil {
		t.Fatalf("delete captcha record failed: %v", err)
	}
	_, found, err = repo.GetCaptchaRecordByID(ctx, input.ID)
	if err != nil {
		t.Fatalf("get deleted captcha record failed: %v", err)
	}
	if found {
		t.Fatal("expected captcha record deleted")
	}
}

func TestGormCaptchaStoreRepository_LongIDMappedAndIsolated(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-captcha-store-repository-long-id?mode=memory&cache=shared",
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

	repo := NewGormCaptchaStoreRepository(database.ORM)
	now := time.Date(2026, 2, 28, 13, 0, 0, 0, time.UTC)

	longID := strings.Repeat("captcha-", 16)
	if err := repo.UpsertCaptchaRecord(ctx, captchastore.Record{
		ID:        longID,
		Value:     "LONG",
		ExpiresAt: now.Add(15 * time.Minute),
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert long id record failed: %v", err)
	}

	mapped := mapCaptchaStoreID(longID)
	if mapped == captchaStoreIDPrefix+longID {
		t.Fatalf("expected long id to be hashed, got %q", mapped)
	}
	if len(mapped) > captchaStoreIDMaxLength {
		t.Fatalf("expected mapped id length <= %d, got %d", captchaStoreIDMaxLength, len(mapped))
	}

	got, found, err := repo.GetCaptchaRecordByID(ctx, longID)
	if err != nil {
		t.Fatalf("get long id record failed: %v", err)
	}
	if !found || got.Value != "LONG" {
		t.Fatalf("unexpected get long id result: found=%v record=%+v", found, got)
	}

	normalChallenge := &models.AuthCaptchaChallenge{
		CaptchaID:         "normal-login-captcha-id",
		Scene:             "login",
		SubjectHash:       strings.Repeat("a", 64),
		Level:             4,
		AnswerHash:        strings.Repeat("b", 64),
		AnswerSalt:        "salt",
		IssuedIPHash:      strings.Repeat("c", 64),
		ExpiresAt:         now.Add(30 * time.Minute),
		ConsumedAt:        nil,
		FailedVerifyCount: 0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := database.ORM.WithContext(ctx).Create(normalChallenge).Error; err != nil {
		t.Fatalf("seed normal captcha challenge failed: %v", err)
	}

	if err := repo.DeleteCaptchaRecordByID(ctx, normalChallenge.CaptchaID); err != nil {
		t.Fatalf("delete store record by normal id failed: %v", err)
	}
	var normalCount int64
	if err := database.ORM.WithContext(ctx).
		Model(&models.AuthCaptchaChallenge{}).
		Where("captcha_id = ? AND scene = ?", normalChallenge.CaptchaID, "login").
		Count(&normalCount).Error; err != nil {
		t.Fatalf("count normal challenge failed: %v", err)
	}
	if normalCount != 1 {
		t.Fatalf("expected normal login challenge unaffected, got count=%d", normalCount)
	}
}
