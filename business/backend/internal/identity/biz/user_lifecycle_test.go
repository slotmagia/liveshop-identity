package biz

import (
	"context"
	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

type userLifecycleRecorder struct {
	create CreateOperator
	reset  ResetCredential
	own    ChangeOwnCredential
}

func (*userLifecycleRecorder) ListUsers(context.Context, UserScope) ([]ManagedUser, error) {
	return nil, nil
}
func (*userLifecycleRecorder) ListMembers(context.Context, MemberQuery) (MemberPage, error) {
	return MemberPage{}, nil
}
func (*userLifecycleRecorder) GetUser(context.Context, UserScope, string) (ManagedUser, error) {
	return ManagedUser{}, nil
}
func (*userLifecycleRecorder) OwnAccount(context.Context, UserScope, string) (ManagedUser, error) {
	return ManagedUser{}, nil
}
func (r *userLifecycleRecorder) CreatePlatformOperator(_ context.Context, c CreateOperator) (ManagedUser, error) {
	r.create = c
	return ManagedUser{}, nil
}
func (*userLifecycleRecorder) ChangeUserStatus(context.Context, ChangeUserStatus) (ManagedUser, error) {
	return ManagedUser{}, nil
}
func (r *userLifecycleRecorder) ResetCredential(_ context.Context, c ResetCredential) (ManagedCredential, error) {
	r.reset = c
	return ManagedCredential{}, nil
}
func (r *userLifecycleRecorder) ChangeOwnCredential(_ context.Context, c ChangeOwnCredential) (ChangeOwnCredentialResult, error) {
	r.own = c
	return ChangeOwnCredentialResult{}, nil
}
func (*userLifecycleRecorder) ListSessions(context.Context, UserScope, string) ([]ManagedSession, error) {
	return nil, nil
}
func (*userLifecycleRecorder) ListOwnSessions(context.Context, UserScope, string) ([]ManagedSession, error) {
	return nil, nil
}
func (*userLifecycleRecorder) RevokeSessions(context.Context, RevokeSessions) error { return nil }
func (*userLifecycleRecorder) RevokeOwnSessions(context.Context, RevokeSessions) error {
	return nil
}
func (*userLifecycleRecorder) ValidateCurrentAuthorization(context.Context, modulesession.Claims) error {
	return nil
}

func TestUserLifecycleNormalizesCreateAndUsesStablePasswordDigest(t *testing.T) {
	repository := &userLifecycleRecorder{}
	service := NewUserLifecycle(repository)
	_, err := service.CreateOperator(context.Background(), CreateOperator{IdempotencyKey: "key", OperationID: "operation", Subject: "subject", DisplayName: " Operator ", Username: " UserName ", Password: "password-one", OrganizationID: 1, RoleIDs: []int64{3, 2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	if repository.create.DisplayName != "Operator" || repository.create.Username != "username" || len(repository.create.RoleIDs) != 2 || repository.create.RoleIDs[0] != 2 {
		t.Fatalf("not normalized: %+v", repository.create)
	}
	first := ResetCredential{IdempotencyKey: "reset", OperationID: "reset", Subject: "subject", ActorSubject: "actor", Scope: UserScope{OrganizationID: 1}, CredentialID: 1, ExpectedCredentialVersion: 1, Password: "password-two"}
	if _, err := service.ResetCredential(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	hash := repository.reset.RequestHash
	if _, err := service.ResetCredential(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if repository.reset.RequestHash != hash {
		t.Fatal("identical reset command hash changed")
	}
	first.Password = "password-three"
	if _, err := service.ResetCredential(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if repository.reset.RequestHash == hash {
		t.Fatal("different password reused command hash")
	}
}

func TestUserLifecycleChangeOwnCredentialHashesPasswordsAndRejectsSelfReuse(t *testing.T) {
	repository := &userLifecycleRecorder{}
	service := NewUserLifecycle(repository)
	scope := UserScope{OrganizationID: 9, MerchantID: 7}
	valid := ChangeOwnCredential{IdempotencyKey: "change", OperationID: "change", Subject: "owner-1", ActorSubject: "owner-1", SessionID: "session-1", OldPassword: "password-old", Password: "password-new", Scope: scope, ExpectedCredentialVersion: 1}
	if _, err := service.ChangeOwnCredential(context.Background(), valid); err != nil {
		t.Fatal(err)
	}
	if repository.own.Password != "password-new" || repository.own.RequestHash == [32]byte{} {
		t.Fatalf("command not forwarded: %+v", repository.own)
	}
	hash := repository.own.RequestHash
	if _, err := service.ChangeOwnCredential(context.Background(), valid); err != nil {
		t.Fatal(err)
	}
	if repository.own.RequestHash != hash {
		t.Fatal("identical change-own command hash changed")
	}
	valid.Password = "password-alt"
	if _, err := service.ChangeOwnCredential(context.Background(), valid); err != nil {
		t.Fatal(err)
	}
	if repository.own.RequestHash == hash {
		t.Fatal("different new password reused command hash")
	}
	same := valid
	same.Password = same.OldPassword
	if _, err := service.ChangeOwnCredential(context.Background(), same); err == nil {
		t.Fatal("identical new and old password accepted")
	}
	missingSession := valid
	missingSession.SessionID = ""
	if _, err := service.ChangeOwnCredential(context.Background(), missingSession); err == nil {
		t.Fatal("missing session accepted")
	}
	foreign := valid
	foreign.ActorSubject = "other"
	if _, err := service.ChangeOwnCredential(context.Background(), foreign); err == nil {
		t.Fatal("foreign actor accepted")
	}
}

func TestUserLifecycleRejectsClosedStatusTarget(t *testing.T) {
	service := NewUserLifecycle(&userLifecycleRecorder{})
	_, err := service.ChangeStatus(context.Background(), ChangeUserStatus{IdempotencyKey: "key", OperationID: "operation", Subject: "subject", ActorSubject: "actor", Scope: UserScope{OrganizationID: 1}, ExpectedIdentityVersion: 1, ExpectedAccessVersion: 1, Target: model.StatusClosed})
	if err == nil {
		t.Fatal("closed target accepted")
	}
}

func TestMemberQueryNormalizeRejectsPlatformScopeAndUnknownFilters(t *testing.T) {
	if _, err := (MemberQuery{Scope: UserScope{OrganizationID: 1}}).Normalize(); err == nil {
		t.Fatal("platform scope accepted")
	}
	if _, err := (MemberQuery{Scope: UserScope{OrganizationID: 9, MerchantID: 7}, MemberType: "OWNER"}).Normalize(); err == nil {
		t.Fatal("owner member type accepted")
	}
	normalized, err := (MemberQuery{Scope: UserScope{OrganizationID: 9, MerchantID: 7}, Keyword: "  Ada  ", MemberType: "staff", Status: "active"}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Keyword != "Ada" || normalized.MemberType != "STAFF" || normalized.Status != "ACTIVE" || normalized.Page != 1 || normalized.PageSize != 20 {
		t.Fatalf("not normalized: %+v", normalized)
	}
}
