package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type accountDirectoryRepo struct {
	principal model.PrincipalContext
	directory biz.OrganizationDirectory
}

func (s accountDirectoryRepo) ResolvePrincipalContext(context.Context, string) (model.PrincipalContext, error) {
	return s.principal, nil
}
func (accountDirectoryRepo) ValidateActiveSession(context.Context, string, string, model.SelectedContext, uint64) error {
	return nil
}
func (accountDirectoryRepo) ResolveShopByID(context.Context, int64) (model.ShopResolution, error) {
	return model.ShopResolution{}, nil
}
func (accountDirectoryRepo) ListOrganizationSubtree(context.Context, int64, int64) ([]int64, uint64, error) {
	return nil, 0, nil
}
func (accountDirectoryRepo) BatchGetSubjects(context.Context, []string) ([]model.Subject, error) {
	return nil, nil
}
func (accountDirectoryRepo) ResolveLegacySubjects(context.Context, principal.Realm, []int64) ([]model.Subject, error) {
	return nil, nil
}
func (s accountDirectoryRepo) ListOrganizationDirectory(context.Context, int64, int64) (biz.OrganizationDirectory, error) {
	return s.directory, nil
}
func (accountDirectoryRepo) CreateOrganizationUnit(context.Context, biz.CreateOrganizationUnit) (biz.OrganizationUnitResult, error) {
	return biz.OrganizationUnitResult{}, nil
}
func (accountDirectoryRepo) ProvisionMember(context.Context, biz.ProvisionMember) (biz.ProvisionMemberResult, error) {
	return biz.ProvisionMemberResult{}, nil
}
func (accountDirectoryRepo) ReplaceMemberAccess(context.Context, biz.ReplaceMemberAccess) (biz.ProvisionMemberResult, error) {
	return biz.ProvisionMemberResult{}, nil
}
func (accountDirectoryRepo) UpdateMember(context.Context, biz.UpdateMember) (biz.UpdateMemberResult, error) {
	return biz.UpdateMemberResult{}, nil
}

func sampleDirectory() biz.OrganizationDirectory {
	return biz.OrganizationDirectory{
		Organization: model.Organization{ID: 9, Type: model.OrganizationMerchant, MerchantID: 7, Name: "示例商户", Status: model.StatusActive, Version: 1},
		Units:        []biz.OrganizationUnit{{ID: 1, Name: "总部", Status: model.StatusActive, Version: 1}},
		Members: []biz.MemberDirectoryItem{
			{
				Member:      model.WorkforceMember{ID: 1, OrganizationID: 9, MerchantID: 7, Subject: "owner-1", Type: model.MemberOwner, Status: model.MemberActive, AccessVersion: 1},
				DisplayName: "店主", PrincipalType: principal.TypeMerchantOwner, SubjectStatus: model.StatusActive, SubjectVersion: 1,
				Credential: biz.ManagedCredential{ID: 8, Version: 1, Kind: "USERNAME", Identifier: "owner", Status: "ACTIVE"},
			},
			{
				Member:      model.WorkforceMember{ID: 2, OrganizationID: 9, MerchantID: 7, Subject: "staff-1", Type: model.MemberStaff, Status: model.MemberActive, AccessVersion: 1},
				DisplayName: "店员", PrincipalType: principal.TypeMerchantStaff, SubjectStatus: model.StatusActive, SubjectVersion: 1,
				Credential: biz.ManagedCredential{ID: 9, Version: 1, Kind: "USERNAME", Identifier: "ada", Status: "ACTIVE"},
				ShopIDs:    []int64{101},
			},
			{
				Member:      model.WorkforceMember{ID: 3, OrganizationID: 9, MerchantID: 7, Subject: "anchor-1", Type: model.MemberAnchor, Status: model.MemberActive, AccessVersion: 1},
				DisplayName: "主播", PrincipalType: principal.TypeShopAnchor, SubjectStatus: model.StatusActive, SubjectVersion: 1,
			},
		},
		Shops: []biz.ShopDirectoryItem{
			{Context: model.ShopContext{MerchantID: 7, ShopID: 101}, Name: "主店", Code: "main", Status: model.StatusActive, Version: 1},
			{Context: model.ShopContext{MerchantID: 7, ShopID: 102}, Name: "分店", Code: "branch", Status: model.StatusActive, Version: 1},
		},
	}
}

func ownerPrincipal() model.PrincipalContext {
	return model.PrincipalContext{
		Subject:      model.Subject{ID: "owner-1", Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantOwner, DisplayName: "店主", Status: model.StatusActive, Version: 1},
		Organization: model.Organization{ID: 9, Type: model.OrganizationMerchant, MerchantID: 7, Name: "示例商户", Status: model.StatusActive, Version: 1},
		Member:       model.WorkforceMember{ID: 1, OrganizationID: 9, MerchantID: 7, Subject: "owner-1", Type: model.MemberOwner, Status: model.MemberActive, AccessVersion: 1},
	}
}

func staffPrincipal() model.PrincipalContext {
	return model.PrincipalContext{
		Subject:        model.Subject{ID: "staff-1", Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff, DisplayName: "店员", Status: model.StatusActive, Version: 1},
		Organization:   model.Organization{ID: 9, Type: model.OrganizationMerchant, MerchantID: 7, Name: "示例商户", Status: model.StatusActive, Version: 1},
		Member:         model.WorkforceMember{ID: 2, OrganizationID: 9, MerchantID: 7, Subject: "staff-1", Type: model.MemberStaff, Status: model.MemberActive, AccessVersion: 1},
		AvailableShops: []model.ShopContext{{MerchantID: 7, ShopID: 101}},
	}
}

func merchStaffShopContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{
		Subject: "staff-1", PrincipalType: principal.TypeMerchantStaff, OrganizationID: 9, MerchantID: 7, ShopID: 101,
	})
}

func TestAccountRequiresMerchantContext(t *testing.T) {
	logic := New(nil, biz.NewDirectory(accountDirectoryRepo{principal: ownerPrincipal(), directory: sampleDirectory()}), nil, nil, nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil)
	if _, err := logic.Account(context.Background()); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestAccountOverviewIsReadableByOwnerAndStaff(t *testing.T) {
	logic := New(nil, biz.NewDirectory(accountDirectoryRepo{principal: ownerPrincipal(), directory: sampleDirectory()}), nil, nil, nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil)
	owner, err := logic.Account(merchOwnerContext())
	if err != nil {
		t.Fatal(err)
	}
	if !owner.Owner || owner.Account != "owner" || owner.Merchant.Name != "示例商户" || len(owner.Shops) != 2 || owner.Organization.MemberCount != 2 || owner.Organization.ShopCount != 2 {
		t.Fatalf("owner=%+v", owner)
	}

	logic = New(nil, biz.NewDirectory(accountDirectoryRepo{principal: staffPrincipal(), directory: sampleDirectory()}), nil, nil, nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil)
	staff, err := logic.Account(merchStaffShopContext())
	if err != nil {
		t.Fatal(err)
	}
	if staff.Owner || staff.Account != "ada" || staff.CurrentShopID != 101 || len(staff.Shops) != 1 || staff.Shops[0].ID != 101 {
		t.Fatalf("staff=%+v", staff)
	}
}

type securityUsersRepo struct {
	own    biz.ManagedUser
	change biz.ChangeOwnCredential
	result biz.ChangeOwnCredentialResult
}

func (s *securityUsersRepo) ListUsers(context.Context, biz.UserScope) ([]biz.ManagedUser, error) {
	return nil, nil
}
func (s *securityUsersRepo) ListMembers(context.Context, biz.MemberQuery) (biz.MemberPage, error) {
	return biz.MemberPage{}, nil
}
func (s *securityUsersRepo) GetUser(context.Context, biz.UserScope, string) (biz.ManagedUser, error) {
	return biz.ManagedUser{}, nil
}
func (s *securityUsersRepo) OwnAccount(_ context.Context, _ biz.UserScope, subject string) (biz.ManagedUser, error) {
	if s.own.Subject.ID != "" && s.own.Subject.ID != subject {
		return biz.ManagedUser{}, model.ErrNotFound
	}
	return s.own, nil
}
func (s *securityUsersRepo) CreatePlatformOperator(context.Context, biz.CreateOperator) (biz.ManagedUser, error) {
	return biz.ManagedUser{}, nil
}
func (s *securityUsersRepo) ChangeUserStatus(context.Context, biz.ChangeUserStatus) (biz.ManagedUser, error) {
	return biz.ManagedUser{}, nil
}
func (s *securityUsersRepo) ResetCredential(context.Context, biz.ResetCredential) (biz.ManagedCredential, error) {
	return biz.ManagedCredential{}, nil
}
func (s *securityUsersRepo) ChangeOwnCredential(_ context.Context, command biz.ChangeOwnCredential) (biz.ChangeOwnCredentialResult, error) {
	s.change = command
	return s.result, nil
}
func (s *securityUsersRepo) ListSessions(context.Context, biz.UserScope, string) ([]biz.ManagedSession, error) {
	return nil, nil
}
func (s *securityUsersRepo) ListOwnSessions(context.Context, biz.UserScope, string) ([]biz.ManagedSession, error) {
	return nil, nil
}
func (s *securityUsersRepo) RevokeSessions(context.Context, biz.RevokeSessions) error { return nil }
func (s *securityUsersRepo) RevokeOwnSessions(context.Context, biz.RevokeSessions) error {
	return nil
}
func (s *securityUsersRepo) ValidateCurrentAuthorization(context.Context, modulesession.Claims) error {
	return nil
}

func ownerSecurityUser() biz.ManagedUser {
	return biz.ManagedUser{
		Subject:        model.Subject{ID: "owner-1", DisplayName: "店主", PrincipalType: principal.TypeMerchantOwner, Status: model.StatusActive, Version: 1},
		Member:         model.WorkforceMember{ID: 1, OrganizationID: 9, MerchantID: 7, Type: model.MemberOwner, Status: model.MemberActive, Subject: "owner-1"},
		Credential:     biz.ManagedCredential{ID: 8, Version: 3, Kind: "USERNAME", Identifier: "owner", Status: model.StatusActive},
		ActiveSessions: 2,
	}
}

func staffSecurityUser() biz.ManagedUser {
	return biz.ManagedUser{
		Subject:        model.Subject{ID: "staff-1", DisplayName: "店员", PrincipalType: principal.TypeMerchantStaff, Status: model.StatusActive, Version: 1},
		Member:         model.WorkforceMember{ID: 2, OrganizationID: 9, MerchantID: 7, Type: model.MemberStaff, Status: model.MemberActive, Subject: "staff-1"},
		Credential:     biz.ManagedCredential{ID: 9, Version: 1, Kind: "USERNAME", Identifier: "ada", Status: model.StatusActive},
		ActiveSessions: 1,
	}
}

func merchOwnerSessionContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{
		Subject: "owner-1", PrincipalType: principal.TypeMerchantOwner, OrganizationID: 9, MerchantID: 7, SessionID: "session-owner",
	})
}

func TestAccountSecurityIsReadableByOwnerAndStaff(t *testing.T) {
	ownerLogic := New(nil, nil, nil, biz.NewUserLifecycle(&securityUsersRepo{own: ownerSecurityUser()}), nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil)
	owner, err := ownerLogic.AccountSecurity(merchOwnerContext())
	if err != nil {
		t.Fatal(err)
	}
	if !owner.Owner || owner.Account != "owner" || owner.Credential.Version != 3 || owner.ActiveSessions != 2 {
		t.Fatalf("owner=%+v", owner)
	}

	staffLogic := New(nil, nil, nil, biz.NewUserLifecycle(&securityUsersRepo{own: staffSecurityUser()}), nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil)
	staff, err := staffLogic.AccountSecurity(merchStaffShopContext())
	if err != nil {
		t.Fatal(err)
	}
	if staff.Owner || staff.Account != "ada" || staff.Subject != "staff-1" {
		t.Fatalf("staff=%+v", staff)
	}
}

func TestAccountSecurityRequiresMerchantContext(t *testing.T) {
	logic := New(nil, nil, nil, biz.NewUserLifecycle(&securityUsersRepo{own: ownerSecurityUser()}), nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil)
	if _, err := logic.AccountSecurity(context.Background()); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestChangeOwnCredentialUsesClaimsSubjectAndSession(t *testing.T) {
	repo := &securityUsersRepo{result: biz.ChangeOwnCredentialResult{
		Credential:      biz.ManagedCredential{ID: 8, Version: 4, Kind: "USERNAME", Identifier: "owner", Status: model.StatusActive},
		RevokedSessions: 1, CurrentRetained: true,
	}}
	logic := New(nil, nil, nil, biz.NewUserLifecycle(repo), nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil)
	if _, err := logic.ChangeOwnCredential(merchOwnerContext(), appmodel.ChangeOwnCredential{CommandKey: "change", ExpectedVersion: 3, OldPassword: "password-old", Password: "password-new"}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("missing session error=%v", err)
	}
	result, err := logic.ChangeOwnCredential(merchOwnerSessionContext(), appmodel.ChangeOwnCredential{CommandKey: "change", ExpectedVersion: 3, OldPassword: "password-old", Password: "password-new"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.change.Subject != "owner-1" || repo.change.ActorSubject != "owner-1" || repo.change.SessionID != "session-owner" || repo.change.ExpectedCredentialVersion != 3 {
		t.Fatalf("command=%+v", repo.change)
	}
	if !result.CurrentRetained || result.RevokedSessions != 1 || result.Credential.Version != 4 {
		t.Fatalf("result=%+v", result)
	}
}
