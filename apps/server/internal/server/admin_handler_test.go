package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
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

func TestRouter_AdminUserListRequiresPlatformAdmin(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "users-space-admin@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "users-platform-admin@example.com")
	_, _, _ = registerAccessUser(t, serve, "users-normal@example.com")

	grantAdminRole(t, database, spaceAdminUserID, "space_admin")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	spaceAdminReq := httptest.NewRequest(http.MethodGet, "/api/admin/users?page=1&pageSize=20", nil)
	spaceAdminReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminRec := serve(spaceAdminReq)
	if spaceAdminRec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for space_admin listing users, got %d body=%s", spaceAdminRec.Code, spaceAdminRec.Body.String())
	}

	platformAdminReq := httptest.NewRequest(http.MethodGet, "/api/admin/users?page=1&pageSize=20", nil)
	platformAdminReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformAdminRec := serve(platformAdminReq)
	if platformAdminRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for platform_admin listing users, got %d body=%s", platformAdminRec.Code, platformAdminRec.Body.String())
	}

	var payload struct {
		Items []struct {
			UserID string `json:"userId"`
		} `json:"items"`
		Pagination struct {
			Page     int   `json:"page"`
			PageSize int   `json:"pageSize"`
			Total    int64 `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(platformAdminRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode admin users payload failed: %v", err)
	}
	if payload.Pagination.Page != 1 || payload.Pagination.PageSize != 20 {
		t.Fatalf(
			"expected pagination page=1 pageSize=20, got page=%d pageSize=%d",
			payload.Pagination.Page,
			payload.Pagination.PageSize,
		)
	}
	if payload.Pagination.Total < int64(len(payload.Items)) {
		t.Fatalf("expected total >= item count, total=%d items=%d", payload.Pagination.Total, len(payload.Items))
	}
}

func TestRouter_AdminUserBanRevokesSessionAndAccess(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "ban-platform-admin@example.com")
	targetUserID, targetAccessToken, targetRefreshToken := registerAccessUserWithRefresh(t, serve, "ban-target@example.com")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	targetMeBeforeReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	targetMeBeforeReq.Header.Set("Authorization", "Bearer "+targetAccessToken)
	targetMeBeforeRec := serve(targetMeBeforeReq)
	if targetMeBeforeRec.Code != http.StatusOK {
		t.Fatalf("expected target /auth/me before ban status 200, got %d body=%s", targetMeBeforeRec.Code, targetMeBeforeRec.Body.String())
	}

	banBody := []byte(`{"status":"banned","reason":"违规内容"}`)
	banReq := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+targetUserID+"/status", bytes.NewReader(banBody))
	banReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	banReq.Header.Set("Content-Type", "application/json")
	banRec := serve(banReq)
	if banRec.Code != http.StatusOK {
		t.Fatalf("expected ban user status 200, got %d body=%s", banRec.Code, banRec.Body.String())
	}

	targetMeAfterReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	targetMeAfterReq.Header.Set("Authorization", "Bearer "+targetAccessToken)
	targetMeAfterRec := serve(targetMeAfterReq)
	if targetMeAfterRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected target /auth/me after ban status 401, got %d body=%s", targetMeAfterRec.Code, targetMeAfterRec.Body.String())
	}

	refreshBody := []byte(`{"refreshToken":"` + targetRefreshToken + `"}`)
	targetRefreshReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(refreshBody))
	targetRefreshReq.Header.Set("Content-Type", "application/json")
	targetRefreshRec := serve(targetRefreshReq)
	if targetRefreshRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected target refresh after ban status 401, got %d body=%s", targetRefreshRec.Code, targetRefreshRec.Body.String())
	}

	var revokedSessionCount int64
	if err := database.ORM.Table("user_sessions").
		Where("user_id = ? AND revoked_at IS NOT NULL", targetUserID).
		Count(&revokedSessionCount).Error; err != nil {
		t.Fatalf("query revoked sessions failed: %v", err)
	}
	if revokedSessionCount == 0 {
		t.Fatal("expected at least one revoked session after banning user")
	}

	var userStatusAuditCount int64
	if err := database.ORM.Table("audit_logs").
		Where("module = ? AND action = ? AND target_type = ? AND target_id = ?", "user", "update", "user", targetUserID).
		Count(&userStatusAuditCount).Error; err != nil {
		t.Fatalf("query user status audit logs failed: %v", err)
	}
	if userStatusAuditCount == 0 {
		t.Fatal("expected at least one user status audit log after banning user")
	}
}

func TestRouter_AdminUserDeleteSoftDeleteAndHideFromDefaultList(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "delete-platform-admin@example.com")
	targetUserID, targetAccessToken, targetRefreshToken := registerAccessUserWithRefresh(t, serve, "delete-target@example.com")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+targetUserID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	deleteRec := serve(deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected delete user status 204, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	var persistedUser struct {
		Status    string     `gorm:"column:status"`
		DeletedAt *time.Time `gorm:"column:deleted_at"`
	}
	if err := database.ORM.Table("users").
		Select("status", "deleted_at").
		Where("user_id = ?", targetUserID).
		Scan(&persistedUser).Error; err != nil {
		t.Fatalf("query deleted user status failed: %v", err)
	}
	if persistedUser.Status != "deleted" {
		t.Fatalf("expected deleted user status=deleted, got %s", persistedUser.Status)
	}
	if persistedUser.DeletedAt == nil {
		t.Fatal("expected deleted_at to be set for soft-deleted user")
	}

	targetMeReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	targetMeReq.Header.Set("Authorization", "Bearer "+targetAccessToken)
	targetMeRec := serve(targetMeReq)
	if targetMeRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected deleted user /auth/me status 401, got %d body=%s", targetMeRec.Code, targetMeRec.Body.String())
	}

	refreshBody := []byte(`{"refreshToken":"` + targetRefreshToken + `"}`)
	targetRefreshReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(refreshBody))
	targetRefreshReq.Header.Set("Content-Type", "application/json")
	targetRefreshRec := serve(targetRefreshReq)
	if targetRefreshRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected deleted user refresh status 401, got %d body=%s", targetRefreshRec.Code, targetRefreshRec.Body.String())
	}

	loginBody := []byte(`{"email":"delete-target@example.com","password":"123456"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := serve(loginReq)
	if loginRec.Code != http.StatusForbidden {
		t.Fatalf("expected deleted user login status 403, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}

	listDefaultReq := httptest.NewRequest(http.MethodGet, "/api/admin/users?page=1&pageSize=100", nil)
	listDefaultReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	listDefaultRec := serve(listDefaultReq)
	if listDefaultRec.Code != http.StatusOK {
		t.Fatalf("expected list default users status 200, got %d body=%s", listDefaultRec.Code, listDefaultRec.Body.String())
	}
	if strings.Contains(listDefaultRec.Body.String(), targetUserID) {
		t.Fatalf("expected deleted user to be hidden from default list, body=%s", listDefaultRec.Body.String())
	}

	listDeletedReq := httptest.NewRequest(http.MethodGet, "/api/admin/users?status=deleted&page=1&pageSize=100", nil)
	listDeletedReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	listDeletedRec := serve(listDeletedReq)
	if listDeletedRec.Code != http.StatusOK {
		t.Fatalf("expected list deleted users status 200, got %d body=%s", listDeletedRec.Code, listDeletedRec.Body.String())
	}
	if !strings.Contains(listDeletedRec.Body.String(), targetUserID) {
		t.Fatalf("expected deleted user to appear in deleted list, body=%s", listDeletedRec.Body.String())
	}

	var userDeleteAuditCount int64
	if err := database.ORM.Table("audit_logs").
		Where("module = ? AND action = ? AND target_type = ? AND target_id = ?", "user", "delete", "user", targetUserID).
		Count(&userDeleteAuditCount).Error; err != nil {
		t.Fatalf("query user delete audit logs failed: %v", err)
	}
	if userDeleteAuditCount == 0 {
		t.Fatal("expected at least one user delete audit log")
	}
}

func TestRouter_AdminUserSelfOperationBlocked(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "self-platform-admin@example.com")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	banBody := []byte(`{"status":"banned","reason":"self-ban"}`)
	banReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/users/"+platformAdminUserID+"/status",
		bytes.NewReader(banBody),
	)
	banReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	banReq.Header.Set("Content-Type", "application/json")
	banRec := serve(banReq)
	if banRec.Code != http.StatusBadRequest {
		t.Fatalf("expected self ban status 400, got %d body=%s", banRec.Code, banRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+platformAdminUserID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	deleteRec := serve(deleteReq)
	if deleteRec.Code != http.StatusBadRequest {
		t.Fatalf("expected self delete status 400, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestRouter_AdminSpaceListRespectsScope(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "space-list-owner@example.com")
	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "space-list-admin@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "space-list-platform@example.com")

	grantAdminRole(t, database, spaceAdminUserID, "space_admin")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	spaceIDA := "01h1adminspacelist000000000001"
	spaceIDB := "01h1adminspacelist000000000002"
	insertAdminTestSpace(t, database, spaceIDA, "Space A", ownerUserID, "member")
	insertAdminTestSpace(t, database, spaceIDB, "Space B", ownerUserID, "public")

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("space_admin_scopes").Create(map[string]any{
		"user_id":    spaceAdminUserID,
		"space_id":   spaceIDA,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("insert space admin scope failed: %v", err)
	}

	spaceAdminReq := httptest.NewRequest(http.MethodGet, "/api/admin/spaces?page=1&pageSize=50", nil)
	spaceAdminReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminRec := serve(spaceAdminReq)
	if spaceAdminRec.Code != http.StatusOK {
		t.Fatalf("expected space_admin list spaces status 200, got %d body=%s", spaceAdminRec.Code, spaceAdminRec.Body.String())
	}
	if !strings.Contains(spaceAdminRec.Body.String(), spaceIDA) {
		t.Fatalf("expected space_admin list to contain scoped space %s, body=%s", spaceIDA, spaceAdminRec.Body.String())
	}
	if strings.Contains(spaceAdminRec.Body.String(), spaceIDB) {
		t.Fatalf("expected space_admin list to hide unscoped space %s, body=%s", spaceIDB, spaceAdminRec.Body.String())
	}

	platformReq := httptest.NewRequest(http.MethodGet, "/api/admin/spaces?page=1&pageSize=50", nil)
	platformReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformRec := serve(platformReq)
	if platformRec.Code != http.StatusOK {
		t.Fatalf("expected platform_admin list spaces status 200, got %d body=%s", platformRec.Code, platformRec.Body.String())
	}
	if !strings.Contains(platformRec.Body.String(), spaceIDA) || !strings.Contains(platformRec.Body.String(), spaceIDB) {
		t.Fatalf("expected platform_admin list to contain both spaces, body=%s", platformRec.Body.String())
	}
}

func TestRouter_AdminSpaceUpdateDeleteAndScopeGuard(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "space-meta-owner@example.com")
	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "space-meta-admin@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "space-meta-platform@example.com")

	grantAdminRole(t, database, spaceAdminUserID, "space_admin")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	spaceIDA := "01h1adminspacemeta000000000001"
	spaceIDB := "01h1adminspacemeta000000000002"
	insertAdminTestSpace(t, database, spaceIDA, "Space Meta A", ownerUserID, "member")
	insertAdminTestSpace(t, database, spaceIDB, "Space Meta B", ownerUserID, "member")
	spaceADocID := "01h1adminspacemeta000000000d1"
	insertAdminTestDocument(
		t,
		database,
		spaceIDA,
		"01h1adminspacemeta000000000n1",
		spaceADocID,
		"Space Meta A Doc",
		"public",
	)

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("space_admin_scopes").Create(map[string]any{
		"user_id":    spaceAdminUserID,
		"space_id":   spaceIDA,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("insert space admin scope failed: %v", err)
	}

	updateBody := []byte(`{"name":"Scoped Updated Space","visibility":"public"}`)
	updateAllowedReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/spaces/"+spaceIDA+"/metadata",
		bytes.NewReader(updateBody),
	)
	updateAllowedReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	updateAllowedReq.Header.Set("Content-Type", "application/json")
	updateAllowedRec := serve(updateAllowedReq)
	if updateAllowedRec.Code != http.StatusOK {
		t.Fatalf("expected scoped admin update allowed space status 200, got %d body=%s", updateAllowedRec.Code, updateAllowedRec.Body.String())
	}

	updateDeniedReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/spaces/"+spaceIDB+"/metadata",
		bytes.NewReader(updateBody),
	)
	updateDeniedReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	updateDeniedReq.Header.Set("Content-Type", "application/json")
	updateDeniedRec := serve(updateDeniedReq)
	if updateDeniedRec.Code != http.StatusForbidden {
		t.Fatalf("expected scoped admin update unscoped space status 403, got %d body=%s", updateDeniedRec.Code, updateDeniedRec.Body.String())
	}

	updateByPlatformReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/spaces/"+spaceIDB+"/metadata",
		bytes.NewReader([]byte(`{"name":"Platform Updated Space","visibility":"authenticated"}`)),
	)
	updateByPlatformReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	updateByPlatformReq.Header.Set("Content-Type", "application/json")
	updateByPlatformRec := serve(updateByPlatformReq)
	if updateByPlatformRec.Code != http.StatusOK {
		t.Fatalf("expected platform admin update space status 200, got %d body=%s", updateByPlatformRec.Code, updateByPlatformRec.Body.String())
	}

	var updatedSpace struct {
		Name       string `gorm:"column:name"`
		Visibility string `gorm:"column:visibility"`
	}
	if err := database.ORM.Table("spaces").
		Select("name", "visibility").
		Where("space_id = ?", spaceIDB).
		Scan(&updatedSpace).Error; err != nil {
		t.Fatalf("query updated space metadata failed: %v", err)
	}
	if updatedSpace.Name != "Platform Updated Space" || updatedSpace.Visibility != "authenticated" {
		t.Fatalf("unexpected updated space metadata: %+v", updatedSpace)
	}

	deleteAllowedReq := httptest.NewRequest(http.MethodDelete, "/api/admin/spaces/"+spaceIDA, nil)
	deleteAllowedReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	deleteAllowedRec := serve(deleteAllowedReq)
	if deleteAllowedRec.Code != http.StatusNoContent {
		t.Fatalf("expected scoped admin delete allowed space status 204, got %d body=%s", deleteAllowedRec.Code, deleteAllowedRec.Body.String())
	}

	deleteDeniedReq := httptest.NewRequest(http.MethodDelete, "/api/admin/spaces/"+spaceIDB, nil)
	deleteDeniedReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	deleteDeniedRec := serve(deleteDeniedReq)
	if deleteDeniedRec.Code != http.StatusForbidden {
		t.Fatalf("expected scoped admin delete unscoped space status 403, got %d body=%s", deleteDeniedRec.Code, deleteDeniedRec.Body.String())
	}

	var deletedSpace struct {
		Status    string     `gorm:"column:status"`
		DeletedAt *time.Time `gorm:"column:deleted_at"`
	}
	if err := database.ORM.Table("spaces").
		Select("status", "deleted_at").
		Where("space_id = ?", spaceIDA).
		Scan(&deletedSpace).Error; err != nil {
		t.Fatalf("query deleted space status failed: %v", err)
	}
	if deletedSpace.Status != "deleted" || deletedSpace.DeletedAt == nil {
		t.Fatalf("expected deleted space status=deleted and deleted_at set, got %+v", deletedSpace)
	}
	var deletedSpaceDoc struct {
		Status    string     `gorm:"column:status"`
		DeletedAt *time.Time `gorm:"column:deleted_at"`
	}
	if err := database.ORM.Table("documents").
		Select("status", "deleted_at").
		Where("document_id = ?", spaceADocID).
		Scan(&deletedSpaceDoc).Error; err != nil {
		t.Fatalf("query deleted space cascade document status failed: %v", err)
	}
	if deletedSpaceDoc.Status != "deleted" || deletedSpaceDoc.DeletedAt == nil {
		t.Fatalf("expected deleted space document status=deleted and deleted_at set, got %+v", deletedSpaceDoc)
	}

	readDeletedSpaceReq := httptest.NewRequest(http.MethodGet, "/api/spaces/"+spaceIDA, nil)
	readDeletedSpaceRec := serve(readDeletedSpaceReq)
	if readDeletedSpaceRec.Code != http.StatusForbidden {
		t.Fatalf("expected reading deleted space status 403, got %d body=%s", readDeletedSpaceRec.Code, readDeletedSpaceRec.Body.String())
	}

	readDeletedSpaceDocReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+spaceADocID, nil)
	readDeletedSpaceDocRec := serve(readDeletedSpaceDocReq)
	if readDeletedSpaceDocRec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected reading document in deleted space status 403, got %d body=%s",
			readDeletedSpaceDocRec.Code,
			readDeletedSpaceDocRec.Body.String(),
		)
	}

	var spaceMetadataAuditCount int64
	if err := database.ORM.Table("audit_logs").
		Where("module = ? AND action = ? AND target_type = ? AND target_id = ?", "space", "update", "space", spaceIDA).
		Count(&spaceMetadataAuditCount).Error; err != nil {
		t.Fatalf("query space update audit logs failed: %v", err)
	}
	if spaceMetadataAuditCount == 0 {
		t.Fatal("expected at least one space update audit log")
	}

	var spaceDeleteAuditCount int64
	if err := database.ORM.Table("audit_logs").
		Where("module = ? AND action = ? AND target_type = ? AND target_id = ?", "space", "delete", "space", spaceIDA).
		Count(&spaceDeleteAuditCount).Error; err != nil {
		t.Fatalf("query space delete audit logs failed: %v", err)
	}
	if spaceDeleteAuditCount == 0 {
		t.Fatal("expected at least one space delete audit log")
	}
}

func TestRouter_AdminSpaceStatusUpdateAndScopeGuard(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "space-status-owner@example.com")
	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "space-status-admin@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "space-status-platform@example.com")

	grantAdminRole(t, database, spaceAdminUserID, "space_admin")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	spaceIDA := "01h1adminspacestatus0000000001"
	spaceIDB := "01h1adminspacestatus0000000002"
	insertAdminTestSpace(t, database, spaceIDA, "Space Status A", ownerUserID, "member")
	insertAdminTestSpace(t, database, spaceIDB, "Space Status B", ownerUserID, "member")

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("space_admin_scopes").Create(map[string]any{
		"user_id":    spaceAdminUserID,
		"space_id":   spaceIDA,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("insert space admin scope failed: %v", err)
	}

	invalidBanReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/spaces/"+spaceIDA+"/status",
		bytes.NewReader([]byte(`{"status":"banned"}`)),
	)
	invalidBanReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	invalidBanReq.Header.Set("Content-Type", "application/json")
	invalidBanRec := serve(invalidBanReq)
	if invalidBanRec.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected banning space without reason status 400, got %d body=%s",
			invalidBanRec.Code,
			invalidBanRec.Body.String(),
		)
	}

	scopedBanReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/spaces/"+spaceIDA+"/status",
		bytes.NewReader([]byte(`{"status":"banned","reason":"违规内容"}`)),
	)
	scopedBanReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	scopedBanReq.Header.Set("Content-Type", "application/json")
	scopedBanRec := serve(scopedBanReq)
	if scopedBanRec.Code != http.StatusOK {
		t.Fatalf("expected scoped admin ban space status 200, got %d body=%s", scopedBanRec.Code, scopedBanRec.Body.String())
	}
	readBannedSpaceReq := httptest.NewRequest(http.MethodGet, "/api/spaces/"+spaceIDA, nil)
	readBannedSpaceRec := serve(readBannedSpaceReq)
	if readBannedSpaceRec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected reading banned space status 403, got %d body=%s",
			readBannedSpaceRec.Code,
			readBannedSpaceRec.Body.String(),
		)
	}

	var scopedSpace struct {
		Status       string     `gorm:"column:status"`
		BannedReason string     `gorm:"column:banned_reason"`
		BannedAt     *time.Time `gorm:"column:banned_at"`
	}
	if err := database.ORM.Table("spaces").
		Select("status", "banned_reason", "banned_at").
		Where("space_id = ?", spaceIDA).
		Scan(&scopedSpace).Error; err != nil {
		t.Fatalf("query banned space status failed: %v", err)
	}
	if scopedSpace.Status != "banned" || scopedSpace.BannedReason != "违规内容" || scopedSpace.BannedAt == nil {
		t.Fatalf("expected scoped space banned with reason, got %+v", scopedSpace)
	}

	unscopedBanReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/spaces/"+spaceIDB+"/status",
		bytes.NewReader([]byte(`{"status":"banned","reason":"越权封禁"}`)),
	)
	unscopedBanReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	unscopedBanReq.Header.Set("Content-Type", "application/json")
	unscopedBanRec := serve(unscopedBanReq)
	if unscopedBanRec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected scoped admin ban unscoped space status 403, got %d body=%s",
			unscopedBanRec.Code,
			unscopedBanRec.Body.String(),
		)
	}

	platformBanReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/spaces/"+spaceIDB+"/status",
		bytes.NewReader([]byte(`{"status":"banned","reason":"平台封禁"}`)),
	)
	platformBanReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformBanReq.Header.Set("Content-Type", "application/json")
	platformBanRec := serve(platformBanReq)
	if platformBanRec.Code != http.StatusOK {
		t.Fatalf("expected platform admin ban space status 200, got %d body=%s", platformBanRec.Code, platformBanRec.Body.String())
	}

	platformUnbanReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/spaces/"+spaceIDB+"/status",
		bytes.NewReader([]byte(`{"status":"active"}`)),
	)
	platformUnbanReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformUnbanReq.Header.Set("Content-Type", "application/json")
	platformUnbanRec := serve(platformUnbanReq)
	if platformUnbanRec.Code != http.StatusOK {
		t.Fatalf("expected platform admin unban space status 200, got %d body=%s", platformUnbanRec.Code, platformUnbanRec.Body.String())
	}

	var unbannedSpace struct {
		Status       string     `gorm:"column:status"`
		BannedReason string     `gorm:"column:banned_reason"`
		BannedAt     *time.Time `gorm:"column:banned_at"`
	}
	if err := database.ORM.Table("spaces").
		Select("status", "banned_reason", "banned_at").
		Where("space_id = ?", spaceIDB).
		Scan(&unbannedSpace).Error; err != nil {
		t.Fatalf("query unbanned space status failed: %v", err)
	}
	if unbannedSpace.Status != "active" || unbannedSpace.BannedReason != "" || unbannedSpace.BannedAt != nil {
		t.Fatalf("expected unbanned space status cleanup, got %+v", unbannedSpace)
	}
}

func TestRouter_AdminDocumentListUpdateDeleteAndScopeGuard(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "doc-admin-owner@example.com")
	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "doc-admin-space@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "doc-admin-platform@example.com")

	grantAdminRole(t, database, spaceAdminUserID, "space_admin")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	spaceIDA := "01h1admindocspace000000000001"
	spaceIDB := "01h1admindocspace000000000002"
	insertAdminTestSpace(t, database, spaceIDA, "Doc Space A", ownerUserID, "member")
	insertAdminTestSpace(t, database, spaceIDB, "Doc Space B", ownerUserID, "public")

	docIDA := "01h1admindocument000000000001"
	docIDB := "01h1admindocument000000000002"
	insertAdminTestDocument(t, database, spaceIDA, "01h1admindocnode000000000001", docIDA, "Scoped Doc A", "public")
	insertAdminTestDocument(t, database, spaceIDB, "01h1admindocnode000000000002", docIDB, "Unscoped Doc B", "authenticated")

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("space_admin_scopes").Create(map[string]any{
		"user_id":    spaceAdminUserID,
		"space_id":   spaceIDA,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("insert space admin scope failed: %v", err)
	}

	spaceAdminListReq := httptest.NewRequest(http.MethodGet, "/api/admin/documents?page=1&pageSize=50", nil)
	spaceAdminListReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminListRec := serve(spaceAdminListReq)
	if spaceAdminListRec.Code != http.StatusOK {
		t.Fatalf("expected space_admin list documents status 200, got %d body=%s", spaceAdminListRec.Code, spaceAdminListRec.Body.String())
	}
	if !strings.Contains(spaceAdminListRec.Body.String(), docIDA) {
		t.Fatalf("expected space_admin list to contain scoped document %s, body=%s", docIDA, spaceAdminListRec.Body.String())
	}
	if strings.Contains(spaceAdminListRec.Body.String(), docIDB) {
		t.Fatalf("expected space_admin list to hide unscoped document %s, body=%s", docIDB, spaceAdminListRec.Body.String())
	}

	banWithoutReasonReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/documents/"+docIDA+"/status",
		bytes.NewReader([]byte(`{"status":"banned"}`)),
	)
	banWithoutReasonReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	banWithoutReasonReq.Header.Set("Content-Type", "application/json")
	banWithoutReasonRec := serve(banWithoutReasonReq)
	if banWithoutReasonRec.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected banning document without reason status 400, got %d body=%s",
			banWithoutReasonRec.Code,
			banWithoutReasonRec.Body.String(),
		)
	}

	banScopedReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/documents/"+docIDA+"/status",
		bytes.NewReader([]byte(`{"status":"banned","reason":"违规文档"}`)),
	)
	banScopedReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	banScopedReq.Header.Set("Content-Type", "application/json")
	banScopedRec := serve(banScopedReq)
	if banScopedRec.Code != http.StatusOK {
		t.Fatalf("expected scoped admin ban document status 200, got %d body=%s", banScopedRec.Code, banScopedRec.Body.String())
	}
	readBannedDocumentReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+docIDA, nil)
	readBannedDocumentRec := serve(readBannedDocumentReq)
	if readBannedDocumentRec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected reading banned document status 403, got %d body=%s",
			readBannedDocumentRec.Code,
			readBannedDocumentRec.Body.String(),
		)
	}

	var bannedDocument struct {
		Status       string     `gorm:"column:status"`
		BannedReason string     `gorm:"column:banned_reason"`
		BannedAt     *time.Time `gorm:"column:banned_at"`
	}
	if err := database.ORM.Table("documents").
		Select("status", "banned_reason", "banned_at").
		Where("document_id = ?", docIDA).
		Scan(&bannedDocument).Error; err != nil {
		t.Fatalf("query banned document failed: %v", err)
	}
	if bannedDocument.Status != "banned" || bannedDocument.BannedReason != "违规文档" || bannedDocument.BannedAt == nil {
		t.Fatalf("expected banned scoped document with reason, got %+v", bannedDocument)
	}

	banUnscopedReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/documents/"+docIDB+"/status",
		bytes.NewReader([]byte(`{"status":"banned","reason":"越权封禁"}`)),
	)
	banUnscopedReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	banUnscopedReq.Header.Set("Content-Type", "application/json")
	banUnscopedRec := serve(banUnscopedReq)
	if banUnscopedRec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected scoped admin ban unscoped document status 403, got %d body=%s",
			banUnscopedRec.Code,
			banUnscopedRec.Body.String(),
		)
	}

	platformDeleteReq := httptest.NewRequest(http.MethodDelete, "/api/admin/documents/"+docIDB, nil)
	platformDeleteReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformDeleteRec := serve(platformDeleteReq)
	if platformDeleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected platform admin delete document status 204, got %d body=%s", platformDeleteRec.Code, platformDeleteRec.Body.String())
	}

	var deletedDocument struct {
		Status    string     `gorm:"column:status"`
		DeletedAt *time.Time `gorm:"column:deleted_at"`
	}
	if err := database.ORM.Table("documents").
		Select("status", "deleted_at").
		Where("document_id = ?", docIDB).
		Scan(&deletedDocument).Error; err != nil {
		t.Fatalf("query deleted document failed: %v", err)
	}
	if deletedDocument.Status != "deleted" || deletedDocument.DeletedAt == nil {
		t.Fatalf("expected deleted document status=deleted and deleted_at set, got %+v", deletedDocument)
	}

	platformDefaultListReq := httptest.NewRequest(http.MethodGet, "/api/admin/documents?page=1&pageSize=50", nil)
	platformDefaultListReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformDefaultListRec := serve(platformDefaultListReq)
	if platformDefaultListRec.Code != http.StatusOK {
		t.Fatalf("expected platform admin default list status 200, got %d body=%s", platformDefaultListRec.Code, platformDefaultListRec.Body.String())
	}
	if strings.Contains(platformDefaultListRec.Body.String(), docIDB) {
		t.Fatalf("expected deleted document hidden from default list, body=%s", platformDefaultListRec.Body.String())
	}

	platformDeletedListReq := httptest.NewRequest(http.MethodGet, "/api/admin/documents?status=deleted&page=1&pageSize=50", nil)
	platformDeletedListReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformDeletedListRec := serve(platformDeletedListReq)
	if platformDeletedListRec.Code != http.StatusOK {
		t.Fatalf("expected platform admin deleted list status 200, got %d body=%s", platformDeletedListRec.Code, platformDeletedListRec.Body.String())
	}
	if !strings.Contains(platformDeletedListRec.Body.String(), docIDB) {
		t.Fatalf("expected deleted document visible in deleted list, body=%s", platformDeletedListRec.Body.String())
	}

	var documentBanAuditCount int64
	if err := database.ORM.Table("audit_logs").
		Where("module = ? AND action = ? AND target_type = ? AND target_id = ?", "document", "update", "document", docIDA).
		Count(&documentBanAuditCount).Error; err != nil {
		t.Fatalf("query document ban audit logs failed: %v", err)
	}
	if documentBanAuditCount == 0 {
		t.Fatal("expected at least one document update audit log")
	}

	var documentDeleteAuditCount int64
	if err := database.ORM.Table("audit_logs").
		Where("module = ? AND action = ? AND target_type = ? AND target_id = ?", "document", "delete", "document", docIDB).
		Count(&documentDeleteAuditCount).Error; err != nil {
		t.Fatalf("query document delete audit logs failed: %v", err)
	}
	if documentDeleteAuditCount == 0 {
		t.Fatal("expected at least one document delete audit log")
	}
}

func TestRouter_AdminThemeCRUDAndPublicThemeVisibility(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "theme-space-admin@example.com")
	grantAdminRole(t, database, spaceAdminUserID, "space_admin")

	themeID := "custom_admin_theme_01"
	createThemeBody := []byte(`{
		"themeId":"custom_admin_theme_01",
		"name":"Admin Theme",
		"description":"Admin created theme",
		"variables":{"--pd-preview-text-color":"#111111"},
		"syntaxTheme":"one-light",
		"codeBlockStyle":{"background":"#ffffff"},
		"codeBlockCodeStyle":{"fontSize":"13px"},
		"inlineCodeStyle":{"color":"#333333"},
		"customCss":".markdown-body { letter-spacing: 0.01em; }",
		"enabled":true
	}`)
	createThemeReq := httptest.NewRequest(http.MethodPost, "/api/admin/themes", bytes.NewReader(createThemeBody))
	createThemeReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	createThemeReq.Header.Set("Content-Type", "application/json")
	createThemeRec := serve(createThemeReq)
	if createThemeRec.Code != http.StatusCreated {
		t.Fatalf("expected create admin theme status 201, got %d body=%s", createThemeRec.Code, createThemeRec.Body.String())
	}
	if !strings.Contains(createThemeRec.Body.String(), themeID) {
		t.Fatalf("expected create theme response contain theme id %s, body=%s", themeID, createThemeRec.Body.String())
	}

	listAdminThemesReq := httptest.NewRequest(http.MethodGet, "/api/admin/themes", nil)
	listAdminThemesReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	listAdminThemesRec := serve(listAdminThemesReq)
	if listAdminThemesRec.Code != http.StatusOK {
		t.Fatalf("expected list admin themes status 200, got %d body=%s", listAdminThemesRec.Code, listAdminThemesRec.Body.String())
	}
	if !strings.Contains(listAdminThemesRec.Body.String(), themeID) {
		t.Fatalf("expected list admin themes include created theme %s, body=%s", themeID, listAdminThemesRec.Body.String())
	}

	disableThemeReq := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/themes/"+themeID,
		bytes.NewReader([]byte(`{"enabled":false}`)),
	)
	disableThemeReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	disableThemeReq.Header.Set("Content-Type", "application/json")
	disableThemeRec := serve(disableThemeReq)
	if disableThemeRec.Code != http.StatusOK {
		t.Fatalf("expected disable theme status 200, got %d body=%s", disableThemeRec.Code, disableThemeRec.Body.String())
	}

	listPublicThemesReq := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
	listPublicThemesRec := serve(listPublicThemesReq)
	if listPublicThemesRec.Code != http.StatusOK {
		t.Fatalf("expected list public themes status 200, got %d body=%s", listPublicThemesRec.Code, listPublicThemesRec.Body.String())
	}
	if strings.Contains(listPublicThemesRec.Body.String(), themeID) {
		t.Fatalf("expected disabled theme hidden from public themes, body=%s", listPublicThemesRec.Body.String())
	}

	deleteThemeReq := httptest.NewRequest(http.MethodDelete, "/api/admin/themes/"+themeID, nil)
	deleteThemeReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	deleteThemeRec := serve(deleteThemeReq)
	if deleteThemeRec.Code != http.StatusNoContent {
		t.Fatalf("expected delete theme status 204, got %d body=%s", deleteThemeRec.Code, deleteThemeRec.Body.String())
	}

	listAdminThemesAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/api/admin/themes", nil)
	listAdminThemesAfterDeleteReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	listAdminThemesAfterDeleteRec := serve(listAdminThemesAfterDeleteReq)
	if listAdminThemesAfterDeleteRec.Code != http.StatusOK {
		t.Fatalf(
			"expected list admin themes after delete status 200, got %d body=%s",
			listAdminThemesAfterDeleteRec.Code,
			listAdminThemesAfterDeleteRec.Body.String(),
		)
	}
	if strings.Contains(listAdminThemesAfterDeleteRec.Body.String(), themeID) {
		t.Fatalf("expected deleted theme removed from admin themes list, body=%s", listAdminThemesAfterDeleteRec.Body.String())
	}

	deleteBuiltinThemeReq := httptest.NewRequest(http.MethodDelete, "/api/admin/themes/default", nil)
	deleteBuiltinThemeReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	deleteBuiltinThemeRec := serve(deleteBuiltinThemeReq)
	if deleteBuiltinThemeRec.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected deleting builtin theme status 400, got %d body=%s",
			deleteBuiltinThemeRec.Code,
			deleteBuiltinThemeRec.Body.String(),
		)
	}

	var themeAuditCount int64
	if err := database.ORM.Table("audit_logs").
		Where("module = ? AND target_type = ? AND target_id = ?", "theme", "theme", themeID).
		Count(&themeAuditCount).Error; err != nil {
		t.Fatalf("query theme audit logs failed: %v", err)
	}
	if themeAuditCount != 3 {
		t.Fatalf("expected 3 theme audit logs(create/update/delete), got %d", themeAuditCount)
	}

	listThemeAuditReq := httptest.NewRequest(
		http.MethodGet,
		"/api/admin/audits?module=theme&targetId="+themeID+"&page=1&pageSize=20",
		nil,
	)
	listThemeAuditReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	listThemeAuditRec := serve(listThemeAuditReq)
	if listThemeAuditRec.Code != http.StatusOK {
		t.Fatalf(
			"expected list theme audit logs status 200, got %d body=%s",
			listThemeAuditRec.Code,
			listThemeAuditRec.Body.String(),
		)
	}
	var themeAuditPayload struct {
		Items []struct {
			Module   string `json:"module"`
			Action   string `json:"action"`
			TargetID string `json:"targetId"`
		} `json:"items"`
		Pagination struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(listThemeAuditRec.Body.Bytes(), &themeAuditPayload); err != nil {
		t.Fatalf("decode list theme audits response failed: %v", err)
	}
	if themeAuditPayload.Pagination.Total < 3 {
		t.Fatalf("expected theme audit total >= 3, got %d", themeAuditPayload.Pagination.Total)
	}
	if len(themeAuditPayload.Items) == 0 {
		t.Fatal("expected at least one theme audit item")
	}
	actionSet := make(map[string]struct{}, len(themeAuditPayload.Items))
	for _, item := range themeAuditPayload.Items {
		if item.Module != "theme" || item.TargetID != themeID {
			t.Fatalf("unexpected theme audit item: %+v", item)
		}
		actionSet[item.Action] = struct{}{}
	}
	if _, exists := actionSet["create"]; !exists {
		t.Fatalf("expected theme audit action create, actions=%v", actionSet)
	}
	if _, exists := actionSet["update"]; !exists {
		t.Fatalf("expected theme audit action update, actions=%v", actionSet)
	}
	if _, exists := actionSet["delete"]; !exists {
		t.Fatalf("expected theme audit action delete, actions=%v", actionSet)
	}
}

func TestRouter_AdminSystemConfigPlatformOnlyAndVersionControl(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "config-space-admin@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "config-platform-admin@example.com")
	grantAdminRole(t, database, spaceAdminUserID, "space_admin")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	spaceAdminUpsertReq := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/system-configs/site",
		bytes.NewReader([]byte(`{"value":{"allowRegistration":true,"defaultSpaceVisibility":"member"}}`)),
	)
	spaceAdminUpsertReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminUpsertReq.Header.Set("Content-Type", "application/json")
	spaceAdminUpsertRec := serve(spaceAdminUpsertReq)
	if spaceAdminUpsertRec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected space admin upsert system config status 403, got %d body=%s",
			spaceAdminUpsertRec.Code,
			spaceAdminUpsertRec.Body.String(),
		)
	}

	platformCreateReq := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/system-configs/site",
		bytes.NewReader([]byte(`{"value":{"allowRegistration":true,"defaultSpaceVisibility":"member"}}`)),
	)
	platformCreateReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformCreateReq.Header.Set("Content-Type", "application/json")
	platformCreateRec := serve(platformCreateReq)
	if platformCreateRec.Code != http.StatusOK {
		t.Fatalf(
			"expected platform admin create system config status 200, got %d body=%s",
			platformCreateRec.Code,
			platformCreateRec.Body.String(),
		)
	}
	var platformCreatePayload struct {
		ConfigKey string `json:"configKey"`
		Version   int    `json:"version"`
	}
	if err := json.Unmarshal(platformCreateRec.Body.Bytes(), &platformCreatePayload); err != nil {
		t.Fatalf("decode create system config response failed: %v", err)
	}
	if platformCreatePayload.ConfigKey != "site" || platformCreatePayload.Version != 1 {
		t.Fatalf("unexpected create system config payload: %+v", platformCreatePayload)
	}

	platformListReq := httptest.NewRequest(http.MethodGet, "/api/admin/system-configs", nil)
	platformListReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformListRec := serve(platformListReq)
	if platformListRec.Code != http.StatusOK {
		t.Fatalf("expected platform admin list system configs status 200, got %d body=%s", platformListRec.Code, platformListRec.Body.String())
	}
	if !strings.Contains(platformListRec.Body.String(), `"configKey":"site"`) {
		t.Fatalf("expected list system configs include site config, body=%s", platformListRec.Body.String())
	}

	platformUpdateReq := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/system-configs/site",
		bytes.NewReader([]byte(`{"expectedVersion":1,"value":{"allowRegistration":false,"defaultSpaceVisibility":"authenticated"}}`)),
	)
	platformUpdateReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformUpdateReq.Header.Set("Content-Type", "application/json")
	platformUpdateRec := serve(platformUpdateReq)
	if platformUpdateRec.Code != http.StatusOK {
		t.Fatalf(
			"expected platform admin update system config status 200, got %d body=%s",
			platformUpdateRec.Code,
			platformUpdateRec.Body.String(),
		)
	}
	var platformUpdatePayload struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(platformUpdateRec.Body.Bytes(), &platformUpdatePayload); err != nil {
		t.Fatalf("decode update system config response failed: %v", err)
	}
	if platformUpdatePayload.Version != 2 {
		t.Fatalf("expected updated system config version 2, got %d", platformUpdatePayload.Version)
	}

	platformConflictReq := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/system-configs/site",
		bytes.NewReader([]byte(`{"expectedVersion":1,"value":{"allowRegistration":true,"defaultSpaceVisibility":"member"}}`)),
	)
	platformConflictReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformConflictReq.Header.Set("Content-Type", "application/json")
	platformConflictRec := serve(platformConflictReq)
	if platformConflictRec.Code != http.StatusConflict {
		t.Fatalf(
			"expected stale expectedVersion conflict status 409, got %d body=%s",
			platformConflictRec.Code,
			platformConflictRec.Body.String(),
		)
	}

	platformInvalidValueReq := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/system-configs/site",
		bytes.NewReader([]byte(`{"value":{"allowRegistration":true,"defaultSpaceVisibility":"invalid"}}`)),
	)
	platformInvalidValueReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformInvalidValueReq.Header.Set("Content-Type", "application/json")
	platformInvalidValueRec := serve(platformInvalidValueReq)
	if platformInvalidValueRec.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected invalid system config value status 400, got %d body=%s",
			platformInvalidValueRec.Code,
			platformInvalidValueRec.Body.String(),
		)
	}

	var systemConfigAuditCount int64
	if err := database.ORM.Table("audit_logs").
		Where("module = ? AND target_type = ? AND target_id = ?", "system_config", "system_config", "site").
		Count(&systemConfigAuditCount).Error; err != nil {
		t.Fatalf("query system config audit logs failed: %v", err)
	}
	if systemConfigAuditCount != 2 {
		t.Fatalf("expected 2 system config audit logs(create/update), got %d", systemConfigAuditCount)
	}

	platformListAuditReq := httptest.NewRequest(
		http.MethodGet,
		"/api/admin/audits?module=system_config&targetId=site&page=1&pageSize=20",
		nil,
	)
	platformListAuditReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformListAuditRec := serve(platformListAuditReq)
	if platformListAuditRec.Code != http.StatusOK {
		t.Fatalf(
			"expected platform admin list system config audit logs status 200, got %d body=%s",
			platformListAuditRec.Code,
			platformListAuditRec.Body.String(),
		)
	}
	if !strings.Contains(platformListAuditRec.Body.String(), `"module":"system_config"`) {
		t.Fatalf("expected system_config module in audit payload, body=%s", platformListAuditRec.Body.String())
	}

	spaceAdminListAuditReq := httptest.NewRequest(
		http.MethodGet,
		"/api/admin/audits?module=system_config&page=1&pageSize=20",
		nil,
	)
	spaceAdminListAuditReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminListAuditRec := serve(spaceAdminListAuditReq)
	if spaceAdminListAuditRec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected space admin query system_config audit logs status 403, got %d body=%s",
			spaceAdminListAuditRec.Code,
			spaceAdminListAuditRec.Body.String(),
		)
	}
}

func registerAccessUserWithRefresh(
	t *testing.T,
	serve func(*http.Request) *httptest.ResponseRecorder,
	email string,
) (string, string, string) {
	t.Helper()

	body := []byte(`{"email":"` + email + `","password":"123456","name":"Test User"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register failed, status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
		Token        string `json:"token"`
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode register response failed: %v", err)
	}
	if payload.User.ID == "" || payload.Token == "" || payload.RefreshToken == "" {
		t.Fatalf("register response missing id/token/refreshToken, body=%s", rec.Body.String())
	}

	return payload.User.ID, payload.Token, payload.RefreshToken
}

func grantAdminRole(t *testing.T, database *storage.Database, userID string, role string) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("user_admin_roles").Create(map[string]any{
		"user_id":    userID,
		"role":       role,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("grant admin role failed: %v", err)
	}
}

func insertAdminTestSpace(
	t *testing.T,
	database *storage.Database,
	spaceID string,
	name string,
	ownerUserID string,
	visibility string,
) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("spaces").Create(map[string]any{
		"space_id":      spaceID,
		"name":          name,
		"owner_user_id": ownerUserID,
		"visibility":    visibility,
		"status":        "active",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("insert admin test space failed: %v", err)
	}
}

func insertAdminTestDocument(
	t *testing.T,
	database *storage.Database,
	spaceID string,
	nodeID string,
	documentID string,
	title string,
	visibility string,
) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("nodes").Create(map[string]any{
		"node_id":        nodeID,
		"space_id":       spaceID,
		"parent_node_id": nil,
		"type":           "doc",
		"title":          title,
		"sort":           1,
		"created_at":     now,
		"updated_at":     now,
	}).Error; err != nil {
		t.Fatalf("insert admin test node failed: %v", err)
	}

	if err := database.ORM.Table("documents").Create(map[string]any{
		"document_id": documentID,
		"node_id":     nodeID,
		"theme_id":    "default",
		"visibility":  visibility,
		"status":      "active",
		"title":       title,
		"content_md":  "# " + title,
		"version":     1,
		"created_at":  now,
		"updated_at":  now,
	}).Error; err != nil {
		t.Fatalf("insert admin test document failed: %v", err)
	}
}
