package http

import (
	"context"
	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/api/http/v1/users"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type UserController struct{ s service.Admin }

func NewUsers(s service.Admin) *UserController { return &UserController{s: s} }
func (c *UserController) List(ctx context.Context, _ *api.ListReq) (*api.ListRes, error) {
	values, err := c.s.Users(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRes{}
	for _, v := range values {
		out = append(out, wireUser(v))
	}
	return &out, nil
}
func (c *UserController) Detail(ctx context.Context, r *api.DetailReq) (*api.DetailRes, error) {
	v, err := c.s.User(ctx, r.Subject)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.DetailRes(wireUser(v))
	return &out, nil
}
func (c *UserController) Create(ctx context.Context, r *api.CreateReq) (*api.CreateRes, error) {
	v, err := c.s.CreateOperator(ctx, appmodel.CreateOperator{IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID, DisplayName: r.DisplayName, Username: r.Username, Password: r.Password, RoleIDs: r.RoleIDs})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.CreateRes(wireUser(v))
	return &out, nil
}
func (c *UserController) Enable(ctx context.Context, r *api.EnableReq) (*api.EnableRes, error) {
	v, err := c.s.ChangeUserStatus(ctx, appmodel.ChangeUserStatus{Subject: r.Subject, IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID, Status: "ACTIVE", ExpectedIdentityVersion: r.ExpectedIdentityVersion, ExpectedAccessVersion: r.ExpectedAccessVersion})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.EnableRes(wireUser(v))
	return &out, nil
}
func (c *UserController) Disable(ctx context.Context, r *api.DisableReq) (*api.DisableRes, error) {
	v, err := c.s.ChangeUserStatus(ctx, appmodel.ChangeUserStatus{Subject: r.Subject, IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID, Status: "DISABLED", ExpectedIdentityVersion: r.ExpectedIdentityVersion, ExpectedAccessVersion: r.ExpectedAccessVersion})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.DisableRes(wireUser(v))
	return &out, nil
}
func (c *UserController) ResetCredential(ctx context.Context, r *api.ResetCredentialReq) (*api.ResetCredentialRes, error) {
	v, err := c.s.ResetCredential(ctx, appmodel.ResetCredential{Subject: r.Subject, CredentialID: r.CredentialID, IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID, Password: r.Password, ExpectedCredentialVersion: r.ExpectedCredentialVersion})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ResetCredentialRes(wireCredential(v))
	return &out, nil
}

type SessionController struct{ s service.Admin }

func NewSessions(s service.Admin) *SessionController { return &SessionController{s: s} }
func (c *SessionController) Sessions(ctx context.Context, r *api.SessionsReq) (*api.SessionsRes, error) {
	values, err := c.s.Sessions(ctx, r.Subject)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.SessionsRes{}
	for _, v := range values {
		out = append(out, api.Session(v))
	}
	return &out, nil
}
func (c *SessionController) RevokeSessions(ctx context.Context, r *api.RevokeSessionsReq) (*api.MutationRes, error) {
	if err := c.s.RevokeSessions(ctx, appmodel.RevokeSessions{Subject: r.Subject, IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID}); err != nil {
		return nil, web.Failure(err)
	}
	return &api.MutationRes{}, nil
}
func (c *SessionController) RevokeSession(ctx context.Context, r *api.RevokeSessionReq) (*api.MutationRes, error) {
	if err := c.s.RevokeSessions(ctx, appmodel.RevokeSessions{Subject: r.Subject, SessionID: r.SessionID, IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID}); err != nil {
		return nil, web.Failure(err)
	}
	return &api.MutationRes{}, nil
}
func wireCredential(v appmodel.ManagedCredential) api.Credential {
	return api.Credential{ID: v.ID, Version: v.Version, Kind: v.Kind, Identifier: v.Identifier, Status: v.Status}
}
func wireUser(v appmodel.ManagedUser) api.User {
	return api.User{Subject: v.Subject, DisplayName: v.DisplayName, Realm: v.Realm, PrincipalType: v.PrincipalType, SubjectStatus: v.SubjectStatus, SubjectVersion: v.SubjectVersion, MemberID: v.MemberID, OrganizationID: v.OrganizationID, MemberType: v.MemberType, MemberStatus: v.MemberStatus, AccessVersion: v.AccessVersion, Credential: wireCredential(v.Credential), RoleIDs: v.RoleIDs, ActiveSessions: v.ActiveSessions}
}
