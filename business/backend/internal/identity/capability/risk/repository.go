// Package risk owns visitor-risk use cases and repository ports.
package risk

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/risk/model"
)

// Repository is implemented by the Identity data layer.
type Repository interface {
	List(context.Context, model.Query) (model.Page, error)
}

// Events is the capability application boundary.
type Events struct{ repository Repository }

func NewEvents(repository Repository) *Events { return &Events{repository: repository} }

func (e *Events) List(ctx context.Context, query model.Query) (model.Page, error) {
	if e == nil || e.repository == nil {
		return model.Page{}, model.ErrUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.Page{}, err
	}
	page, err := e.repository.List(ctx, normalized)
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
		if item.MerchantID != normalized.MerchantID || item.ShopID != normalized.ShopID {
			return model.Page{}, model.ErrInvalid
		}
	}
	return page, nil
}
