package http

import (
	"context"
	"time"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/apps"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type AppQueryController struct{ service service.Merch }

func NewAppQuery(s service.Merch) *AppQueryController { return &AppQueryController{service: s} }

func (c *AppQueryController) ListShops(ctx context.Context, _ *api.ListShopsReq) (*api.ListShopsRes, error) {
	values, err := c.service.AppShops(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListShopsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Shop{
			ShopID: value.ShopID, MerchantID: value.MerchantID, Name: value.Name, Code: value.Code, Status: value.Status,
		})
	}
	return &out, nil
}

func (c *AppQueryController) ListScopes(ctx context.Context, _ *api.ListScopesReq) (*api.ListScopesRes, error) {
	values, err := c.service.AppScopes(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListScopesRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Scope{Code: value.Code, Group: value.Group, Label: value.Label})
	}
	return &out, nil
}

func (c *AppQueryController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	value, err := c.service.Apps(ctx, appmodel.AppQuery{
		ShopID: request.ShopID, Status: request.Status, Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRes{
		Page: value.Page, PageSize: value.PageSize, Total: value.Total, Items: []api.App{},
		PlatformStatus: value.PlatformStatus, PlatformReasonPublic: value.PlatformReason,
	}
	for _, item := range value.Items {
		out.Items = append(out.Items, wireApp(item))
	}
	return &out, nil
}

type AppWriteController struct{ service service.Merch }

func NewAppWrite(s service.Merch) *AppWriteController { return &AppWriteController{service: s} }

func (c *AppWriteController) Create(ctx context.Context, request *api.CreateReq) (*api.CreateRes, error) {
	value, err := c.service.CreateApp(ctx, appmodel.CreateApp{
		CommandKey: request.CommandKey, ShopID: request.ShopID, Name: request.Name, Scopes: request.Scopes,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateRes{App: wireApp(value.App), ClientSecret: value.ClientSecret, Replayed: value.Replayed}, nil
}

func (c *AppWriteController) Reset(ctx context.Context, request *api.ResetReq) (*api.ResetRes, error) {
	value, err := c.service.ResetAppSecret(ctx, appmodel.ResetAppSecret{
		AppID: request.AppId, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.ResetRes{App: wireApp(value.App), ClientSecret: value.ClientSecret, Replayed: value.Replayed}, nil
}

func (c *AppWriteController) Enable(ctx context.Context, request *api.EnableReq) (*api.EnableRes, error) {
	value, err := c.service.SetAppEnabled(ctx, appmodel.SetAppEnabled{
		AppID: request.AppId, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Enabled: true,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.EnableRes{App: wireApp(value.App), Replayed: value.Replayed}, nil
}

func (c *AppWriteController) Disable(ctx context.Context, request *api.DisableReq) (*api.DisableRes, error) {
	value, err := c.service.SetAppEnabled(ctx, appmodel.SetAppEnabled{
		AppID: request.AppId, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Enabled: false,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.DisableRes{App: wireApp(value.App), Replayed: value.Replayed}, nil
}

func wireApp(value appmodel.App) api.App {
	return api.App{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID, Name: value.Name, ClientID: value.ClientID,
		SecretHint: value.SecretHint, Scopes: value.Scopes, Status: value.Status, Version: value.Version,
		CreatedAt: formatAppTime(value.CreatedAt), UpdatedAt: formatAppTime(value.UpdatedAt),
		PlatformStatus: value.PlatformStatus, PlatformReasonPublic: value.PlatformReason, Editable: value.Editable,
	}
}

func formatAppTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
