package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRouter_AdminMeRequiresAdminRole(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	noTokenReq := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	noTokenRec := serve(noTokenReq)
	if noTokenRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 without token, got %d body=%s", noTokenRec.Code, noTokenRec.Body.String())
	}

	_, _, userToken := registerAccessUser(t, serve, "admin-non-role@example.com")
	userReq := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	userReq.Header.Set("Authorization", "Bearer "+userToken)
	userRec := serve(userReq)
	if userRec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for non-admin, got %d body=%s", userRec.Code, userRec.Body.String())
	}
}

func TestRouter_AdminSpaceScopePermission(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "admin-owner@example.com")
	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "admin-space@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "admin-platform@example.com")

	spaceIDA := "01h1adminspace00000000000001"
	spaceIDB := "01h1adminspace00000000000002"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	if err := database.ORM.Table("spaces").Create([]map[string]any{
		{
			"space_id":      spaceIDA,
			"name":          "Space A",
			"owner_user_id": ownerUserID,
			"visibility":    "member",
			"status":        "active",
			"created_at":    now,
			"updated_at":    now,
		},
		{
			"space_id":      spaceIDB,
			"name":          "Space B",
			"owner_user_id": ownerUserID,
			"visibility":    "member",
			"status":        "active",
			"created_at":    now,
			"updated_at":    now,
		},
	}).Error; err != nil {
		t.Fatalf("insert spaces failed: %v", err)
	}

	if err := database.ORM.Table("user_admin_roles").Create([]map[string]any{
		{
			"user_id":    spaceAdminUserID,
			"role":       "space_admin",
			"created_at": now,
			"updated_at": now,
		},
		{
			"user_id":    platformAdminUserID,
			"role":       "platform_admin",
			"created_at": now,
			"updated_at": now,
		},
	}).Error; err != nil {
		t.Fatalf("insert admin roles failed: %v", err)
	}

	if err := database.ORM.Table("space_admin_scopes").Create(map[string]any{
		"user_id":    spaceAdminUserID,
		"space_id":   spaceIDA,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("insert space scope failed: %v", err)
	}

	spaceAdminAllowedReq := httptest.NewRequest(http.MethodGet, "/api/admin/spaces/"+spaceIDA+"/check", nil)
	spaceAdminAllowedReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminAllowedRec := serve(spaceAdminAllowedReq)
	if spaceAdminAllowedRec.Code != http.StatusOK {
		t.Fatalf(
			"expected space_admin allowed status 200, got %d body=%s",
			spaceAdminAllowedRec.Code,
			spaceAdminAllowedRec.Body.String(),
		)
	}

	spaceAdminDeniedReq := httptest.NewRequest(http.MethodGet, "/api/admin/spaces/"+spaceIDB+"/check", nil)
	spaceAdminDeniedReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminDeniedRec := serve(spaceAdminDeniedReq)
	if spaceAdminDeniedRec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected space_admin denied status 403, got %d body=%s",
			spaceAdminDeniedRec.Code,
			spaceAdminDeniedRec.Body.String(),
		)
	}

	platformAdminReq := httptest.NewRequest(http.MethodGet, "/api/admin/spaces/"+spaceIDB+"/check", nil)
	platformAdminReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformAdminRec := serve(platformAdminReq)
	if platformAdminRec.Code != http.StatusOK {
		t.Fatalf(
			"expected platform_admin allowed status 200, got %d body=%s",
			platformAdminRec.Code,
			platformAdminRec.Body.String(),
		)
	}
}
