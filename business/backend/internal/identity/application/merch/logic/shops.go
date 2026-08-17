package logic

import (
	"context"

	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

func (l *Logic) merchantScope(ctx context.Context) (int64, error) {
	claims := authctx.Caller(ctx)
	if claims.MerchantID <= 0 {
		return 0, model.ErrInvalidContext
	}
	return claims.MerchantID, nil
}

func (l *Logic) requireShopOwner(ctx context.Context) error {
	if authctx.Caller(ctx).PrincipalType != principal.TypeMerchantOwner {
		return model.ErrProtectedOwner
	}
	return nil
}

func projectManagedShop(value shopmodel.Shop) appmodel.ManagedShop {
	return appmodel.ManagedShop{
		ShopID: value.ID, MerchantID: value.MerchantID, Code: value.Code, Subdomain: value.Subdomain,
		Name: value.Name, DefaultLocale: value.DefaultLocale, Currency: value.Currency,
		CategoryCode: value.CategoryCode, Status: string(value.Status), Version: value.Version,
	}
}

func (l *Logic) ShopCategories(ctx context.Context) ([]appmodel.ShopCategoryOption, error) {
	if _, err := l.merchantScope(ctx); err != nil {
		return nil, err
	}
	if l.categories == nil {
		return nil, model.ErrUnavailable
	}
	values, err := l.categories.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.ShopCategoryOption, 0, len(values))
	for _, value := range values {
		if value.Status != shopmodel.CategoryActive {
			continue
		}
		out = append(out, appmodel.ShopCategoryOption{Code: value.Code, Name: value.Name, Icon: value.Icon})
	}
	return out, nil
}

func (l *Logic) ManagedShops(ctx context.Context, query appmodel.ShopQuery) (appmodel.ShopPage, error) {
	merchantID, err := l.merchantScope(ctx)
	if err != nil {
		return appmodel.ShopPage{}, err
	}
	if l.shops == nil {
		return appmodel.ShopPage{}, model.ErrUnavailable
	}
	page, err := l.shops.ListManaged(ctx, shopmodel.Query{
		MerchantID: merchantID, Keyword: query.Keyword, Status: shopmodel.Status(query.Status),
		Page: query.Page, PageSize: query.PageSize,
	})
	if err != nil {
		return appmodel.ShopPage{}, err
	}
	out := appmodel.ShopPage{
		Items: make([]appmodel.ManagedShop, 0, len(page.Items)), Page: page.Page, PageSize: page.PageSize, Total: page.Total,
		Owner: authctx.Caller(ctx).PrincipalType == principal.TypeMerchantOwner,
	}
	for _, item := range page.Items {
		out.Items = append(out.Items, projectManagedShop(item))
	}
	return out, nil
}

func (l *Logic) CurrentShop(ctx context.Context) (appmodel.CurrentShop, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.CurrentShop{}, err
	}
	if l.shops == nil {
		return appmodel.CurrentShop{}, model.ErrUnavailable
	}
	value, err := l.shops.GetManaged(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.CurrentShop{}, err
	}
	return appmodel.CurrentShop{
		Shop:  projectManagedShop(value),
		Owner: authctx.Caller(ctx).PrincipalType == principal.TypeMerchantOwner,
	}, nil
}

func (l *Logic) CreateShop(ctx context.Context, input appmodel.CreateShop) (appmodel.ShopMutation, error) {
	if err := l.requireShopOwner(ctx); err != nil {
		return appmodel.ShopMutation{}, err
	}
	merchantID, err := l.merchantScope(ctx)
	if err != nil {
		return appmodel.ShopMutation{}, err
	}
	if l.shops == nil {
		return appmodel.ShopMutation{}, model.ErrUnavailable
	}
	value, replayed, err := l.shops.Create(ctx, shopmodel.CreateCommand{
		CommandKey: input.CommandKey, MerchantID: merchantID, Name: input.Name, Subdomain: input.Subdomain,
		Currency: input.Currency, CategoryCode: input.CategoryCode, Status: shopmodel.Status(input.Status),
	})
	if err != nil {
		return appmodel.ShopMutation{}, err
	}
	return appmodel.ShopMutation{Shop: projectManagedShop(value), Replayed: replayed}, nil
}

func (l *Logic) UpdateShop(ctx context.Context, input appmodel.UpdateShop) (appmodel.ShopMutation, error) {
	if err := l.requireShopOwner(ctx); err != nil {
		return appmodel.ShopMutation{}, err
	}
	merchantID, err := l.merchantScope(ctx)
	if err != nil {
		return appmodel.ShopMutation{}, err
	}
	if l.shops == nil {
		return appmodel.ShopMutation{}, model.ErrUnavailable
	}
	value, replayed, err := l.shops.Update(ctx, shopmodel.UpdateCommand{
		ShopID: input.ShopID, MerchantID: merchantID, CommandKey: input.CommandKey,
		ExpectedVersion: input.ExpectedVersion, Name: input.Name, Subdomain: input.Subdomain,
	})
	if err != nil {
		return appmodel.ShopMutation{}, err
	}
	return appmodel.ShopMutation{Shop: projectManagedShop(value), Replayed: replayed}, nil
}

func (l *Logic) SetShopEnabled(ctx context.Context, input appmodel.SetShopEnabled) (appmodel.ShopMutation, error) {
	if err := l.requireShopOwner(ctx); err != nil {
		return appmodel.ShopMutation{}, err
	}
	merchantID, err := l.merchantScope(ctx)
	if err != nil {
		return appmodel.ShopMutation{}, err
	}
	if l.shops == nil {
		return appmodel.ShopMutation{}, model.ErrUnavailable
	}
	value, replayed, err := l.shops.SetEnabled(ctx, shopmodel.SetEnabledCommand{
		ShopID: input.ShopID, MerchantID: merchantID, CommandKey: input.CommandKey,
		ExpectedVersion: input.ExpectedVersion, Enabled: input.Enabled,
	})
	if err != nil {
		return appmodel.ShopMutation{}, err
	}
	return appmodel.ShopMutation{Shop: projectManagedShop(value), Replayed: replayed}, nil
}

func (l *Logic) CloseShop(ctx context.Context, input appmodel.CloseShop) (appmodel.ShopMutation, error) {
	if err := l.requireShopOwner(ctx); err != nil {
		return appmodel.ShopMutation{}, err
	}
	merchantID, err := l.merchantScope(ctx)
	if err != nil {
		return appmodel.ShopMutation{}, err
	}
	if l.shops == nil {
		return appmodel.ShopMutation{}, model.ErrUnavailable
	}
	value, replayed, err := l.shops.Close(ctx, shopmodel.CloseCommand{
		ShopID: input.ShopID, MerchantID: merchantID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return appmodel.ShopMutation{}, err
	}
	return appmodel.ShopMutation{Shop: projectManagedShop(value), Replayed: replayed}, nil
}
