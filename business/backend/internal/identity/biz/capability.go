package biz

import (
	"context"
	"time"

	"github.com/lvtuopen-ai/kernel-go/accessidentity"
	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

type CapabilityIssuer interface {
	Sign(modulesession.Claims, time.Duration) (string, error)
}
type CapabilityRequest struct {
	Identity                                         accessidentity.Claims
	ModuleID, ModuleVersion, ContributionID, Surface string
}
type CapabilityGrant struct {
	Token                                                        string `json:"token"`
	ExpiresIn                                                    int64  `json:"expiresIn"`
	RegistryRevision, AuthorizationRevision, EntitlementRevision uint64
	Permissions                                                  []string
	DataScopes                                                   []modulesession.DataScope
}
type RuntimeContributions struct {
	Revision      uint64
	Items         []model.RegistryContribution
	Authorization model.Authorization
}
type RuntimeCatalog struct {
	Revision uint64
	Items    []model.RegistryModule
}
type CapabilityService struct {
	authorization *AuthorizationService
	repository    AuthorizationRepository
	issuer        CapabilityIssuer
	ttl           time.Duration
	maxStaleness  time.Duration
}

func NewCapability(authorization *AuthorizationService, repository AuthorizationRepository, issuer CapabilityIssuer, ttl, maxStaleness time.Duration) *CapabilityService {
	return &CapabilityService{authorization: authorization, repository: repository, issuer: issuer, ttl: ttl, maxStaleness: maxStaleness}
}

func (s *CapabilityService) Issue(ctx context.Context, principal model.PrincipalContext, request CapabilityRequest) (CapabilityGrant, error) {
	if s == nil || s.repository == nil || s.issuer == nil {
		return CapabilityGrant{}, model.ErrUnavailable
	}
	domain := authorizationDomain(principal)
	contribution, authorization, err := s.repository.CapabilitySnapshot(ctx, domain, principal, request.ModuleID, request.ModuleVersion, request.ContributionID, request.Surface, s.maxStaleness)
	if err != nil {
		return CapabilityGrant{}, err
	}
	if contribution.RegistryRevision == 0 || authorization.Revision == 0 || authorization.EntitlementRevision == 0 {
		return CapabilityGrant{}, model.ErrRegistryProjectionStale
	}
	if !authorizedFor(authorization, contribution.RequiredPermissions) {
		return CapabilityGrant{}, model.ErrAuthorizationDenied
	}
	routes := []modulesession.RouteScope{}
	for _, route := range contribution.AllowedRoutes {
		if authorizedFor(authorization, route.RequiredPermissions) {
			routes = append(routes, modulesession.RouteScope{Methods: route.Methods, Prefix: route.Prefix})
		}
	}
	if len(contribution.AllowedRoutes) > 0 && len(routes) == 0 {
		return CapabilityGrant{}, model.ErrAuthorizationDenied
	}
	permissions := filterModule(authorization.Permissions, request.ModuleID+".")
	scopes := filterScopes(authorization.DataScopes, request.ModuleID+".")
	claims := modulesession.Claims{Subject: request.Identity.Subject, Realm: request.Identity.Realm, PrincipalType: request.Identity.PrincipalType, SessionID: request.Identity.SessionID, ModuleID: request.ModuleID, ModuleVersion: request.ModuleVersion, Surface: request.Surface, ContributionID: request.ContributionID, AllowedRoutes: routes, Permissions: permissions, DataScopes: scopes, RegistryRevision: contribution.RegistryRevision, AuthorizationRevision: authorization.Revision, IdentityVersion: principal.Subject.Version, OrganizationVersion: principal.Organization.Version, EntitlementRevision: authorization.EntitlementRevision, ContextVersion: request.Identity.ContextVersion, OrganizationID: request.Identity.OrganizationID, MerchantID: request.Identity.MerchantID, ShopID: request.Identity.ShopID}
	if s.ttl <= 0 || s.ttl > 5*time.Minute {
		return CapabilityGrant{}, model.ErrUnavailable
	}
	token, err := s.issuer.Sign(claims, s.ttl)
	if err != nil {
		return CapabilityGrant{}, err
	}
	return CapabilityGrant{Token: token, ExpiresIn: int64(s.ttl.Seconds()), RegistryRevision: contribution.RegistryRevision, AuthorizationRevision: authorization.Revision, EntitlementRevision: authorization.EntitlementRevision, Permissions: permissions, DataScopes: scopes}, nil
}

func (s *CapabilityService) Contributions(ctx context.Context, principal model.PrincipalContext, surface string) (RuntimeContributions, error) {
	if s == nil || s.repository == nil || surface == "" {
		return RuntimeContributions{}, model.ErrUnavailable
	}
	revision, candidates, authorization, err := s.repository.RuntimeSnapshot(ctx, authorizationDomain(principal), principal, surface, s.maxStaleness)
	if err != nil {
		return RuntimeContributions{}, err
	}
	items := make([]model.RegistryContribution, 0, len(candidates))
	for _, candidate := range candidates {
		if authorizedFor(authorization, candidate.RequiredPermissions) {
			items = append(items, candidate)
		}
	}
	return RuntimeContributions{Revision: revision, Items: items, Authorization: authorization}, nil
}

// Empty requiredPermissions is an explicit Manifest contract for a
// guest-visible contribution or route. Authorization.Has intentionally keeps
// empty input false for protected IAM decisions, so visitor semantics stay
// local to Registry-owned runtime metadata.
func authorizedFor(authorization model.Authorization, required []string) bool {
	return len(required) == 0 || authorization.Has(required...)
}

func (s *CapabilityService) Me(ctx context.Context, principal model.PrincipalContext, surface string) (RuntimeContributions, error) {
	return s.Contributions(ctx, principal, surface)
}

func (s *CapabilityService) Catalog(ctx context.Context, principal model.PrincipalContext) (RuntimeCatalog, error) {
	if s == nil || s.repository == nil {
		return RuntimeCatalog{}, model.ErrUnavailable
	}
	revision, items, authorization, err := s.repository.CatalogSnapshot(ctx, authorizationDomain(principal), principal, s.maxStaleness)
	if err != nil {
		return RuntimeCatalog{}, err
	}
	if !authorization.Has("platform.registry.manage") {
		return RuntimeCatalog{}, model.ErrAuthorizationDenied
	}
	return RuntimeCatalog{Revision: revision, Items: items}, nil
}

func authorizationDomain(context model.PrincipalContext) model.AuthorizationDomain {
	domain := model.AuthorizationDomain{Type: model.AuthorizationPlatform, ID: context.Organization.ID, OrganizationID: context.Organization.ID}
	if (context.Subject.PrincipalType == principal.TypeCustomer || context.Subject.PrincipalType == principal.TypeGuest) && context.Selected.MerchantID > 0 {
		domain = model.AuthorizationDomain{Type: model.AuthorizationMerchant, ID: context.Selected.MerchantID}
	} else if context.Member.MerchantID > 0 {
		domain = model.AuthorizationDomain{Type: model.AuthorizationMerchant, ID: context.Member.MerchantID, OrganizationID: context.Organization.ID}
	}
	return domain
}
func filterModule(values []string, prefix string) []string {
	out := []string{}
	for _, v := range values {
		if len(v) >= len(prefix) && v[:len(prefix)] == prefix {
			out = append(out, v)
		}
	}
	return out
}
func filterScopes(values []modulesession.DataScope, prefix string) []modulesession.DataScope {
	out := []modulesession.DataScope{}
	for _, v := range values {
		if len(v.Resource) >= len(prefix) && v.Resource[:len(prefix)] == prefix {
			out = append(out, v)
		}
	}
	return out
}
