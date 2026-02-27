package service

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

const (
	defaultLDAPConnectTimeout = 3 * time.Second
	defaultLDAPReadTimeout    = 3 * time.Second
	defaultLDAPUserFilter     = "(mail=%s)"
	defaultLDAPIDAttribute    = "entryUUID"
	defaultLDAPEmailAttribute = "mail"
	defaultLDAPNameAttribute  = "cn"
)

// LDAPTLSMode 描述 LDAP 连接加密模式。
type LDAPTLSMode string

const (
	LDAPTLSModeLDAPS    LDAPTLSMode = "ldaps"
	LDAPTLSModeStartTLS LDAPTLSMode = "starttls"
)

// LDAPAuthProviderConfig LDAP 登录 provider 配置。
type LDAPAuthProviderConfig struct {
	ProviderID         string
	Host               string
	Port               int
	TLSMode            LDAPTLSMode
	InsecureSkipVerify bool
	BaseDN             string
	BindDN             string
	BindPassword       string
	UserFilter         string
	IDAttribute        string
	EmailAttribute     string
	NameAttribute      string
	ConnectTimeout     time.Duration
	ReadTimeout        time.Duration
	HealthCheckBaseDN  string
}

type ldapDialer interface {
	DialURL(addr string, opts ...ldap.DialOpt) (ldapConn, error)
}

type ldapConn interface {
	SetTimeout(timeout time.Duration)
	StartTLS(config *tls.Config) error
	Bind(username string, password string) error
	Search(searchRequest *ldap.SearchRequest) (*ldap.SearchResult, error)
	Close() error
}

type goLDAPDialer struct{}

func (goLDAPDialer) DialURL(addr string, opts ...ldap.DialOpt) (ldapConn, error) {
	return ldap.DialURL(addr, opts...)
}

type ldapLoginPrincipal struct {
	ExternalID string
	Email      string
	Name       string
	LoginName  string
}

type ldapAuthLoginProvider struct {
	config           LDAPAuthProviderConfig
	authService      *AuthService
	userRepo         repository.UserRepository
	userIdentityRepo repository.UserIdentityRepository
	dialer           ldapDialer
	now              func() time.Time
}

// NormalizeLDAPAuthProviderConfig 将 LDAP provider 配置归一并校验。
func NormalizeLDAPAuthProviderConfig(raw LDAPAuthProviderConfig) (LDAPAuthProviderConfig, error) {
	cfg := LDAPAuthProviderConfig{
		ProviderID:         strings.TrimSpace(raw.ProviderID),
		Host:               strings.TrimSpace(raw.Host),
		Port:               raw.Port,
		TLSMode:            LDAPTLSMode(strings.ToLower(strings.TrimSpace(string(raw.TLSMode)))),
		InsecureSkipVerify: raw.InsecureSkipVerify,
		BaseDN:             strings.TrimSpace(raw.BaseDN),
		BindDN:             strings.TrimSpace(raw.BindDN),
		BindPassword:       raw.BindPassword,
		UserFilter:         strings.TrimSpace(raw.UserFilter),
		IDAttribute:        strings.TrimSpace(raw.IDAttribute),
		EmailAttribute:     strings.TrimSpace(raw.EmailAttribute),
		NameAttribute:      strings.TrimSpace(raw.NameAttribute),
		ConnectTimeout:     raw.ConnectTimeout,
		ReadTimeout:        raw.ReadTimeout,
		HealthCheckBaseDN:  strings.TrimSpace(raw.HealthCheckBaseDN),
	}

	if cfg.ProviderID == "" {
		cfg.ProviderID = AuthProviderLDAPID
	}
	if cfg.TLSMode == "" {
		cfg.TLSMode = LDAPTLSModeLDAPS
	}
	if cfg.Port <= 0 {
		if cfg.TLSMode == LDAPTLSModeStartTLS {
			cfg.Port = 389
		} else {
			cfg.Port = 636
		}
	}
	if cfg.UserFilter == "" {
		cfg.UserFilter = defaultLDAPUserFilter
	}
	if cfg.IDAttribute == "" {
		cfg.IDAttribute = defaultLDAPIDAttribute
	}
	if cfg.EmailAttribute == "" {
		cfg.EmailAttribute = defaultLDAPEmailAttribute
	}
	if cfg.NameAttribute == "" {
		cfg.NameAttribute = defaultLDAPNameAttribute
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = defaultLDAPConnectTimeout
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = defaultLDAPReadTimeout
	}
	if cfg.HealthCheckBaseDN == "" {
		cfg.HealthCheckBaseDN = cfg.BaseDN
	}

	if cfg.Host == "" {
		return LDAPAuthProviderConfig{}, errors.New("ldap host must not be empty")
	}
	if cfg.BaseDN == "" {
		return LDAPAuthProviderConfig{}, errors.New("ldap base dn must not be empty")
	}
	switch cfg.TLSMode {
	case LDAPTLSModeLDAPS, LDAPTLSModeStartTLS:
	default:
		return LDAPAuthProviderConfig{}, fmt.Errorf("unsupported ldap tls mode %q", cfg.TLSMode)
	}
	if cfg.Port <= 0 {
		return LDAPAuthProviderConfig{}, errors.New("ldap port must be greater than 0")
	}
	if !strings.Contains(cfg.UserFilter, "%s") {
		return LDAPAuthProviderConfig{}, errors.New("ldap user filter must include %s placeholder")
	}
	if cfg.IDAttribute == "" || cfg.EmailAttribute == "" || cfg.NameAttribute == "" {
		return LDAPAuthProviderConfig{}, errors.New("ldap attributes must not be empty")
	}
	if cfg.ConnectTimeout <= 0 || cfg.ReadTimeout <= 0 {
		return LDAPAuthProviderConfig{}, errors.New("ldap timeouts must be greater than 0")
	}
	if cfg.HealthCheckBaseDN == "" {
		return LDAPAuthProviderConfig{}, errors.New("ldap health check base dn must not be empty")
	}

	return cfg, nil
}

// NewLDAPAuthLoginProvider 创建 LDAP 认证 provider。
func NewLDAPAuthLoginProvider(
	config LDAPAuthProviderConfig,
	authService *AuthService,
	userRepo repository.UserRepository,
	userIdentityRepo repository.UserIdentityRepository,
) (*ldapAuthLoginProvider, error) {
	normalizedConfig, err := NormalizeLDAPAuthProviderConfig(config)
	if err != nil {
		return nil, err
	}

	provider := &ldapAuthLoginProvider{
		config:           normalizedConfig,
		authService:      authService,
		userRepo:         userRepo,
		userIdentityRepo: userIdentityRepo,
		dialer:           goLDAPDialer{},
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	return provider, nil
}

func (p *ldapAuthLoginProvider) ProviderID() string {
	return p.config.ProviderID
}

func (p *ldapAuthLoginProvider) Login(
	ctx context.Context,
	identifier string,
	password string,
) (AuthSession, error) {
	if p == nil || p.authService == nil || p.userRepo == nil || p.userIdentityRepo == nil || p.dialer == nil {
		return AuthSession{}, errors.New("ldap auth login provider dependencies are nil")
	}

	normalizedIdentifier := strings.TrimSpace(identifier)
	if normalizedIdentifier == "" || password == "" {
		return AuthSession{}, ErrInvalidCredentials
	}

	principal, err := p.authenticate(ctx, normalizedIdentifier, password)
	if err != nil {
		return AuthSession{}, err
	}
	user, err := p.resolveOrCreateLocalUser(ctx, principal)
	if err != nil {
		return AuthSession{}, err
	}

	now := p.now().UTC()
	if _, err := p.userIdentityRepo.Upsert(ctx, repository.UpsertUserIdentityParams{
		UserID:       user.UserID,
		ProviderType: AuthProviderTypeLDAP,
		ProviderID:   p.ProviderID(),
		ExternalID:   principal.ExternalID,
		LoginName:    principal.LoginName,
		LastLoginAt:  &now,
		Now:          now,
	}); err != nil {
		return AuthSession{}, err
	}

	session, err := p.authService.LoginByUserID(ctx, user.UserID)
	if err != nil {
		return AuthSession{}, err
	}
	return session, nil
}

// CheckHealth 探测 LDAP provider 健康状态：连通性、TLS 升级与 service bind 是否可用。
func (p *ldapAuthLoginProvider) CheckHealth(ctx context.Context) error {
	if p == nil || p.dialer == nil {
		return ErrAuthProviderFailure
	}

	conn, err := p.openConnection(ctx)
	if err != nil {
		return ErrAuthProviderFailure
	}
	defer func() { _ = conn.Close() }()

	if err := p.bindServiceAccount(conn); err != nil {
		return ErrAuthProviderFailure
	}

	result, err := conn.Search(ldap.NewSearchRequest(
		p.config.HealthCheckBaseDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		1,
		ldapSearchTimeLimitSeconds(p.config.ReadTimeout),
		false,
		"(objectClass=*)",
		[]string{"dn"},
		nil,
	))
	if err != nil || result == nil {
		return ErrAuthProviderFailure
	}
	return nil
}

func (p *ldapAuthLoginProvider) authenticate(
	ctx context.Context,
	identifier string,
	password string,
) (ldapLoginPrincipal, error) {
	conn, err := p.openConnection(ctx)
	if err != nil {
		return ldapLoginPrincipal{}, ErrAuthProviderFailure
	}
	defer func() { _ = conn.Close() }()

	if err := p.bindServiceAccount(conn); err != nil {
		return ldapLoginPrincipal{}, err
	}

	entry, err := p.searchUserEntry(conn, identifier)
	if err != nil {
		return ldapLoginPrincipal{}, err
	}

	userDN := strings.TrimSpace(entry.DN)
	if userDN == "" {
		return ldapLoginPrincipal{}, ErrInvalidCredentials
	}

	if err := conn.Bind(userDN, password); err != nil {
		if isLDAPInvalidCredentialsError(err) {
			return ldapLoginPrincipal{}, ErrInvalidCredentials
		}
		return ldapLoginPrincipal{}, ErrAuthProviderFailure
	}

	return p.entryToPrincipal(entry, identifier), nil
}

func (p *ldapAuthLoginProvider) openConnection(ctx context.Context) (ldapConn, error) {
	connectTimeout := p.config.ConnectTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, context.DeadlineExceeded
		}
		if remaining < connectTimeout {
			connectTimeout = remaining
		}
	}
	if connectTimeout <= 0 {
		connectTimeout = defaultLDAPConnectTimeout
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         p.config.Host,
		InsecureSkipVerify: p.config.InsecureSkipVerify,
	}
	dialOptions := []ldap.DialOpt{
		ldap.DialWithDialer(&net.Dialer{Timeout: connectTimeout}),
	}

	url := fmt.Sprintf("ldaps://%s:%d", p.config.Host, p.config.Port)
	if p.config.TLSMode == LDAPTLSModeStartTLS {
		url = fmt.Sprintf("ldap://%s:%d", p.config.Host, p.config.Port)
	} else {
		dialOptions = append(dialOptions, ldap.DialWithTLSConfig(tlsConfig))
	}

	conn, err := p.dialer.DialURL(url, dialOptions...)
	if err != nil {
		return nil, err
	}
	conn.SetTimeout(p.config.ReadTimeout)

	if p.config.TLSMode == LDAPTLSModeStartTLS {
		if err := conn.StartTLS(tlsConfig); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}

	return conn, nil
}

func (p *ldapAuthLoginProvider) bindServiceAccount(conn ldapConn) error {
	if strings.TrimSpace(p.config.BindDN) == "" {
		return nil
	}
	if err := conn.Bind(p.config.BindDN, p.config.BindPassword); err != nil {
		if isLDAPInvalidCredentialsError(err) {
			return ErrInvalidCredentials
		}
		return ErrAuthProviderFailure
	}
	return nil
}

func (p *ldapAuthLoginProvider) searchUserEntry(conn ldapConn, identifier string) (*ldap.Entry, error) {
	filter := fmt.Sprintf(p.config.UserFilter, ldap.EscapeFilter(identifier))
	result, err := conn.Search(ldap.NewSearchRequest(
		p.config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		2,
		ldapSearchTimeLimitSeconds(p.config.ReadTimeout),
		false,
		filter,
		[]string{
			p.config.IDAttribute,
			p.config.EmailAttribute,
			p.config.NameAttribute,
		},
		nil,
	))
	if err != nil {
		return nil, ErrAuthProviderFailure
	}
	if result == nil || len(result.Entries) != 1 {
		return nil, ErrInvalidCredentials
	}
	return result.Entries[0], nil
}

func (p *ldapAuthLoginProvider) entryToPrincipal(entry *ldap.Entry, identifier string) ldapLoginPrincipal {
	externalID := strings.TrimSpace(entry.GetAttributeValue(p.config.IDAttribute))
	if externalID == "" {
		externalID = strings.TrimSpace(entry.DN)
	}
	if externalID == "" {
		externalID = strings.TrimSpace(identifier)
	}

	email := normalizePossibleEmail(entry.GetAttributeValue(p.config.EmailAttribute))
	name := strings.TrimSpace(entry.GetAttributeValue(p.config.NameAttribute))
	if name == "" {
		name = guessDisplayNameFromIdentifier(identifier)
	}

	return ldapLoginPrincipal{
		ExternalID: externalID,
		Email:      email,
		Name:       name,
		LoginName:  strings.TrimSpace(identifier),
	}
}

func (p *ldapAuthLoginProvider) resolveOrCreateLocalUser(
	ctx context.Context,
	principal ldapLoginPrincipal,
) (*models.User, error) {
	identity, err := p.userIdentityRepo.GetByProviderExternalID(ctx, p.ProviderID(), principal.ExternalID)
	if err == nil && identity != nil {
		user, lookupErr := p.userRepo.GetByUserID(ctx, identity.UserID)
		if lookupErr != nil {
			if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
				return nil, ErrInvalidCredentials
			}
			return nil, lookupErr
		}
		return user, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	email := normalizePossibleEmail(principal.Email)
	if email == "" {
		email = normalizePossibleEmail(principal.LoginName)
	}
	if email == "" {
		email = buildLDAPFallbackEmail(p.ProviderID(), principal.ExternalID)
	}

	existingUser, err := p.userRepo.GetByEmail(ctx, email)
	if err == nil && existingUser != nil {
		return existingUser, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	user := &models.User{
		UserID:       strings.ToLower(ulid.Make().String()),
		Email:        email,
		PasswordHash: LDAPUserPasswordPlaceholder,
		Name:         strings.TrimSpace(principal.Name),
		Status:       models.EntityStatusActive,
	}
	if user.Name == "" {
		user.Name = guessDisplayNameFromIdentifier(principal.LoginName)
	}

	if err := p.userRepo.Create(ctx, user); err != nil {
		if isUniqueConstraintError(err) {
			existingUser, lookupErr := p.userRepo.GetByEmail(ctx, email)
			if lookupErr == nil && existingUser != nil {
				return existingUser, nil
			}
			if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
				return nil, lookupErr
			}
		}
		return nil, err
	}
	return user, nil
}

func normalizePossibleEmail(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" || !strings.Contains(trimmed, "@") {
		return ""
	}
	return trimmed
}

func guessDisplayNameFromIdentifier(identifier string) string {
	trimmed := strings.TrimSpace(identifier)
	if trimmed == "" {
		return "LDAP 用户"
	}
	if index := strings.Index(trimmed, "@"); index > 0 {
		trimmed = trimmed[:index]
	}
	if trimmed == "" {
		return "LDAP 用户"
	}
	return trimmed
}

func buildLDAPFallbackEmail(providerID string, externalID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(providerID) + ":" + strings.TrimSpace(externalID)))
	return fmt.Sprintf("ldap-%s@local.invalid", hex.EncodeToString(sum[:8]))
}

func ldapSearchTimeLimitSeconds(readTimeout time.Duration) int {
	if readTimeout <= 0 {
		return 1
	}
	seconds := int(readTimeout / time.Second)
	if seconds <= 0 {
		return 1
	}
	return seconds
}

func isLDAPInvalidCredentialsError(err error) bool {
	if err == nil {
		return false
	}
	if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "invalid credentials") ||
		strings.Contains(message, "invalid dn syntax")
}
