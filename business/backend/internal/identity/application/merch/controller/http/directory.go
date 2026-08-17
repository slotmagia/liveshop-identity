package http

import (
	"context"
	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/directory"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type DirectoryController struct{ service service.Merch }

func NewDirectory(s service.Merch) *DirectoryController { return &DirectoryController{service: s} }
func (c *DirectoryController) Read(ctx context.Context, _ *api.ReadReq) (*api.ReadRes, error) {
	v, e := c.service.Directory(ctx)
	if e != nil {
		return nil, web.Failure(e)
	}
	return &api.ReadRes{Organization: v.Organization, Units: v.Units, Members: v.Members, Shops: v.Shops}, nil
}

type UnitController struct{ service service.Merch }

func NewUnit(s service.Merch) *UnitController { return &UnitController{service: s} }
func (c *UnitController) Create(ctx context.Context, r *api.CreateUnitReq) (*api.MutationRes, error) {
	v, e := c.service.CreateUnit(ctx, appmodel.CreateUnit{IdempotencyKey: r.IdempotencyKey, UnitID: r.UnitID, ParentID: r.ParentID, Name: r.Name, ExpectedVersion: r.ExpectedVersion})
	if e != nil {
		return nil, web.Failure(e)
	}
	return mutation(v), nil
}

type MemberController struct{ service service.Merch }

func NewMember(s service.Merch) *MemberController { return &MemberController{service: s} }
func (c *MemberController) Create(ctx context.Context, r *api.CreateMemberReq) (*api.MutationRes, error) {
	v, e := c.service.CreateMember(ctx, appmodel.CreateMember{IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID, DisplayName: r.DisplayName, MemberType: r.MemberType, Username: r.Username, Password: r.Password, UnitIDs: r.UnitIDs, ShopIDs: r.ShopIDs, RoleIDs: r.RoleIDs})
	if e != nil {
		return nil, web.Failure(e)
	}
	return mutation(v), nil
}
func (c *MemberController) ReplaceAccess(ctx context.Context, r *api.ReplaceAccessReq) (*api.MutationRes, error) {
	v, e := c.service.ReplaceAccess(ctx, appmodel.ReplaceAccess{IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID, MemberID: r.MemberID, MemberType: r.MemberType, ExpectedAccessVersion: r.ExpectedAccessVersion, UnitIDs: r.UnitIDs, ShopIDs: r.ShopIDs})
	if e != nil {
		return nil, web.Failure(e)
	}
	return mutation(v), nil
}
func mutation(v appmodel.Mutation) *api.MutationRes {
	return &api.MutationRes{MemberID: v.MemberID, Subject: v.Subject, Status: v.Status, OperationID: v.OperationID, Version: v.Version}
}
