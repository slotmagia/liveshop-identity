package logic

import (
	"context"
	"time"

	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	customerservicemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

func (l *Logic) CustomerAccountShops(ctx context.Context) ([]appmodel.CustomerAccountShop, error) {
	merchantID, lockedShopID, err := l.customerAccountScope(ctx, 0, false)
	if err != nil {
		return nil, err
	}
	if l.shops == nil {
		return nil, model.ErrUnavailable
	}
	values, err := l.shops.ListByMerchant(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.CustomerAccountShop, 0, len(values))
	for _, value := range values {
		if lockedShopID > 0 && value.ID != lockedShopID {
			continue
		}
		out = append(out, appmodel.CustomerAccountShop{
			ShopID: value.ID, MerchantID: value.MerchantID, Name: value.Name, Code: value.Code, Status: string(value.Status),
		})
	}
	return out, nil
}

func (l *Logic) CustomerAccounts(ctx context.Context, input appmodel.CustomerAccountQuery) (appmodel.CustomerAccountPage, error) {
	merchantID, shopID, err := l.customerAccountScope(ctx, input.ShopID, false)
	if err != nil {
		return appmodel.CustomerAccountPage{}, err
	}
	if l.customerService == nil {
		return appmodel.CustomerAccountPage{}, customerservicemodel.ErrUnavailable
	}
	var status *customerservicemodel.Status
	if input.Status != "" {
		value := customerservicemodel.Status(input.Status)
		status = &value
	}
	page, err := l.customerService.List(ctx, customerservicemodel.Query{
		MerchantID: merchantID, ShopID: shopID, Platform: input.Platform, Account: input.Account,
		Status: status, Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return appmodel.CustomerAccountPage{}, err
	}
	result := appmodel.CustomerAccountPage{Page: page.Page, PageSize: page.PageSize, Total: page.Total, Items: []appmodel.CustomerAccount{}}
	for _, item := range page.Items {
		result.Items = append(result.Items, projectCustomerAccount(item))
	}
	return result, nil
}

func (l *Logic) SaveCustomerAccount(ctx context.Context, input appmodel.SaveCustomerAccount) (appmodel.CustomerAccountResult, error) {
	merchantID, shopID, err := l.customerAccountScope(ctx, input.ShopID, true)
	if err != nil {
		return appmodel.CustomerAccountResult{}, err
	}
	if l.customerService == nil {
		return appmodel.CustomerAccountResult{}, customerservicemodel.ErrUnavailable
	}
	value, replayed, err := l.customerService.Save(ctx, customerservicemodel.SaveCommand{
		CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
		Account: customerservicemodel.Account{
			ID: input.ID, MerchantID: merchantID, ShopID: shopID, Platform: input.Platform, Account: input.Account,
			Nickname: input.Nickname, Status: customerservicemodel.Status(input.Status), Config: input.Config, Remark: input.Remark,
		},
	})
	if err != nil {
		return appmodel.CustomerAccountResult{}, err
	}
	return appmodel.CustomerAccountResult{Account: projectCustomerAccount(value), Replayed: replayed}, nil
}

func (l *Logic) DeleteCustomerAccount(ctx context.Context, input appmodel.DeleteCustomerAccount) (appmodel.CustomerAccountDeleteResult, error) {
	merchantID, shopID, err := l.customerAccountScope(ctx, input.ShopID, true)
	if err != nil {
		return appmodel.CustomerAccountDeleteResult{}, err
	}
	if l.customerService == nil {
		return appmodel.CustomerAccountDeleteResult{}, customerservicemodel.ErrUnavailable
	}
	value, replayed, err := l.customerService.Delete(ctx, customerservicemodel.DeleteCommand{
		AccountID: input.AccountID, MerchantID: merchantID, ShopID: shopID,
		CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return appmodel.CustomerAccountDeleteResult{}, err
	}
	return appmodel.CustomerAccountDeleteResult{ID: value.ID, Deleted: value.Deleted, Version: value.Version, Replayed: replayed}, nil
}

func (l *Logic) customerAccountScope(ctx context.Context, requestedShopID int64, requireShop bool) (int64, int64, error) {
	claims := authctx.Caller(ctx)
	if claims.MerchantID <= 0 {
		return 0, 0, model.ErrInvalidContext
	}
	shopID := requestedShopID
	staffLocked := claims.PrincipalType != principal.TypeMerchantOwner && claims.ShopID > 0
	if shopID <= 0 && staffLocked {
		shopID = claims.ShopID
	}
	if requireShop && shopID <= 0 {
		return 0, 0, customerservicemodel.ErrInvalid
	}
	if shopID <= 0 {
		return claims.MerchantID, 0, nil
	}
	if staffLocked && shopID != claims.ShopID {
		return 0, 0, model.ErrInvalidContext
	}
	if l.shops == nil {
		return 0, 0, model.ErrUnavailable
	}
	shops, err := l.shops.ListByMerchant(ctx, claims.MerchantID)
	if err != nil {
		return 0, 0, err
	}
	for _, shop := range shops {
		if shop.ID == shopID {
			return claims.MerchantID, shopID, nil
		}
	}
	return 0, 0, model.ErrInvalidContext
}

func projectCustomerAccount(value customerservicemodel.Account) appmodel.CustomerAccount {
	createdAt, updatedAt := "", ""
	if !value.CreatedAt.IsZero() {
		createdAt = value.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	if !value.UpdatedAt.IsZero() {
		updatedAt = value.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	return appmodel.CustomerAccount{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID, Platform: value.Platform,
		Account: value.Account, Nickname: value.Nickname, Status: string(value.Status), Config: value.Config,
		Remark: value.Remark, Version: value.Version, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
}
