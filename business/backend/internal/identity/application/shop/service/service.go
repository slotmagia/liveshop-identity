package service

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/appmodel"
)

type Shop interface {
	Health(ctx context.Context) (appmodel.Health, error)
	CreateLoginOTP(ctx context.Context, input appmodel.CreateLoginOTP) (appmodel.LoginOTP, error)
	CreateLogin(ctx context.Context, input appmodel.CreateLogin) (appmodel.Login, error)
	Addresses(ctx context.Context) ([]appmodel.Address, error)
	SaveAddress(ctx context.Context, input appmodel.SaveAddress) (appmodel.Address, error)
	DeleteAddress(ctx context.Context, input appmodel.DeleteAddress) error
	ReplaceDefaultAddress(ctx context.Context, input appmodel.ReplaceDefault) (appmodel.Address, error)
	Wishlist(ctx context.Context, cursor int64, limit int) ([]appmodel.WishlistItem, error)
	AddWishlist(ctx context.Context, input appmodel.AddWishlist) (appmodel.WishlistItem, error)
	RemoveWishlist(ctx context.Context, productID int64) error
	LoginSMSRegions(ctx context.Context, shopCode string) (appmodel.SMSRegions, error)
	Profile(ctx context.Context) (appmodel.Profile, error)
	Aftersales(ctx context.Context, query appmodel.AftersaleQuery) (appmodel.AftersalePage, error)
	Aftersale(ctx context.Context, aftersaleID int64) (appmodel.Aftersale, error)
}
