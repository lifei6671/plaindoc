package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

var (
	ErrAdminSystemConfigInvalidKey      = errors.New("admin system config key is invalid")
	ErrAdminSystemConfigInvalidValue    = errors.New("admin system config value is invalid")
	ErrAdminSystemConfigExpectedVersion = errors.New("admin system config expected version is invalid")
	ErrAdminSystemConfigVersionConflict = errors.New("admin system config version conflict")
)

var systemConfigValidators = map[string]func(map[string]any) error{
	"site":          validateSiteConfig,
	"editor":        validateEditorConfig,
	"security":      validateSecurityConfig,
	"image-hosting": validateImageHostingConfig,
}

// AdminSystemConfigRecord 后台系统配置记录。
type AdminSystemConfigRecord struct {
	ConfigKey       string
	Value           map[string]any
	Version         int
	UpdatedByUserID *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UpsertAdminSystemConfigInput 后台系统配置写入参数。
type UpsertAdminSystemConfigInput struct {
	ActorUserID     string
	RequestID       string
	ConfigKey       string
	Value           any
	ExpectedVersion *int
}

// AdminSystemConfigService 封装后台系统配置读写。
type AdminSystemConfigService struct {
	systemConfigRepo   repository.SystemConfigRepository
	adminAccessService *AdminAccessService
	adminAuditService  *AdminAuditService
}

// NewAdminSystemConfigService 创建后台系统配置服务。
func NewAdminSystemConfigService(
	systemConfigRepo repository.SystemConfigRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
) *AdminSystemConfigService {
	return &AdminSystemConfigService{
		systemConfigRepo:   systemConfigRepo,
		adminAccessService: adminAccessService,
		adminAuditService:  adminAuditService,
	}
}

// ListConfigs 查询后台系统配置。
func (s *AdminSystemConfigService) ListConfigs(
	ctx context.Context,
	actorUserID string,
) ([]AdminSystemConfigRecord, error) {
	if s == nil || s.systemConfigRepo == nil || s.adminAccessService == nil {
		return nil, errors.New("admin system config service dependencies are nil")
	}
	if err := s.ensurePlatformAdmin(ctx, actorUserID); err != nil {
		return nil, err
	}

	configs, err := s.systemConfigRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]AdminSystemConfigRecord, 0, len(configs))
	for _, item := range configs {
		record, mapErr := mapSystemConfigToRecord(item)
		if mapErr != nil {
			return nil, mapErr
		}
		items = append(items, record)
	}
	return items, nil
}

// UpsertConfig 创建或更新系统配置（包含 schema 校验与版本控制）。
func (s *AdminSystemConfigService) UpsertConfig(
	ctx context.Context,
	input UpsertAdminSystemConfigInput,
) (AdminSystemConfigRecord, error) {
	if s == nil || s.systemConfigRepo == nil || s.adminAccessService == nil {
		return AdminSystemConfigRecord{}, errors.New("admin system config service dependencies are nil")
	}
	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return AdminSystemConfigRecord{}, err
	}

	configKey, validator, err := resolveSystemConfigValidator(input.ConfigKey)
	if err != nil {
		return AdminSystemConfigRecord{}, err
	}

	valueMap, ok := input.Value.(map[string]any)
	if !ok || valueMap == nil {
		return AdminSystemConfigRecord{}, ErrAdminSystemConfigInvalidValue
	}
	if err := validator(valueMap); err != nil {
		return AdminSystemConfigRecord{}, fmt.Errorf("%w: %v", ErrAdminSystemConfigInvalidValue, err)
	}
	valueJSONBytes, err := json.Marshal(valueMap)
	if err != nil {
		return AdminSystemConfigRecord{}, err
	}
	valueJSON := string(valueJSONBytes)

	existing, err := s.systemConfigRepo.GetByConfigKey(ctx, configKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return AdminSystemConfigRecord{}, err
	}

	now := time.Now().UTC()
	actorUserID := strings.TrimSpace(input.ActorUserID)
	var updatedBy *string
	if actorUserID != "" {
		updatedBy = &actorUserID
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		if input.ExpectedVersion != nil && *input.ExpectedVersion != 0 {
			return AdminSystemConfigRecord{}, ErrAdminSystemConfigVersionConflict
		}

		createConfig := &models.SystemConfig{
			ConfigKey:       configKey,
			ConfigValueJSON: valueJSON,
			Version:         1,
			UpdatedByUserID: updatedBy,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := s.systemConfigRepo.Create(ctx, createConfig); err != nil {
			return AdminSystemConfigRecord{}, err
		}

		latestConfig, err := s.systemConfigRepo.GetByConfigKey(ctx, configKey)
		if err != nil {
			return AdminSystemConfigRecord{}, err
		}
		record, err := mapSystemConfigToRecord(*latestConfig)
		if err != nil {
			return AdminSystemConfigRecord{}, err
		}

		if err := s.recordSystemConfigAudit(
			ctx,
			AdminAuditActionCreate,
			record,
			input.ExpectedVersion,
			valueMap,
		); err != nil {
			return AdminSystemConfigRecord{}, err
		}

		return record, nil
	}

	if existing == nil {
		return AdminSystemConfigRecord{}, ErrAdminSystemConfigVersionConflict
	}
	if input.ExpectedVersion != nil && *input.ExpectedVersion <= 0 {
		return AdminSystemConfigRecord{}, ErrAdminSystemConfigExpectedVersion
	}
	if input.ExpectedVersion != nil && existing.Version != *input.ExpectedVersion {
		return AdminSystemConfigRecord{}, ErrAdminSystemConfigVersionConflict
	}

	expectedVersion := existing.Version
	updated, err := s.systemConfigRepo.UpdateByVersion(ctx, repository.UpdateSystemConfigByVersionParams{
		ConfigKey:       configKey,
		ConfigValueJSON: valueJSON,
		ExpectedVersion: expectedVersion,
		NextVersion:     expectedVersion + 1,
		UpdatedByUserID: updatedBy,
		UpdatedAt:       now,
	})
	if err != nil {
		return AdminSystemConfigRecord{}, err
	}
	if !updated {
		return AdminSystemConfigRecord{}, ErrAdminSystemConfigVersionConflict
	}

	latestConfig, err := s.systemConfigRepo.GetByConfigKey(ctx, configKey)
	if err != nil {
		return AdminSystemConfigRecord{}, err
	}
	record, err := mapSystemConfigToRecord(*latestConfig)
	if err != nil {
		return AdminSystemConfigRecord{}, err
	}

	if err := s.recordSystemConfigAudit(
		ctx,
		AdminAuditActionUpdate,
		record,
		&expectedVersion,
		valueMap,
	); err != nil {
		return AdminSystemConfigRecord{}, err
	}

	return record, nil
}

func (s *AdminSystemConfigService) ensurePlatformAdmin(ctx context.Context, actorUserID string) error {
	userID := strings.TrimSpace(actorUserID)
	if userID == "" {
		return ErrAdminForbidden
	}

	isPlatformAdmin, err := s.adminAccessService.IsPlatformAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if !isPlatformAdmin {
		return ErrAdminForbidden
	}
	return nil
}

func resolveSystemConfigValidator(
	rawConfigKey string,
) (string, func(map[string]any) error, error) {
	configKey := strings.ToLower(strings.TrimSpace(rawConfigKey))
	validator, exists := systemConfigValidators[configKey]
	if !exists {
		return "", nil, ErrAdminSystemConfigInvalidKey
	}
	return configKey, validator, nil
}

func (s *AdminSystemConfigService) recordSystemConfigAudit(
	ctx context.Context,
	action AdminAuditAction,
	record AdminSystemConfigRecord,
	expectedVersion *int,
	valueMap map[string]any,
) error {
	if s == nil || s.adminAuditService == nil {
		return nil
	}

	summaryPrefix := "system config updated: "
	if action == AdminAuditActionCreate {
		summaryPrefix = "system config created: "
	}

	detail := map[string]any{
		"configKey": record.ConfigKey,
		"version":   record.Version,
		"value":     valueMap,
	}
	if expectedVersion != nil {
		detail["expectedVersion"] = *expectedVersion
	}

	return s.adminAuditService.Record(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSystemConfig,
		Action:     action,
		TargetType: "system_config",
		TargetID:   record.ConfigKey,
		Summary:    summaryPrefix + record.ConfigKey,
		Detail:     detail,
	})
}

func mapSystemConfigToRecord(value models.SystemConfig) (AdminSystemConfigRecord, error) {
	var payload map[string]any
	if strings.TrimSpace(value.ConfigValueJSON) == "" {
		payload = map[string]any{}
	} else {
		if err := json.Unmarshal([]byte(value.ConfigValueJSON), &payload); err != nil {
			return AdminSystemConfigRecord{}, err
		}
		if payload == nil {
			payload = map[string]any{}
		}
	}

	return AdminSystemConfigRecord{
		ConfigKey:       value.ConfigKey,
		Value:           payload,
		Version:         value.Version,
		UpdatedByUserID: value.UpdatedByUserID,
		CreatedAt:       value.CreatedAt,
		UpdatedAt:       value.UpdatedAt,
	}, nil
}

func validateSiteConfig(payload map[string]any) error {
	requiredKeys := map[string]struct{}{
		"allowRegistration":      {},
		"defaultSpaceVisibility": {},
	}
	if err := validateNoUnknownKeys(payload, requiredKeys); err != nil {
		return err
	}

	if _, err := getRequiredBool(payload, "allowRegistration"); err != nil {
		return err
	}

	defaultVisibility, err := getRequiredString(payload, "defaultSpaceVisibility")
	if err != nil {
		return err
	}
	if !models.IsValidVisibility(models.Visibility(defaultVisibility)) {
		return fmt.Errorf("defaultSpaceVisibility must be public/authenticated/member")
	}

	return nil
}

func validateEditorConfig(payload map[string]any) error {
	requiredKeys := map[string]struct{}{
		"autosaveIntervalSeconds": {},
		"maxDocumentSizeKB":       {},
	}
	if err := validateNoUnknownKeys(payload, requiredKeys); err != nil {
		return err
	}

	autosaveInterval, err := getRequiredInt(payload, "autosaveIntervalSeconds")
	if err != nil {
		return err
	}
	if autosaveInterval < 5 || autosaveInterval > 600 {
		return fmt.Errorf("autosaveIntervalSeconds must be between 5 and 600")
	}

	maxDocumentSizeKB, err := getRequiredInt(payload, "maxDocumentSizeKB")
	if err != nil {
		return err
	}
	if maxDocumentSizeKB < 64 || maxDocumentSizeKB > 4096 {
		return fmt.Errorf("maxDocumentSizeKB must be between 64 and 4096")
	}

	return nil
}

func validateSecurityConfig(payload map[string]any) error {
	requiredKeys := map[string]struct{}{
		"accessTokenTTLMinutes":  {},
		"refreshTokenTTLMinutes": {},
	}
	if err := validateNoUnknownKeys(payload, requiredKeys); err != nil {
		return err
	}

	accessTokenTTL, err := getRequiredInt(payload, "accessTokenTTLMinutes")
	if err != nil {
		return err
	}
	if accessTokenTTL < 5 || accessTokenTTL > 1440 {
		return fmt.Errorf("accessTokenTTLMinutes must be between 5 and 1440")
	}

	refreshTokenTTL, err := getRequiredInt(payload, "refreshTokenTTLMinutes")
	if err != nil {
		return err
	}
	if refreshTokenTTL < 60 || refreshTokenTTL > 43200 {
		return fmt.Errorf("refreshTokenTTLMinutes must be between 60 and 43200")
	}

	return nil
}

func validateImageHostingConfig(payload map[string]any) error {
	requiredKeys := map[string]struct{}{
		"defaultProvider": {},
		"cloudflareR2":    {},
		"aliyunOss":       {},
		"local":           {},
	}
	if err := validateNoUnknownKeys(payload, requiredKeys); err != nil {
		return err
	}

	defaultProvider, err := getRequiredString(payload, "defaultProvider")
	if err != nil {
		return err
	}
	switch defaultProvider {
	case string(ImageHostingProviderLocal),
		string(ImageHostingProviderCloudflareR2),
		string(ImageHostingProviderAliyunOSS):
	default:
		return fmt.Errorf("defaultProvider must be local/cloudflare-r2/aliyun-oss")
	}

	cloudflareR2, err := getRequiredObject(payload, "cloudflareR2")
	if err != nil {
		return err
	}
	if err := validateNoUnknownKeys(cloudflareR2, map[string]struct{}{
		"accountId":       {},
		"bucket":          {},
		"accessKeyId":     {},
		"secretAccessKey": {},
		"publicBaseUrl":   {},
	}); err != nil {
		return fmt.Errorf("cloudflareR2 %w", err)
	}
	cloudflareAccountID, err := getRequiredStringAllowEmpty(cloudflareR2, "accountId")
	if err != nil {
		return err
	}
	cloudflareBucket, err := getRequiredStringAllowEmpty(cloudflareR2, "bucket")
	if err != nil {
		return err
	}
	cloudflareAccessKeyID, err := getRequiredStringAllowEmpty(cloudflareR2, "accessKeyId")
	if err != nil {
		return err
	}
	cloudflareSecretAccessKey, err := getRequiredStringAllowEmpty(cloudflareR2, "secretAccessKey")
	if err != nil {
		return err
	}
	cloudflarePublicBaseURL, err := getRequiredStringAllowEmpty(cloudflareR2, "publicBaseUrl")
	if err != nil {
		return err
	}

	aliyunOSS, err := getRequiredObject(payload, "aliyunOss")
	if err != nil {
		return err
	}
	if err := validateNoUnknownKeys(aliyunOSS, map[string]struct{}{
		"region":          {},
		"bucket":          {},
		"endpoint":        {},
		"accessKeyId":     {},
		"accessKeySecret": {},
		"publicBaseUrl":   {},
	}); err != nil {
		return fmt.Errorf("aliyunOss %w", err)
	}
	aliyunRegion, err := getRequiredStringAllowEmpty(aliyunOSS, "region")
	if err != nil {
		return err
	}
	aliyunBucket, err := getRequiredStringAllowEmpty(aliyunOSS, "bucket")
	if err != nil {
		return err
	}
	aliyunEndpoint, err := getRequiredStringAllowEmpty(aliyunOSS, "endpoint")
	if err != nil {
		return err
	}
	aliyunAccessKeyID, err := getRequiredStringAllowEmpty(aliyunOSS, "accessKeyId")
	if err != nil {
		return err
	}
	aliyunAccessKeySecret, err := getRequiredStringAllowEmpty(aliyunOSS, "accessKeySecret")
	if err != nil {
		return err
	}
	aliyunPublicBaseURL, err := getRequiredStringAllowEmpty(aliyunOSS, "publicBaseUrl")
	if err != nil {
		return err
	}

	local, err := getRequiredObject(payload, "local")
	if err != nil {
		return err
	}
	if err := validateNoUnknownKeys(local, map[string]struct{}{
		"uploadEndpoint": {},
		"publicBaseUrl":  {},
	}); err != nil {
		return fmt.Errorf("local %w", err)
	}
	localUploadEndpoint, err := getRequiredString(local, "uploadEndpoint")
	if err != nil {
		return err
	}
	localPublicBaseURL, err := getRequiredString(local, "publicBaseUrl")
	if err != nil {
		return err
	}

	switch defaultProvider {
	case string(ImageHostingProviderCloudflareR2):
		if cloudflareAccountID == "" ||
			cloudflareBucket == "" ||
			cloudflareAccessKeyID == "" ||
			cloudflareSecretAccessKey == "" ||
			cloudflarePublicBaseURL == "" {
			return fmt.Errorf("cloudflareR2 is incomplete for default provider")
		}
	case string(ImageHostingProviderAliyunOSS):
		if aliyunBucket == "" || aliyunAccessKeyID == "" || aliyunAccessKeySecret == "" || aliyunPublicBaseURL == "" {
			return fmt.Errorf("aliyunOss is incomplete for default provider")
		}
		if aliyunRegion == "" && aliyunEndpoint == "" {
			return fmt.Errorf("aliyunOss requires endpoint or region for default provider")
		}
	case string(ImageHostingProviderLocal):
		if localUploadEndpoint == "" || localPublicBaseURL == "" {
			return fmt.Errorf("local is incomplete for default provider")
		}
	}

	return nil
}

func validateNoUnknownKeys(payload map[string]any, allowed map[string]struct{}) error {
	if len(payload) != len(allowed) {
		return fmt.Errorf("unexpected config keys")
	}
	for key := range payload {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("unknown config key %q", key)
		}
	}
	return nil
}

func getRequiredBool(payload map[string]any, key string) (bool, error) {
	rawValue, ok := payload[key]
	if !ok {
		return false, fmt.Errorf("%s is required", key)
	}
	value, ok := rawValue.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be boolean", key)
	}
	return value, nil
}

func getRequiredString(payload map[string]any, key string) (string, error) {
	rawValue, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	value, ok := rawValue.(string)
	if !ok {
		return "", fmt.Errorf("%s must be string", key)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s must not be empty", key)
	}
	return value, nil
}

func getRequiredStringAllowEmpty(payload map[string]any, key string) (string, error) {
	rawValue, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	value, ok := rawValue.(string)
	if !ok {
		return "", fmt.Errorf("%s must be string", key)
	}
	return strings.TrimSpace(value), nil
}

func getRequiredObject(payload map[string]any, key string) (map[string]any, error) {
	rawValue, ok := payload[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	value, ok := rawValue.(map[string]any)
	if !ok || value == nil {
		return nil, fmt.Errorf("%s must be object", key)
	}
	return value, nil
}

func getRequiredInt(payload map[string]any, key string) (int, error) {
	rawValue, ok := payload[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	number, ok := rawValue.(float64)
	if !ok {
		return 0, fmt.Errorf("%s must be number", key)
	}
	if number != math.Trunc(number) {
		return 0, fmt.Errorf("%s must be integer", key)
	}
	return int(number), nil
}
