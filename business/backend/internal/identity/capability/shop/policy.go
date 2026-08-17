package shop

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type PolicyRepository interface {
	ListPolicies(context.Context, model.PolicyQuery) (model.PolicyPage, error)
	SavePolicy(context.Context, model.SavePolicyCommand) (model.Policy, bool, error)
	PublishPolicy(context.Context, model.PublishPolicyCommand) (model.Policy, bool, error)
}

type Policies struct{ repository PolicyRepository }

func NewPolicies(repository PolicyRepository) *Policies {
	return &Policies{repository: repository}
}

func (p *Policies) List(ctx context.Context, query model.PolicyQuery) (model.PolicyPage, error) {
	if p == nil || p.repository == nil {
		return model.PolicyPage{}, model.ErrUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.PolicyPage{}, err
	}
	page, err := p.repository.ListPolicies(ctx, normalized)
	if err != nil {
		return model.PolicyPage{}, err
	}
	if page.Page != normalized.Page || page.PageSize != normalized.PageSize || page.Total < 0 {
		return model.PolicyPage{}, model.ErrPolicyInvalid
	}
	for _, item := range page.Items {
		if err := item.ValidatePersisted(); err != nil {
			return model.PolicyPage{}, model.ErrPolicyInvalid
		}
		if item.MerchantID != normalized.MerchantID || item.ShopID != normalized.ShopID {
			return model.PolicyPage{}, model.ErrPolicyInvalid
		}
	}
	return page, nil
}

func (p *Policies) Save(ctx context.Context, command model.SavePolicyCommand) (model.Policy, bool, error) {
	if p == nil || p.repository == nil {
		return model.Policy{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Policy{}, false, err
	}
	return p.repository.SavePolicy(ctx, normalized)
}

func (p *Policies) Publish(ctx context.Context, command model.PublishPolicyCommand) (model.Policy, bool, error) {
	if p == nil || p.repository == nil {
		return model.Policy{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Policy{}, false, err
	}
	return p.repository.PublishPolicy(ctx, normalized)
}
