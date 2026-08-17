package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/shops"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type ShopQueryController struct{ service service.Merch }

func NewShopQuery(s service.Merch) *ShopQueryController { return &ShopQueryController{service: s} }

func (c *ShopQueryController) ListCategories(ctx context.Context, _ *api.ListCategoriesReq) (*api.ListCategoriesRes, error) {
	values, err := c.service.ShopCategories(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListCategoriesRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Category{Code: value.Code, Name: value.Name, Icon: value.Icon})
	}
	return &out, nil
}

func (c *ShopQueryController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	value, err := c.service.ManagedShops(ctx, appmodel.ShopQuery{
		Keyword: request.Keyword, Status: request.Status, Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRes{Page: value.Page, PageSize: value.PageSize, Total: value.Total, Owner: value.Owner, Items: []api.Shop{}}
	for _, item := range value.Items {
		out.Items = append(out.Items, wireManagedShop(item))
	}
	return &out, nil
}

func (c *ShopQueryController) Current(ctx context.Context, _ *api.CurrentReq) (*api.CurrentRes, error) {
	value, err := c.service.CurrentShop(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CurrentRes{Shop: wireManagedShop(value.Shop), Owner: value.Owner}, nil
}

type ShopWriteController struct{ service service.Merch }

func NewShopWrite(s service.Merch) *ShopWriteController { return &ShopWriteController{service: s} }

func (c *ShopWriteController) Create(ctx context.Context, request *api.CreateReq) (*api.CreateRes, error) {
	value, err := c.service.CreateShop(ctx, appmodel.CreateShop{
		CommandKey: request.CommandKey, Name: request.Name, Subdomain: request.Subdomain,
		Currency: request.Currency, CategoryCode: request.CategoryCode, Status: request.Status,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateRes{Shop: wireManagedShop(value.Shop), Replayed: value.Replayed}, nil
}

func (c *ShopWriteController) Update(ctx context.Context, request *api.UpdateReq) (*api.UpdateRes, error) {
	value, err := c.service.UpdateShop(ctx, appmodel.UpdateShop{
		ShopID: request.ShopId, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
		Name: request.Name, Subdomain: request.Subdomain,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.UpdateRes{Shop: wireManagedShop(value.Shop), Replayed: value.Replayed}, nil
}

func (c *ShopWriteController) Enable(ctx context.Context, request *api.EnableReq) (*api.EnableRes, error) {
	value, err := c.service.SetShopEnabled(ctx, appmodel.SetShopEnabled{
		ShopID: request.ShopId, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Enabled: true,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.EnableRes{Shop: wireManagedShop(value.Shop), Replayed: value.Replayed}, nil
}

func (c *ShopWriteController) Disable(ctx context.Context, request *api.DisableReq) (*api.DisableRes, error) {
	value, err := c.service.SetShopEnabled(ctx, appmodel.SetShopEnabled{
		ShopID: request.ShopId, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Enabled: false,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.DisableRes{Shop: wireManagedShop(value.Shop), Replayed: value.Replayed}, nil
}

func (c *ShopWriteController) CloseShop(ctx context.Context, request *api.CloseReq) (*api.CloseRes, error) {
	value, err := c.service.CloseShop(ctx, appmodel.CloseShop{
		ShopID: request.ShopId, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CloseRes{Shop: wireManagedShop(value.Shop), Replayed: value.Replayed}, nil
}

func wireManagedShop(value appmodel.ManagedShop) api.Shop {
	return api.Shop{
		ShopID: value.ShopID, MerchantID: value.MerchantID, Code: value.Code, Subdomain: value.Subdomain,
		Name: value.Name, DefaultLocale: value.DefaultLocale, Currency: value.Currency,
		CategoryCode: value.CategoryCode, Status: value.Status, Version: value.Version,
	}
}
