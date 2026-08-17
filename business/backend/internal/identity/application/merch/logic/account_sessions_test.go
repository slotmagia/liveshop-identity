package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type accountSessionRepo struct {
	sessions []biz.ManagedSession
	revokes  []biz.RevokeSessions
}

func (*accountSessionRepo) ListUsers(context.Context, biz.UserScope) ([]biz.ManagedUser, error) {
	return nil, nil
}
func (*accountSessionRepo) ListMembers(context.Context, biz.MemberQuery) (biz.MemberPage, error) {
	return biz.MemberPage{}, nil
}
func (*accountSessionRepo) GetUser(context.Context, biz.UserScope, string) (biz.ManagedUser, error) {
	return biz.ManagedUser{}, nil
}
func (*accountSessionRepo) OwnAccount(context.Context, biz.UserScope, string) (biz.ManagedUser, error) {
	return biz.ManagedUser{}, nil
}
func (*accountSessionRepo) CreatePlatformOperator(context.Context, biz.CreateOperator) (biz.ManagedUser, error) {
	return biz.ManagedUser{}, nil
}
func (*accountSessionRepo) ChangeUserStatus(context.Context, biz.ChangeUserStatus) (biz.ManagedUser, error) {
	return biz.ManagedUser{}, nil
}
func (*accountSessionRepo) ResetCredential(context.Context, biz.ResetCredential) (biz.ManagedCredential, error) {
	return biz.ManagedCredential{}, nil
}
func (*accountSessionRepo) ChangeOwnCredential(context.Context, biz.ChangeOwnCredential) (biz.ChangeOwnCredentialResult, error) {
	return biz.ChangeOwnCredentialResult{}, nil
}
func (r *accountSessionRepo) ListSessions(_ context.Context, _ biz.UserScope, subject string) ([]biz.ManagedSession, error) {
	if subject != "staff-1" {
		return nil, model.ErrNotFound
	}
	return append([]biz.ManagedSession(nil), r.sessions...), nil
}
func (r *accountSessionRepo) ListOwnSessions(_ context.Context, _ biz.UserScope, subject string) ([]biz.ManagedSession, error) {
	if subject != "staff-1" && subject != "owner-1" {
		return nil, model.ErrNotFound
	}
	return append([]biz.ManagedSession(nil), r.sessions...), nil
}
func (r *accountSessionRepo) RevokeSessions(_ context.Context, command biz.RevokeSessions) error {
	if command.Subject != "staff-1" {
		return model.ErrNotFound
	}
	return r.recordRevoke(command)
}
func (r *accountSessionRepo) RevokeOwnSessions(_ context.Context, command biz.RevokeSessions) error {
	if command.Subject != "staff-1" && command.Subject != "owner-1" {
		return model.ErrNotFound
	}
	return r.recordRevoke(command)
}
func (r *accountSessionRepo) recordRevoke(command biz.RevokeSessions) error {
	for _, session := range r.sessions {
		if session.ID == command.SessionID {
			r.revokes = append(r.revokes, command)
			return nil
		}
	}
	return model.ErrNotFound
}
func (*accountSessionRepo) ValidateCurrentAuthorization(context.Context, modulesession.Claims) error {
	return nil
}

func sampleAccountSessions() []biz.ManagedSession {
	now := time.Now().UTC()
	return []biz.ManagedSession{
		{ID: "current-session", DeviceName: "Chrome", IPAddress: "10.0.0.2", Status: "ACTIVE", CreatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339Nano), LastRefreshedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339Nano)},
		{ID: "other-session", DeviceName: "Phone", IPAddress: "10.0.0.3", Status: "ACTIVE", CreatedAt: now.Add(-3 * time.Hour).Format(time.RFC3339Nano), LastRefreshedAt: now.Add(-2 * time.Hour).Format(time.RFC3339Nano), ExpiresAt: now.Add(2 * time.Hour).Format(time.RFC3339Nano)},
		{ID: "revoked-session", DeviceName: "Tablet", IPAddress: "10.0.0.4", Status: "REVOKED", CreatedAt: now.Add(-4 * time.Hour).Format(time.RFC3339Nano), LastRefreshedAt: now.Add(-3 * time.Hour).Format(time.RFC3339Nano), ExpiresAt: now.Add(30 * time.Minute).Format(time.RFC3339Nano)},
		{ID: "expired-session", DeviceName: "Old", IPAddress: "10.0.0.5", Status: "ACTIVE", CreatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339Nano), LastRefreshedAt: now.Add(-25 * time.Hour).Format(time.RFC3339Nano), ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339Nano)},
	}
}

func merchAccountSessionContext(subject string, principalType principal.Type) context.Context {
	return authctx.With(context.Background(), modulesession.Claims{
		Subject: subject, SessionID: "current-session", PrincipalType: principalType, OrganizationID: 9, MerchantID: 7, ShopID: 101,
	})
}

func accountSessionLogic(sessions []biz.ManagedSession) (*Logic, *accountSessionRepo) {
	repo := &accountSessionRepo{sessions: sessions}
	return New(nil, nil, nil, biz.NewUserLifecycle(repo), nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil), repo
}

func TestAccountSessionsRequireMerchantContext(t *testing.T) {
	logic, _ := accountSessionLogic(sampleAccountSessions())
	if _, err := logic.AccountSessions(context.Background(), appmodel.AccountSessionQuery{}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestAccountSessionsAreReadableByOwnerAndStaffAndHideExpired(t *testing.T) {
	logic, _ := accountSessionLogic(sampleAccountSessions())
	page, err := logic.AccountSessions(merchAccountSessionContext("staff-1", principal.TypeMerchantStaff), appmodel.AccountSessionQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Items) != 3 || !page.Items[0].Current || page.Items[0].ID != "current-session" {
		t.Fatalf("page=%+v", page)
	}
	for _, item := range page.Items {
		if item.ID == "expired-session" {
			t.Fatalf("expired session leaked: %+v", page.Items)
		}
	}

	owner, err := logic.AccountSessions(merchAccountSessionContext("owner-1", principal.TypeMerchantOwner), appmodel.AccountSessionQuery{Status: "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	if owner.Total != 2 {
		t.Fatalf("active=%+v", owner)
	}
}

func TestAccountSessionsRejectInvalidStatus(t *testing.T) {
	logic, _ := accountSessionLogic(sampleAccountSessions())
	if _, err := logic.AccountSessions(merchAccountSessionContext("staff-1", principal.TypeMerchantStaff), appmodel.AccountSessionQuery{Status: "closed"}); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("error=%v", err)
	}
}

func TestRevokeAccountSessionUsesCallerSubjectAndMarksCurrent(t *testing.T) {
	logic, repo := accountSessionLogic(sampleAccountSessions())
	result, err := logic.RevokeAccountSession(merchAccountSessionContext("staff-1", principal.TypeMerchantStaff), appmodel.RevokeAccountSession{
		SessionID: "other-session", IdempotencyKey: "key-1", OperationID: "op-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentRevoked || len(repo.revokes) != 1 || repo.revokes[0].Subject != "staff-1" || repo.revokes[0].ActorSubject != "staff-1" || repo.revokes[0].Reason != "SELF_REVOKED" {
		t.Fatalf("result=%+v revokes=%+v", result, repo.revokes)
	}

	current, err := logic.RevokeAccountSession(merchAccountSessionContext("staff-1", principal.TypeMerchantStaff), appmodel.RevokeAccountSession{
		SessionID: "current-session", IdempotencyKey: "key-2", OperationID: "op-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !current.CurrentRevoked {
		t.Fatalf("current=%+v", current)
	}

	owner, err := logic.RevokeAccountSession(merchAccountSessionContext("owner-1", principal.TypeMerchantOwner), appmodel.RevokeAccountSession{
		SessionID: "other-session", IdempotencyKey: "key-owner", OperationID: "op-owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	if owner.CurrentRevoked || repo.revokes[len(repo.revokes)-1].Subject != "owner-1" {
		t.Fatalf("owner=%+v revokes=%+v", owner, repo.revokes)
	}
}

func TestRevokeAccountSessionRejectsForeignSessionId(t *testing.T) {
	logic, repo := accountSessionLogic(sampleAccountSessions())
	if _, err := logic.RevokeAccountSession(merchAccountSessionContext("staff-1", principal.TypeMerchantStaff), appmodel.RevokeAccountSession{
		SessionID: "missing-session", IdempotencyKey: "key-3", OperationID: "op-3",
	}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("error=%v", err)
	}
	if len(repo.revokes) != 0 {
		t.Fatalf("revokes=%+v", repo.revokes)
	}
}
