package merchant_governance

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
)

// Capabilities is the merchant-governance application boundary.
type Capabilities struct{ repository Repository }

func NewCapabilities(repository Repository) *Capabilities {
	return &Capabilities{repository: repository}
}

func (c *Capabilities) Catalog() []model.Module {
	return model.Catalog()
}

func (c *Capabilities) List(ctx context.Context, query model.Query) (model.Page, error) {
	if c == nil || c.repository == nil {
		return model.Page{}, model.ErrUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.Page{}, err
	}
	stored, err := c.repository.List(ctx, normalized)
	if err != nil {
		return model.Page{}, err
	}
	byModule := make(map[string]model.Capability, len(stored.Items))
	for _, item := range stored.Items {
		if err := item.ValidatePersisted(); err != nil || item.MerchantID != normalized.MerchantID || item.ShopID != normalized.ShopID {
			return model.Page{}, model.ErrInvalid
		}
		item.ModuleLabel = model.ModuleLabel(item.Module)
		byModule[item.Module] = item
	}
	items := make([]model.Capability, 0, len(model.Catalog()))
	for _, module := range model.Catalog() {
		if normalized.Module != "" && normalized.Module != module.Key {
			continue
		}
		if item, ok := byModule[module.Key]; ok {
			items = append(items, item)
			continue
		}
		items = append(items, model.Capability{
			MerchantID:     normalized.MerchantID,
			ShopID:         normalized.ShopID,
			Module:         module.Key,
			ModuleLabel:    module.Label,
			Name:           module.Label,
			MerchantStatus: model.MerchantUnset,
			PlatformStatus: model.PlatformActive,
		})
	}
	return model.Page{Items: items}, nil
}

func (c *Capabilities) Audit(ctx context.Context, query model.AuditQuery) ([]model.AuditItem, error) {
	if c == nil || c.repository == nil {
		return nil, model.ErrUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	return c.repository.Audit(ctx, normalized)
}

func (c *Capabilities) Intervene(ctx context.Context, command model.InterveneCommand) (model.Capability, bool, error) {
	if c == nil || c.repository == nil {
		return model.Capability{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Capability{}, false, err
	}
	return c.repository.Intervene(ctx, normalized)
}
