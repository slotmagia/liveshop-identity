package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/api/http/v1/shop"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type ShopController struct{ service service.Admin }

func NewShops(service service.Admin) *ShopController { return &ShopController{service: service} }

func (c *ShopController) ListMerchants(ctx context.Context, _ *api.ListMerchantsReq) (*api.ListMerchantsRes, error) {
	values, err := c.service.ShopMerchants(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListMerchantsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Merchant{MerchantID: value.ID, Name: value.Name, ExternalID: value.ExternalID, Status: value.Status, Version: value.Version})
	}
	return &out, nil
}

func (c *ShopController) ListShops(ctx context.Context, request *api.ListShopsReq) (*api.ListShopsRes, error) {
	values, err := c.service.DirectoryShops(ctx, request.MerchantID)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListShopsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Shop{ShopID: value.ID, MerchantID: value.MerchantID,
			Code: value.Code, Subdomain: value.Subdomain, Name: value.Name,
			DefaultLocale: value.DefaultLocale, Currency: value.Currency, Status: value.Status, Version: value.Version})
	}
	return &out, nil
}
