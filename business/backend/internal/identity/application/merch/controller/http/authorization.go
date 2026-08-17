package http

import (
	"context"
	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/authorization"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type AuthorizationController struct{ s service.Merch }

func NewAuthorization(s service.Merch) *AuthorizationController {
	return &AuthorizationController{s: s}
}
func (c *AuthorizationController) Permissions(ctx context.Context, _ *api.PermissionsReq) (*api.PermissionsRes, error) {
	v, e := c.s.Permissions(ctx)
	if e != nil {
		return nil, web.Failure(e)
	}
	out := api.PermissionsRes{}
	for _, p := range v {
		out = append(out, api.Permission{ModuleID: p.ModuleID, Code: p.Code, Name: p.Name, Resource: p.Resource, Action: p.Action, Description: p.Description, RegistryRevision: p.RegistryRevision})
	}
	return &out, nil
}
func (c *AuthorizationController) Roles(ctx context.Context, _ *api.RolesReq) (*api.RolesRes, error) {
	v, e := c.s.Roles(ctx)
	if e != nil {
		return nil, web.Failure(e)
	}
	out := api.RolesRes{}
	for _, r := range v {
		out = append(out, mrole(r))
	}
	return &out, nil
}
func (c *AuthorizationController) PutRole(ctx context.Context, r *api.PutRoleReq) (*api.PutRoleRes, error) {
	v, e := c.s.PutRole(ctx, appmodel.PutRole{RoleID: r.RoleID, ExpectedVersion: r.ExpectedVersion, Code: r.Code, Name: r.Name, Status: r.Status})
	if e != nil {
		return nil, web.Failure(e)
	}
	out := api.PutRoleRes(mrole(v))
	return &out, nil
}
func (c *AuthorizationController) PutPolicy(ctx context.Context, r *api.PutPolicyReq) (*api.PutPolicyRes, error) {
	scopes := []appmodel.ScopeRule{}
	for _, s := range r.Scopes {
		scopes = append(scopes, appmodel.ScopeRule{Resource: s.Resource, Type: s.Type, ReferenceIDs: s.ReferenceIDs})
	}
	v, e := c.s.PutRolePolicy(ctx, appmodel.PutRolePolicy{RoleID: r.RoleID, ExpectedVersion: r.ExpectedVersion, Permissions: r.Permissions, Scopes: scopes})
	if e != nil {
		return nil, web.Failure(e)
	}
	out := api.PutPolicyRes(mrole(v))
	return &out, nil
}
func (c *AuthorizationController) PutGrants(ctx context.Context, r *api.PutGrantsReq) (*api.PutGrantsRes, error) {
	if e := c.s.PutSubjectGrants(ctx, appmodel.PutSubjectGrants{Subject: r.Subject, RoleIDs: r.RoleIDs, OperationID: r.OperationID, AccessVersion: r.AccessVersion}); e != nil {
		return nil, web.Failure(e)
	}
	return &api.PutGrantsRes{}, nil
}
func mrole(r appmodel.Role) api.Role {
	return api.Role{ID: r.ID, Code: r.Code, Name: r.Name, Status: r.Status, SystemRole: r.SystemRole, Version: r.Version}
}
