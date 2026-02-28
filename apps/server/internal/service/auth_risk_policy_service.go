package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	defaultAuthRiskWindowSeconds     = 15 * 60
	defaultAuthRiskLockSeconds       = 24 * 60 * 60
	defaultAuthRiskCaptchaTTLSeconds = 120
	defaultLoginCaptchaLevel1        = 3
	defaultLoginCaptchaLevel2        = 6
	defaultLoginCaptchaLevel3        = 9
	defaultLoginLockThreshold        = 12
	defaultRegisterCaptchaLevel1     = 2
	defaultRegisterCaptchaLevel2     = 5
	defaultRegisterCaptchaLevel3     = 8
	defaultRegisterLockThreshold     = 10
	minAuthRiskWindowSeconds         = 60
	maxAuthRiskWindowSeconds         = 24 * 60 * 60
	minAuthRiskLockSeconds           = 300
	maxAuthRiskLockSeconds           = 7 * 24 * 60 * 60
	minAuthRiskCaptchaTTLSeconds     = 30
	maxAuthRiskCaptchaTTLSeconds     = 15 * 60
	minAuthRiskThreshold             = 1
	maxAuthRiskThreshold             = 10_000
)

// AuthRiskThresholds 描述场景阈值（验证码分级 + 锁定阈值）。
type AuthRiskThresholds struct {
	CaptchaLevel1 int
	CaptchaLevel2 int
	CaptchaLevel3 int
	Lock          int
}

// AuthRiskPolicy 为认证风控策略解析结果。
type AuthRiskPolicy struct {
	Enabled            bool
	WindowSeconds      int
	LockSeconds        int
	CaptchaTTLSeconds  int
	LoginThresholds    AuthRiskThresholds
	RegisterThresholds AuthRiskThresholds
}

type authRiskPolicyPayload struct {
	RiskControl *authRiskControlPayload `json:"riskControl"`
}

type authRiskControlPayload struct {
	Enabled            *bool                     `json:"enabled"`
	WindowSeconds      *int                      `json:"windowSeconds"`
	LockSeconds        *int                      `json:"lockSeconds"`
	Captcha            *authRiskCaptchaPayload   `json:"captcha"`
	LoginThresholds    *authRiskThresholdPayload `json:"loginThresholds"`
	RegisterThresholds *authRiskThresholdPayload `json:"registerThresholds"`
}

type authRiskCaptchaPayload struct {
	TTLSeconds *int `json:"ttlSeconds"`
}

type authRiskThresholdPayload struct {
	L1   *int `json:"l1"`
	L2   *int `json:"l2"`
	L3   *int `json:"l3"`
	Lock *int `json:"lock"`
}

// AuthRiskPolicyService 从系统配置解析认证风控策略。
type AuthRiskPolicyService struct {
	systemConfigRepo repository.SystemConfigRepository
}

// NewAuthRiskPolicyService 创建认证风控策略服务。
func NewAuthRiskPolicyService(systemConfigRepo repository.SystemConfigRepository) *AuthRiskPolicyService {
	return &AuthRiskPolicyService{
		systemConfigRepo: systemConfigRepo,
	}
}

// Resolve 返回认证风控策略；当配置缺失或解析失败时回退默认值。
func (s *AuthRiskPolicyService) Resolve(ctx context.Context) AuthRiskPolicy {
	policy := defaultAuthRiskPolicy()
	if s == nil || s.systemConfigRepo == nil {
		return policy
	}

	config, err := s.systemConfigRepo.GetByConfigKey(ctx, SystemConfigKeyAuth)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return policy
		}
		return policy
	}
	if config == nil || strings.TrimSpace(config.ConfigValueJSON) == "" {
		return policy
	}

	var payload authRiskPolicyPayload
	if err := json.Unmarshal([]byte(config.ConfigValueJSON), &payload); err != nil {
		return policy
	}
	if payload.RiskControl == nil {
		return policy
	}

	patchAuthRiskPolicy(&policy, payload.RiskControl)
	normalizeAuthRiskPolicy(&policy)
	return policy
}

func defaultAuthRiskPolicy() AuthRiskPolicy {
	return AuthRiskPolicy{
		Enabled:           true,
		WindowSeconds:     defaultAuthRiskWindowSeconds,
		LockSeconds:       defaultAuthRiskLockSeconds,
		CaptchaTTLSeconds: defaultAuthRiskCaptchaTTLSeconds,
		LoginThresholds: AuthRiskThresholds{
			CaptchaLevel1: defaultLoginCaptchaLevel1,
			CaptchaLevel2: defaultLoginCaptchaLevel2,
			CaptchaLevel3: defaultLoginCaptchaLevel3,
			Lock:          defaultLoginLockThreshold,
		},
		RegisterThresholds: AuthRiskThresholds{
			CaptchaLevel1: defaultRegisterCaptchaLevel1,
			CaptchaLevel2: defaultRegisterCaptchaLevel2,
			CaptchaLevel3: defaultRegisterCaptchaLevel3,
			Lock:          defaultRegisterLockThreshold,
		},
	}
}

func patchAuthRiskPolicy(policy *AuthRiskPolicy, patch *authRiskControlPayload) {
	if policy == nil || patch == nil {
		return
	}
	if patch.Enabled != nil {
		policy.Enabled = *patch.Enabled
	}
	if patch.WindowSeconds != nil {
		policy.WindowSeconds = *patch.WindowSeconds
	}
	if patch.LockSeconds != nil {
		policy.LockSeconds = *patch.LockSeconds
	}
	if patch.Captcha != nil && patch.Captcha.TTLSeconds != nil {
		policy.CaptchaTTLSeconds = *patch.Captcha.TTLSeconds
	}
	patchAuthRiskThresholds(&policy.LoginThresholds, patch.LoginThresholds)
	patchAuthRiskThresholds(&policy.RegisterThresholds, patch.RegisterThresholds)
}

func patchAuthRiskThresholds(target *AuthRiskThresholds, patch *authRiskThresholdPayload) {
	if target == nil || patch == nil {
		return
	}
	if patch.L1 != nil {
		target.CaptchaLevel1 = *patch.L1
	}
	if patch.L2 != nil {
		target.CaptchaLevel2 = *patch.L2
	}
	if patch.L3 != nil {
		target.CaptchaLevel3 = *patch.L3
	}
	if patch.Lock != nil {
		target.Lock = *patch.Lock
	}
}

func normalizeAuthRiskPolicy(policy *AuthRiskPolicy) {
	if policy == nil {
		return
	}
	policy.WindowSeconds = clampInt(policy.WindowSeconds, minAuthRiskWindowSeconds, maxAuthRiskWindowSeconds)
	policy.LockSeconds = clampInt(policy.LockSeconds, minAuthRiskLockSeconds, maxAuthRiskLockSeconds)
	policy.CaptchaTTLSeconds = clampInt(
		policy.CaptchaTTLSeconds,
		minAuthRiskCaptchaTTLSeconds,
		maxAuthRiskCaptchaTTLSeconds,
	)
	normalizeAuthRiskThresholds(&policy.LoginThresholds)
	normalizeAuthRiskThresholds(&policy.RegisterThresholds)
}

func normalizeAuthRiskThresholds(value *AuthRiskThresholds) {
	if value == nil {
		return
	}
	value.CaptchaLevel1 = clampInt(value.CaptchaLevel1, minAuthRiskThreshold, maxAuthRiskThreshold)
	value.CaptchaLevel2 = clampInt(value.CaptchaLevel2, minAuthRiskThreshold, maxAuthRiskThreshold)
	value.CaptchaLevel3 = clampInt(value.CaptchaLevel3, minAuthRiskThreshold, maxAuthRiskThreshold)
	value.Lock = clampInt(value.Lock, minAuthRiskThreshold, maxAuthRiskThreshold)

	if value.CaptchaLevel2 < value.CaptchaLevel1 {
		value.CaptchaLevel2 = value.CaptchaLevel1
	}
	if value.CaptchaLevel3 < value.CaptchaLevel2 {
		value.CaptchaLevel3 = value.CaptchaLevel2
	}
	if value.Lock < value.CaptchaLevel3 {
		value.Lock = value.CaptchaLevel3
	}
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
