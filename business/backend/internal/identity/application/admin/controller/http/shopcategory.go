package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/api/http/v1/shopcategory"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type ShopCategoryController struct{ service service.Admin }

func NewShopCategories(service service.Admin) *ShopCategoryController {
	return &ShopCategoryController{service: service}
}

func (c *ShopCategoryController) List(ctx context.Context, _ *api.ListReq) (*api.ListRes, error) {
	values, err := c.service.ShopCategories(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListRes, 0, len(values))
	for _, value := range values {
		out = append(out, wireShopCategory(value))
	}
	return &out, nil
}

func (c *ShopCategoryController) Create(ctx context.Context, request *api.CreateReq) (*api.SaveRes, error) {
	return c.save(ctx, request.CommandKey, request.ExpectedVersion, appmodel.ShopCategory{
		Code: request.Code, Name: request.Name, Icon: request.Icon, Sort: request.Sort, Status: request.Status,
	})
}

func (c *ShopCategoryController) Update(ctx context.Context, request *api.UpdateReq) (*api.SaveRes, error) {
	return c.save(ctx, request.CommandKey, request.ExpectedVersion, appmodel.ShopCategory{
		ID: request.CategoryID, Code: request.Code, Name: request.Name, Icon: request.Icon, Sort: request.Sort, Status: request.Status,
	})
}

func (c *ShopCategoryController) save(ctx context.Context, commandKey string, expectedVersion uint64, category appmodel.ShopCategory) (*api.SaveRes, error) {
	result, err := c.service.SaveShopCategory(ctx, appmodel.SaveShopCategory{CommandKey: commandKey, ExpectedVersion: expectedVersion, Category: category})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.SaveRes{Category: wireShopCategory(result.Category), Replayed: result.Replayed}, nil
}

func (c *ShopCategoryController) Enable(ctx context.Context, request *api.EnableReq) (*api.EnableRes, error) {
	result, err := c.service.SetShopCategoryEnabled(ctx, appmodel.SetShopCategoryEnabled{
		CategoryID: request.CategoryID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Enabled: true,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	response := api.EnableRes{Category: wireShopCategory(result.Category), Replayed: result.Replayed}
	return &response, nil
}

func (c *ShopCategoryController) Disable(ctx context.Context, request *api.DisableReq) (*api.DisableRes, error) {
	result, err := c.service.SetShopCategoryEnabled(ctx, appmodel.SetShopCategoryEnabled{
		CategoryID: request.CategoryID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Enabled: false,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	response := api.DisableRes{Category: wireShopCategory(result.Category), Replayed: result.Replayed}
	return &response, nil
}

func (c *ShopCategoryController) Retire(ctx context.Context, request *api.RetireReq) (*api.RetireRes, error) {
	result, err := c.service.RetireShopCategory(ctx, appmodel.RetireShopCategory{
		CategoryID: request.CategoryID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	response := api.RetireRes{Category: wireShopCategory(result.Category), Replayed: result.Replayed}
	return &response, nil
}

func wireShopCategory(value appmodel.ShopCategory) api.Category {
	return api.Category{ID: value.ID, Code: value.Code, Name: value.Name, Icon: value.Icon, Sort: value.Sort,
		Status: value.Status, Version: value.Version, UsedShopCount: value.UsedShopCount}
}
