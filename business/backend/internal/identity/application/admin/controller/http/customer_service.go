package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/api/http/v1/customerservice"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type CustomerServiceController struct{ service service.Admin }

func NewCustomerService(service service.Admin) *CustomerServiceController {
	return &CustomerServiceController{service: service}
}

func (c *CustomerServiceController) ListMerchants(ctx context.Context, _ *api.ListMerchantsReq) (*api.ListMerchantsRes, error) {
	values, err := c.service.ShopMerchants(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListMerchantsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Merchant{MerchantID: value.ID, Name: value.Name, Status: value.Status})
	}
	return &out, nil
}

func (c *CustomerServiceController) ListShops(ctx context.Context, request *api.ListShopsReq) (*api.ListShopsRes, error) {
	values, err := c.service.DirectoryShops(ctx, request.MerchantID)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListShopsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Shop{ShopID: value.ID, MerchantID: value.MerchantID,
			Name: value.Name, Code: value.Code, Status: value.Status})
	}
	return &out, nil
}

func (c *CustomerServiceController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	value, err := c.service.CustomerServiceAccounts(ctx, appmodel.CustomerServiceQuery{MerchantID: request.MerchantID, ShopID: request.ShopID,
		Platform: request.Platform, Account: request.Account, Status: request.Status, Page: request.Page, PageSize: request.PageSize})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRes{Page: value.Page, PageSize: value.PageSize, Total: value.Total, Items: []api.Account{}}
	for _, item := range value.Items {
		out.Items = append(out.Items, projectCustomerServiceAPI(item))
	}
	return &out, nil
}

func (c *CustomerServiceController) Create(ctx context.Context, request *api.CreateReq) (*api.CreateRes, error) {
	value, err := c.service.SaveCustomerServiceAccount(ctx, saveCustomerServiceInput(0, 0, request.SaveFields))
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateRes{Account: projectCustomerServiceAPI(value.Account), Replayed: value.Replayed}, nil
}

func (c *CustomerServiceController) Update(ctx context.Context, request *api.UpdateReq) (*api.UpdateRes, error) {
	value, err := c.service.SaveCustomerServiceAccount(ctx, saveCustomerServiceInput(request.AccountID, request.ExpectedVersion, request.SaveFields))
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.UpdateRes{Account: projectCustomerServiceAPI(value.Account), Replayed: value.Replayed}, nil
}

func (c *CustomerServiceController) Delete(ctx context.Context, request *api.DeleteReq) (*api.DeleteRes, error) {
	value, err := c.service.DeleteCustomerServiceAccount(ctx, appmodel.DeleteCustomerServiceAccount{AccountID: request.AccountID,
		MerchantID: request.MerchantID, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.DeleteRes{ID: value.ID, Deleted: value.Deleted, Version: value.Version, Replayed: value.Replayed}, nil
}

func saveCustomerServiceInput(id int64, expectedVersion uint64, value api.SaveFields) appmodel.SaveCustomerServiceAccount {
	return appmodel.SaveCustomerServiceAccount{CommandKey: value.CommandKey, ExpectedVersion: expectedVersion,
		Account: appmodel.CustomerServiceAccount{ID: id, MerchantID: value.MerchantID, ShopID: value.ShopID,
			Platform: value.Platform, Account: value.Account, Nickname: value.Nickname, Status: value.Status,
			Config: value.Config, Remark: value.Remark}}
}

func projectCustomerServiceAPI(value appmodel.CustomerServiceAccount) api.Account {
	return api.Account{ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID,
		Platform: value.Platform, Account: value.Account, Nickname: value.Nickname,
		Status: value.Status, Config: value.Config, Remark: value.Remark, Version: value.Version,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
