package customer

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer/model"
)

type Repository interface {
	ListAddresses(context.Context, model.Tenant, string) ([]model.Address, error)
	SaveAddress(context.Context, model.SaveAddressCommand) (model.Address, bool, error)
	DeleteAddress(context.Context, model.DeleteAddressCommand) (bool, error)
	ReplaceDefault(context.Context, model.ReplaceDefaultCommand) (model.Address, bool, error)
	ListWishlist(context.Context, model.Tenant, string, int64, int) ([]model.WishlistItem, error)
	AddWishlist(context.Context, model.AddWishlistCommand) (model.WishlistItem, bool, error)
	RemoveWishlist(context.Context, model.RemoveWishlistCommand) error
}
