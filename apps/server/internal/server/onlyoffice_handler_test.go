package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRouter_OnlyOfficeConfig(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, accessToken := registerAccessUser(t, serve, "onlyoffice-config@example.com")
	if ownerUserID == "" {
		t.Fatal("expected owner user id")
	}

	getConfigReq := httptest.NewRequest(http.MethodGet, "/api/onlyoffice", nil)
	getConfigReq.Header.Set("Authorization", "Bearer "+accessToken)
	getConfigRec := serve(getConfigReq)
	if getConfigRec.Code != http.StatusOK {
		t.Fatalf("expected get onlyoffice config status 200, got %d body=%s", getConfigRec.Code, getConfigRec.Body.String())
	}

	configPayload := decodeJSONResultData[map[string]any](t, getConfigRec.Body.Bytes())
	enabled, _ := configPayload["enabled"].(bool)
	if enabled {
		t.Fatalf("expected default onlyoffice enabled=false, got %+v", configPayload)
	}
	if _, exists := configPayload["jwtSecret"]; exists {
		t.Fatalf("unexpected jwtSecret leak in client response: %+v", configPayload)
	}
	if _, exists := configPayload["documentServerUrl"]; exists {
		t.Fatalf("unexpected documentServerUrl leak in client response: %+v", configPayload)
	}
	if _, exists := configPayload["callbackPublicBaseUrl"]; exists {
		t.Fatalf("unexpected callbackPublicBaseUrl leak in client response: %+v", configPayload)
	}

	now := time.Now().UTC()
	if err := database.ORM.Table("system_configs").Create(map[string]any{
		"config_key":         "onlyoffice",
		"config_value_json":  `{"enabled":true,"documentServerUrl":"https://onlyoffice.example.com","callbackPublicBaseUrl":"https://api.example.com","jwtSecret":"secret"}`,
		"version":            1,
		"updated_by_user_id": nil,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert onlyoffice system config failed: %v", err)
	}

	getConfigAfterInsertReq := httptest.NewRequest(http.MethodGet, "/api/onlyoffice", nil)
	getConfigAfterInsertReq.Header.Set("Authorization", "Bearer "+accessToken)
	getConfigAfterInsertRec := serve(getConfigAfterInsertReq)
	if getConfigAfterInsertRec.Code != http.StatusOK {
		t.Fatalf(
			"expected get onlyoffice config status 200 after insert, got %d body=%s",
			getConfigAfterInsertRec.Code,
			getConfigAfterInsertRec.Body.String(),
		)
	}

	configAfterInsertPayload := decodeJSONResultData[map[string]any](t, getConfigAfterInsertRec.Body.Bytes())
	enabledAfterInsert, _ := configAfterInsertPayload["enabled"].(bool)
	if !enabledAfterInsert {
		t.Fatalf("expected onlyoffice enabled=true after insert, got %+v", configAfterInsertPayload)
	}
	if _, exists := configAfterInsertPayload["jwtSecret"]; exists {
		t.Fatalf("unexpected jwtSecret leak after insert: %+v", configAfterInsertPayload)
	}

	if err := database.ORM.Table("system_configs").Create(map[string]any{
		"config_key":         "onlyoffice-integration",
		"config_value_json":  `{"enabled":false,"documentServerUrl":"https://new-onlyoffice.example.com","callbackPublicBaseUrl":"https://new-api.example.com","jwtSecret":"new-secret"}`,
		"version":            1,
		"updated_by_user_id": nil,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert onlyoffice-integration system config failed: %v", err)
	}

	getConfigWithNewKeyReq := httptest.NewRequest(http.MethodGet, "/api/onlyoffice", nil)
	getConfigWithNewKeyReq.Header.Set("Authorization", "Bearer "+accessToken)
	getConfigWithNewKeyRec := serve(getConfigWithNewKeyReq)
	if getConfigWithNewKeyRec.Code != http.StatusOK {
		t.Fatalf("expected get onlyoffice config status 200 after new key insert, got %d body=%s", getConfigWithNewKeyRec.Code, getConfigWithNewKeyRec.Body.String())
	}
	configWithNewKeyPayload := decodeJSONResultData[map[string]any](t, getConfigWithNewKeyRec.Body.Bytes())
	enabledWithNewKey, _ := configWithNewKeyPayload["enabled"].(bool)
	if enabledWithNewKey {
		t.Fatalf("expected onlyoffice new key to override legacy key and return enabled=false, got %+v", configWithNewKeyPayload)
	}
}
