package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/lifei6671/plaindoc/apps/server/internal/config"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

type mockLDAPDialer struct {
	conn     ldapConn
	err      error
	dialURL  string
	dialOpts int
}

func (d *mockLDAPDialer) DialURL(addr string, opts ...ldap.DialOpt) (ldapConn, error) {
	d.dialURL = addr
	d.dialOpts = len(opts)
	if d.err != nil {
		return nil, d.err
	}
	return d.conn, nil
}

type ldapBindCall struct {
	username string
	password string
}

type mockLDAPConn struct {
	bindCalls       []ldapBindCall
	bindErrors      map[string]error
	searchRequest   *ldap.SearchRequest
	searchResult    *ldap.SearchResult
	searchErr       error
	startTLSErr     error
	startTLSCalls   int
	timeout         time.Duration
	closedCallCount int
}

func (c *mockLDAPConn) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

func (c *mockLDAPConn) StartTLS(_ *tls.Config) error {
	c.startTLSCalls += 1
	return c.startTLSErr
}

func (c *mockLDAPConn) Bind(username string, password string) error {
	c.bindCalls = append(c.bindCalls, ldapBindCall{username: username, password: password})
	if c.bindErrors == nil {
		return nil
	}
	if err, ok := c.bindErrors[username]; ok {
		return err
	}
	return nil
}

func (c *mockLDAPConn) Search(searchRequest *ldap.SearchRequest) (*ldap.SearchResult, error) {
	c.searchRequest = searchRequest
	if c.searchErr != nil {
		return nil, c.searchErr
	}
	return c.searchResult, nil
}

func (c *mockLDAPConn) Close() error {
	c.closedCallCount += 1
	return nil
}

func TestLDAPAuthLoginProvider_LoginSuccess(t *testing.T) {
	provider, cleanup := setupLDAPAuthProviderTest(t)
	defer cleanup()

	mockConn := &mockLDAPConn{
		searchResult: &ldap.SearchResult{
			Entries: []*ldap.Entry{
				{
					DN: "uid=alice,ou=people,dc=example,dc=com",
					Attributes: []*ldap.EntryAttribute{
						{Name: "entryUUID", Values: []string{"entry-uuid-1"}},
						{Name: "mail", Values: []string{"alice@example.com"}},
						{Name: "cn", Values: []string{"Alice"}},
					},
				},
			},
		},
	}
	provider.dialer = &mockLDAPDialer{conn: mockConn}

	session, err := provider.Login(context.Background(), "alice@example.com", "ldap-password")
	if err != nil {
		t.Fatalf("expected ldap login success, got err=%v", err)
	}
	if session.User.ID == "" || session.Token == "" || session.RefreshToken == "" {
		t.Fatalf("expected session fields non-empty, got %+v", session)
	}
	if session.User.Email != "alice@example.com" {
		t.Fatalf("expected user email alice@example.com, got %q", session.User.Email)
	}
	if len(mockConn.bindCalls) != 2 {
		t.Fatalf("expected bind call count 2 (service + user), got %d", len(mockConn.bindCalls))
	}

	user, err := provider.userRepo.GetByEmail(context.Background(), "alice@example.com")
	if err != nil {
		t.Fatalf("expected local user created, got err=%v", err)
	}
	if user.PasswordHash != LDAPUserPasswordPlaceholder {
		t.Fatalf("expected ldap password placeholder, got %q", user.PasswordHash)
	}

	identity, err := provider.userIdentityRepo.GetByProviderExternalID(
		context.Background(),
		provider.ProviderID(),
		"entry-uuid-1",
	)
	if err != nil {
		t.Fatalf("expected ldap identity persisted, got err=%v", err)
	}
	if identity.UserID != user.UserID {
		t.Fatalf("expected identity user_id=%q, got %q", user.UserID, identity.UserID)
	}
}

func TestLDAPAuthLoginProvider_LoginInvalidCredentials(t *testing.T) {
	provider, cleanup := setupLDAPAuthProviderTest(t)
	defer cleanup()

	userDN := "uid=alice,ou=people,dc=example,dc=com"
	mockConn := &mockLDAPConn{
		bindErrors: map[string]error{
			userDN: ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New("invalid credentials")),
		},
		searchResult: &ldap.SearchResult{
			Entries: []*ldap.Entry{
				{
					DN: userDN,
					Attributes: []*ldap.EntryAttribute{
						{Name: "entryUUID", Values: []string{"entry-uuid-1"}},
						{Name: "mail", Values: []string{"alice@example.com"}},
						{Name: "cn", Values: []string{"Alice"}},
					},
				},
			},
		},
	}
	provider.dialer = &mockLDAPDialer{conn: mockConn}

	_, err := provider.Login(context.Background(), "alice@example.com", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	_, lookupErr := provider.userRepo.GetByEmail(context.Background(), "alice@example.com")
	if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		t.Fatalf("expected local user not created when ldap credentials invalid, got err=%v", lookupErr)
	}
}

func TestLDAPAuthLoginProvider_CheckHealth(t *testing.T) {
	provider, cleanup := setupLDAPAuthProviderTest(t)
	defer cleanup()

	mockConn := &mockLDAPConn{
		searchResult: &ldap.SearchResult{
			Entries: []*ldap.Entry{
				{
					DN: "dc=example,dc=com",
					Attributes: []*ldap.EntryAttribute{
						{Name: "dn", Values: []string{"dc=example,dc=com"}},
					},
				},
			},
		},
	}
	provider.dialer = &mockLDAPDialer{conn: mockConn}

	if err := provider.CheckHealth(context.Background()); err != nil {
		t.Fatalf("expected health check success, got err=%v", err)
	}
	if mockConn.searchRequest == nil || mockConn.searchRequest.Filter != "(objectClass=*)" {
		t.Fatalf("expected health check search filter objectClass, got %+v", mockConn.searchRequest)
	}

	provider.dialer = &mockLDAPDialer{err: errors.New("dial failed")}
	if err := provider.CheckHealth(context.Background()); !errors.Is(err, ErrAuthProviderFailure) {
		t.Fatalf("expected ErrAuthProviderFailure when dial failed, got %v", err)
	}
}

func TestLDAPAuthLoginProvider_CheckHealthPlain(t *testing.T) {
	provider, cleanup := setupLDAPAuthProviderTest(t)
	defer cleanup()
	provider.config.TLSMode = LDAPTLSModePlain
	provider.config.Port = 389

	mockConn := &mockLDAPConn{
		searchResult: &ldap.SearchResult{
			Entries: []*ldap.Entry{
				{
					DN: "dc=example,dc=com",
					Attributes: []*ldap.EntryAttribute{
						{Name: "dn", Values: []string{"dc=example,dc=com"}},
					},
				},
			},
		},
	}
	mockDialer := &mockLDAPDialer{conn: mockConn}
	provider.dialer = mockDialer

	if err := provider.CheckHealth(context.Background()); err != nil {
		t.Fatalf("expected plain health check success, got err=%v", err)
	}
	if mockDialer.dialURL != "ldap://ldap.example.com:389" {
		t.Fatalf("expected plain mode dial url ldap://ldap.example.com:389, got %s", mockDialer.dialURL)
	}
	if mockConn.startTLSCalls != 0 {
		t.Fatalf("expected plain mode not call StartTLS, got %d", mockConn.startTLSCalls)
	}
}

func TestNormalizeLDAPAuthProviderConfig_PlainMode(t *testing.T) {
	cfg, err := NormalizeLDAPAuthProviderConfig(LDAPAuthProviderConfig{
		Host:           "ldap.example.com",
		TLSMode:        LDAPTLSModePlain,
		BaseDN:         "dc=example,dc=com",
		UserFilter:     "(mail=%s)",
		IDAttribute:    "entryUUID",
		EmailAttribute: "mail",
		NameAttribute:  "cn",
		ConnectTimeout: 3 * time.Second,
		ReadTimeout:    3 * time.Second,
	})
	if err != nil {
		t.Fatalf("expected plain mode config valid, got err=%v", err)
	}
	if cfg.Port != 389 {
		t.Fatalf("expected plain mode default port 389, got %d", cfg.Port)
	}
}

func setupLDAPAuthProviderTest(t *testing.T) (*ldapAuthLoginProvider, func()) {
	t.Helper()

	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    fmt.Sprintf("file:test-ldap-auth-provider-%d?mode=memory&cache=shared", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	ctx := context.Background()
	if err := storage.MigrateUp(ctx, database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}

	userRepo := repository.NewGormUserRepository(database.ORM)
	userIdentityRepo := repository.NewGormUserIdentityRepository(database.ORM)
	userSessionRepo := repository.NewGormUserSessionRepository(database.ORM)
	authService := NewAuthService(userRepo, userSessionRepo, config.JWTConfig{
		Secret:          "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	provider, err := NewLDAPAuthLoginProvider(LDAPAuthProviderConfig{
		ProviderID:     "corp-ldap",
		Host:           "ldap.example.com",
		Port:           636,
		TLSMode:        LDAPTLSModeLDAPS,
		BaseDN:         "dc=example,dc=com",
		BindDN:         "cn=readonly,dc=example,dc=com",
		BindPassword:   "secret",
		UserFilter:     "(mail=%s)",
		IDAttribute:    "entryUUID",
		EmailAttribute: "mail",
		NameAttribute:  "cn",
		ConnectTimeout: 2 * time.Second,
		ReadTimeout:    2 * time.Second,
	}, authService, userRepo, userIdentityRepo)
	if err != nil {
		t.Fatalf("new ldap auth provider failed: %v", err)
	}

	cleanup := func() {
		_ = database.Close()
	}
	return provider, cleanup
}
