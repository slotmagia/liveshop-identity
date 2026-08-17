package shop

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type Directory struct{ repository Repository }

func NewDirectory(repository Repository) *Directory { return &Directory{repository: repository} }

func (d *Directory) ready() error {
	if d == nil || d.repository == nil {
		return model.ErrUnavailable
	}
	return nil
}

func (d *Directory) List(ctx context.Context, merchantID int64) ([]model.Shop, error) {
	if err := d.ready(); err != nil {
		return nil, err
	}
	values, err := d.repository.ListShops(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if err := value.Validate(); err != nil {
			return nil, err
		}
		if merchantID > 0 && value.MerchantID != merchantID {
			return nil, model.ErrInvalidShop
		}
	}
	return values, nil
}

func (d *Directory) ListByMerchant(ctx context.Context, merchantID int64) ([]model.Shop, error) {
	if merchantID <= 0 {
		return nil, model.ErrInvalidMerchantID
	}
	return d.List(ctx, merchantID)
}

func (d *Directory) ListManaged(ctx context.Context, query model.Query) (model.Page, error) {
	if err := d.ready(); err != nil {
		return model.Page{}, err
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.Page{}, err
	}
	page, err := d.repository.ListManagedShops(ctx, normalized)
	if err != nil {
		return model.Page{}, err
	}
	if page.Page != normalized.Page || page.PageSize != normalized.PageSize || page.Total < 0 {
		return model.Page{}, model.ErrInvalidShop
	}
	for _, value := range page.Items {
		if err := value.Validate(); err != nil {
			return model.Page{}, err
		}
		if value.MerchantID != normalized.MerchantID {
			return model.Page{}, model.ErrInvalidShop
		}
	}
	return page, nil
}

func (d *Directory) GetManaged(ctx context.Context, merchantID, shopID int64) (model.Shop, error) {
	if err := d.ready(); err != nil {
		return model.Shop{}, err
	}
	if merchantID <= 0 {
		return model.Shop{}, model.ErrInvalidMerchantID
	}
	if shopID <= 0 {
		return model.Shop{}, model.ErrInvalidShop
	}
	value, err := d.repository.GetManagedShop(ctx, merchantID, shopID)
	if err != nil {
		return model.Shop{}, err
	}
	if err := value.Validate(); err != nil {
		return model.Shop{}, err
	}
	if value.MerchantID != merchantID || value.ID != shopID {
		return model.Shop{}, model.ErrInvalidShop
	}
	return value, nil
}

func (d *Directory) Create(ctx context.Context, command model.CreateCommand) (model.Shop, bool, error) {
	if err := d.ready(); err != nil {
		return model.Shop{}, false, err
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Shop{}, false, err
	}
	return acceptManagedShop(d.repository.CreateShop(ctx, normalized))
}

func (d *Directory) Update(ctx context.Context, command model.UpdateCommand) (model.Shop, bool, error) {
	if err := d.ready(); err != nil {
		return model.Shop{}, false, err
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Shop{}, false, err
	}
	return acceptManagedShop(d.repository.UpdateShop(ctx, normalized))
}

func (d *Directory) SetEnabled(ctx context.Context, command model.SetEnabledCommand) (model.Shop, bool, error) {
	if err := d.ready(); err != nil {
		return model.Shop{}, false, err
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Shop{}, false, err
	}
	return acceptManagedShop(d.repository.SetShopEnabled(ctx, normalized))
}

func (d *Directory) Close(ctx context.Context, command model.CloseCommand) (model.Shop, bool, error) {
	if err := d.ready(); err != nil {
		return model.Shop{}, false, err
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Shop{}, false, err
	}
	value, replayed, err := d.repository.CloseShop(ctx, normalized)
	if err != nil {
		return model.Shop{}, false, err
	}
	if err := value.ValidatePersisted(); err != nil || value.Status != model.StatusClosed {
		return model.Shop{}, false, model.ErrInvalidShop
	}
	return value, replayed, nil
}

func acceptManagedShop(value model.Shop, replayed bool, err error) (model.Shop, bool, error) {
	if err != nil {
		return model.Shop{}, false, err
	}
	if err := value.Validate(); err != nil {
		return model.Shop{}, false, err
	}
	return value, replayed, nil
}
