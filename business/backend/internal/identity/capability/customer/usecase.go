package customer

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer/model"
)

type Book struct{ repository Repository }

func NewBook(repository Repository) *Book { return &Book{repository: repository} }

func (b *Book) Addresses(ctx context.Context, tenant model.Tenant, subject string) ([]model.Address, error) {
	if b == nil || b.repository == nil {
		return nil, model.ErrUnavailable
	}
	if !tenant.Valid() || subject == "" {
		return nil, model.ErrInvalid
	}
	return b.repository.ListAddresses(ctx, tenant, subject)
}

func (b *Book) SaveAddress(ctx context.Context, command model.SaveAddressCommand) (model.Address, bool, error) {
	if b == nil || b.repository == nil {
		return model.Address{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Address{}, false, err
	}
	return b.repository.SaveAddress(ctx, normalized)
}

func (b *Book) DeleteAddress(ctx context.Context, command model.DeleteAddressCommand) (bool, error) {
	if b == nil || b.repository == nil {
		return false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return false, err
	}
	return b.repository.DeleteAddress(ctx, normalized)
}

func (b *Book) ReplaceDefault(ctx context.Context, command model.ReplaceDefaultCommand) (model.Address, bool, error) {
	if b == nil || b.repository == nil {
		return model.Address{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Address{}, false, err
	}
	return b.repository.ReplaceDefault(ctx, normalized)
}

func (b *Book) Wishlist(ctx context.Context, tenant model.Tenant, subject string, cursor int64, limit int) ([]model.WishlistItem, error) {
	if b == nil || b.repository == nil {
		return nil, model.ErrUnavailable
	}
	if !tenant.Valid() || subject == "" {
		return nil, model.ErrInvalid
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if cursor < 0 {
		return nil, model.ErrInvalid
	}
	return b.repository.ListWishlist(ctx, tenant, subject, cursor, limit)
}

func (b *Book) AddWishlist(ctx context.Context, command model.AddWishlistCommand) (model.WishlistItem, bool, error) {
	if b == nil || b.repository == nil {
		return model.WishlistItem{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.WishlistItem{}, false, err
	}
	return b.repository.AddWishlist(ctx, normalized)
}

func (b *Book) RemoveWishlist(ctx context.Context, command model.RemoveWishlistCommand) error {
	if b == nil || b.repository == nil {
		return model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return err
	}
	return b.repository.RemoveWishlist(ctx, normalized)
}
