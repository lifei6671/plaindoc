package repository

import (
	"context"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func TestGormSearchVisibilityRepository_ResolveUserRoleLevel(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-visibility-repo-role-level?mode=memory&cache=shared",
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
	users := []models.User{
		{
			UserID:       "owner-1",
			Email:        "owner-1@example.com",
			PasswordHash: "hash",
			Name:         "owner-1",
			Status:       models.EntityStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			UserID:       "collab-1",
			Email:        "collab-1@example.com",
			PasswordHash: "hash",
			Name:         "collab-1",
			Status:       models.EntityStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			UserID:       "reader-1",
			Email:        "reader-1@example.com",
			PasswordHash: "hash",
			Name:         "reader-1",
			Status:       models.EntityStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			UserID:       "outsider-1",
			Email:        "outsider-1@example.com",
			PasswordHash: "hash",
			Name:         "outsider-1",
			Status:       models.EntityStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
	if err := database.ORM.WithContext(ctx).Create(&users).Error; err != nil {
		t.Fatalf("seed users failed: %v", err)
	}

	space := models.Space{
		SpaceID:     "space-1",
		Name:        "space-1",
		OwnerUserID: "owner-1",
		Visibility:  models.VisibilityMember,
		Status:      models.EntityStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := database.ORM.WithContext(ctx).Create(&space).Error; err != nil {
		t.Fatalf("seed space failed: %v", err)
	}

	spaceMembers := []models.SpaceMember{
		{
			SpaceID:   "space-1",
			UserID:    "collab-1",
			Role:      models.RoleCollaborator,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			SpaceID:   "space-1",
			UserID:    "reader-1",
			Role:      models.RoleReader,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := database.ORM.WithContext(ctx).Create(&spaceMembers).Error; err != nil {
		t.Fatalf("seed space members failed: %v", err)
	}

	repo := NewGormSearchVisibilityRepository(database.ORM)

	testCases := []struct {
		name        string
		spaceID     string
		actorUserID string
		expected    int
	}{
		{
			name:        "anonymous",
			spaceID:     "space-1",
			actorUserID: "",
			expected:    0,
		},
		{
			name:        "unknown-space",
			spaceID:     "space-not-exists",
			actorUserID: "owner-1",
			expected:    0,
		},
		{
			name:        "space-owner",
			spaceID:     "space-1",
			actorUserID: "owner-1",
			expected:    3,
		},
		{
			name:        "space-collaborator",
			spaceID:     "space-1",
			actorUserID: "collab-1",
			expected:    2,
		},
		{
			name:        "space-reader",
			spaceID:     "space-1",
			actorUserID: "reader-1",
			expected:    1,
		},
		{
			name:        "logged-in-non-member",
			spaceID:     "space-1",
			actorUserID: "outsider-1",
			expected:    0,
		},
	}

	for _, item := range testCases {
		item := item
		t.Run(item.name, func(t *testing.T) {
			roleLevel, err := repo.ResolveUserRoleLevel(ctx, item.spaceID, item.actorUserID)
			if err != nil {
				t.Fatalf("resolve user role level failed: %v", err)
			}
			if roleLevel != item.expected {
				t.Fatalf("expected role level=%d, got=%d", item.expected, roleLevel)
			}
		})
	}

	t.Run("resolve-role-levels-by-spaces", func(t *testing.T) {
		roleLevels, roleErr := repo.ResolveUserRoleLevelsBySpaces(ctx, "collab-1", []string{
			"space-1",
			"space-not-exists",
		})
		if roleErr != nil {
			t.Fatalf("resolve role levels by spaces failed: %v", roleErr)
		}
		if roleLevels["space-1"] != 2 {
			t.Fatalf("expected role level of space-1 = 2, got=%d", roleLevels["space-1"])
		}
		if _, exists := roleLevels["space-not-exists"]; exists {
			t.Fatalf("unexpected role level for non-existing space")
		}
	})
}
