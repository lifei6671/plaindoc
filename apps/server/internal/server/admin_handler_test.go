package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
)

func TestRouter_AdminMeRequiresAdminRole(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	noTokenReq := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	noTokenRec := serve(noTokenReq)
	if noTokenRec.Code != http.StatusForbidden {
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

	payload := decodeJSONResultData[struct {
		Items []struct {
			UserID string `json:"userId"`
		} `json:"items"`
		Pagination struct {
			Page     int   `json:"page"`
			PageSize int   `json:"pageSize"`
			Total    int64 `json:"total"`
		} `json:"pagination"`
	}](t, platformAdminRec.Body.Bytes())
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

func TestRouter_AdminUserCreateByPlatformAdmin(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "create-users-space-admin@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "create-users-platform-admin@example.com")
	grantAdminRole(t, database, spaceAdminUserID, "space_admin")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	createBody := []byte(`{"email":"created-user@example.com","name":"Created User","password":"123456","role":"space_admin"}`)
	spaceAdminCreateReq := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewReader(createBody))
	spaceAdminCreateReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminCreateReq.Header.Set("Content-Type", "application/json")
	spaceAdminCreateRec := serve(spaceAdminCreateReq)
	if spaceAdminCreateRec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected status 403 for space_admin creating user, got %d body=%s",
			spaceAdminCreateRec.Code,
			spaceAdminCreateRec.Body.String(),
		)
	}

	platformCreateReq := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewReader(createBody))
	platformCreateReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformCreateReq.Header.Set("Content-Type", "application/json")
	platformCreateRec := serve(platformCreateReq)
	if platformCreateRec.Code != http.StatusOK {
		t.Fatalf(
			"expected status 201 for platform_admin creating user, got %d body=%s",
			platformCreateRec.Code,
			platformCreateRec.Body.String(),
		)
	}

	createPayload := decodeJSONResultData[struct {
		UserID string `json:"userId"`
		Email  string `json:"email"`
		Name   string `json:"name"`
		Role   string `json:"role"`
		Status string `json:"status"`
	}](t, platformCreateRec.Body.Bytes())
	if createPayload.UserID == "" {
		t.Fatalf("expected created user id, body=%s", platformCreateRec.Body.String())
	}
	if createPayload.Email != "created-user@example.com" || createPayload.Name != "Created User" {
		t.Fatalf("unexpected create payload: %+v", createPayload)
	}
	if createPayload.Status != "active" {
		t.Fatalf("expected created user status active, got %s", createPayload.Status)
	}
	if createPayload.Role != "space_admin" {
		t.Fatalf("expected created user role space_admin, got %s", createPayload.Role)
	}

	var userRoleCount int64
	if err := database.ORM.Table("user_admin_roles").
		Where("user_id = ? AND role = ?", createPayload.UserID, "space_admin").
		Count(&userRoleCount).Error; err != nil {
		t.Fatalf("query created user role failed: %v", err)
	}
	if userRoleCount == 0 {
		t.Fatal("expected created user role space_admin persisted")
	}

	loginBody := []byte(`{"email":"created-user@example.com","password":"123456"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := serve(loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected created user login status 200, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}

	duplicateCreateReq := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewReader(createBody))
	duplicateCreateReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	duplicateCreateReq.Header.Set("Content-Type", "application/json")
	duplicateCreateRec := serve(duplicateCreateReq)
	if duplicateCreateRec.Code != http.StatusOK {
		t.Fatalf("expected duplicate create status 409, got %d body=%s", duplicateCreateRec.Code, duplicateCreateRec.Body.String())
	}

	var userCreateAuditCount int64
	if err := database.ORM.Table("audit_logs").
		Where("module = ? AND action = ? AND target_type = ? AND target_id = ?", "user", "create", "user", createPayload.UserID).
		Count(&userCreateAuditCount).Error; err != nil {
		t.Fatalf("query user create audit logs failed: %v", err)
	}
	if userCreateAuditCount == 0 {
		t.Fatal("expected at least one user create audit log")
	}
}

func TestRouter_AdminUserRoleUpdateByPlatformAdmin(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "role-space-admin@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "role-platform-admin@example.com")
	targetUserID, _, _ := registerAccessUser(t, serve, "role-target@example.com")

	grantAdminRole(t, database, spaceAdminUserID, "space_admin")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	spaceAdminUpdateReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/users/"+targetUserID+"/role",
		bytes.NewReader([]byte(`{"role":"space_admin"}`)),
	)
	spaceAdminUpdateReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminUpdateReq.Header.Set("Content-Type", "application/json")
	attachAdminOperationToken(
		t,
		serve,
		spaceAdminUpdateReq,
		spaceAdminToken,
		"user.update_role",
		"user",
		targetUserID,
	)
	spaceAdminUpdateRec := serve(spaceAdminUpdateReq)
	if spaceAdminUpdateRec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected space_admin update role status 403, got %d body=%s",
			spaceAdminUpdateRec.Code,
			spaceAdminUpdateRec.Body.String(),
		)
	}

	updateReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/users/"+targetUserID+"/role",
		bytes.NewReader([]byte(`{"role":"space_admin"}`)),
	)
	updateReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	updateReq.Header.Set("Content-Type", "application/json")
	attachAdminOperationToken(
		t,
		serve,
		updateReq,
		platformAdminToken,
		"user.update_role",
		"user",
		targetUserID,
	)
	updateRec := serve(updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update role status 200, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	updatePayload := decodeJSONResultData[struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}](t, updateRec.Body.Bytes())
	if updatePayload.UserID != targetUserID {
		t.Fatalf("expected update payload user id %s, got %s", targetUserID, updatePayload.UserID)
	}
	if updatePayload.Role != "space_admin" {
		t.Fatalf("expected update payload role space_admin, got %s", updatePayload.Role)
	}

	var targetRoleCount int64
	if err := database.ORM.Table("user_admin_roles").
		Where("user_id = ? AND role = ?", targetUserID, "space_admin").
		Count(&targetRoleCount).Error; err != nil {
		t.Fatalf("query updated user role failed: %v", err)
	}
	if targetRoleCount == 0 {
		t.Fatal("expected target user role updated to space_admin")
	}

	selfUpdateReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/users/"+platformAdminUserID+"/role",
		bytes.NewReader([]byte(`{"role":"user"}`)),
	)
	selfUpdateReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	selfUpdateReq.Header.Set("Content-Type", "application/json")
	attachAdminOperationToken(
		t,
		serve,
		selfUpdateReq,
		platformAdminToken,
		"user.update_role",
		"user",
		platformAdminUserID,
	)
	selfUpdateRec := serve(selfUpdateReq)
	if selfUpdateRec.Code != http.StatusOK {
		t.Fatalf("expected self update role status 400, got %d body=%s", selfUpdateRec.Code, selfUpdateRec.Body.String())
	}

	var userRoleAuditCount int64
	if err := database.ORM.Table("audit_logs").
		Where("module = ? AND action = ? AND target_type = ? AND target_id = ?", "user", "update", "user", targetUserID).
		Where("summary LIKE ?", "user role updated:%").
		Count(&userRoleAuditCount).Error; err != nil {
		t.Fatalf("query user role update audit logs failed: %v", err)
	}
	if userRoleAuditCount == 0 {
		t.Fatal("expected at least one user role update audit log")
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
	attachAdminOperationToken(
		t,
		serve,
		banReq,
		platformAdminToken,
		"user.update_status",
		"user",
		targetUserID,
	)
	banRec := serve(banReq)
	if banRec.Code != http.StatusOK {
		t.Fatalf("expected ban user status 200, got %d body=%s", banRec.Code, banRec.Body.String())
	}

	targetMeAfterReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	targetMeAfterReq.Header.Set("Authorization", "Bearer "+targetAccessToken)
	targetMeAfterRec := serve(targetMeAfterReq)
	if targetMeAfterRec.Code != http.StatusForbidden {
		t.Fatalf("expected target /auth/me after ban status 401, got %d body=%s", targetMeAfterRec.Code, targetMeAfterRec.Body.String())
	}

	refreshBody := []byte(`{"refreshToken":"` + targetRefreshToken + `"}`)
	targetRefreshReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(refreshBody))
	targetRefreshReq.Header.Set("Content-Type", "application/json")
	targetRefreshRec := serve(targetRefreshReq)
	if targetRefreshRec.Code != http.StatusForbidden {
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
	attachAdminOperationToken(
		t,
		serve,
		deleteReq,
		platformAdminToken,
		"user.delete",
		"user",
		targetUserID,
	)
	deleteRec := serve(deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete user status 200, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
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
	if targetMeRec.Code != http.StatusForbidden {
		t.Fatalf("expected deleted user /auth/me status 401, got %d body=%s", targetMeRec.Code, targetMeRec.Body.String())
	}

	refreshBody := []byte(`{"refreshToken":"` + targetRefreshToken + `"}`)
	targetRefreshReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(refreshBody))
	targetRefreshReq.Header.Set("Content-Type", "application/json")
	targetRefreshRec := serve(targetRefreshReq)
	if targetRefreshRec.Code != http.StatusForbidden {
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
	attachAdminOperationToken(
		t,
		serve,
		banReq,
		platformAdminToken,
		"user.update_status",
		"user",
		platformAdminUserID,
	)
	banRec := serve(banReq)
	if banRec.Code != http.StatusOK {
		t.Fatalf("expected self ban status 400, got %d body=%s", banRec.Code, banRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+platformAdminUserID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	attachAdminOperationToken(
		t,
		serve,
		deleteReq,
		platformAdminToken,
		"user.delete",
		"user",
		platformAdminUserID,
	)
	deleteRec := serve(deleteReq)
	if deleteRec.Code != http.StatusOK {
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

func TestRouter_AdminSpaceCreateWithCustomIDAndConflict(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "create-spaceid-admin@example.com")
	grantAdminRole(t, database, spaceAdminUserID, "space_admin")

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/spaces",
		bytes.NewReader([]byte(`{"spaceId":"Team_Docs-01","name":"Custom Space","visibility":"member"}`)),
	)
	createReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := serve(createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create space status 200, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	createPayload := decodeJSONResultData[struct {
		SpaceID string `json:"spaceId"`
		Name    string `json:"name"`
	}](t, createRec.Body.Bytes())
	if createPayload.SpaceID != "team_docs-01" || createPayload.Name != "Custom Space" {
		t.Fatalf("unexpected custom space create payload: %+v", createPayload)
	}

	var persistedCount int64
	if err := database.ORM.Table("spaces").
		Where("space_id = ?", "team_docs-01").
		Count(&persistedCount).Error; err != nil {
		t.Fatalf("query custom space id failed: %v", err)
	}
	if persistedCount != 1 {
		t.Fatalf("expected exactly one custom space id record, got %d", persistedCount)
	}

	duplicateReq := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/spaces",
		bytes.NewReader([]byte(`{"spaceId":"team_docs-01","name":"Duplicate Space","visibility":"member"}`)),
	)
	duplicateReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	duplicateReq.Header.Set("Content-Type", "application/json")
	duplicateRec := serve(duplicateReq)
	if duplicateRec.Code != http.StatusOK {
		t.Fatalf("expected duplicate create space status 200, got %d body=%s", duplicateRec.Code, duplicateRec.Body.String())
	}
	if decodeJSONResultCode(t, duplicateRec.Body.Bytes()) != response.ResolveErrorCode("SPACE_ALREADY_EXISTS") {
		t.Fatalf(
			"expected duplicate create code %d, got %d body=%s",
			response.ResolveErrorCode("SPACE_ALREADY_EXISTS"),
			decodeJSONResultCode(t, duplicateRec.Body.Bytes()),
			duplicateRec.Body.String(),
		)
	}
}

func TestRouter_AdminSpaceCategoriesManageAndApply(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "space-category-admin@example.com")
	grantAdminRole(t, database, spaceAdminUserID, "space_admin")

	initialListReq := httptest.NewRequest(http.MethodGet, "/api/admin/spaces/categories", nil)
	initialListReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	initialListRec := serve(initialListReq)
	if initialListRec.Code != http.StatusOK {
		t.Fatalf("expected initial list space categories status 200, got %d body=%s", initialListRec.Code, initialListRec.Body.String())
	}
	initialListPayload := decodeJSONResultData[struct {
		Items []struct {
			CategoryID string `json:"categoryId"`
			Name       string `json:"name"`
			IsDefault  bool   `json:"isDefault"`
		} `json:"items"`
	}](t, initialListRec.Body.Bytes())
	if len(initialListPayload.Items) == 0 {
		t.Fatalf("expected default category exists, got empty")
	}

	defaultCategoryID := ""
	for _, item := range initialListPayload.Items {
		if item.IsDefault {
			defaultCategoryID = item.CategoryID
			break
		}
	}
	if defaultCategoryID == "" {
		t.Fatalf("expected default category id in payload: %+v", initialListPayload.Items)
	}

	createCategoryReq := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/spaces/categories",
		bytes.NewReader([]byte(`{"name":"产品文档"}`)),
	)
	createCategoryReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	createCategoryReq.Header.Set("Content-Type", "application/json")
	createCategoryRec := serve(createCategoryReq)
	if createCategoryRec.Code != http.StatusOK {
		t.Fatalf(
			"expected create space category status 200, got %d body=%s",
			createCategoryRec.Code,
			createCategoryRec.Body.String(),
		)
	}
	createCategoryPayload := decodeJSONResultData[struct {
		CategoryID string `json:"categoryId"`
		Name       string `json:"name"`
	}](t, createCategoryRec.Body.Bytes())
	if createCategoryPayload.CategoryID == "" || createCategoryPayload.Name != "产品文档" {
		t.Fatalf("unexpected create category payload: %+v", createCategoryPayload)
	}

	secondCategoryReq := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/spaces/categories",
		bytes.NewReader([]byte(`{"name":"技术文档"}`)),
	)
	secondCategoryReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	secondCategoryReq.Header.Set("Content-Type", "application/json")
	secondCategoryRec := serve(secondCategoryReq)
	if secondCategoryRec.Code != http.StatusOK {
		t.Fatalf(
			"expected second category status 200, got %d body=%s",
			secondCategoryRec.Code,
			secondCategoryRec.Body.String(),
		)
	}
	secondCategoryPayload := decodeJSONResultData[struct {
		CategoryID string `json:"categoryId"`
		Name       string `json:"name"`
	}](t, secondCategoryRec.Body.Bytes())
	if secondCategoryPayload.CategoryID == "" || secondCategoryPayload.Name != "技术文档" {
		t.Fatalf("unexpected second category payload: %+v", secondCategoryPayload)
	}

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/spaces",
		bytes.NewReader([]byte(`{"name":"Category Space","description":"with category","categoryId":"`+createCategoryPayload.CategoryID+`","visibility":"member"}`)),
	)
	createReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := serve(createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create space with category status 200, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	createPayload := decodeJSONResultData[struct {
		SpaceID    string `json:"spaceId"`
		CategoryID string `json:"categoryId"`
		Category   string `json:"category"`
	}](t, createRec.Body.Bytes())
	if createPayload.SpaceID == "" || createPayload.CategoryID != createCategoryPayload.CategoryID || createPayload.Category != "产品文档" {
		t.Fatalf("unexpected create space payload: %+v", createPayload)
	}

	var createdSpace struct {
		CategoryID string `gorm:"column:category_id"`
		Category   string `gorm:"column:category"`
	}
	if err := database.ORM.Table("spaces").
		Select("category_id", "category").
		Where("space_id = ?", createPayload.SpaceID).
		Scan(&createdSpace).Error; err != nil {
		t.Fatalf("query created space category failed: %v", err)
	}
	if createdSpace.CategoryID != createCategoryPayload.CategoryID || createdSpace.Category != "产品文档" {
		t.Fatalf(
			"expected created space category (%s, 产品文档), got (%s, %s)",
			createCategoryPayload.CategoryID,
			createdSpace.CategoryID,
			createdSpace.Category,
		)
	}

	metadataReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/spaces/"+createPayload.SpaceID+"/metadata",
		bytes.NewReader([]byte(`{"categoryId":"`+secondCategoryPayload.CategoryID+`"}`)),
	)
	metadataReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	metadataReq.Header.Set("Content-Type", "application/json")
	metadataRec := serve(metadataReq)
	if metadataRec.Code != http.StatusOK {
		t.Fatalf("expected update space metadata category status 200, got %d body=%s", metadataRec.Code, metadataRec.Body.String())
	}
	metadataPayload := decodeJSONResultData[struct {
		CategoryID string `json:"categoryId"`
		Category   string `json:"category"`
	}](t, metadataRec.Body.Bytes())
	if metadataPayload.CategoryID != secondCategoryPayload.CategoryID || metadataPayload.Category != "技术文档" {
		t.Fatalf(
			"expected updated space category (%s, 技术文档), got (%s, %s) code=%d body=%s",
			secondCategoryPayload.CategoryID,
			metadataPayload.CategoryID,
			metadataPayload.Category,
			decodeJSONResultCode(t, metadataRec.Body.Bytes()),
			metadataRec.Body.String(),
		)
	}

	deleteCategoryReq := httptest.NewRequest(
		http.MethodDelete,
		"/api/admin/spaces/categories/"+secondCategoryPayload.CategoryID,
		nil,
	)
	deleteCategoryReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	deleteCategoryRec := serve(deleteCategoryReq)
	if deleteCategoryRec.Code != http.StatusOK {
		t.Fatalf(
			"expected delete category status 200, got %d body=%s",
			deleteCategoryRec.Code,
			deleteCategoryRec.Body.String(),
		)
	}
	deleteCategoryPayload := decodeJSONResultData[struct {
		CategoryID             string `json:"categoryId"`
		ReassignedToCategoryID string `json:"reassignedToCategoryId"`
		MovedSpaceCount        int64  `json:"movedSpaceCount"`
	}](t, deleteCategoryRec.Body.Bytes())
	if deleteCategoryPayload.CategoryID != secondCategoryPayload.CategoryID {
		t.Fatalf("unexpected delete payload: %+v", deleteCategoryPayload)
	}
	if deleteCategoryPayload.ReassignedToCategoryID != defaultCategoryID {
		t.Fatalf(
			"expected reassigned default category id %s, got %s",
			defaultCategoryID,
			deleteCategoryPayload.ReassignedToCategoryID,
		)
	}
	if deleteCategoryPayload.MovedSpaceCount <= 0 {
		t.Fatalf("expected moved space count > 0, got %d", deleteCategoryPayload.MovedSpaceCount)
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
	attachAdminOperationToken(
		t,
		serve,
		deleteAllowedReq,
		spaceAdminToken,
		"space.delete",
		"space",
		spaceIDA,
	)
	deleteAllowedRec := serve(deleteAllowedReq)
	if deleteAllowedRec.Code != http.StatusOK {
		t.Fatalf("expected scoped admin delete allowed space status 200, got %d body=%s", deleteAllowedRec.Code, deleteAllowedRec.Body.String())
	}

	deleteDeniedReq := httptest.NewRequest(http.MethodDelete, "/api/admin/spaces/"+spaceIDB, nil)
	deleteDeniedReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	attachAdminOperationToken(
		t,
		serve,
		deleteDeniedReq,
		spaceAdminToken,
		"space.delete",
		"space",
		spaceIDB,
	)
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

func TestRouter_AdminSpaceMemberManagement(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "space-member-owner@example.com")
	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "space-member-admin@example.com")
	targetUserID, _, _ := registerAccessUser(t, serve, "space-member-target@example.com")

	grantAdminRole(t, database, spaceAdminUserID, "space_admin")

	spaceIDA := "01h1adminspacemember0000000001"
	spaceIDB := "01h1adminspacemember0000000002"
	insertAdminTestSpace(t, database, spaceIDA, "Space Member A", ownerUserID, "member")
	insertAdminTestSpace(t, database, spaceIDB, "Space Member B", ownerUserID, "member")

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("space_admin_scopes").Create(map[string]any{
		"user_id":    spaceAdminUserID,
		"space_id":   spaceIDA,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("insert space admin scope failed: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/admin/spaces/"+spaceIDA+"/members", nil)
	listReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	listRec := serve(listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list members status 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), ownerUserID) || !strings.Contains(listRec.Body.String(), `"isOwner":true`) {
		t.Fatalf("expected list members contains owner, body=%s", listRec.Body.String())
	}

	addReq := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/spaces/"+spaceIDA+"/members",
		bytes.NewReader([]byte(`{"targetUserId":"`+targetUserID+`","role":"collaborator"}`)),
	)
	addReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	addReq.Header.Set("Content-Type", "application/json")
	addRec := serve(addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("expected add member status 200, got %d body=%s", addRec.Code, addRec.Body.String())
	}
	if !strings.Contains(addRec.Body.String(), targetUserID) || !strings.Contains(addRec.Body.String(), `"role":"collaborator"`) {
		t.Fatalf("unexpected add member response body=%s", addRec.Body.String())
	}

	var memberCount int64
	if err := database.ORM.Table("space_members").
		Where("space_id = ? AND user_id = ? AND role = ?", spaceIDA, targetUserID, "collaborator").
		Count(&memberCount).Error; err != nil {
		t.Fatalf("query inserted member failed: %v", err)
	}
	if memberCount == 0 {
		t.Fatal("expected target member inserted with collaborator role")
	}

	updateReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/spaces/"+spaceIDA+"/members/"+targetUserID,
		bytes.NewReader([]byte(`{"role":"reader"}`)),
	)
	updateReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := serve(updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update member role status 200, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}
	if !strings.Contains(updateRec.Body.String(), `"role":"reader"`) {
		t.Fatalf("expected updated member role reader, body=%s", updateRec.Body.String())
	}

	memberCount = 0
	if err := database.ORM.Table("space_members").
		Where("space_id = ? AND user_id = ? AND role = ?", spaceIDA, targetUserID, "reader").
		Count(&memberCount).Error; err != nil {
		t.Fatalf("query updated member role failed: %v", err)
	}
	if memberCount == 0 {
		t.Fatal("expected target member role updated to reader")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/admin/spaces/"+spaceIDA+"/members/"+targetUserID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	deleteRec := serve(deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete member status 200, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	memberCount = 0
	if err := database.ORM.Table("space_members").
		Where("space_id = ? AND user_id = ?", spaceIDA, targetUserID).
		Count(&memberCount).Error; err != nil {
		t.Fatalf("query deleted member failed: %v", err)
	}
	if memberCount != 0 {
		t.Fatalf("expected target member removed, remains=%d", memberCount)
	}

	removeOwnerReq := httptest.NewRequest(http.MethodDelete, "/api/admin/spaces/"+spaceIDA+"/members/"+ownerUserID, nil)
	removeOwnerReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	removeOwnerRec := serve(removeOwnerReq)
	if removeOwnerRec.Code != http.StatusOK {
		t.Fatalf("expected delete owner member status 400, got %d body=%s", removeOwnerRec.Code, removeOwnerRec.Body.String())
	}

	unscopedReq := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/spaces/"+spaceIDB+"/members",
		bytes.NewReader([]byte(`{"targetUserId":"`+targetUserID+`","role":"reader"}`)),
	)
	unscopedReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	unscopedReq.Header.Set("Content-Type", "application/json")
	unscopedRec := serve(unscopedReq)
	if unscopedRec.Code != http.StatusForbidden {
		t.Fatalf("expected add member on unscoped space status 403, got %d body=%s", unscopedRec.Code, unscopedRec.Body.String())
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
	attachAdminOperationToken(
		t,
		serve,
		invalidBanReq,
		spaceAdminToken,
		"space.update_status",
		"space",
		spaceIDA,
	)
	invalidBanRec := serve(invalidBanReq)
	if invalidBanRec.Code != http.StatusOK {
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
	attachAdminOperationToken(
		t,
		serve,
		scopedBanReq,
		spaceAdminToken,
		"space.update_status",
		"space",
		spaceIDA,
	)
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
	attachAdminOperationToken(
		t,
		serve,
		unscopedBanReq,
		spaceAdminToken,
		"space.update_status",
		"space",
		spaceIDB,
	)
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
	attachAdminOperationToken(
		t,
		serve,
		platformBanReq,
		platformAdminToken,
		"space.update_status",
		"space",
		spaceIDB,
	)
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
	attachAdminOperationToken(
		t,
		serve,
		platformUnbanReq,
		platformAdminToken,
		"space.update_status",
		"space",
		spaceIDB,
	)
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
	attachAdminOperationToken(
		t,
		serve,
		banWithoutReasonReq,
		spaceAdminToken,
		"document.update_status",
		"document",
		docIDA,
	)
	banWithoutReasonRec := serve(banWithoutReasonReq)
	if banWithoutReasonRec.Code != http.StatusOK {
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
	attachAdminOperationToken(
		t,
		serve,
		banScopedReq,
		spaceAdminToken,
		"document.update_status",
		"document",
		docIDA,
	)
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
	attachAdminOperationToken(
		t,
		serve,
		banUnscopedReq,
		spaceAdminToken,
		"document.update_status",
		"document",
		docIDB,
	)
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
	attachAdminOperationToken(
		t,
		serve,
		platformDeleteReq,
		platformAdminToken,
		"document.delete",
		"document",
		docIDB,
	)
	platformDeleteRec := serve(platformDeleteReq)
	if platformDeleteRec.Code != http.StatusOK {
		t.Fatalf("expected platform admin delete document status 200, got %d body=%s", platformDeleteRec.Code, platformDeleteRec.Body.String())
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
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "theme-platform-admin@example.com")
	grantAdminRole(t, database, spaceAdminUserID, "space_admin")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

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
	spaceAdminCreateThemeReq := httptest.NewRequest(http.MethodPost, "/api/admin/themes", bytes.NewReader(createThemeBody))
	spaceAdminCreateThemeReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminCreateThemeReq.Header.Set("Content-Type", "application/json")
	spaceAdminCreateThemeRec := serve(spaceAdminCreateThemeReq)
	if spaceAdminCreateThemeRec.Code != http.StatusForbidden {
		t.Fatalf("expected space admin create admin theme status 403, got %d body=%s", spaceAdminCreateThemeRec.Code, spaceAdminCreateThemeRec.Body.String())
	}

	createThemeReq := httptest.NewRequest(http.MethodPost, "/api/admin/themes", bytes.NewReader(createThemeBody))
	createThemeReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	createThemeReq.Header.Set("Content-Type", "application/json")
	createThemeRec := serve(createThemeReq)
	if createThemeRec.Code != http.StatusOK {
		t.Fatalf("expected create admin theme status 201, got %d body=%s", createThemeRec.Code, createThemeRec.Body.String())
	}
	if !strings.Contains(createThemeRec.Body.String(), themeID) {
		t.Fatalf("expected create theme response contain theme id %s, body=%s", themeID, createThemeRec.Body.String())
	}

	listAdminThemesReq := httptest.NewRequest(http.MethodGet, "/api/admin/themes", nil)
	listAdminThemesReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	listAdminThemesRec := serve(listAdminThemesReq)
	if listAdminThemesRec.Code != http.StatusOK {
		t.Fatalf("expected list admin themes status 200, got %d body=%s", listAdminThemesRec.Code, listAdminThemesRec.Body.String())
	}
	if !strings.Contains(listAdminThemesRec.Body.String(), themeID) {
		t.Fatalf("expected list admin themes include created theme %s, body=%s", themeID, listAdminThemesRec.Body.String())
	}

	spaceAdminListAdminThemesReq := httptest.NewRequest(http.MethodGet, "/api/admin/themes", nil)
	spaceAdminListAdminThemesReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminListAdminThemesRec := serve(spaceAdminListAdminThemesReq)
	if spaceAdminListAdminThemesRec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected space admin list admin themes status 403, got %d body=%s",
			spaceAdminListAdminThemesRec.Code,
			spaceAdminListAdminThemesRec.Body.String(),
		)
	}

	disableThemeReq := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/themes/"+themeID,
		bytes.NewReader([]byte(`{"enabled":false}`)),
	)
	disableThemeReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	disableThemeReq.Header.Set("Content-Type", "application/json")
	attachAdminOperationToken(
		t,
		serve,
		disableThemeReq,
		platformAdminToken,
		"theme.update",
		"theme",
		themeID,
	)
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
	deleteThemeReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	attachAdminOperationToken(
		t,
		serve,
		deleteThemeReq,
		platformAdminToken,
		"theme.delete",
		"theme",
		themeID,
	)
	deleteThemeRec := serve(deleteThemeReq)
	if deleteThemeRec.Code != http.StatusOK {
		t.Fatalf("expected delete theme status 200, got %d body=%s", deleteThemeRec.Code, deleteThemeRec.Body.String())
	}

	listAdminThemesAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/api/admin/themes", nil)
	listAdminThemesAfterDeleteReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
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
	deleteBuiltinThemeReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	attachAdminOperationToken(
		t,
		serve,
		deleteBuiltinThemeReq,
		platformAdminToken,
		"theme.delete",
		"theme",
		"default",
	)
	deleteBuiltinThemeRec := serve(deleteBuiltinThemeReq)
	if deleteBuiltinThemeRec.Code != http.StatusOK {
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
	listThemeAuditReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	listThemeAuditRec := serve(listThemeAuditReq)
	if listThemeAuditRec.Code != http.StatusOK {
		t.Fatalf(
			"expected list theme audit logs status 200, got %d body=%s",
			listThemeAuditRec.Code,
			listThemeAuditRec.Body.String(),
		)
	}
	themeAuditPayload := decodeJSONResultData[struct {
		Items []struct {
			Module   string `json:"module"`
			Action   string `json:"action"`
			TargetID string `json:"targetId"`
		} `json:"items"`
		Pagination struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}](t, listThemeAuditRec.Body.Bytes())
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

	spaceAdminListThemeAuditReq := httptest.NewRequest(
		http.MethodGet,
		"/api/admin/audits?module=theme&targetId="+themeID+"&page=1&pageSize=20",
		nil,
	)
	spaceAdminListThemeAuditReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminListThemeAuditRec := serve(spaceAdminListThemeAuditReq)
	if spaceAdminListThemeAuditRec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected space admin list theme audit logs status 403, got %d body=%s",
			spaceAdminListThemeAuditRec.Code,
			spaceAdminListThemeAuditRec.Body.String(),
		)
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
	attachAdminOperationToken(
		t,
		serve,
		platformCreateReq,
		platformAdminToken,
		"system_config.upsert",
		"system_config",
		"site",
	)
	platformCreateRec := serve(platformCreateReq)
	if platformCreateRec.Code != http.StatusOK {
		t.Fatalf(
			"expected platform admin create system config status 200, got %d body=%s",
			platformCreateRec.Code,
			platformCreateRec.Body.String(),
		)
	}
	platformCreatePayload := decodeJSONResultData[struct {
		ConfigKey string `json:"configKey"`
		Version   int    `json:"version"`
	}](t, platformCreateRec.Body.Bytes())
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
	attachAdminOperationToken(
		t,
		serve,
		platformUpdateReq,
		platformAdminToken,
		"system_config.upsert",
		"system_config",
		"site",
	)
	platformUpdateRec := serve(platformUpdateReq)
	if platformUpdateRec.Code != http.StatusOK {
		t.Fatalf(
			"expected platform admin update system config status 200, got %d body=%s",
			platformUpdateRec.Code,
			platformUpdateRec.Body.String(),
		)
	}
	platformUpdatePayload := decodeJSONResultData[struct {
		Version int `json:"version"`
	}](t, platformUpdateRec.Body.Bytes())
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
	attachAdminOperationToken(
		t,
		serve,
		platformConflictReq,
		platformAdminToken,
		"system_config.upsert",
		"system_config",
		"site",
	)
	platformConflictRec := serve(platformConflictReq)
	if platformConflictRec.Code != http.StatusOK {
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
	attachAdminOperationToken(
		t,
		serve,
		platformInvalidValueReq,
		platformAdminToken,
		"system_config.upsert",
		"system_config",
		"site",
	)
	platformInvalidValueRec := serve(platformInvalidValueReq)
	if platformInvalidValueRec.Code != http.StatusOK {
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

func TestRouter_AdminSystemConfig_SitemapConfigValidation(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "config-sitemap-platform-admin@example.com")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	validReq := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/system-configs/sitemap",
		bytes.NewReader([]byte(`{"value":{"generationMode":"updated_within_days","maxUpdatedWithinDays":30}}`)),
	)
	validReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	validReq.Header.Set("Content-Type", "application/json")
	attachAdminOperationToken(
		t,
		serve,
		validReq,
		platformAdminToken,
		"system_config.upsert",
		"system_config",
		"sitemap",
	)
	validRec := serve(validReq)
	if validRec.Code != http.StatusOK {
		t.Fatalf("expected upsert sitemap config status 200, got %d body=%s", validRec.Code, validRec.Body.String())
	}
	validPayload := decodeJSONResultData[struct {
		ConfigKey string `json:"configKey"`
		Version   int    `json:"version"`
	}](t, validRec.Body.Bytes())
	if validPayload.ConfigKey != "sitemap" || validPayload.Version != 1 {
		t.Fatalf("unexpected sitemap config payload: %+v", validPayload)
	}

	invalidReq := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/system-configs/sitemap",
		bytes.NewReader([]byte(`{"expectedVersion":1,"value":{"generationMode":"bad_mode","maxUpdatedWithinDays":30}}`)),
	)
	invalidReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	invalidReq.Header.Set("Content-Type", "application/json")
	attachAdminOperationToken(
		t,
		serve,
		invalidReq,
		platformAdminToken,
		"system_config.upsert",
		"system_config",
		"sitemap",
	)
	invalidRec := serve(invalidReq)
	if invalidRec.Code != http.StatusOK {
		t.Fatalf("expected invalid sitemap config status 400, got %d body=%s", invalidRec.Code, invalidRec.Body.String())
	}
	if decodeJSONResultCode(t, invalidRec.Body.Bytes()) != response.ResolveErrorCode("INVALID_CONFIG_VALUE") {
		t.Fatalf(
			"expected code %d, got %d body=%s",
			response.ResolveErrorCode("INVALID_CONFIG_VALUE"),
			decodeJSONResultCode(t, invalidRec.Body.Bytes()),
			invalidRec.Body.String(),
		)
	}
}

func TestRouter_AdminOperationTokenRequiredAndReplayGuard(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "op-token-platform-admin@example.com")
	targetUserIDA, _, _ := registerAccessUserWithRefresh(t, serve, "op-token-target-a@example.com")
	targetUserIDB, _, _ := registerAccessUserWithRefresh(t, serve, "op-token-target-b@example.com")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	withoutTokenReq := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+targetUserIDA, nil)
	withoutTokenReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	withoutTokenRec := serve(withoutTokenReq)
	if withoutTokenRec.Code != http.StatusOK {
		t.Fatalf(
			"expected delete user without operation token status 400, got %d body=%s",
			withoutTokenRec.Code,
			withoutTokenRec.Body.String(),
		)
	}
	if decodeJSONResultCode(t, withoutTokenRec.Body.Bytes()) != response.ResolveErrorCode("OPERATION_TOKEN_REQUIRED") {
		t.Fatalf(
			"expected code %d, got %d body=%s",
			response.ResolveErrorCode("OPERATION_TOKEN_REQUIRED"),
			decodeJSONResultCode(t, withoutTokenRec.Body.Bytes()),
			withoutTokenRec.Body.String(),
		)
	}

	scopeMismatchReq := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+targetUserIDB, nil)
	scopeMismatchReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	scopeMismatchToken := issueAdminOperationToken(
		t,
		serve,
		platformAdminToken,
		"user.delete",
		"user",
		targetUserIDA,
	)
	scopeMismatchReq.Header.Set("X-Admin-Operation-Token", scopeMismatchToken)
	scopeMismatchRec := serve(scopeMismatchReq)
	if scopeMismatchRec.Code != http.StatusOK {
		t.Fatalf(
			"expected scope mismatch operation token status 409, got %d body=%s",
			scopeMismatchRec.Code,
			scopeMismatchRec.Body.String(),
		)
	}
	if decodeJSONResultCode(t, scopeMismatchRec.Body.Bytes()) != response.ResolveErrorCode("OPERATION_TOKEN_SCOPE_MISMATCH") {
		t.Fatalf(
			"expected code %d, got %d body=%s",
			response.ResolveErrorCode("OPERATION_TOKEN_SCOPE_MISMATCH"),
			decodeJSONResultCode(t, scopeMismatchRec.Body.Bytes()),
			scopeMismatchRec.Body.String(),
		)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+targetUserIDA, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	replayToken := issueAdminOperationToken(
		t,
		serve,
		platformAdminToken,
		"user.delete",
		"user",
		targetUserIDA,
	)
	deleteReq.Header.Set("X-Admin-Operation-Token", replayToken)
	deleteRec := serve(deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete user with operation token status 200, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	replayReq := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+targetUserIDA, nil)
	replayReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	replayReq.Header.Set("X-Admin-Operation-Token", replayToken)
	replayRec := serve(replayReq)
	if replayRec.Code != http.StatusOK {
		t.Fatalf(
			"expected replayed operation token status 409, got %d body=%s",
			replayRec.Code,
			replayRec.Body.String(),
		)
	}
	if decodeJSONResultCode(t, replayRec.Body.Bytes()) != response.ResolveErrorCode("OPERATION_TOKEN_REPLAYED") {
		t.Fatalf(
			"expected code %d, got %d body=%s",
			response.ResolveErrorCode("OPERATION_TOKEN_REPLAYED"),
			decodeJSONResultCode(t, replayRec.Body.Bytes()),
			replayRec.Body.String(),
		)
	}
}

func TestRouter_AdminOperationTokenIssuePermissionMatrix(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	_, _, normalUserToken := registerAccessUser(t, serve, "op-token-normal-user@example.com")
	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "op-token-space-admin@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "op-token-platform-admin-2@example.com")
	grantAdminRole(t, database, spaceAdminUserID, "space_admin")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	noTokenReq := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/operation-tokens",
		bytes.NewReader([]byte(`{"operation":"space.delete","targetType":"space","targetId":"space-1"}`)),
	)
	noTokenReq.Header.Set("Content-Type", "application/json")
	noTokenRec := serve(noTokenReq)
	if noTokenRec.Code != http.StatusForbidden {
		t.Fatalf("expected issue operation token without auth status 401, got %d body=%s", noTokenRec.Code, noTokenRec.Body.String())
	}

	normalUserReq := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/operation-tokens",
		bytes.NewReader([]byte(`{"operation":"space.delete","targetType":"space","targetId":"space-1"}`)),
	)
	normalUserReq.Header.Set("Authorization", "Bearer "+normalUserToken)
	normalUserReq.Header.Set("Content-Type", "application/json")
	normalUserRec := serve(normalUserReq)
	if normalUserRec.Code != http.StatusForbidden {
		t.Fatalf("expected issue operation token by normal user status 403, got %d body=%s", normalUserRec.Code, normalUserRec.Body.String())
	}

	invalidOperationReq := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/operation-tokens",
		bytes.NewReader([]byte(`{"operation":"","targetType":"space","targetId":"space-1"}`)),
	)
	invalidOperationReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	invalidOperationReq.Header.Set("Content-Type", "application/json")
	invalidOperationRec := serve(invalidOperationReq)
	if invalidOperationRec.Code != http.StatusOK {
		t.Fatalf(
			"expected issue operation token with invalid operation status 400, got %d body=%s",
			invalidOperationRec.Code,
			invalidOperationRec.Body.String(),
		)
	}

	spaceAdminReq := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/operation-tokens",
		bytes.NewReader([]byte(`{"operation":"space.delete","targetType":"space","targetId":"space-1"}`)),
	)
	spaceAdminReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminReq.Header.Set("Content-Type", "application/json")
	spaceAdminRec := serve(spaceAdminReq)
	if spaceAdminRec.Code != http.StatusOK {
		t.Fatalf("expected issue operation token by space admin status 200, got %d body=%s", spaceAdminRec.Code, spaceAdminRec.Body.String())
	}
	if !strings.Contains(spaceAdminRec.Body.String(), `"token":"`) {
		t.Fatalf("expected issue operation token response contains token, body=%s", spaceAdminRec.Body.String())
	}

	platformAdminReq := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/operation-tokens",
		bytes.NewReader([]byte(`{"operation":"user.delete","targetType":"user","targetId":"user-1"}`)),
	)
	platformAdminReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformAdminReq.Header.Set("Content-Type", "application/json")
	platformAdminRec := serve(platformAdminReq)
	if platformAdminRec.Code != http.StatusOK {
		t.Fatalf("expected issue operation token by platform admin status 200, got %d body=%s", platformAdminRec.Code, platformAdminRec.Body.String())
	}
}

func TestRouter_AdminOperationTokenActorBinding(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "op-token-owner@example.com")
	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "op-token-actor-space-admin@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "op-token-actor-platform-admin@example.com")
	grantAdminRole(t, database, spaceAdminUserID, "space_admin")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	spaceID := "01h1admintokenactor000000000001"
	insertAdminTestSpace(t, database, spaceID, "Token Actor Space", ownerUserID, "member")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("space_admin_scopes").Create(map[string]any{
		"user_id":    spaceAdminUserID,
		"space_id":   spaceID,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("insert space admin scope failed: %v", err)
	}

	spaceAdminTokenForDelete := issueAdminOperationToken(
		t,
		serve,
		spaceAdminToken,
		"space.delete",
		"space",
		spaceID,
	)

	platformDeleteReq := httptest.NewRequest(http.MethodDelete, "/api/admin/spaces/"+spaceID, nil)
	platformDeleteReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	platformDeleteReq.Header.Set("X-Admin-Operation-Token", spaceAdminTokenForDelete)
	platformDeleteRec := serve(platformDeleteReq)
	if platformDeleteRec.Code != http.StatusOK {
		t.Fatalf(
			"expected cross actor operation token usage status 409, got %d body=%s",
			platformDeleteRec.Code,
			platformDeleteRec.Body.String(),
		)
	}
	if decodeJSONResultCode(t, platformDeleteRec.Body.Bytes()) != response.ResolveErrorCode("OPERATION_TOKEN_INVALID") {
		t.Fatalf(
			"expected code %d, got %d body=%s",
			response.ResolveErrorCode("OPERATION_TOKEN_INVALID"),
			decodeJSONResultCode(t, platformDeleteRec.Body.Bytes()),
			platformDeleteRec.Body.String(),
		)
	}

	spaceAdminDeleteReq := httptest.NewRequest(http.MethodDelete, "/api/admin/spaces/"+spaceID, nil)
	spaceAdminDeleteReq.Header.Set("Authorization", "Bearer "+spaceAdminToken)
	spaceAdminDeleteReq.Header.Set("X-Admin-Operation-Token", spaceAdminTokenForDelete)
	spaceAdminDeleteRec := serve(spaceAdminDeleteReq)
	if spaceAdminDeleteRec.Code != http.StatusOK {
		t.Fatalf(
			"expected space admin consume own operation token delete status 200, got %d body=%s",
			spaceAdminDeleteRec.Code,
			spaceAdminDeleteRec.Body.String(),
		)
	}
}

func TestRouter_AdminAuditRequestIDFromContext(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "audit-requestid-admin@example.com")
	targetUserID, _, _ := registerAccessUserWithRefresh(t, serve, "audit-requestid-target@example.com")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	operationToken := issueAdminOperationToken(
		t,
		serve,
		platformAdminToken,
		"user.update_status",
		"user",
		targetUserID,
	)

	requestID := "ctx-request-id-001"
	updateReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/users/"+targetUserID+"/status",
		bytes.NewReader([]byte(`{"status":"banned","reason":"request id audit check"}`)),
	)
	updateReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("X-Request-Id", requestID)
	updateReq.Header.Set("X-Admin-Operation-Token", operationToken)
	updateRec := serve(updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update user status 200, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	var latestAudit struct {
		RequestID string `gorm:"column:request_id"`
	}
	if err := database.ORM.Table("audit_logs").
		Select("request_id").
		Where("module = ? AND action = ? AND target_type = ? AND target_id = ?", "user", "update", "user", targetUserID).
		Order("id DESC").
		Limit(1).
		Scan(&latestAudit).Error; err != nil {
		t.Fatalf("query audit request id failed: %v", err)
	}
	if latestAudit.RequestID != requestID {
		t.Fatalf("expected audit request id %s, got %s", requestID, latestAudit.RequestID)
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
	if rec.Code != http.StatusOK {
		t.Fatalf("register failed, status=%d body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONResultData[struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
		Token        string `json:"token"`
		RefreshToken string `json:"refreshToken"`
	}](t, rec.Body.Bytes())
	if payload.User.ID == "" || payload.Token == "" || payload.RefreshToken == "" {
		t.Fatalf("register response missing id/token/refreshToken, body=%s", rec.Body.String())
	}

	return payload.User.ID, payload.Token, payload.RefreshToken
}

func attachAdminOperationToken(
	t *testing.T,
	serve func(*http.Request) *httptest.ResponseRecorder,
	req *http.Request,
	actorToken string,
	operation string,
	targetType string,
	targetID string,
) {
	t.Helper()
	token := issueAdminOperationToken(t, serve, actorToken, operation, targetType, targetID)
	req.Header.Set("X-Admin-Operation-Token", token)
}

func issueAdminOperationToken(
	t *testing.T,
	serve func(*http.Request) *httptest.ResponseRecorder,
	actorToken string,
	operation string,
	targetType string,
	targetID string,
) string {
	t.Helper()

	requestBody, err := json.Marshal(map[string]string{
		"operation":  operation,
		"targetType": targetType,
		"targetId":   targetID,
	})
	if err != nil {
		t.Fatalf("marshal operation token request failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/operation-tokens", bytes.NewReader(requestBody))
	req.Header.Set("Authorization", "Bearer "+actorToken)
	req.Header.Set("Content-Type", "application/json")
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf(
			"issue operation token failed, status=%d body=%s",
			rec.Code,
			rec.Body.String(),
		)
	}

	payload := decodeJSONResultData[struct {
		Token string `json:"token"`
	}](t, rec.Body.Bytes())
	if strings.TrimSpace(payload.Token) == "" {
		t.Fatalf("operation token response missing token, body=%s", rec.Body.String())
	}
	return payload.Token
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
