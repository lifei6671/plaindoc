package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func setupAuthTestRouter(t *testing.T) (*storage.Database, func(*http.Request) *httptest.ResponseRecorder) {
	t.Helper()

	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-auth-handler?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}

	if err := storage.MigrateUp(context.Background(), database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}

	logger := logit.NewLogger(slog.LevelError)
	router := NewRouter(testConfig(), logger, database.ORM)
	serve := func(req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	return database, serve
}

func TestRouter_AuthRegisterAndMe(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	registerBody := []byte(`{"email":"demo@example.com","password":"123456","name":"Demo User"}`)
	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := serve(registerReq)
	if registerRec.Code != http.StatusOK {
		t.Fatalf("expected status 201, got %d, body=%s", registerRec.Code, registerRec.Body.String())
	}

	registerPayload := decodeJSONResultData[struct {
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"user"`
		Token        string `json:"token"`
		RefreshToken string `json:"refreshToken"`
	}](t, registerRec.Body.Bytes())
	if registerPayload.User.ID == "" {
		t.Fatal("expected user id in register response")
	}
	if registerPayload.Token == "" || registerPayload.RefreshToken == "" {
		t.Fatal("expected access/refresh token in register response")
	}
	assertSessionCookiesExist(t, registerRec)

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+registerPayload.Token)
	meRec := serve(meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", meRec.Code, meRec.Body.String())
	}

	mePayload := decodeJSONResultData[struct {
		User struct {
			Email string `json:"email"`
		} `json:"user"`
	}](t, meRec.Body.Bytes())
	if mePayload.User.Email != "demo@example.com" {
		t.Fatalf("expected me email demo@example.com, got %s", mePayload.User.Email)
	}

	meByCookieReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meByCookieReq.AddCookie(extractCookieByName(t, registerRec, "accessToken"))
	meByCookieRec := serve(meByCookieReq)
	if meByCookieRec.Code != http.StatusOK {
		t.Fatalf("expected cookie me status 200, got %d, body=%s", meByCookieRec.Code, meByCookieRec.Body.String())
	}
	meByCookiePayload := decodeJSONResultData[struct {
		User struct {
			Email string `json:"email"`
		} `json:"user"`
	}](t, meByCookieRec.Body.Bytes())
	if meByCookiePayload.User.Email != "demo@example.com" {
		t.Fatalf("expected cookie me email demo@example.com, got %s", meByCookiePayload.User.Email)
	}
}

func TestRouter_AuthLoginInvalidCredentials(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	registerBody := []byte(`{"email":"user@example.com","password":"123456","name":"User"}`)
	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := serve(registerReq)
	if registerRec.Code != http.StatusOK {
		t.Fatalf("register failed, status=%d body=%s", registerRec.Code, registerRec.Body.String())
	}

	loginBody := []byte(`{"email":"user@example.com","password":"wrong-password"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := serve(loginReq)
	if loginRec.Code != http.StatusForbidden {
		t.Fatalf("expected status 401, got %d, body=%s", loginRec.Code, loginRec.Body.String())
	}
}

func TestRouter_AuthRefresh(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	registerBody := []byte(`{"email":"refresh@example.com","password":"123456","name":"Refresh User"}`)
	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := serve(registerReq)
	if registerRec.Code != http.StatusOK {
		t.Fatalf("register failed, status=%d body=%s", registerRec.Code, registerRec.Body.String())
	}

	registerPayload := decodeJSONResultData[struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refreshToken"`
	}](t, registerRec.Body.Bytes())
	if registerPayload.RefreshToken == "" {
		t.Fatal("expected refresh token in register response")
	}

	refreshBody := []byte(`{"refreshToken":"` + registerPayload.RefreshToken + `"}`)
	refreshReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshRec := serve(refreshReq)
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", refreshRec.Code, refreshRec.Body.String())
	}

	refreshPayload := decodeJSONResultData[struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refreshToken"`
	}](t, refreshRec.Body.Bytes())
	if refreshPayload.Token == "" || refreshPayload.RefreshToken == "" {
		t.Fatal("expected refreshed token and refresh token")
	}

	oldTokenRefreshReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(refreshBody))
	oldTokenRefreshReq.Header.Set("Content-Type", "application/json")
	oldTokenRefreshRec := serve(oldTokenRefreshReq)
	if oldTokenRefreshRec.Code != http.StatusForbidden {
		t.Fatalf("expected old refresh token to be rejected with 401, got %d, body=%s", oldTokenRefreshRec.Code, oldTokenRefreshRec.Body.String())
	}

	nextRefreshBody := []byte(`{"refreshToken":"` + refreshPayload.RefreshToken + `"}`)
	nextRefreshReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(nextRefreshBody))
	nextRefreshReq.Header.Set("Content-Type", "application/json")
	nextRefreshRec := serve(nextRefreshReq)
	if nextRefreshRec.Code != http.StatusOK {
		t.Fatalf("expected rotated refresh token to be accepted with 200, got %d, body=%s", nextRefreshRec.Code, nextRefreshRec.Body.String())
	}
	refreshCookie := extractCookieByName(t, nextRefreshRec, "refreshToken")

	refreshByCookieReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	refreshByCookieReq.AddCookie(refreshCookie)
	refreshByCookieRec := serve(refreshByCookieReq)
	if refreshByCookieRec.Code != http.StatusOK {
		t.Fatalf("expected cookie refresh status 200, got %d, body=%s", refreshByCookieRec.Code, refreshByCookieRec.Body.String())
	}
	assertSessionCookiesExist(t, refreshByCookieRec)
}

func TestRouter_AuthLogout(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	assertSessionCookiesCleared(t, rec)
}

func TestRouter_AuthLogoutRevokesSession(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	registerBody := []byte(`{"email":"logout@example.com","password":"123456","name":"Logout User"}`)
	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := serve(registerReq)
	if registerRec.Code != http.StatusOK {
		t.Fatalf("register failed, status=%d body=%s", registerRec.Code, registerRec.Body.String())
	}

	registerPayload := decodeJSONResultData[struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refreshToken"`
	}](t, registerRec.Body.Bytes())
	if registerPayload.Token == "" || registerPayload.RefreshToken == "" {
		t.Fatal("expected access/refresh token in register response")
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+registerPayload.Token)
	logoutRec := serve(logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout failed, status=%d body=%s", logoutRec.Code, logoutRec.Body.String())
	}
	assertSessionCookiesCleared(t, logoutRec)

	refreshBody := []byte(`{"refreshToken":"` + registerPayload.RefreshToken + `"}`)
	refreshReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshRec := serve(refreshReq)
	if refreshRec.Code != http.StatusForbidden {
		t.Fatalf("expected revoked session refresh token to be rejected with 401, got %d, body=%s", refreshRec.Code, refreshRec.Body.String())
	}
}

func extractCookieByName(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("expected cookie %q in response", name)
	return nil
}

func assertSessionCookiesExist(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	accessCookie := extractCookieByName(t, rec, "accessToken")
	refreshCookie := extractCookieByName(t, rec, "refreshToken")
	if strings.TrimSpace(accessCookie.Value) == "" {
		t.Fatal("expected non-empty accessToken cookie")
	}
	if strings.TrimSpace(refreshCookie.Value) == "" {
		t.Fatal("expected non-empty refreshToken cookie")
	}
}

func assertSessionCookiesCleared(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	accessCookie := extractCookieByName(t, rec, "accessToken")
	refreshCookie := extractCookieByName(t, rec, "refreshToken")
	if accessCookie.MaxAge >= 0 {
		t.Fatalf("expected cleared accessToken cookie max-age < 0, got %d", accessCookie.MaxAge)
	}
	if refreshCookie.MaxAge >= 0 {
		t.Fatalf("expected cleared refreshToken cookie max-age < 0, got %d", refreshCookie.MaxAge)
	}
}

func TestRouter_AuthRegisterDisabledBySiteConfig(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	now := time.Now().UTC()
	if err := database.ORM.WithContext(context.Background()).Create(&models.SystemConfig{
		ConfigKey:       "site",
		ConfigValueJSON: `{"allowRegistration":false,"defaultSpaceVisibility":"member"}`,
		Version:         1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed site config failed: %v", err)
	}

	registerBody := []byte(`{"email":"closed@example.com","password":"123456","name":"Closed User"}`)
	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := serve(registerReq)
	if registerRec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d, body=%s", registerRec.Code, registerRec.Body.String())
	}

	var payload struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(registerRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode register disabled response failed: %v", err)
	}
	if payload.Code != response.ResolveErrorCode("REGISTRATION_DISABLED") {
		t.Fatalf("expected error code %d, got %d", response.ResolveErrorCode("REGISTRATION_DISABLED"), payload.Code)
	}
}
