package service

import (
	"context"
	"errors"
)

type localAuthLoginProvider struct {
	authService *AuthService
}

// NewLocalAuthLoginProvider 创建本地账号密码认证 provider。
func NewLocalAuthLoginProvider(authService *AuthService) AuthLoginProvider {
	return &localAuthLoginProvider{
		authService: authService,
	}
}

func (p *localAuthLoginProvider) ProviderID() string {
	return AuthProviderLocalID
}

func (p *localAuthLoginProvider) Login(
	ctx context.Context,
	identifier string,
	password string,
) (AuthSession, error) {
	if p == nil || p.authService == nil {
		return AuthSession{}, errors.New("local auth login provider dependencies are nil")
	}
	return p.authService.Login(ctx, identifier, password)
}
