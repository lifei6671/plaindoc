package service

import (
	"context"
	"errors"
	"testing"
)

type stubAuthLoginProvider struct {
	id         string
	session    AuthSession
	err        error
	callCount  int
	identifier string
	password   string
}

func (p *stubAuthLoginProvider) ProviderID() string {
	return p.id
}

func (p *stubAuthLoginProvider) Login(
	_ context.Context,
	identifier string,
	password string,
) (AuthSession, error) {
	p.callCount += 1
	p.identifier = identifier
	p.password = password
	if p.err != nil {
		return AuthSession{}, p.err
	}
	return p.session, nil
}

func TestAuthLoginOrchestrator_LoginUsesDefaultProvider(t *testing.T) {
	localProvider := &stubAuthLoginProvider{
		id: AuthProviderLocalID,
		session: AuthSession{
			User: AuthUser{
				ID:    "user-1",
				Email: "demo@example.com",
				Name:  "Demo",
			},
			Token:        "access-token",
			RefreshToken: "refresh-token",
		},
	}

	orchestrator := NewAuthLoginOrchestrator(AuthProviderLocalID, localProvider)
	session, err := orchestrator.Login(context.Background(), AuthProviderLoginInput{
		Identifier: "demo@example.com",
		Password:   "123456",
	})
	if err != nil {
		t.Fatalf("expected login success, got err=%v", err)
	}
	if session.User.ID != "user-1" || session.Token != "access-token" {
		t.Fatalf("unexpected session payload: %+v", session)
	}
	if localProvider.callCount != 1 {
		t.Fatalf("expected local provider call count 1, got %d", localProvider.callCount)
	}
	if localProvider.identifier != "demo@example.com" || localProvider.password != "123456" {
		t.Fatalf(
			"unexpected provider login params identifier=%q password=%q",
			localProvider.identifier,
			localProvider.password,
		)
	}
}

func TestAuthLoginOrchestrator_LoginWithUnavailableProvider(t *testing.T) {
	orchestrator := NewAuthLoginOrchestrator(AuthProviderLocalID)
	_, err := orchestrator.Login(context.Background(), AuthProviderLoginInput{
		Provider:   "ldap",
		Identifier: "demo@example.com",
		Password:   "123456",
	})
	if !errors.Is(err, ErrAuthProviderUnavailable) {
		t.Fatalf("expected ErrAuthProviderUnavailable, got %v", err)
	}
}
