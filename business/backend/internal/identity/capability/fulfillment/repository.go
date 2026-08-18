package fulfillment

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

// Repository is implemented by the Identity data layer.
type Repository interface {
	List(context.Context, model.Query) (model.Page, error)
	Get(context.Context, int64, int64, int64) (model.Complaint, error)
	Review(context.Context, model.ReviewCommand) (model.Complaint, bool, error)
}

// AftersaleRepository is implemented by the Identity data layer for aftersale tickets.
type AftersaleRepository interface {
	ListAftersales(context.Context, model.AftersaleQuery) (model.AftersalePage, error)
	GetAftersale(context.Context, int64, int64, int64) (model.Aftersale, error)
	ReviewAftersale(context.Context, model.ReviewAftersaleCommand) (model.Aftersale, bool, error)
	ReceiveAftersale(context.Context, model.ReceiveAftersaleCommand) (model.Aftersale, bool, error)
}

// Complaints is the capability application boundary.
type Complaints struct{ repository Repository }

func NewComplaints(repository Repository) *Complaints {
	return &Complaints{repository: repository}
}

func (c *Complaints) List(ctx context.Context, query model.Query) (model.Page, error) {
	if c == nil || c.repository == nil {
		return model.Page{}, model.ErrUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.Page{}, err
	}
	page, err := c.repository.List(ctx, normalized)
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

func (c *Complaints) Get(ctx context.Context, merchantID, shopID, complaintID int64) (model.Complaint, error) {
	if c == nil || c.repository == nil {
		return model.Complaint{}, model.ErrUnavailable
	}
	if merchantID <= 0 || shopID <= 0 || complaintID <= 0 {
		return model.Complaint{}, model.ErrInvalid
	}
	value, err := c.repository.Get(ctx, merchantID, shopID, complaintID)
	if err != nil {
		return model.Complaint{}, err
	}
	if err := value.ValidatePersisted(); err != nil {
		return model.Complaint{}, model.ErrInvalid
	}
	if value.MerchantID != merchantID || value.ShopID != shopID || value.ID != complaintID {
		return model.Complaint{}, model.ErrInvalid
	}
	return value, nil
}

func (c *Complaints) Review(ctx context.Context, command model.ReviewCommand) (model.Complaint, bool, error) {
	if c == nil || c.repository == nil {
		return model.Complaint{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Complaint{}, false, err
	}
	value, replayed, err := c.repository.Review(ctx, normalized)
	if err != nil {
		return model.Complaint{}, false, err
	}
	if err := value.ValidatePersisted(); err != nil {
		return model.Complaint{}, false, model.ErrInvalid
	}
	if value.MerchantID != normalized.MerchantID || value.ShopID != normalized.ShopID || value.ID != normalized.ComplaintID {
		return model.Complaint{}, false, model.ErrInvalid
	}
	return value, replayed, nil
}
