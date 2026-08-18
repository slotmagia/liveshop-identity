package fulfillment

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

type Aftersales struct{ repository AftersaleRepository }

func NewAftersales(repository AftersaleRepository) *Aftersales {
	return &Aftersales{repository: repository}
}

func (a *Aftersales) List(ctx context.Context, query model.AftersaleQuery) (model.AftersalePage, error) {
	if a == nil || a.repository == nil {
		return model.AftersalePage{}, model.ErrAftersaleUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.AftersalePage{}, err
	}
	page, err := a.repository.ListAftersales(ctx, normalized)
	if err != nil {
		return model.AftersalePage{}, err
	}
	if page.Page != normalized.Page || page.PageSize != normalized.PageSize || page.Total < 0 {
		return model.AftersalePage{}, model.ErrAftersaleInvalid
	}
	for _, item := range page.Items {
		if err := item.ValidatePersisted(); err != nil {
			return model.AftersalePage{}, model.ErrAftersaleInvalid
		}
		if item.MerchantID != normalized.MerchantID || item.ShopID != normalized.ShopID {
			return model.AftersalePage{}, model.ErrAftersaleInvalid
		}
	}
	return page, nil
}

func (a *Aftersales) Get(ctx context.Context, merchantID, shopID, aftersaleID int64) (model.Aftersale, error) {
	if a == nil || a.repository == nil {
		return model.Aftersale{}, model.ErrAftersaleUnavailable
	}
	if merchantID <= 0 || shopID <= 0 || aftersaleID <= 0 {
		return model.Aftersale{}, model.ErrAftersaleInvalid
	}
	value, err := a.repository.GetAftersale(ctx, merchantID, shopID, aftersaleID)
	if err != nil {
		return model.Aftersale{}, err
	}
	if err := value.ValidatePersisted(); err != nil {
		return model.Aftersale{}, model.ErrAftersaleInvalid
	}
	if value.MerchantID != merchantID || value.ShopID != shopID || value.ID != aftersaleID {
		return model.Aftersale{}, model.ErrAftersaleInvalid
	}
	return value, nil
}

func (a *Aftersales) Review(ctx context.Context, command model.ReviewAftersaleCommand) (model.Aftersale, bool, error) {
	if a == nil || a.repository == nil {
		return model.Aftersale{}, false, model.ErrAftersaleUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Aftersale{}, false, err
	}
	value, replayed, err := a.repository.ReviewAftersale(ctx, normalized)
	if err != nil {
		return model.Aftersale{}, false, err
	}
	return aftersaleWriteResult(value, replayed, normalized.MerchantID, normalized.ShopID, normalized.AftersaleID)
}

func (a *Aftersales) Receive(ctx context.Context, command model.ReceiveAftersaleCommand) (model.Aftersale, bool, error) {
	if a == nil || a.repository == nil {
		return model.Aftersale{}, false, model.ErrAftersaleUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Aftersale{}, false, err
	}
	value, replayed, err := a.repository.ReceiveAftersale(ctx, normalized)
	if err != nil {
		return model.Aftersale{}, false, err
	}
	return aftersaleWriteResult(value, replayed, normalized.MerchantID, normalized.ShopID, normalized.AftersaleID)
}

func aftersaleWriteResult(value model.Aftersale, replayed bool, merchantID, shopID, aftersaleID int64) (model.Aftersale, bool, error) {
	if err := value.ValidatePersisted(); err != nil {
		return model.Aftersale{}, false, model.ErrAftersaleInvalid
	}
	if value.MerchantID != merchantID || value.ShopID != shopID || value.ID != aftersaleID {
		return model.Aftersale{}, false, model.ErrAftersaleInvalid
	}
	return value, replayed, nil
}
