package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/customeraccounts"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type CustomerAccountController struct{ service service.Merch }

func NewCustomerAccounts(service service.Merch) *CustomerAccountController {
	return &CustomerAccountController{service: service}
}

func NewCustomerAccount(service service.Merch) *CustomerAccountController {
	return NewCustomerAccounts(service)
}

func (c *CustomerAccountController) ListShops(ctx context.Context, _ *api.ListShopsReq) (*api.ListShopsRes, error) {
	values, err := c.service.CustomerAccountShops(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListShopsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Shop{ShopID: value.ShopID, MerchantID: value.MerchantID, Name: value.Name, Code: value.Code, Status: value.Status})
	}
	return &out, nil
}

func (c *CustomerAccountController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	value, err := c.service.CustomerAccounts(ctx, appmodel.CustomerAccountQuery{
		ShopID: request.ShopID, Platform: request.Platform, Account: request.Account, Status: request.Status,
		Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRes{Page: value.Page, PageSize: value.PageSize, Total: value.Total, Items: []api.Account{}}
	for _, item := range value.Items {
		out.Items = append(out.Items, projectCustomerAccountAPI(item))
	}
	return &out, nil
}

func (c *CustomerAccountController) Create(ctx context.Context, request *api.CreateReq) (*api.CreateRes, error) {
	value, err := c.service.SaveCustomerAccount(ctx, saveCustomerAccountInput(0, 0, request.SaveFields))
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateRes{Account: projectCustomerAccountAPI(value.Account), Replayed: value.Replayed}, nil
}

func (c *CustomerAccountController) Update(ctx context.Context, request *api.UpdateReq) (*api.UpdateRes, error) {
	value, err := c.service.SaveCustomerAccount(ctx, saveCustomerAccountInput(request.AccountID, request.ExpectedVersion, request.SaveFields))
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.UpdateRes{Account: projectCustomerAccountAPI(value.Account), Replayed: value.Replayed}, nil
}

func (c *CustomerAccountController) Delete(ctx context.Context, request *api.DeleteReq) (*api.DeleteRes, error) {
	value, err := c.service.DeleteCustomerAccount(ctx, appmodel.DeleteCustomerAccount{
		AccountID: request.AccountID, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.DeleteRes{ID: value.ID, Deleted: value.Deleted, Version: value.Version, Replayed: value.Replayed}, nil
}

func saveCustomerAccountInput(id int64, expectedVersion uint64, value api.SaveFields) appmodel.SaveCustomerAccount {
	return appmodel.SaveCustomerAccount{
		CommandKey: value.CommandKey, ExpectedVersion: expectedVersion, ShopID: value.ShopID, ID: id,
		Platform: value.Platform, Account: value.Account, Nickname: value.Nickname, Status: value.Status,
		Config: value.Config, Remark: value.Remark,
	}
}

func projectCustomerAccountAPI(value appmodel.CustomerAccount) api.Account {
	return api.Account{ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID,
		Platform: value.Platform, Account: value.Account, Nickname: value.Nickname,
		Status: value.Status, Config: value.Config, Remark: value.Remark, Version: value.Version,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
