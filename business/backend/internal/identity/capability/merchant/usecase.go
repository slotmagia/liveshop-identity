package merchant

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
)

type Directory struct{ repository Repository }

func NewDirectory(repository Repository) *Directory { return &Directory{repository: repository} }

func (d *Directory) List(ctx context.Context) ([]model.Merchant, error) {
	if d == nil || d.repository == nil {
		return nil, model.ErrUnavailable
	}
	values, err := d.repository.ListMerchants(ctx)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if err := value.Validate(); err != nil {
			return nil, err
		}
	}
	return values, nil
}

func (d *Directory) Page(ctx context.Context, query model.Query) (model.Page, error) {
	if d == nil || d.repository == nil {
		return model.Page{}, model.ErrUnavailable
	}
	return d.repository.ListMerchantPage(ctx, query.Normalize())
}

func (d *Directory) Get(ctx context.Context, merchantID int64) (model.Record, error) {
	if d == nil || d.repository == nil {
		return model.Record{}, model.ErrUnavailable
	}
	if merchantID <= 0 {
		return model.Record{}, model.ErrInvalid
	}
	return d.repository.GetMerchant(ctx, merchantID)
}

func (d *Directory) Create(ctx context.Context, command model.CreateCommand) (model.CreateResult, bool, error) {
	if d == nil || d.repository == nil {
		return model.CreateResult{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.CreateResult{}, false, err
	}
	return d.repository.CreateMerchant(ctx, normalized)
}

func (d *Directory) Update(ctx context.Context, command model.UpdateCommand) (model.Record, bool, error) {
	if d == nil || d.repository == nil {
		return model.Record{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Record{}, false, err
	}
	return d.repository.UpdateMerchant(ctx, normalized)
}

func (d *Directory) UpdateProfile(ctx context.Context, command model.UpdateProfileCommand) (model.Record, bool, error) {
	if d == nil || d.repository == nil {
		return model.Record{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Record{}, false, err
	}
	return d.repository.UpdateProfile(ctx, normalized)
}

func (d *Directory) ResetPassword(ctx context.Context, command model.ResetPasswordCommand) (bool, error) {
	if d == nil || d.repository == nil {
		return false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return false, err
	}
	return d.repository.ResetOwnerPassword(ctx, normalized)
}

func (d *Directory) Close(ctx context.Context, command model.CloseCommand) (model.Record, bool, error) {
	if d == nil || d.repository == nil {
		return model.Record{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Record{}, false, err
	}
	return d.repository.CloseMerchant(ctx, normalized)
}
