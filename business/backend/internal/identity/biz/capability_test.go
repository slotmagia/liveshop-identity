package biz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lvtuopen-ai/kernel-go/accessidentity"
	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

type capabilityRepo struct {
	contribution  model.RegistryContribution
	authorization model.Authorization
	items         []model.RegistryContribution
}

func (*capabilityRepo) Permissions(context.Context, model.AuthorizationDomain) ([]model.Permission, error) {
	return nil, nil
}
func (*capabilityRepo) Roles(context.Context, model.AuthorizationDomain) ([]model.Role, error) {
	return nil, nil
}
func (*capabilityRepo) PutRole(context.Context, model.AuthorizationDomain, model.Role, int64) (model.Role, error) {
	return model.Role{}, nil
}
func (*capabilityRepo) SetRolePolicy(context.Context, model.AuthorizationDomain, int64, int64, model.RolePolicy) (model.Role, error) {
	return model.Role{}, nil
}
func (*capabilityRepo) ReplaceSubjectGrants(context.Context, model.AuthorizationDomain, string, []int64, string, uint64) error {
	return nil
}
func (r *capabilityRepo) Effective(context.Context, model.AuthorizationDomain, model.PrincipalContext) (model.Authorization, error) {
	return r.authorization, nil
}
func (r *capabilityRepo) CapabilitySnapshot(context.Context, model.AuthorizationDomain, model.PrincipalContext, string, string, string, string, time.Duration) (model.RegistryContribution, model.Authorization, error) {
	return r.contribution, r.authorization, nil
}
func (r *capabilityRepo) RuntimeSnapshot(context.Context, model.AuthorizationDomain, model.PrincipalContext, string, time.Duration) (uint64, []model.RegistryContribution, model.Authorization, error) {
	return 9, r.items, r.authorization, nil
}
func (r *capabilityRepo) CatalogSnapshot(context.Context, model.AuthorizationDomain, model.PrincipalContext, time.Duration) (uint64, []model.RegistryModule, model.Authorization, error) {
	return 9, nil, r.authorization, nil
}

type captureIssuer struct {
	claims modulesession.Claims
	ttl    time.Duration
}

func (i *captureIssuer) Sign(c modulesession.Claims, ttl time.Duration) (string, error) {
	i.claims = c
	i.ttl = ttl
	return "signed", nil
}

func testPrincipal() model.PrincipalContext {
	return model.PrincipalContext{Subject: model.Subject{ID: "subject", Realm: principal.RealmPlatform, PrincipalType: principal.TypePlatformOperator, Status: model.StatusActive, Version: 7}, Organization: model.Organization{ID: 1, Type: model.OrganizationPlatform, Status: model.StatusActive, Version: 8}, Member: model.WorkforceMember{OrganizationID: 1, Subject: "subject", Type: model.MemberOperator, Status: model.MemberActive, AccessVersion: 3}}
}
func testIdentity() accessidentity.Claims {
	return accessidentity.Claims{Subject: "subject", Realm: principal.RealmPlatform, PrincipalType: principal.TypePlatformOperator, SessionID: "session", OrganizationID: 1, ContextVersion: 4}
}

func TestAuthorizationDomainUsesSelectedShopMerchantForCustomer(t *testing.T) {
	context := model.PrincipalContext{
		Subject:  model.Subject{ID: "customer", Realm: principal.RealmCustomer, PrincipalType: principal.TypeCustomer, Status: model.StatusActive, Version: 1},
		Selected: model.SelectedContext{ShopContext: model.ShopContext{MerchantID: 2001, ShopID: 3001}},
	}
	domain := authorizationDomain(context)
	if domain.Type != model.AuthorizationMerchant || domain.ID != 2001 || domain.OrganizationID != 0 {
		t.Fatalf("customer authorization domain escaped selected merchant: %#v", domain)
	}
}

func TestCapabilityIssueCarriesRequiredRevisionsAndConfiguredTTL(t *testing.T) {
	repo := &capabilityRepo{contribution: model.RegistryContribution{RegistryRevision: 9, ModuleID: "identity", ModuleVersion: "1.0.0", ContributionID: "page", Surface: "admin", RequiredPermissions: []string{"identity.directory.read"}, AllowedRoutes: []model.RegistryAllowedRoute{{Methods: []string{"GET"}, Prefix: "/admin/identity", RequiredPermissions: []string{"identity.directory.read"}}}}, authorization: model.Authorization{Revision: 10, EntitlementRevision: 11, Permissions: []string{"identity.directory.read"}, IdentityVersion: 7, OrganizationVersion: 8}}
	issuer := &captureIssuer{}
	service := NewCapability(NewAuthorization(repo), repo, issuer, 2*time.Minute, time.Minute)
	grant, err := service.Issue(context.Background(), testPrincipal(), CapabilityRequest{Identity: testIdentity(), ModuleID: "identity", ModuleVersion: "1.0.0", ContributionID: "page", Surface: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if grant.Token != "signed" || issuer.ttl != 2*time.Minute || issuer.claims.RegistryRevision != 9 || issuer.claims.AuthorizationRevision != 10 || issuer.claims.EntitlementRevision != 11 {
		t.Fatalf("unexpected grant: %#v %#v", grant, issuer.claims)
	}
}

func TestCapabilityIssueFailsClosedWithoutRequiredPermission(t *testing.T) {
	repo := &capabilityRepo{contribution: model.RegistryContribution{RegistryRevision: 9, RequiredPermissions: []string{"identity.authorization.manage"}}, authorization: model.Authorization{Revision: 10, EntitlementRevision: 11, Permissions: []string{"identity.directory.read"}}}
	_, err := NewCapability(NewAuthorization(repo), repo, &captureIssuer{}, time.Minute, time.Minute).Issue(context.Background(), testPrincipal(), CapabilityRequest{Identity: testIdentity(), ModuleID: "identity", Surface: "admin"})
	if !errors.Is(err, model.ErrAuthorizationDenied) {
		t.Fatalf("expected fail closed, got %v", err)
	}
}

func TestCapabilityIssueAllowsStaticContributionWithoutRequestScope(t *testing.T) {
	repo := &capabilityRepo{contribution: model.RegistryContribution{
		RegistryRevision: 9, ModuleID: "identity", ModuleVersion: "1.0.0", ContributionID: "placeholder", Surface: "admin",
		RequiredPermissions: []string{"identity.placeholder.read"}, AllowedRoutes: []model.RegistryAllowedRoute{},
	}, authorization: model.Authorization{Revision: 10, EntitlementRevision: 11, Permissions: []string{"identity.placeholder.read"}, IdentityVersion: 7, OrganizationVersion: 8}}
	issuer := &captureIssuer{}
	grant, err := NewCapability(NewAuthorization(repo), repo, issuer, time.Minute, time.Minute).Issue(context.Background(), testPrincipal(), CapabilityRequest{Identity: testIdentity(), ModuleID: "identity", ModuleVersion: "1.0.0", ContributionID: "placeholder", Surface: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if grant.Token != "signed" || len(issuer.claims.AllowedRoutes) != 0 {
		t.Fatalf("static contribution received unexpected request scope: %#v", issuer.claims.AllowedRoutes)
	}
}

func TestRuntimeContributionsExcludeUnauthorizedArtifacts(t *testing.T) {
	repo := &capabilityRepo{authorization: model.Authorization{Revision: 1, EntitlementRevision: 2, Permissions: []string{"identity.directory.read"}}, items: []model.RegistryContribution{{ContributionID: "guest", RequiredPermissions: []string{}}, {ContributionID: "allowed", RequiredPermissions: []string{"identity.directory.read"}}, {ContributionID: "secret", Artifact: model.RegistryArtifact{Entry: "https://secret"}, RequiredPermissions: []string{"identity.authorization.manage"}}}}
	snapshot, err := NewCapability(NewAuthorization(repo), repo, &captureIssuer{}, time.Minute, time.Minute).Contributions(context.Background(), testPrincipal(), "admin")
	if err != nil || len(snapshot.Items) != 2 || snapshot.Items[0].ContributionID != "guest" || snapshot.Items[1].ContributionID != "allowed" {
		t.Fatalf("unauthorized contribution leaked: %#v %v", snapshot, err)
	}
}

func TestCapabilityIssueAllowsGuestVisibleRouteWithoutRolePermission(t *testing.T) {
	repo := &capabilityRepo{contribution: model.RegistryContribution{
		RegistryRevision: 9, ModuleID: "catalog", ModuleVersion: "1.0.0", ContributionID: "catalog.shop", Surface: "shop",
		RequiredPermissions: []string{}, AllowedRoutes: []model.RegistryAllowedRoute{{Methods: []string{"GET"}, Prefix: "/shop/catalog", RequiredPermissions: []string{}}},
	}, authorization: model.Authorization{Revision: 10, EntitlementRevision: 11, Permissions: []string{}, IdentityVersion: 1}}
	issuer := &captureIssuer{}
	_, err := NewCapability(NewAuthorization(repo), repo, issuer, time.Minute, time.Minute).Issue(context.Background(), testPrincipal(), CapabilityRequest{Identity: testIdentity(), ModuleID: "catalog", ModuleVersion: "1.0.0", ContributionID: "catalog.shop", Surface: "shop"})
	if err != nil {
		t.Fatal(err)
	}
	if len(issuer.claims.AllowedRoutes) != 1 || len(issuer.claims.Permissions) != 0 {
		t.Fatalf("unexpected guest grant: %#v", issuer.claims)
	}
}
