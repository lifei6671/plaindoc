package service

import (
	"context"
	"errors"
)

const (
	// AuthProviderLocalID 本地账号密码认证 provider 标识。
	AuthProviderLocalID = "local"
	// AuthProviderLDAPID 默认 LDAP 认证 provider 标识。
	AuthProviderLDAPID = "ldap"
	// AuthProviderTypeLocal 本地账号 provider 类型。
	AuthProviderTypeLocal = "local"
	// AuthProviderTypeLDAP LDAP provider 类型。
	AuthProviderTypeLDAP = "ldap"
	// LDAPUserPasswordPlaceholder 仅 LDAP 账号在 users.password_hash 字段的占位值。
	LDAPUserPasswordPlaceholder = "!ldap!"
)

var (
	// ErrAuthProviderUnavailable 表示请求的认证 provider 不存在或不可用。
	ErrAuthProviderUnavailable = errors.New("auth provider unavailable")
	// ErrAuthProviderFailure 表示认证 provider 内部失败（如外部依赖异常）。
	ErrAuthProviderFailure = errors.New("auth provider failure")
)

// AuthProviderLoginInput 描述统一登录编排入参，供多 provider 场景复用。
type AuthProviderLoginInput struct {
	Provider   string
	Identifier string
	Password   string
}

// AuthLoginProvider 约束单个认证 provider 的最小能力。
type AuthLoginProvider interface {
	ProviderID() string
	Login(ctx context.Context, identifier string, password string) (AuthSession, error)
}

// AuthLoginProviderHealthChecker 约束 provider 健康探测能力（供后台“测试连接”等场景复用）。
type AuthLoginProviderHealthChecker interface {
	CheckHealth(ctx context.Context) error
}
