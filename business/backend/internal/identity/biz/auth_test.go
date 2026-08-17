package biz

import (
	"context"
	"testing"
	"time"

	"github.com/lvtuopen-ai/kernel-go/principal"
)

type authRepositoryStub struct {
	guestCalls int
}

func (*authRepositoryStub) Login(context.Context, LoginCommand) (AuthenticatedSession, error) {
	return AuthenticatedSession{}, nil
}
func (r *authRepositoryStub) Guest(context.Context, GuestCommand) (AuthenticatedSession, error) {
	r.guestCalls++
	return AuthenticatedSession{}, nil
}
func (*authRepositoryStub) RotateRefresh(context.Context, principal.Realm, string, time.Time) (AuthenticatedSession, string, error) {
	return AuthenticatedSession{}, "", nil
}
func (*authRepositoryStub) Logout(context.Context, principal.Realm, string) error { return nil }
func (*authRepositoryStub) SwitchContext(context.Context, SwitchContextCommand) (AuthenticatedSession, error) {
	return AuthenticatedSession{}, nil
}

func TestGuestRequiresStableIdentifiersAndShop(t *testing.T) {
	repository := &authRepositoryStub{}
	authentication := NewAuthentication(repository, &Directory{})
	valid := GuestCommand{Subject: "guest-1", ShopCode: "shop-one", SessionID: "session-1", FamilyID: "family-1", ExpiresAt: time.Now().Add(time.Hour)}
	if _, err := authentication.Guest(context.Background(), valid); err != nil {
		t.Fatal(err)
	}
	if repository.guestCalls != 1 {
		t.Fatal("valid guest command did not reach the repository")
	}
	valid.ShopCode = ""
	if _, err := authentication.Guest(context.Background(), valid); err != ErrInvalidCredentials {
		t.Fatalf("missing shop code returned %v", err)
	}
	if repository.guestCalls != 1 {
		t.Fatal("invalid guest command reached the repository")
	}
}
