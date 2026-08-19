package biz

import (
	"context"
	"time"

	"github.com/lvtuopen-ai/kernel-go/principal"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

var ErrInvalidCredentials = modelError("identity: invalid credentials")

type modelError string

func (e modelError) Error() string { return string(e) }

type AuthRepository interface {
	Login(ctx context.Context, command LoginCommand) (AuthenticatedSession, error)
	Guest(ctx context.Context, command GuestCommand) (AuthenticatedSession, error)
	RotateRefresh(ctx context.Context, expectedRealm principal.Realm, refreshToken string, expiresAt time.Time) (AuthenticatedSession, string, error)
	Logout(ctx context.Context, expectedRealm principal.Realm, refreshToken string) error
	SwitchContext(ctx context.Context, command SwitchContextCommand) (AuthenticatedSession, error)
}

type GuestCommand struct {
	Subject     string
	ShopCode    string
	SessionID   string
	FamilyID    string
	RefreshHash [32]byte
	ExpiresAt   time.Time
	DeviceName  string
	IPAddress   string
	UserAgent   string
}

type LoginCommand struct {
	Realm             principal.Realm
	Username          string
	Password          string
	ShopCode          string
	ChallengeID       string
	GuestRefreshToken string
	SessionID         string
	FamilyID          string
	RefreshHash       [32]byte
	ExpiresAt         time.Time
	DeviceName        string
	IPAddress         string
	UserAgent         string
}

type AuthenticatedSession struct {
	SessionID      string
	Subject        model.Subject
	Member         model.WorkforceMember
	Organization   model.Organization
	Selected       model.SelectedContext
	ContextVersion uint64
}

type SwitchContextCommand struct {
	SessionID              string
	Subject                string
	Selected               model.SelectedContext
	ExpectedContextVersion uint64
}

type Authentication struct {
	repository AuthRepository
	directory  *Directory
}

func NewAuthentication(repository AuthRepository, directory *Directory) *Authentication {
	return &Authentication{repository: repository, directory: directory}
}

func (a *Authentication) Login(ctx context.Context, command LoginCommand) (AuthenticatedSession, error) {
	if a.repository == nil || a.directory == nil {
		return AuthenticatedSession{}, model.ErrUnavailable
	}
	if !command.Realm.Valid() || command.SessionID == "" || command.FamilyID == "" || command.ExpiresAt.IsZero() {
		return AuthenticatedSession{}, ErrInvalidCredentials
	}
	hasPassword := command.Username != "" && command.Password != ""
	hasChallenge := command.ChallengeID != ""
	if hasPassword == hasChallenge {
		return AuthenticatedSession{}, ErrInvalidCredentials
	}
	if hasChallenge {
		if command.Realm != principal.RealmCustomer || command.ShopCode == "" || command.Username != "" || command.Password != "" || !validChallengeID(command.ChallengeID) {
			return AuthenticatedSession{}, ErrInvalidCredentials
		}
	}
	return a.repository.Login(ctx, command)
}

func validChallengeID(id string) bool {
	if len(id) != 64 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func (a *Authentication) Guest(ctx context.Context, command GuestCommand) (AuthenticatedSession, error) {
	if a.repository == nil || a.directory == nil {
		return AuthenticatedSession{}, model.ErrUnavailable
	}
	if command.Subject == "" || command.ShopCode == "" || command.SessionID == "" || command.FamilyID == "" || command.ExpiresAt.IsZero() {
		return AuthenticatedSession{}, ErrInvalidCredentials
	}
	return a.repository.Guest(ctx, command)
}

func (a *Authentication) RotateRefresh(ctx context.Context, expectedRealm principal.Realm, refreshToken string, expiresAt time.Time) (AuthenticatedSession, string, error) {
	if a.repository == nil || !expectedRealm.Valid() || refreshToken == "" || expiresAt.IsZero() {
		return AuthenticatedSession{}, "", ErrInvalidCredentials
	}
	return a.repository.RotateRefresh(ctx, expectedRealm, refreshToken, expiresAt)
}

func (a *Authentication) Logout(ctx context.Context, expectedRealm principal.Realm, refreshToken string) error {
	if a.repository == nil {
		return model.ErrUnavailable
	}
	if refreshToken == "" {
		return nil
	}
	if !expectedRealm.Valid() {
		return ErrInvalidCredentials
	}
	return a.repository.Logout(ctx, expectedRealm, refreshToken)
}

func (a *Authentication) SwitchContext(ctx context.Context, command SwitchContextCommand) (AuthenticatedSession, error) {
	if a.repository == nil || a.directory == nil {
		return AuthenticatedSession{}, model.ErrUnavailable
	}
	if _, err := a.directory.ValidateSelectedContext(ctx, command.Subject, command.Selected, 0, 0); err != nil {
		return AuthenticatedSession{}, err
	}
	return a.repository.SwitchContext(ctx, command)
}
