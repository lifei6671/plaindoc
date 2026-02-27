package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

var systemConfigValidators = map[string]func(map[string]any) error{
	"site":                          validateSiteConfig,
	"editor":                        validateEditorConfig,
	"security":                      validateSecurityConfig,
	SystemConfigKeyAuth:             validateAuthConfig,
	"image-hosting":                 validateImageHostingConfig,
	SitemapConfigKey:                validateSitemapConfig,
	HomepageAnonymousCacheConfigKey: validateHomepageAnonymousCacheConfig,
}

const (
	SystemConfigKeyAuth  = "auth"
	authConfigSecretMask = "********"
)

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

// TestAdminSystemConfigLDAPConnectionInput 后台 LDAP 连接测试参数。
type TestAdminSystemConfigLDAPConnectionInput struct {
	ActorUserID string
	Value       any
	ProviderID  string
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
) (result []AdminSystemConfigRecord, err error) {
	defer func() {
		err = errcode.MapAdminSystemConfigError(err)
	}()

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
) (result AdminSystemConfigRecord, err error) {
	defer func() {
		err = errcode.MapAdminSystemConfigError(err)
	}()

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
	existingNotFound := false
	existing, err := s.systemConfigRepo.GetByConfigKey(ctx, configKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			existingNotFound = true
		} else {
			return AdminSystemConfigRecord{}, err
		}
	}

	valueMap, ok := input.Value.(map[string]any)
	if !ok || valueMap == nil {
		return AdminSystemConfigRecord{}, errcode.ErrAdminSystemConfigInvalidValue
	}
	if configKey == SystemConfigKeyAuth {
		valueMap, err = normalizeAuthConfigSecretsForPersist(valueMap, existing)
		if err != nil {
			return AdminSystemConfigRecord{}, fmt.Errorf("%w: %v", errcode.ErrAdminSystemConfigInvalidValue, err)
		}
	}
	if err := validator(valueMap); err != nil {
		return AdminSystemConfigRecord{}, fmt.Errorf("%w: %v", errcode.ErrAdminSystemConfigInvalidValue, err)
	}
	valueJSONBytes, err := json.Marshal(valueMap)
	if err != nil {
		return AdminSystemConfigRecord{}, err
	}
	valueJSON := string(valueJSONBytes)

	now := time.Now().UTC()
	actorUserID := strings.TrimSpace(input.ActorUserID)
	var updatedBy *string
	if actorUserID != "" {
		updatedBy = &actorUserID
	}

	if existingNotFound {
		if input.ExpectedVersion != nil && *input.ExpectedVersion != 0 {
			return AdminSystemConfigRecord{}, errcode.ErrAdminSystemConfigVersionConflict
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
		return AdminSystemConfigRecord{}, errcode.ErrAdminSystemConfigVersionConflict
	}
	if input.ExpectedVersion != nil && *input.ExpectedVersion <= 0 {
		return AdminSystemConfigRecord{}, errcode.ErrAdminSystemConfigExpectedVersion
	}
	if input.ExpectedVersion != nil && existing.Version != *input.ExpectedVersion {
		return AdminSystemConfigRecord{}, errcode.ErrAdminSystemConfigVersionConflict
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
		return AdminSystemConfigRecord{}, errcode.ErrAdminSystemConfigVersionConflict
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

// TestLDAPConnection 测试 auth 配置中的 LDAP provider 连通性，不落库存储。
func (s *AdminSystemConfigService) TestLDAPConnection(
	ctx context.Context,
	input TestAdminSystemConfigLDAPConnectionInput,
) (err error) {
	defer func() {
		logit.SetRequestAttrs(ctx, logit.Error("errmsg", err))
		err = errcode.MapAdminSystemConfigError(err)
	}()

	if s == nil || s.systemConfigRepo == nil || s.adminAccessService == nil {
		return errors.New("admin system config service dependencies are nil")
	}
	if err := s.ensurePlatformAdmin(ctx, input.ActorUserID); err != nil {
		return err
	}

	valueMap, ok := input.Value.(map[string]any)
	if !ok || valueMap == nil {
		return errcode.ErrAdminSystemConfigInvalidValue
	}
	existing, err := s.systemConfigRepo.GetByConfigKey(ctx, SystemConfigKeyAuth)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	normalizedValueMap, err := normalizeAuthConfigSecretsForPersist(valueMap, existing)
	if err != nil {
		return fmt.Errorf("%w: %v", errcode.ErrAdminSystemConfigInvalidValue, err)
	}
	if err := validateAuthConfig(normalizedValueMap); err != nil {
		return fmt.Errorf("%w: %v", errcode.ErrAdminSystemConfigInvalidValue, err)
	}

	providerID := strings.TrimSpace(input.ProviderID)
	if providerID == "" {
		defaultProviderID, err := getRequiredString(normalizedValueMap, "defaultProviderId")
		if err != nil {
			return fmt.Errorf("%w: %v", errcode.ErrAdminSystemConfigInvalidValue, err)
		}
		providerID = defaultProviderID
	}
	ldapProviderConfig, err := buildLDAPProviderConfigFromAuthConfig(normalizedValueMap, providerID)
	if err != nil {
		return fmt.Errorf("%w: %v", errcode.ErrAdminSystemConfigInvalidValue, err)
	}

	ldapProvider, err := NewLDAPAuthLoginProvider(ldapProviderConfig, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", errcode.ErrAdminSystemConfigInvalidValue, err)
	}
	if err := ldapProvider.CheckHealth(ctx); err != nil {
		return fmt.Errorf("%w: ldap provider test failed", errcode.ErrAdminSystemConfigInvalidValue)
	}

	return nil
}

func (s *AdminSystemConfigService) ensurePlatformAdmin(ctx context.Context, actorUserID string) error {
	userID := strings.TrimSpace(actorUserID)
	if userID == "" {
		return errcode.ErrAdminForbidden
	}

	isPlatformAdmin, err := s.adminAccessService.IsPlatformAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if !isPlatformAdmin {
		return errcode.ErrAdminForbidden
	}
	return nil
}

func resolveSystemConfigValidator(
	rawConfigKey string,
) (string, func(map[string]any) error, error) {
	configKey := strings.ToLower(strings.TrimSpace(rawConfigKey))
	validator, exists := systemConfigValidators[configKey]
	if !exists {
		return "", nil, errcode.ErrAdminSystemConfigInvalidKey
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

	detailValue := valueMap
	if strings.EqualFold(strings.TrimSpace(record.ConfigKey), SystemConfigKeyAuth) {
		detailValue = maskAuthConfigSecrets(valueMap)
	}

	detail := map[string]any{
		"configKey": record.ConfigKey,
		"version":   record.Version,
		"value":     detailValue,
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
	if strings.EqualFold(strings.TrimSpace(value.ConfigKey), SystemConfigKeyAuth) {
		payload = maskAuthConfigSecrets(payload)
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

func validateAuthConfig(payload map[string]any) error {
	requiredKeys := map[string]struct{}{
		"loginMode":         {},
		"defaultProviderId": {},
		"allowUserRegister": {},
		"providers":         {},
		"breakGlass":        {},
	}
	if err := validateNoUnknownKeys(payload, requiredKeys); err != nil {
		return err
	}

	loginMode, err := getRequiredString(payload, "loginMode")
	if err != nil {
		return err
	}
	switch loginMode {
	case "local_only", "ldap_only", "mixed":
	default:
		return fmt.Errorf("loginMode must be local_only/ldap_only/mixed")
	}

	defaultProviderID, err := getRequiredString(payload, "defaultProviderId")
	if err != nil {
		return err
	}

	allowUserRegister, err := getRequiredBool(payload, "allowUserRegister")
	if err != nil {
		return err
	}
	if loginMode == "ldap_only" && allowUserRegister {
		return fmt.Errorf("allowUserRegister must be false in ldap_only mode")
	}

	providers, err := getRequiredArray(payload, "providers")
	if err != nil {
		return err
	}
	providerIndexByID := make(map[string]int, len(providers))
	enabledLDAPProviderCount := 0
	for index, rawProvider := range providers {
		provider, ok := rawProvider.(map[string]any)
		if !ok || provider == nil {
			return fmt.Errorf("providers[%d] must be object", index)
		}

		if err := validateNoUnknownKeys(provider, map[string]struct{}{
			"id":         {},
			"name":       {},
			"type":       {},
			"enabled":    {},
			"priority":   {},
			"matchRules": {},
			"ldap":       {},
		}); err != nil {
			return fmt.Errorf("providers[%d] %w", index, err)
		}

		providerID, err := getRequiredString(provider, "id")
		if err != nil {
			return fmt.Errorf("providers[%d] %w", index, err)
		}
		if _, exists := providerIndexByID[providerID]; exists {
			return fmt.Errorf("providers[%d] duplicated id %q", index, providerID)
		}
		providerIndexByID[providerID] = index

		if _, err := getRequiredString(provider, "name"); err != nil {
			return fmt.Errorf("providers[%d] %w", index, err)
		}
		providerType, err := getRequiredString(provider, "type")
		if err != nil {
			return fmt.Errorf("providers[%d] %w", index, err)
		}
		if providerType != AuthProviderTypeLDAP {
			return fmt.Errorf("providers[%d].type must be ldap", index)
		}

		providerEnabled, err := getRequiredBool(provider, "enabled")
		if err != nil {
			return fmt.Errorf("providers[%d] %w", index, err)
		}
		if providerEnabled {
			enabledLDAPProviderCount += 1
		}

		priority, err := getRequiredInt(provider, "priority")
		if err != nil {
			return fmt.Errorf("providers[%d] %w", index, err)
		}
		if priority < 0 || priority > 10000 {
			return fmt.Errorf("providers[%d].priority must be between 0 and 10000", index)
		}

		matchRules, err := getRequiredObject(provider, "matchRules")
		if err != nil {
			return fmt.Errorf("providers[%d] %w", index, err)
		}
		if err := validateNoUnknownKeys(matchRules, map[string]struct{}{
			"emailDomains":  {},
			"usernameRegex": {},
		}); err != nil {
			return fmt.Errorf("providers[%d].matchRules %w", index, err)
		}
		emailDomains, err := getRequiredArray(matchRules, "emailDomains")
		if err != nil {
			return fmt.Errorf("providers[%d].matchRules %w", index, err)
		}
		for domainIndex, rawDomain := range emailDomains {
			domain, ok := rawDomain.(string)
			if !ok {
				return fmt.Errorf("providers[%d].matchRules.emailDomains[%d] must be string", index, domainIndex)
			}
			normalizedDomain := strings.TrimSpace(domain)
			if normalizedDomain == "" {
				return fmt.Errorf("providers[%d].matchRules.emailDomains[%d] must not be empty", index, domainIndex)
			}
			if strings.Contains(normalizedDomain, "@") || strings.Contains(normalizedDomain, " ") {
				return fmt.Errorf("providers[%d].matchRules.emailDomains[%d] is invalid", index, domainIndex)
			}
		}
		usernameRegex, err := getRequiredStringAllowEmpty(matchRules, "usernameRegex")
		if err != nil {
			return fmt.Errorf("providers[%d].matchRules %w", index, err)
		}
		if usernameRegex != "" {
			if _, compileErr := regexp.Compile(usernameRegex); compileErr != nil {
				return fmt.Errorf("providers[%d].matchRules.usernameRegex is invalid", index)
			}
		}

		ldapConfig, err := getRequiredObject(provider, "ldap")
		if err != nil {
			return fmt.Errorf("providers[%d] %w", index, err)
		}
		if err := validateNoUnknownKeys(ldapConfig, map[string]struct{}{
			"host":                   {},
			"port":                   {},
			"tlsMode":                {},
			"baseDN":                 {},
			"bindDN":                 {},
			"bindPasswordCiphertext": {},
			"userFilter":             {},
			"idAttribute":            {},
			"emailAttribute":         {},
			"nameAttribute":          {},
			"groupAttribute":         {},
			"connectTimeoutMs":       {},
			"readTimeoutMs":          {},
		}); err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		if _, err := getRequiredString(ldapConfig, "host"); err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		ldapPort, err := getRequiredInt(ldapConfig, "port")
		if err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		if ldapPort <= 0 || ldapPort > 65535 {
			return fmt.Errorf("providers[%d].ldap.port must be between 1 and 65535", index)
		}
		ldapTLSMode, err := getRequiredString(ldapConfig, "tlsMode")
		if err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		switch ldapTLSMode {
		case string(LDAPTLSModeLDAPS), string(LDAPTLSModeStartTLS):
		default:
			return fmt.Errorf("providers[%d].ldap.tlsMode must be ldaps/starttls", index)
		}
		if _, err := getRequiredString(ldapConfig, "baseDN"); err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		if _, err := getRequiredStringAllowEmpty(ldapConfig, "bindDN"); err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		if _, err := getRequiredStringAllowEmpty(ldapConfig, "bindPasswordCiphertext"); err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		userFilter, err := getRequiredString(ldapConfig, "userFilter")
		if err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		if !strings.Contains(userFilter, "%s") {
			return fmt.Errorf("providers[%d].ldap.userFilter must include %%s placeholder", index)
		}
		if _, err := getRequiredString(ldapConfig, "idAttribute"); err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		if _, err := getRequiredString(ldapConfig, "emailAttribute"); err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		if _, err := getRequiredString(ldapConfig, "nameAttribute"); err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		if _, err := getRequiredStringAllowEmpty(ldapConfig, "groupAttribute"); err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		connectTimeoutMS, err := getRequiredInt(ldapConfig, "connectTimeoutMs")
		if err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		if connectTimeoutMS < 100 || connectTimeoutMS > 30000 {
			return fmt.Errorf("providers[%d].ldap.connectTimeoutMs must be between 100 and 30000", index)
		}
		readTimeoutMS, err := getRequiredInt(ldapConfig, "readTimeoutMs")
		if err != nil {
			return fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		if readTimeoutMS < 100 || readTimeoutMS > 30000 {
			return fmt.Errorf("providers[%d].ldap.readTimeoutMs must be between 100 and 30000", index)
		}
	}

	if defaultProviderID != AuthProviderLocalID {
		if _, exists := providerIndexByID[defaultProviderID]; !exists {
			return fmt.Errorf("defaultProviderId must be local or one of providers.id")
		}
	}
	if loginMode == "ldap_only" {
		if defaultProviderID == AuthProviderLocalID {
			return fmt.Errorf("defaultProviderId must not be local in ldap_only mode")
		}
		if enabledLDAPProviderCount == 0 {
			return fmt.Errorf("ldap_only mode requires at least one enabled ldap provider")
		}
	}

	breakGlass, err := getRequiredObject(payload, "breakGlass")
	if err != nil {
		return err
	}
	if err := validateNoUnknownKeys(breakGlass, map[string]struct{}{
		"enabled":          {},
		"localAdminEmails": {},
	}); err != nil {
		return fmt.Errorf("breakGlass %w", err)
	}
	breakGlassEnabled, err := getRequiredBool(breakGlass, "enabled")
	if err != nil {
		return fmt.Errorf("breakGlass %w", err)
	}
	localAdminEmails, err := getRequiredArray(breakGlass, "localAdminEmails")
	if err != nil {
		return fmt.Errorf("breakGlass %w", err)
	}
	for index, rawEmail := range localAdminEmails {
		email, ok := rawEmail.(string)
		if !ok {
			return fmt.Errorf("breakGlass.localAdminEmails[%d] must be string", index)
		}
		normalizedEmail := strings.TrimSpace(email)
		if normalizedEmail == "" || !strings.Contains(normalizedEmail, "@") {
			return fmt.Errorf("breakGlass.localAdminEmails[%d] is invalid", index)
		}
	}
	if breakGlassEnabled && loginMode == "ldap_only" && len(localAdminEmails) == 0 {
		return fmt.Errorf("breakGlass.localAdminEmails must not be empty when breakGlass is enabled")
	}

	return nil
}

func validateSitemapConfig(payload map[string]any) error {
	requiredKeys := map[string]struct{}{
		"generationMode":       {},
		"maxUpdatedWithinDays": {},
	}
	if err := validateNoUnknownKeys(payload, requiredKeys); err != nil {
		return err
	}

	generationMode, err := getRequiredString(payload, "generationMode")
	if err != nil {
		return err
	}
	switch normalizeSitemapGenerationMode(generationMode) {
	case SitemapGenerationModeAllPublic, SitemapGenerationModeUpdatedWithinDays:
	default:
		return fmt.Errorf("generationMode must be all_public/updated_within_days")
	}

	maxUpdatedWithinDays, err := getRequiredInt(payload, "maxUpdatedWithinDays")
	if err != nil {
		return err
	}
	if maxUpdatedWithinDays < sitemapMinMaxUpdatedWithinDays || maxUpdatedWithinDays > sitemapMaxMaxUpdatedWithinDays {
		return fmt.Errorf(
			"maxUpdatedWithinDays must be between %d and %d",
			sitemapMinMaxUpdatedWithinDays,
			sitemapMaxMaxUpdatedWithinDays,
		)
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

func validateHomepageAnonymousCacheConfig(payload map[string]any) error {
	requiredKeys := map[string]struct{}{
		"maxAgeSeconds":               {},
		"sMaxAgeSeconds":              {},
		"staleWhileRevalidateSeconds": {},
	}
	if err := validateNoUnknownKeys(payload, requiredKeys); err != nil {
		return err
	}

	maxAgeSeconds, err := getRequiredInt(payload, "maxAgeSeconds")
	if err != nil {
		return err
	}
	if maxAgeSeconds < 0 || maxAgeSeconds > 86400 {
		return fmt.Errorf("maxAgeSeconds must be between 0 and 86400")
	}

	sMaxAgeSeconds, err := getRequiredInt(payload, "sMaxAgeSeconds")
	if err != nil {
		return err
	}
	if sMaxAgeSeconds < 0 || sMaxAgeSeconds > 86400 {
		return fmt.Errorf("sMaxAgeSeconds must be between 0 and 86400")
	}

	staleWhileRevalidateSeconds, err := getRequiredInt(payload, "staleWhileRevalidateSeconds")
	if err != nil {
		return err
	}
	if staleWhileRevalidateSeconds < 0 || staleWhileRevalidateSeconds > 86400 {
		return fmt.Errorf("staleWhileRevalidateSeconds must be between 0 and 86400")
	}

	return nil
}

func buildLDAPProviderConfigFromAuthConfig(
	authConfig map[string]any,
	providerID string,
) (LDAPAuthProviderConfig, error) {
	providers, err := getRequiredArray(authConfig, "providers")
	if err != nil {
		return LDAPAuthProviderConfig{}, err
	}

	normalizedProviderID := strings.TrimSpace(providerID)
	for index, rawProvider := range providers {
		provider, ok := rawProvider.(map[string]any)
		if !ok || provider == nil {
			continue
		}
		currentProviderID, _ := getRequiredString(provider, "id")
		if currentProviderID != normalizedProviderID {
			continue
		}
		providerType, _ := getRequiredString(provider, "type")
		if providerType != AuthProviderTypeLDAP {
			return LDAPAuthProviderConfig{}, fmt.Errorf("providers[%d].type must be ldap", index)
		}
		ldapConfig, err := getRequiredObject(provider, "ldap")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}

		host, err := getRequiredString(ldapConfig, "host")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}
		port, err := getRequiredInt(ldapConfig, "port")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}
		tlsMode, err := getRequiredString(ldapConfig, "tlsMode")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}
		baseDN, err := getRequiredString(ldapConfig, "baseDN")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}
		bindDN, err := getRequiredStringAllowEmpty(ldapConfig, "bindDN")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}
		bindPassword, err := getRequiredStringAllowEmpty(ldapConfig, "bindPasswordCiphertext")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}
		userFilter, err := getRequiredString(ldapConfig, "userFilter")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}
		idAttribute, err := getRequiredString(ldapConfig, "idAttribute")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}
		emailAttribute, err := getRequiredString(ldapConfig, "emailAttribute")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}
		nameAttribute, err := getRequiredString(ldapConfig, "nameAttribute")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}
		connectTimeoutMS, err := getRequiredInt(ldapConfig, "connectTimeoutMs")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}
		readTimeoutMS, err := getRequiredInt(ldapConfig, "readTimeoutMs")
		if err != nil {
			return LDAPAuthProviderConfig{}, err
		}

		return NormalizeLDAPAuthProviderConfig(LDAPAuthProviderConfig{
			ProviderID:     currentProviderID,
			Host:           host,
			Port:           port,
			TLSMode:        LDAPTLSMode(strings.ToLower(tlsMode)),
			BaseDN:         baseDN,
			BindDN:         bindDN,
			BindPassword:   bindPassword,
			UserFilter:     userFilter,
			IDAttribute:    idAttribute,
			EmailAttribute: emailAttribute,
			NameAttribute:  nameAttribute,
			ConnectTimeout: time.Duration(connectTimeoutMS) * time.Millisecond,
			ReadTimeout:    time.Duration(readTimeoutMS) * time.Millisecond,
		})
	}

	return LDAPAuthProviderConfig{}, fmt.Errorf("provider %q not found", normalizedProviderID)
}

func normalizeAuthConfigSecretsForPersist(
	value map[string]any,
	existing *models.SystemConfig,
) (map[string]any, error) {
	normalizedValue, err := cloneMapAny(value)
	if err != nil {
		return nil, err
	}

	normalizedExisting := map[string]any{}
	if existing != nil && strings.TrimSpace(existing.ConfigValueJSON) != "" {
		if err := json.Unmarshal([]byte(existing.ConfigValueJSON), &normalizedExisting); err != nil {
			return nil, err
		}
	}

	providers, err := getRequiredArray(normalizedValue, "providers")
	if err != nil {
		return nil, err
	}
	existingPasswordByProviderID := buildAuthProviderPasswordMap(normalizedExisting)

	for index, rawProvider := range providers {
		provider, ok := rawProvider.(map[string]any)
		if !ok || provider == nil {
			return nil, fmt.Errorf("providers[%d] must be object", index)
		}
		providerID, err := getRequiredString(provider, "id")
		if err != nil {
			return nil, fmt.Errorf("providers[%d] %w", index, err)
		}
		ldapConfig, err := getRequiredObject(provider, "ldap")
		if err != nil {
			return nil, fmt.Errorf("providers[%d] %w", index, err)
		}
		bindPassword, err := getRequiredStringAllowEmpty(ldapConfig, "bindPasswordCiphertext")
		if err != nil {
			return nil, fmt.Errorf("providers[%d].ldap %w", index, err)
		}
		if bindPassword != authConfigSecretMask {
			continue
		}
		existingPassword, exists := existingPasswordByProviderID[providerID]
		if !exists {
			return nil, fmt.Errorf("providers[%d].ldap.bindPasswordCiphertext is masked but no stored secret exists", index)
		}
		ldapConfig["bindPasswordCiphertext"] = existingPassword
	}

	return normalizedValue, nil
}

func maskAuthConfigSecrets(value map[string]any) map[string]any {
	clonedValue, err := cloneMapAny(value)
	if err != nil {
		return map[string]any{}
	}

	providers, err := getRequiredArray(clonedValue, "providers")
	if err != nil {
		return clonedValue
	}
	for _, rawProvider := range providers {
		provider, ok := rawProvider.(map[string]any)
		if !ok || provider == nil {
			continue
		}
		ldapConfig, err := getRequiredObject(provider, "ldap")
		if err != nil {
			continue
		}
		bindPassword, err := getRequiredStringAllowEmpty(ldapConfig, "bindPasswordCiphertext")
		if err != nil || bindPassword == "" {
			continue
		}
		ldapConfig["bindPasswordCiphertext"] = authConfigSecretMask
	}

	return clonedValue
}

func buildAuthProviderPasswordMap(config map[string]any) map[string]string {
	passwordByProviderID := map[string]string{}
	providers, err := getRequiredArray(config, "providers")
	if err != nil {
		return passwordByProviderID
	}
	for _, rawProvider := range providers {
		provider, ok := rawProvider.(map[string]any)
		if !ok || provider == nil {
			continue
		}
		providerID, err := getRequiredString(provider, "id")
		if err != nil {
			continue
		}
		ldapConfig, err := getRequiredObject(provider, "ldap")
		if err != nil {
			continue
		}
		bindPassword, err := getRequiredStringAllowEmpty(ldapConfig, "bindPasswordCiphertext")
		if err != nil || bindPassword == "" {
			continue
		}
		passwordByProviderID[providerID] = bindPassword
	}
	return passwordByProviderID
}

func cloneMapAny(value map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var cloned map[string]any
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	if cloned == nil {
		return map[string]any{}, nil
	}
	return cloned, nil
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

func getRequiredArray(payload map[string]any, key string) ([]any, error) {
	rawValue, ok := payload[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	value, ok := rawValue.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be array", key)
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
