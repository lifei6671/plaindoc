package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	authLoginModeLocalOnly = "local_only"
	authLoginModeLDAPOnly  = "ldap_only"
	authLoginModeMixed     = "mixed"
)

type siteConfigPolicy struct {
	AllowRegistration *bool `json:"allowRegistration"`
}

type authConfigPolicy struct {
	LoginMode         string                     `json:"loginMode"`
	DefaultProviderID string                     `json:"defaultProviderId"`
	AllowUserRegister *bool                      `json:"allowUserRegister"`
	Providers         []authConfigProviderPolicy `json:"providers"`
}

type authConfigProviderPolicy struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

type AuthLoginProviderOption struct {
	ID       string
	Name     string
	Type     string
	Priority int
}

type AuthLoginOptions struct {
	LoginMode         string
	DefaultProviderID string
	AllowUserRegister bool
	Providers         []AuthLoginProviderOption
}

// AuthRegistrationPolicyService 负责读取注册相关系统配置。
type AuthRegistrationPolicyService struct {
	systemConfigRepo repository.SystemConfigRepository
}

// NewAuthRegistrationPolicyService 创建注册策略服务。
func NewAuthRegistrationPolicyService(systemConfigRepo repository.SystemConfigRepository) *AuthRegistrationPolicyService {
	return &AuthRegistrationPolicyService{
		systemConfigRepo: systemConfigRepo,
	}
}

// AllowRegistration 判断是否允许新用户注册；缺省配置时默认允许。
func (s *AuthRegistrationPolicyService) AllowRegistration(ctx context.Context) (bool, error) {
	options, err := s.ResolveLoginOptions(ctx)
	if err != nil {
		return false, err
	}
	return options.AllowUserRegister, nil
}

// ResolveLoginOptions 解析登录模式、provider 列表与注册开关，供登录页与注册策略复用。
func (s *AuthRegistrationPolicyService) ResolveLoginOptions(ctx context.Context) (AuthLoginOptions, error) {
	options := defaultAuthLoginOptions()
	if s == nil || s.systemConfigRepo == nil {
		return options, nil
	}

	sitePolicy, err := s.loadSiteConfigPolicy(ctx)
	if err != nil {
		return AuthLoginOptions{}, err
	}
	authPolicy, hasAuthPolicy, err := s.loadAuthConfigPolicy(ctx)
	if err != nil {
		return AuthLoginOptions{}, err
	}
	if hasAuthPolicy {
		options = mergeAuthLoginOptions(options, authPolicy)
	}
	if sitePolicy.AllowRegistration != nil {
		options.AllowUserRegister = options.AllowUserRegister && *sitePolicy.AllowRegistration
	}
	if options.LoginMode == authLoginModeLDAPOnly {
		options.AllowUserRegister = false
	}
	return options, nil
}

func (s *AuthRegistrationPolicyService) loadSiteConfigPolicy(ctx context.Context) (siteConfigPolicy, error) {
	config, err := s.systemConfigRepo.GetByConfigKey(ctx, "site")
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return siteConfigPolicy{}, nil
		}
		return siteConfigPolicy{}, err
	}
	if config == nil {
		return siteConfigPolicy{}, nil
	}

	var payload siteConfigPolicy
	if err := json.Unmarshal([]byte(config.ConfigValueJSON), &payload); err != nil {
		return siteConfigPolicy{}, err
	}
	return payload, nil
}

func (s *AuthRegistrationPolicyService) loadAuthConfigPolicy(
	ctx context.Context,
) (authConfigPolicy, bool, error) {
	config, err := s.systemConfigRepo.GetByConfigKey(ctx, SystemConfigKeyAuth)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return authConfigPolicy{}, false, nil
		}
		return authConfigPolicy{}, false, err
	}
	if config == nil || strings.TrimSpace(config.ConfigValueJSON) == "" {
		return authConfigPolicy{}, false, nil
	}

	var payload authConfigPolicy
	if err := json.Unmarshal([]byte(config.ConfigValueJSON), &payload); err != nil {
		return authConfigPolicy{}, false, err
	}
	return payload, true, nil
}

func defaultAuthLoginOptions() AuthLoginOptions {
	return AuthLoginOptions{
		LoginMode:         authLoginModeLocalOnly,
		DefaultProviderID: AuthProviderLocalID,
		AllowUserRegister: true,
		Providers:         []AuthLoginProviderOption{},
	}
}

func mergeAuthLoginOptions(base AuthLoginOptions, policy authConfigPolicy) AuthLoginOptions {
	options := base
	options.LoginMode = normalizeAuthLoginMode(policy.LoginMode, base.LoginMode)

	normalizedDefaultProviderID := strings.TrimSpace(policy.DefaultProviderID)
	if normalizedDefaultProviderID != "" {
		options.DefaultProviderID = normalizedDefaultProviderID
	}
	if policy.AllowUserRegister != nil {
		options.AllowUserRegister = *policy.AllowUserRegister
	}

	providers := make([]AuthLoginProviderOption, 0, len(policy.Providers))
	seenProviderIDs := make(map[string]struct{}, len(policy.Providers))
	for _, provider := range policy.Providers {
		if !provider.Enabled {
			continue
		}
		providerID := strings.TrimSpace(provider.ID)
		if providerID == "" {
			continue
		}
		if _, duplicated := seenProviderIDs[providerID]; duplicated {
			continue
		}
		seenProviderIDs[providerID] = struct{}{}

		providerType := strings.ToLower(strings.TrimSpace(provider.Type))
		if providerType == "" {
			providerType = AuthProviderTypeLDAP
		}
		if providerType != AuthProviderTypeLDAP {
			continue
		}
		providerName := strings.TrimSpace(provider.Name)
		if providerName == "" {
			providerName = providerID
		}

		providers = append(providers, AuthLoginProviderOption{
			ID:       providerID,
			Name:     providerName,
			Type:     providerType,
			Priority: provider.Priority,
		})
	}

	sort.SliceStable(providers, func(i int, j int) bool {
		if providers[i].Priority == providers[j].Priority {
			return providers[i].ID < providers[j].ID
		}
		return providers[i].Priority > providers[j].Priority
	})

	options.Providers = providers
	if options.LoginMode == authLoginModeLocalOnly {
		options.DefaultProviderID = AuthProviderLocalID
		options.Providers = []AuthLoginProviderOption{}
		return options
	}

	if options.DefaultProviderID == AuthProviderLocalID {
		if options.LoginMode == authLoginModeLDAPOnly && len(options.Providers) > 0 {
			options.DefaultProviderID = options.Providers[0].ID
		}
		return options
	}

	for _, provider := range options.Providers {
		if provider.ID == options.DefaultProviderID {
			return options
		}
	}
	if len(options.Providers) > 0 {
		options.DefaultProviderID = options.Providers[0].ID
	} else {
		options.DefaultProviderID = AuthProviderLocalID
	}
	return options
}

func normalizeAuthLoginMode(raw string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case authLoginModeLocalOnly, authLoginModeLDAPOnly, authLoginModeMixed:
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return fallback
	}
}
