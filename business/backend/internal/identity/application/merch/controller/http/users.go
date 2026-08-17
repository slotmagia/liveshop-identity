package http

import (
	"context"
	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/users"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type UserController struct{ s service.Merch }

func NewUsers(s service.Merch) *UserController { return &UserController{s: s} }

func (c *UserController) Options(ctx context.Context, _ *api.OptionsReq) (*api.OptionsRes, error) {
	value, err := c.s.MemberOptions(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := &api.OptionsRes{Shops: make([]api.Shop, 0, len(value.Shops)), Roles: make([]api.Role, 0, len(value.Roles)), Units: make([]api.Unit, 0, len(value.Units))}
	for _, shop := range value.Shops {
		out.Shops = append(out.Shops, api.Shop{ID: shop.ID, MerchantID: shop.MerchantID, Name: shop.Name, Code: shop.Code, Status: shop.Status, Version: shop.Version})
	}
	for _, role := range value.Roles {
		out.Roles = append(out.Roles, api.Role{ID: role.ID, Code: role.Code, Name: role.Name, Status: role.Status, SystemRole: role.SystemRole, Version: role.Version})
	}
	for _, unit := range value.Units {
		out.Units = append(out.Units, api.Unit{ID: unit.ID, ParentID: unit.ParentID, Name: unit.Name, Status: unit.Status, Version: unit.Version})
	}
	return out, nil
}

func (c *UserController) List(ctx context.Context, r *api.ListReq) (*api.ListRes, error) {
	page, err := c.s.Members(ctx, appmodel.MemberQuery{Keyword: r.Keyword, MemberType: r.MemberType, Status: r.Status, ShopID: r.ShopID, Page: r.Page, PageSize: r.PageSize})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := &api.ListRes{Items: make([]api.Member, 0, len(page.Items)), Page: page.Page, PageSize: page.PageSize, Total: page.Total}
	for _, item := range page.Items {
		out.Items = append(out.Items, wireMember(item))
	}
	return out, nil
}

func (c *UserController) Detail(ctx context.Context, r *api.DetailReq) (*api.DetailRes, error) {
	value, err := c.s.Member(ctx, r.Subject)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.DetailRes(wireMember(value))
	return &out, nil
}

func (c *UserController) Update(ctx context.Context, r *api.UpdateReq) (*api.UpdateRes, error) {
	value, err := c.s.UpdateMember(ctx, appmodel.UpdateMember{Subject: r.Subject, IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID, DisplayName: r.DisplayName, MemberType: r.MemberType, ExpectedIdentityVersion: r.ExpectedIdentityVersion, ExpectedAccessVersion: r.ExpectedAccessVersion, UnitIDs: r.UnitIDs, ShopIDs: r.ShopIDs, RoleIDs: r.RoleIDs})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.UpdateRes{MemberID: value.MemberID, Subject: value.Subject, Status: value.Status, OperationID: value.OperationID, IdentityVersion: value.IdentityVersion, AccessVersion: value.AccessVersion}, nil
}

func (c *UserController) ResetCredential(ctx context.Context, r *api.ResetCredentialReq) (*api.ResetCredentialRes, error) {
	v, err := c.s.ResetCredential(ctx, appmodel.ResetCredential{Subject: r.Subject, CredentialID: r.CredentialID, IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID, Password: r.Password, ExpectedCredentialVersion: r.ExpectedCredentialVersion})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ResetCredentialRes{ID: v.ID, Version: v.Version, Kind: v.Kind, Identifier: v.Identifier, Status: v.Status}
	return &out, nil
}
func (c *UserController) Enable(ctx context.Context, r *api.EnableReq) (*api.EnableRes, error) {
	v, err := c.s.ChangeMemberStatus(ctx, appmodel.ChangeMemberStatus{Subject: r.Subject, IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID, Status: "ACTIVE", ExpectedIdentityVersion: r.ExpectedIdentityVersion, ExpectedAccessVersion: r.ExpectedAccessVersion})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.EnableRes{Subject: v.Subject, Status: v.Status, IdentityVersion: v.IdentityVersion, AccessVersion: v.AccessVersion}, nil
}
func (c *UserController) Disable(ctx context.Context, r *api.DisableReq) (*api.DisableRes, error) {
	v, err := c.s.ChangeMemberStatus(ctx, appmodel.ChangeMemberStatus{Subject: r.Subject, IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID, Status: "DISABLED", ExpectedIdentityVersion: r.ExpectedIdentityVersion, ExpectedAccessVersion: r.ExpectedAccessVersion})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.DisableRes{Subject: v.Subject, Status: v.Status, IdentityVersion: v.IdentityVersion, AccessVersion: v.AccessVersion}, nil
}

type SessionController struct{ s service.Merch }

func NewSessions(s service.Merch) *SessionController { return &SessionController{s: s} }
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

func wireMember(value appmodel.MemberRecord) api.Member {
	return api.Member{
		ID: value.ID, Subject: value.Subject, DisplayName: value.DisplayName, Type: value.Type, Status: value.Status,
		MemberStatus: value.MemberStatus, PrincipalType: value.PrincipalType, AccessVersion: value.AccessVersion, SubjectVersion: value.SubjectVersion,
		Credential: api.Credential{ID: value.Credential.ID, Version: value.Credential.Version, Kind: value.Credential.Kind, Identifier: value.Credential.Identifier, Status: value.Credential.Status},
		RoleIDs:    value.RoleIDs, UnitIDs: value.UnitIDs, ShopIDs: value.ShopIDs, ActiveSessions: value.ActiveSessions,
	}
}
