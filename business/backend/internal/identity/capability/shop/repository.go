// Package shop defines the shop capability use cases and repository ports.
package shop

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type Repository interface {
	ListShops(context.Context, int64) ([]model.Shop, error)
	ListShopsByMerchant(context.Context, int64) ([]model.Shop, error)
	ListManagedShops(context.Context, model.Query) (model.Page, error)
	GetManagedShop(context.Context, int64, int64) (model.Shop, error)
	GetShopByCode(context.Context, string) (model.Shop, error)
	GetShopBySubdomain(context.Context, string) (model.Shop, error)
	CreateShop(context.Context, model.CreateCommand) (model.Shop, bool, error)
	UpdateShop(context.Context, model.UpdateCommand) (model.Shop, bool, error)
	SetShopEnabled(context.Context, model.SetEnabledCommand) (model.Shop, bool, error)
	CloseShop(context.Context, model.CloseCommand) (model.Shop, bool, error)
}
