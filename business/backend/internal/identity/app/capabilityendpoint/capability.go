package capabilityendpoint

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/lvtuopen-ai/kernel-go/accessidentity"
	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/middleware"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type Endpoint struct {
	directory     *biz.Directory
	capability    *biz.CapabilityService
	verifier      *accessidentity.Verifier
	health        *biz.Health
	registryReady func(context.Context) error
}

func New(directory *biz.Directory, capability *biz.CapabilityService, verifier *accessidentity.Verifier, health *biz.Health, registryReady func(context.Context) error) (*Endpoint, error) {
	if directory == nil || capability == nil || verifier == nil || health == nil || registryReady == nil {
		return nil, model.ErrUnavailable
	}
	return &Endpoint{directory: directory, capability: capability, verifier: verifier, health: health, registryReady: registryReady}, nil
}
func Register(root *ghttp.RouterGroup, e *Endpoint) {
	root.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(web.ResponseHandler)
		group.Bind(&healthEndpoint{endpoint: e})
	})
	root.Group("/runtime/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(web.ResponseHandler)
		group.Middleware(middleware.RequestMetadata)
		group.Bind(e)
	})
}

type healthEndpoint struct{ endpoint *Endpoint }
type HealthReq struct {
	g.Meta `path:"/healthz" method:"get" tags:"Identity-health"`
}
type HealthRes struct {
	Status string `json:"status"`
}

func (e *healthEndpoint) Health(ctx context.Context, _ *HealthReq) (*HealthRes, error) {
	return &HealthRes{Status: "UP"}, nil
}

type ReadyReq struct {
	g.Meta `path:"/readyz" method:"get" tags:"Identity-health"`
}
type ReadyRes struct {
	Status string `json:"status"`
}

func (e *healthEndpoint) Ready(ctx context.Context, _ *ReadyReq) (*ReadyRes, error) {
	if _, err := e.endpoint.health.Check(ctx); err != nil {
		return nil, web.Failure(err)
	}
	if err := e.endpoint.registryReady(ctx); err != nil {
		return nil, web.Failure(err)
	}
	return &ReadyRes{Status: "READY"}, nil
}

type IssueReq struct {
	g.Meta         `path:"/module-sessions" method:"post" tags:"Identity-runtime"`
	ModuleID       string `json:"moduleId"`
	ModuleVersion  string `json:"moduleVersion"`
	ContributionID string `json:"contributionId"`
	Surface        string `json:"surface"`
}
type IssueRes struct {
	Token                 string                    `json:"token"`
	ExpiresIn             int64                     `json:"expiresIn"`
	Tenant                Tenant                    `json:"tenant"`
	RegistryRevision      uint64                    `json:"registryRevision"`
	AuthorizationRevision uint64                    `json:"authorizationRevision"`
	EntitlementRevision   uint64                    `json:"entitlementRevision"`
	Permissions           []string                  `json:"permissions"`
	DataScopes            []modulesession.DataScope `json:"dataScopes"`
}
type Tenant struct {
	MerchantID int64 `json:"merchantId"`
	ShopID     int64 `json:"shopId"`
}

func (e *Endpoint) Issue(ctx context.Context, r *IssueReq) (*IssueRes, error) {
	claims, principal, err := e.principal(ctx, r.Surface)
	if err != nil {
		return nil, err
	}
	grant, err := e.capability.Issue(ctx, principal, biz.CapabilityRequest{Identity: claims, ModuleID: r.ModuleID, ModuleVersion: r.ModuleVersion, ContributionID: r.ContributionID, Surface: r.Surface})
	if err != nil {
		return nil, web.Failure(err)
	}
	return issueResponse(grant, claims), nil
}

func issueResponse(grant biz.CapabilityGrant, claims accessidentity.Claims) *IssueRes {
	return &IssueRes{Token: grant.Token, ExpiresIn: grant.ExpiresIn, Tenant: Tenant{MerchantID: claims.MerchantID, ShopID: claims.ShopID}, RegistryRevision: grant.RegistryRevision, AuthorizationRevision: grant.AuthorizationRevision, EntitlementRevision: grant.EntitlementRevision, Permissions: grant.Permissions, DataScopes: grant.DataScopes}
}

type ContributionsReq struct {
	g.Meta  `path:"/contributions" method:"get" tags:"Identity-runtime"`
	Surface string `json:"surface" in:"query"`
}
type RuntimeContribution struct {
	ModuleID      string         `json:"moduleId"`
	ModuleVersion string         `json:"moduleVersion"`
	Contribution  map[string]any `json:"contribution"`
}
type ContributionsRes struct {
	Revision uint64                `json:"revision"`
	Items    []RuntimeContribution `json:"items"`
}

func (e *Endpoint) Contributions(ctx context.Context, r *ContributionsReq) (*ContributionsRes, error) {
	_, principal, err := e.principal(ctx, r.Surface)
	if err != nil {
		return nil, err
	}
	snapshot, err := e.capability.Contributions(ctx, principal, r.Surface)
	if err != nil {
		return nil, web.Failure(err)
	}
	items := make([]RuntimeContribution, 0, len(snapshot.Items))
	for _, candidate := range snapshot.Items {
		raw, _ := json.Marshal(candidate)
		var contribution map[string]any
		if json.Unmarshal(raw, &contribution) != nil {
			return nil, web.Failure(model.ErrRegistryProjectionStale)
		}
		delete(contribution, "registryRevision")
		delete(contribution, "moduleId")
		delete(contribution, "moduleVersion")
		delete(contribution, "surface")
		delete(contribution, "contributionId")
		contribution["id"] = candidate.ContributionID
		contribution["surface"] = candidate.Surface
		items = append(items, RuntimeContribution{ModuleID: candidate.ModuleID, ModuleVersion: candidate.ModuleVersion, Contribution: contribution})
	}
	return &ContributionsRes{Revision: snapshot.Revision, Items: items}, nil
}

type MeReq struct {
	g.Meta  `path:"/iam/me" method:"get" tags:"Identity-runtime"`
	Surface string `json:"surface" in:"query"`
}
type MeRes struct {
	RegistryRevision      uint64 `json:"registryRevision"`
	AuthorizationRevision uint64 `json:"authorizationRevision"`
	EntitlementRevision   uint64 `json:"entitlementRevision"`
	IdentityVersion       uint64 `json:"identityVersion"`
	OrganizationVersion   uint64 `json:"organizationVersion"`
	Permissions           any    `json:"permissions"`
	DataScopes            any    `json:"dataScopes"`
}

func (e *Endpoint) Me(ctx context.Context, r *MeReq) (*MeRes, error) {
	surface := r.Surface
	if surface == "" {
		surface = g.RequestFromCtx(ctx).Header.Get("X-Liveshop-Surface")
	}
	_, principal, err := e.principal(ctx, surface)
	if err != nil {
		return nil, err
	}
	snapshot, err := e.capability.Me(ctx, principal, surface)
	if err != nil {
		return nil, web.Failure(err)
	}
	authorization := snapshot.Authorization
	return &MeRes{RegistryRevision: snapshot.Revision, AuthorizationRevision: authorization.Revision, EntitlementRevision: authorization.EntitlementRevision, IdentityVersion: authorization.IdentityVersion, OrganizationVersion: authorization.OrganizationVersion, Permissions: authorization.Permissions, DataScopes: authorization.DataScopes}, nil
}

type CatalogReq struct {
	g.Meta `path:"/module-catalog" method:"get" tags:"Identity-runtime"`
}
type ModuleCatalog struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	ActiveVersion string            `json:"activeVersion"`
	Releases      []json.RawMessage `json:"releases"`
}
type CatalogRes struct {
	Revision uint64          `json:"revision"`
	Items    []ModuleCatalog `json:"items"`
}

func (e *Endpoint) Catalog(ctx context.Context, _ *CatalogReq) (*CatalogRes, error) {
	_, principal, err := e.principal(ctx, "admin")
	if err != nil {
		return nil, err
	}
	snapshot, err := e.capability.Catalog(ctx, principal)
	if err != nil {
		return nil, web.Failure(err)
	}
	items := make([]ModuleCatalog, 0, len(snapshot.Items))
	for _, module := range snapshot.Items {
		items = append(items, ModuleCatalog{ID: module.ID, Name: module.Name, ActiveVersion: module.Version, Releases: []json.RawMessage{module.Release}})
	}
	return &CatalogRes{Revision: snapshot.Revision, Items: items}, nil
}

func (e *Endpoint) principal(ctx context.Context, surface string) (accessidentity.Claims, model.PrincipalContext, error) {
	request := g.RequestFromCtx(ctx)
	token, ok := accessidentity.Bearer(request.Header.Get("Authorization"))
	if !ok {
		return accessidentity.Claims{}, model.PrincipalContext{}, unauthorized()
	}
	claims, err := e.verifier.Verify(token)
	if err != nil || surface == "" || !claims.Realm.AllowsSurface(surface) {
		return accessidentity.Claims{}, model.PrincipalContext{}, unauthorized()
	}
	selected := model.SelectedContext{OrganizationID: claims.OrganizationID, ShopContext: model.ShopContext{MerchantID: claims.MerchantID, ShopID: claims.ShopID}}
	principal, err := e.directory.ResolveAuthenticatedPrincipalContext(ctx, claims.SessionID, claims.Subject, selected, claims.ContextVersion)
	if err != nil {
		return accessidentity.Claims{}, model.PrincipalContext{}, web.Failure(err)
	}
	return claims, principal, nil
}
func unauthorized() error {
	return &web.HTTPError{Status: http.StatusUnauthorized, Cause: errors.New("valid access identity is required")}
}
