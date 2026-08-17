package customer_service

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service/model"
)

// Accounts is the capability application boundary.
type Accounts struct{ repository Repository }

func NewAccounts(repository Repository) *Accounts { return &Accounts{repository: repository} }

func (a *Accounts) List(ctx context.Context, query model.Query) (model.Page, error) {
	if a == nil || a.repository == nil {
		return model.Page{}, model.ErrUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.Page{}, err
	}
	page, err := a.repository.List(ctx, normalized)
	if err != nil {
		return model.Page{}, err
	}
	if page.Page != normalized.Page || page.PageSize != normalized.PageSize || page.Total < 0 {
		return model.Page{}, model.ErrInvalid
	}
	for _, item := range page.Items {
		if err := item.ValidatePersisted(); err != nil {
			return model.Page{}, model.ErrInvalid
		}
		if normalized.MerchantID > 0 && item.MerchantID != normalized.MerchantID {
			return model.Page{}, model.ErrInvalid
		}
		if normalized.ShopID > 0 && item.ShopID != normalized.ShopID {
			return model.Page{}, model.ErrInvalid
		}
	}
	return page, nil
}

func (a *Accounts) Save(ctx context.Context, command model.SaveCommand) (model.Account, bool, error) {
	if a == nil || a.repository == nil {
		return model.Account{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Account{}, false, err
	}
	return a.repository.Save(ctx, normalized)
}

func (a *Accounts) Delete(ctx context.Context, command model.DeleteCommand) (model.DeleteResult, bool, error) {
	if a == nil || a.repository == nil {
		return model.DeleteResult{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.DeleteResult{}, false, err
	}
	return a.repository.Delete(ctx, normalized)
}
