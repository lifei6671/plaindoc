package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

func TestGormUserIdentityRepository_UpsertAndQuery(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-user-identity-repository?mode=memory&cache=shared",
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

	user := &models.User{
		UserID:       "01kcl5vq8qv3j9f4m3y9n4w6a1",
		Email:        "identity-test@example.com",
		PasswordHash: "hashed",
		Name:         "Identity Test",
		Status:       models.EntityStatusActive,
	}
	if err := database.ORM.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}

	repo := NewGormUserIdentityRepository(database.ORM)
	now := time.Now().UTC()
	record, err := repo.Upsert(ctx, UpsertUserIdentityParams{
		UserID:       user.UserID,
		ProviderType: "ldap",
		ProviderID:   "corp-ldap",
		ExternalID:   "entry-uuid-1",
		LoginName:    "tester",
		LastLoginAt:  &now,
		Now:          now,
	})
	if err != nil {
		t.Fatalf("upsert user identity failed: %v", err)
	}
	if record == nil {
		t.Fatal("expected non-nil identity record")
	}
	if record.UserID != user.UserID || record.ProviderID != "corp-ldap" || record.ExternalID != "entry-uuid-1" {
		t.Fatalf("unexpected identity after upsert: %+v", record)
	}

	nextLoginAt := now.Add(5 * time.Minute)
	updated, err := repo.Upsert(ctx, UpsertUserIdentityParams{
		UserID:       user.UserID,
		ProviderType: "ldap",
		ProviderID:   "corp-ldap",
		ExternalID:   "entry-uuid-1",
		LoginName:    "tester-new",
		LastLoginAt:  &nextLoginAt,
		Now:          nextLoginAt,
	})
	if err != nil {
		t.Fatalf("upsert existing user identity failed: %v", err)
	}
	if updated == nil {
		t.Fatal("expected non-nil updated identity record")
	}
	if updated.LoginName != "tester-new" {
		t.Fatalf("expected updated login name tester-new, got %q", updated.LoginName)
	}
	if updated.LastLoginAt == nil || !updated.LastLoginAt.Equal(nextLoginAt) {
		t.Fatalf("expected updated last login at %s, got %+v", nextLoginAt, updated.LastLoginAt)
	}

	found, err := repo.GetByProviderExternalID(ctx, "corp-ldap", "entry-uuid-1")
	if err != nil {
		t.Fatalf("query identity by provider/external id failed: %v", err)
	}
	if found.UserID != user.UserID {
		t.Fatalf("expected user id %q, got %q", user.UserID, found.UserID)
	}

	list, err := repo.ListByUserID(ctx, user.UserID)
	if err != nil {
		t.Fatalf("list identities by user id failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected identity list length 1, got %d", len(list))
	}

	_, err = repo.GetByProviderExternalID(ctx, "corp-ldap", "missing")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound for missing identity, got %v", err)
	}
}
