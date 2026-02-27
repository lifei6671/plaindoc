package service

import (
	"context"
	"errors"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
)

// AuthLoginOrchestrator 统一编排“选择 provider -> 执行登录”流程。
type AuthLoginOrchestrator struct {
	defaultProviderID string
	providers         map[string]AuthLoginProvider
}

// NewAuthLoginOrchestrator 创建登录编排器；defaultProviderID 为空时默认回退 local。
func NewAuthLoginOrchestrator(
	defaultProviderID string,
	providers ...AuthLoginProvider,
) *AuthLoginOrchestrator {
	providerMap := make(map[string]AuthLoginProvider, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		providerID := strings.TrimSpace(provider.ProviderID())
		if providerID == "" {
			continue
		}
		providerMap[providerID] = provider
	}

	normalizedDefaultProviderID := strings.TrimSpace(defaultProviderID)
	if normalizedDefaultProviderID == "" {
		normalizedDefaultProviderID = AuthProviderLocalID
	}
	if _, ok := providerMap[normalizedDefaultProviderID]; !ok {
		if _, hasLocalProvider := providerMap[AuthProviderLocalID]; hasLocalProvider {
			normalizedDefaultProviderID = AuthProviderLocalID
		}
	}

	return &AuthLoginOrchestrator{
		defaultProviderID: normalizedDefaultProviderID,
		providers:         providerMap,
	}
}

// Login 执行统一登录流程：按 provider 选择规则路由后调用对应 provider。
func (o *AuthLoginOrchestrator) Login(
	ctx context.Context,
	input AuthProviderLoginInput,
) (AuthSession, error) {
	if o == nil {
		return AuthSession{}, errors.New("auth login orchestrator is nil")
	}

	providerID := strings.TrimSpace(input.Provider)
	if providerID == "" {
		providerID = o.defaultProviderID
	}
	if providerID == "" {
		providerID = AuthProviderLocalID
	}
	logit.SetRequestAttrs(ctx, logit.String("auth_provider", providerID))

	provider, ok := o.providers[providerID]
	if !ok || provider == nil {
		logit.SetRequestAttrs(
			ctx,
			logit.String("auth_login_result", "failed"),
			logit.String("auth_login_reason", "provider_unavailable"),
		)
		return AuthSession{}, ErrAuthProviderUnavailable
	}

	session, err := provider.Login(
		ctx,
		strings.TrimSpace(input.Identifier),
		input.Password,
	)
	if err != nil {
		logit.SetRequestAttrs(
			ctx,
			logit.String("auth_login_result", "failed"),
			logit.String("auth_login_reason", classifyAuthLoginFailureReason(err)),
		)
		return AuthSession{}, err
	}

	logit.SetRequestAttrs(
		ctx,
		logit.String("auth_login_result", "success"),
		logit.String("auth_login_reason", ""),
	)
	return session, nil
}

func classifyAuthLoginFailureReason(err error) string {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return "invalid_credentials"
	case errors.Is(err, ErrUserBanned):
		return "user_banned"
	case errors.Is(err, ErrUserDeleted):
		return "user_deleted"
	case errors.Is(err, ErrAuthProviderFailure):
		return "provider_failure"
	case errors.Is(err, ErrAuthProviderUnavailable):
		return "provider_unavailable"
	default:
		return "unknown"
	}
}
