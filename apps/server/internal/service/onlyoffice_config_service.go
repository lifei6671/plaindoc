package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	// SystemConfigKeyOnlyOffice ONLYOFFICE 系统配置键。
	SystemConfigKeyOnlyOffice = "onlyoffice"
	onlyOfficeSecretMask      = "********"
)

// OnlyOfficeConfig ONLYOFFICE 运行配置。
type OnlyOfficeConfig struct {
	Enabled               bool   `json:"enabled"`
	DocumentServerURL     string `json:"documentServerUrl"`
	CallbackPublicBaseURL string `json:"callbackPublicBaseUrl"`
	JWTSecret             string `json:"jwtSecret"`
}

// OnlyOfficeConfigService 统一读取 ONLYOFFICE 系统配置。
type OnlyOfficeConfigService struct {
	systemConfigRepo repository.SystemConfigRepository
}

// NewOnlyOfficeConfigService 创建 ONLYOFFICE 配置服务。
func NewOnlyOfficeConfigService(systemConfigRepo repository.SystemConfigRepository) *OnlyOfficeConfigService {
	return &OnlyOfficeConfigService{
		systemConfigRepo: systemConfigRepo,
	}
}

// DefaultOnlyOfficeConfig 返回 ONLYOFFICE 默认配置。
func DefaultOnlyOfficeConfig() OnlyOfficeConfig {
	return OnlyOfficeConfig{
		Enabled:               false,
		DocumentServerURL:     "",
		CallbackPublicBaseURL: "",
		JWTSecret:             "",
	}
}

// NormalizeOnlyOfficeConfig 将任意 ONLYOFFICE 配置归一为可用结构。
func NormalizeOnlyOfficeConfig(value map[string]any) OnlyOfficeConfig {
	config := DefaultOnlyOfficeConfig()
	if value == nil {
		return config
	}

	config.Enabled = readBool(value, "enabled", config.Enabled)
	config.DocumentServerURL = normalizeOnlyOfficeAbsoluteURL(readString(value, "documentServerUrl"))
	config.CallbackPublicBaseURL = normalizeOnlyOfficeAbsoluteURL(readString(value, "callbackPublicBaseUrl"))
	config.JWTSecret = readString(value, "jwtSecret")
	return config
}

// ToMap 将 ONLYOFFICE 配置序列化为可持久化结构。
func (c OnlyOfficeConfig) ToMap() map[string]any {
	return map[string]any{
		"enabled":               c.Enabled,
		"documentServerUrl":     strings.TrimSpace(c.DocumentServerURL),
		"callbackPublicBaseUrl": strings.TrimSpace(c.CallbackPublicBaseURL),
		"jwtSecret":             strings.TrimSpace(c.JWTSecret),
	}
}

// GetConfig 返回当前生效 ONLYOFFICE 配置；未配置时回退默认值。
func (s *OnlyOfficeConfigService) GetConfig(ctx context.Context) (OnlyOfficeConfig, error) {
	defaultConfig := DefaultOnlyOfficeConfig()
	if s == nil || s.systemConfigRepo == nil {
		return defaultConfig, nil
	}

	record, err := s.systemConfigRepo.GetByConfigKey(ctx, SystemConfigKeyOnlyOffice)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultConfig, nil
		}
		return defaultConfig, err
	}
	if record == nil || strings.TrimSpace(record.ConfigValueJSON) == "" {
		return defaultConfig, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(record.ConfigValueJSON), &payload); err != nil {
		return defaultConfig, err
	}
	return NormalizeOnlyOfficeConfig(payload), nil
}

func validateOnlyOfficeConfig(payload map[string]any) error {
	requiredKeys := map[string]struct{}{
		"enabled":               {},
		"documentServerUrl":     {},
		"callbackPublicBaseUrl": {},
		"jwtSecret":             {},
	}
	if err := validateNoUnknownKeys(payload, requiredKeys); err != nil {
		return err
	}

	enabled, err := getRequiredBool(payload, "enabled")
	if err != nil {
		return err
	}
	documentServerURL, err := getRequiredStringAllowEmpty(payload, "documentServerUrl")
	if err != nil {
		return err
	}
	callbackPublicBaseURL, err := getRequiredStringAllowEmpty(payload, "callbackPublicBaseUrl")
	if err != nil {
		return err
	}
	jwtSecret, err := getRequiredStringAllowEmpty(payload, "jwtSecret")
	if err != nil {
		return err
	}
	if len([]rune(jwtSecret)) > 512 {
		return fmt.Errorf("jwtSecret must be at most 512 characters")
	}
	if !enabled {
		return nil
	}
	if err := validateOnlyOfficeRequiredAbsoluteURL("documentServerUrl", documentServerURL); err != nil {
		return err
	}
	if err := validateOnlyOfficeRequiredAbsoluteURL("callbackPublicBaseUrl", callbackPublicBaseURL); err != nil {
		return err
	}
	return nil
}

func maskOnlyOfficeConfigSecrets(value map[string]any) map[string]any {
	normalized, err := cloneMapAny(value)
	if err != nil {
		return map[string]any{}
	}
	jwtSecret, hasJWTSecret, jwtSecretErr := getOptionalString(normalized, "jwtSecret")
	if jwtSecretErr == nil && hasJWTSecret && strings.TrimSpace(jwtSecret) != "" {
		normalized["jwtSecret"] = onlyOfficeSecretMask
	}
	return normalized
}

func normalizeOnlyOfficeConfigSecretsForPersist(
	value map[string]any,
	existing *models.SystemConfig,
) (map[string]any, error) {
	normalizedValue, err := cloneMapAny(value)
	if err != nil {
		return nil, err
	}

	jwtSecret, err := getRequiredStringAllowEmpty(normalizedValue, "jwtSecret")
	if err != nil {
		return nil, err
	}
	if jwtSecret != onlyOfficeSecretMask {
		return normalizedValue, nil
	}

	if existing == nil || strings.TrimSpace(existing.ConfigValueJSON) == "" {
		return nil, fmt.Errorf("jwtSecret is masked but no stored secret exists")
	}

	var existingPayload map[string]any
	if err := json.Unmarshal([]byte(existing.ConfigValueJSON), &existingPayload); err != nil {
		return nil, err
	}
	existingJWTSecret, _, err := getOptionalString(existingPayload, "jwtSecret")
	if err != nil {
		return nil, err
	}
	normalizedValue["jwtSecret"] = existingJWTSecret
	return normalizedValue, nil
}

func normalizeOnlyOfficeAbsoluteURL(rawValue string) string {
	trimmed := strings.TrimSpace(rawValue)
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed == nil {
		return trimmed
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return trimmed
	}
	return parsed.String()
}

func validateOnlyOfficeRequiredAbsoluteURL(field string, rawValue string) error {
	normalized := normalizeOnlyOfficeAbsoluteURL(rawValue)
	if normalized == "" {
		return fmt.Errorf("%s must not be empty when onlyoffice is enabled", field)
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed == nil {
		return fmt.Errorf("%s must be valid http/https URL", field)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%s must be valid http/https URL", field)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("%s must be valid http/https URL", field)
	}
	return nil
}
