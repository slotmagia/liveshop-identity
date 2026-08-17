package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/account"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type AccountController struct{ service service.Merch }

func NewAccount(s service.Merch) *AccountController { return &AccountController{service: s} }

func (c *AccountController) Get(ctx context.Context, _ *api.GetReq) (*api.GetRes, error) {
	value, err := c.service.Account(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	res := wireAccount(value)
	return &res, nil
}

func (c *AccountController) Sessions(ctx context.Context, r *api.SessionsReq) (*api.SessionsRes, error) {
	page, err := c.service.AccountSessions(ctx, appmodel.AccountSessionQuery{Status: r.Status, Page: r.Page, PageSize: r.PageSize})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := &api.SessionsRes{Items: make([]api.Session, 0, len(page.Items)), Page: page.Page, PageSize: page.PageSize, Total: page.Total}
	for _, item := range page.Items {
		out.Items = append(out.Items, api.Session{
			ID: item.ID, DeviceName: item.DeviceName, IPAddress: item.IPAddress, UserAgent: item.UserAgent,
			Status: item.Status, CreatedAt: item.CreatedAt, LastRefreshedAt: item.LastRefreshedAt, ExpiresAt: item.ExpiresAt, Current: item.Current,
		})
	}
	return out, nil
}

func (c *AccountController) RevokeSession(ctx context.Context, r *api.RevokeSessionReq) (*api.RevokeSessionRes, error) {
	value, err := c.service.RevokeAccountSession(ctx, appmodel.RevokeAccountSession{SessionID: r.SessionID, IdempotencyKey: r.IdempotencyKey, OperationID: r.OperationID})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.RevokeSessionRes{CurrentRevoked: value.CurrentRevoked}, nil
}

func (c *AccountController) Security(ctx context.Context, _ *api.SecurityGetReq) (*api.SecurityGetRes, error) {
	value, err := c.service.AccountSecurity(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.SecurityGetRes{
		Subject: value.Subject, DisplayName: value.DisplayName, Account: value.Account,
		PrincipalType: value.PrincipalType, Owner: value.Owner, Status: value.Status, ActiveSessions: value.ActiveSessions,
		Credential: api.Credential{ID: value.Credential.ID, Version: value.Credential.Version, Kind: value.Credential.Kind, Identifier: value.Credential.Identifier, Status: value.Credential.Status},
	}, nil
}

func (c *AccountController) UpdateCredentials(ctx context.Context, request *api.CredentialsUpdateReq) (*api.CredentialsUpdateRes, error) {
	value, err := c.service.ChangeOwnCredential(ctx, appmodel.ChangeOwnCredential{
		CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, OldPassword: request.OldPassword, Password: request.Password,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CredentialsUpdateRes{
		Credential:      api.Credential{ID: value.Credential.ID, Version: value.Credential.Version, Kind: value.Credential.Kind, Identifier: value.Credential.Identifier, Status: value.Credential.Status},
		RevokedSessions: value.RevokedSessions, CurrentRetained: value.CurrentRetained, Replayed: value.Replayed,
	}, nil
}

func wireAccount(value appmodel.Account) api.GetRes {
	out := api.GetRes{
		Subject: value.Subject, DisplayName: value.DisplayName, Account: value.Account,
		PrincipalType: value.PrincipalType, Owner: value.Owner, Status: value.Status,
		Merchant:      api.Merchant{MerchantID: value.Merchant.MerchantID, Name: value.Merchant.Name, Status: value.Merchant.Status},
		CurrentShopID: value.CurrentShopID, Shops: make([]api.Shop, 0, len(value.Shops)),
		Subscription: api.Subscription{
			MerchantID: value.Subscription.MerchantID, PlanID: value.Subscription.PlanID,
			PlanCode: value.Subscription.PlanCode, PlanName: value.Subscription.PlanName,
			ExpiresAt: value.Subscription.ExpiresAt, Version: value.Subscription.Version,
			ProductLimit: value.Subscription.ProductLimit, QuotaConfigured: value.Subscription.QuotaConfigured,
			PermissionNames: append([]string{}, value.Subscription.PermissionNames...),
		},
		PermissionNames: append([]string{}, value.PermissionNames...),
		Organization: api.Organization{
			ID: value.Organization.ID, Name: value.Organization.Name, Status: value.Organization.Status,
			UnitCount: value.Organization.UnitCount, MemberCount: value.Organization.MemberCount, ShopCount: value.Organization.ShopCount,
		},
	}
	if out.Subscription.PermissionNames == nil {
		out.Subscription.PermissionNames = []string{}
	}
	if out.PermissionNames == nil {
		out.PermissionNames = []string{}
	}
	for _, shop := range value.Shops {
		out.Shops = append(out.Shops, api.Shop{
			ID: shop.ID, MerchantID: shop.MerchantID, Name: shop.Name, Code: shop.Code, Status: shop.Status, Version: shop.Version,
		})
	}
	return out
}
