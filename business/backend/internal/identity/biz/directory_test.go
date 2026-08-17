package biz

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/kernel-go/principal"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

type directoryStub struct {
	principal      model.PrincipalContext
	shop           model.ShopResolution
	shopErr        error
	provisionCalls int
	commands       []ProvisionMember
	updates        []UpdateMember
	legacyRealm    principal.Realm
	legacyUIDs     []int64
}

func (s *directoryStub) ResolvePrincipalContext(context.Context, string) (model.PrincipalContext, error) {
	return s.principal, nil
}
func (*directoryStub) ValidateActiveSession(context.Context, string, string, model.SelectedContext, uint64) error {
	return nil
}
func (s *directoryStub) ResolveShopByID(context.Context, int64) (model.ShopResolution, error) {
	return s.shop, s.shopErr
}
func (*directoryStub) ListOrganizationSubtree(context.Context, int64, int64) ([]int64, uint64, error) {
	return nil, 0, nil
}
func (*directoryStub) BatchGetSubjects(context.Context, []string) ([]model.Subject, error) {
	return nil, nil
}
func (s *directoryStub) ResolveLegacySubjects(_ context.Context, realm principal.Realm, legacyUIDs []int64) ([]model.Subject, error) {
	s.legacyRealm = realm
	s.legacyUIDs = append([]int64(nil), legacyUIDs...)
	return []model.Subject{{ID: "customer", Realm: realm, PrincipalType: principal.TypeCustomer, LegacyUID: legacyUIDs[0], Status: model.StatusActive, Version: 1}}, nil
}
func (*directoryStub) ListOrganizationDirectory(context.Context, int64, int64) (OrganizationDirectory, error) {
	return OrganizationDirectory{}, nil
}
func (*directoryStub) CreateOrganizationUnit(context.Context, CreateOrganizationUnit) (OrganizationUnitResult, error) {
	return OrganizationUnitResult{}, nil
}
func (s *directoryStub) ProvisionMember(_ context.Context, command ProvisionMember) (ProvisionMemberResult, error) {
	s.provisionCalls++
	s.commands = append(s.commands, command)
	return ProvisionMemberResult{Subject: command.Subject, Status: model.MemberActive, AccessVersion: 1, OperationID: command.OperationID}, nil
}

func TestAuthenticatedGuestUsesTheAlreadyValidatedSessionShop(t *testing.T) {
	shop := model.ShopContext{MerchantID: 2, ShopID: 4}
	repository := &directoryStub{principal: model.PrincipalContext{Subject: model.Subject{ID: "guest-1", Realm: principal.RealmCustomer, PrincipalType: principal.TypeGuest, Status: model.StatusActive, Version: 1}}}
	resolved, err := NewDirectory(repository).ResolveAuthenticatedPrincipalContext(context.Background(), "session-1", "guest-1", model.SelectedContext{ShopContext: shop}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Selected.ShopContext != shop {
		t.Fatalf("guest session shop was lost: %#v", resolved.Selected)
	}
	if _, err := NewDirectory(repository).ResolvePrincipalContext(context.Background(), "guest-1", model.SelectedContext{ShopContext: shop}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("untrusted guest shop selection was accepted: %v", err)
	}
}

func TestProvisionMemberIdempotencyHashUsesNormalizedBusinessInput(t *testing.T) {
	repository := &directoryStub{}
	command := ProvisionMember{OperationID: "op", IdempotencyKey: "key", Subject: "subject", Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff, DisplayName: "Staff", OrganizationID: 9, MerchantID: 7, MemberType: model.MemberStaff, AssignmentKind: model.AssignmentOperate, ShopIDs: []int64{102, 101}, CredentialKind: "USERNAME", CredentialNamespace: "GLOBAL", NormalizedIdentifier: "staff", Password: "password", RoleIDs: []int64{2, 1, 2}}
	directory := NewDirectory(repository)
	if _, err := directory.ProvisionMember(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if _, err := directory.ProvisionMember(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if repository.commands[0].RequestHash != repository.commands[1].RequestHash {
		t.Fatal("same business request produced different idempotency hashes")
	}
	if got := repository.commands[0].RoleIDs; len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("role ids were not normalized: %v", got)
	}
}

func TestLegacySubjectsValidatesAndDeduplicatesUIDs(t *testing.T) {
	repository := &directoryStub{}
	directory := NewDirectory(repository)
	items, err := directory.LegacySubjects(context.Background(), principal.RealmCustomer, []int64{19, 19, 23})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || repository.legacyRealm != principal.RealmCustomer || len(repository.legacyUIDs) != 2 || repository.legacyUIDs[0] != 19 || repository.legacyUIDs[1] != 23 {
		t.Fatalf("unexpected legacy resolution: realm=%q uids=%v items=%v", repository.legacyRealm, repository.legacyUIDs, items)
	}
}

func TestResolveShopPublishesAuthoritativeCurrency(t *testing.T) {
	want := model.ShopResolution{
		Context:  model.ShopContext{MerchantID: 7, ShopID: 101},
		Currency: "CNY",
		Status:   model.StatusActive,
		Version:  4,
	}
	got, err := NewDirectory(&directoryStub{shop: want}).ResolveShopByID(context.Background(), 101)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolved shop = %+v, want %+v", got, want)
	}
}

func TestResolveShopFailsClosedForInvalidCurrency(t *testing.T) {
	for _, currency := range []string{"", "cny", "US", "US12"} {
		t.Run(currency, func(t *testing.T) {
			shop := model.ShopResolution{
				Context:  model.ShopContext{MerchantID: 7, ShopID: 101},
				Currency: currency,
				Status:   model.StatusActive,
				Version:  4,
			}
			_, err := NewDirectory(&directoryStub{shop: shop}).ResolveShopByID(context.Background(), 101)
			if !errors.Is(err, model.ErrInvalidShopCurrency) {
				t.Fatalf("currency %q error = %v, want invalid shop currency", currency, err)
			}
		})
	}
}

func TestLegacySubjectsRejectsInvalidScope(t *testing.T) {
	directory := NewDirectory(&directoryStub{})
	for _, test := range []struct {
		name  string
		realm principal.Realm
		uids  []int64
	}{
		{name: "missing realm", uids: []int64{1}},
		{name: "empty", realm: principal.RealmCustomer},
		{name: "invalid uid", realm: principal.RealmCustomer, uids: []int64{0}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := directory.LegacySubjects(context.Background(), test.realm, test.uids); !errors.Is(err, model.ErrInvalidContext) {
				t.Fatalf("error = %v, want invalid context", err)
			}
		})
	}
}

func TestProvisionMemberIdempotencyHashRejectsChangedPasswordOrRoles(t *testing.T) {
	base := ProvisionMember{OperationID: "op", IdempotencyKey: "key", Subject: "subject", Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff, DisplayName: "Staff", OrganizationID: 9, MerchantID: 7, MemberType: model.MemberStaff, AssignmentKind: model.AssignmentOperate, ShopIDs: []int64{101}, CredentialKind: "USERNAME", CredentialNamespace: "GLOBAL", NormalizedIdentifier: "staff", Password: "password", RoleIDs: []int64{1}}
	repository := &directoryStub{}
	directory := NewDirectory(repository)
	if _, err := directory.ProvisionMember(context.Background(), base); err != nil {
		t.Fatal(err)
	}
	changedPassword := base
	changedPassword.Password = "different-password"
	if _, err := directory.ProvisionMember(context.Background(), changedPassword); err != nil {
		t.Fatal(err)
	}
	changedRoles := base
	changedRoles.RoleIDs = []int64{2}
	if _, err := directory.ProvisionMember(context.Background(), changedRoles); err != nil {
		t.Fatal(err)
	}
	if repository.commands[0].RequestHash == repository.commands[1].RequestHash {
		t.Fatal("changed password reused the original request hash")
	}
	if repository.commands[0].RequestHash == repository.commands[2].RequestHash {
		t.Fatal("changed roles reused the original request hash")
	}
}
func (*directoryStub) ReplaceMemberAccess(context.Context, ReplaceMemberAccess) (ProvisionMemberResult, error) {
	return ProvisionMemberResult{}, nil
}
func (s *directoryStub) UpdateMember(_ context.Context, command UpdateMember) (UpdateMemberResult, error) {
	s.updates = append(s.updates, command)
	return UpdateMemberResult{Subject: command.Subject, Status: model.MemberActive, IdentityVersion: command.ExpectedIdentityVersion, AccessVersion: command.ExpectedAccessVersion, OperationID: command.OperationID}, nil
}

func TestStaffContextCannotSelectUnassignedShop(t *testing.T) {
	assigned := model.ShopContext{MerchantID: 7, ShopID: 101}
	repository := &directoryStub{principal: model.PrincipalContext{
		Subject:        model.Subject{ID: "staff", Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff, Status: model.StatusActive, Version: 3},
		Organization:   model.Organization{ID: 9, Type: model.OrganizationMerchant, MerchantID: 7, Status: model.StatusActive, Version: 2},
		Member:         model.WorkforceMember{ID: 8, OrganizationID: 9, MerchantID: 7, Subject: "staff", Type: model.MemberStaff, Status: model.MemberActive, AccessVersion: 4},
		AvailableShops: []model.ShopContext{assigned},
	}}
	directory := NewDirectory(repository)

	selected := model.SelectedContext{OrganizationID: 9, ShopContext: model.ShopContext{MerchantID: 7, ShopID: 102}}
	_, err := directory.ResolvePrincipalContext(context.Background(), "staff", selected)
	if !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("expected invalid context, got %v", err)
	}
}

func TestSuspendedMemberFailsClosed(t *testing.T) {
	shop := model.ShopContext{MerchantID: 7, ShopID: 101}
	repository := &directoryStub{principal: model.PrincipalContext{
		Subject:        model.Subject{ID: "staff", Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff, Status: model.StatusActive, Version: 3},
		Organization:   model.Organization{ID: 9, Type: model.OrganizationMerchant, MerchantID: 7, Status: model.StatusActive, Version: 2},
		Member:         model.WorkforceMember{ID: 8, OrganizationID: 9, MerchantID: 7, Subject: "staff", Type: model.MemberStaff, Status: model.MemberSuspended, AccessVersion: 5},
		AvailableShops: []model.ShopContext{shop},
	}}
	_, err := NewDirectory(repository).ResolvePrincipalContext(context.Background(), "staff", model.SelectedContext{OrganizationID: 9, ShopContext: shop})
	if !errors.Is(err, model.ErrInactive) {
		t.Fatalf("expected inactive, got %v", err)
	}
}

func TestAnchorProvisioningRequiresExactlyOneAnchorShop(t *testing.T) {
	repository := &directoryStub{}
	directory := NewDirectory(repository)
	command := ProvisionMember{
		OperationID: "operation-1", IdempotencyKey: "request-1", Subject: "anchor-1",
		Realm: principal.RealmMerchant, PrincipalType: principal.TypeShopAnchor,
		DisplayName: "Anchor", OrganizationID: 9, MerchantID: 7, MemberType: model.MemberAnchor,
		AssignmentKind: model.AssignmentAnchor, ShopIDs: []int64{101, 102},
		CredentialKind: "USERNAME", CredentialNamespace: "SHOP",
		NormalizedIdentifier: "anchor", Password: "password",
		RoleIDs: []int64{1},
	}
	_, err := directory.ProvisionMember(context.Background(), command)
	if !errors.Is(err, model.ErrInvalidAssignment) {
		t.Fatalf("expected invalid assignment, got %v", err)
	}
	if repository.provisionCalls != 0 {
		t.Fatalf("invalid command reached repository %d times", repository.provisionCalls)
	}
}

func TestValidStaffProvisioningRequiresInitialBusinessGrant(t *testing.T) {
	repository := &directoryStub{}
	command := ProvisionMember{
		OperationID: "operation-1", IdempotencyKey: "request-1", Subject: "staff-1",
		Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff,
		DisplayName: "Staff", OrganizationID: 9, MerchantID: 7, MemberType: model.MemberStaff,
		AssignmentKind: model.AssignmentOperate, ShopIDs: []int64{101, 102},
		CredentialKind: "USERNAME", CredentialNamespace: "GLOBAL",
		NormalizedIdentifier: "staff", Password: "password",
		RoleIDs: []int64{1},
	}
	result, err := NewDirectory(repository).ProvisionMember(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != model.MemberActive || result.AccessVersion != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCustomerUsesCredentialShopWithoutWorkforceMembership(t *testing.T) {
	shop := model.ShopContext{MerchantID: 7, ShopID: 101}
	repository := &directoryStub{principal: model.PrincipalContext{
		Subject:        model.Subject{ID: "customer", Realm: principal.RealmCustomer, PrincipalType: principal.TypeCustomer, Status: model.StatusActive, Version: 1},
		AvailableShops: []model.ShopContext{shop},
	}}
	resolved, err := NewDirectory(repository).ResolvePrincipalContext(context.Background(), "customer", model.SelectedContext{ShopContext: shop})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Member.ID != 0 {
		t.Fatalf("customer became workforce member: %+v", resolved.Member)
	}
}

func TestUpdateMemberRequiresStaffShopsAndHashesNormalizedInput(t *testing.T) {
	repository := &directoryStub{}
	directory := NewDirectory(repository)
	base := UpdateMember{OperationID: "op", IdempotencyKey: "key", Subject: "staff-1", MerchantID: 7, DisplayName: " Staff ", MemberType: model.MemberStaff, ExpectedIdentityVersion: 1, ExpectedAccessVersion: 1, AssignmentKind: model.AssignmentOperate, RoleIDs: []int64{2, 1, 1}}
	if _, err := directory.UpdateMember(context.Background(), base); !errors.Is(err, model.ErrInvalidAssignment) {
		t.Fatalf("staff without shops accepted: %v", err)
	}
	base.ShopIDs = []int64{102, 101}
	first, err := directory.UpdateMember(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directory.UpdateMember(context.Background(), base); err != nil {
		t.Fatal(err)
	}
	if repository.updates[0].RequestHash != repository.updates[1].RequestHash {
		t.Fatal("identical update hashed differently")
	}
	if repository.updates[0].DisplayName != "Staff" || len(repository.updates[0].ShopIDs) != 2 || repository.updates[0].ShopIDs[0] != 101 {
		t.Fatalf("not normalized: %+v", repository.updates[0])
	}
	if first.Subject != "staff-1" {
		t.Fatalf("result=%+v", first)
	}
	changed := base
	changed.DisplayName = "Other"
	if _, err := directory.UpdateMember(context.Background(), changed); err != nil {
		t.Fatal(err)
	}
	if repository.updates[0].RequestHash == repository.updates[2].RequestHash {
		t.Fatal("changed display name reused the original request hash")
	}
}

func TestUpdateMemberRejectsAnchorWithoutExactlyOneShop(t *testing.T) {
	repository := &directoryStub{}
	command := UpdateMember{OperationID: "op", IdempotencyKey: "key", Subject: "anchor-1", MerchantID: 7, DisplayName: "Anchor", MemberType: model.MemberAnchor, ExpectedIdentityVersion: 1, ExpectedAccessVersion: 1, AssignmentKind: model.AssignmentAnchor, ShopIDs: []int64{101, 102}, RoleIDs: []int64{1}}
	if _, err := NewDirectory(repository).UpdateMember(context.Background(), command); !errors.Is(err, model.ErrInvalidAssignment) {
		t.Fatalf("expected invalid assignment, got %v", err)
	}
	if len(repository.updates) != 0 {
		t.Fatalf("invalid command reached repository %d times", len(repository.updates))
	}
}
