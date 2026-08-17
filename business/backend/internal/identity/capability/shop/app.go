package shop

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type AppRepository interface {
	ListApps(context.Context, model.AppQuery) (model.AppPage, error)
	CreateApp(context.Context, model.CreateAppCommand) (model.AppMutation, bool, error)
	ResetAppSecret(context.Context, model.ResetAppSecretCommand) (model.AppMutation, bool, error)
	SetAppEnabled(context.Context, model.SetAppEnabledCommand) (model.App, bool, error)
}

type PrivateApps struct{ repository AppRepository }

func NewPrivateApps(repository AppRepository) *PrivateApps {
	return &PrivateApps{repository: repository}
}

func (p *PrivateApps) List(ctx context.Context, query model.AppQuery) (model.AppPage, error) {
	if p == nil || p.repository == nil {
		return model.AppPage{}, model.ErrUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.AppPage{}, err
	}
	page, err := p.repository.ListApps(ctx, normalized)
	if err != nil {
		return model.AppPage{}, err
	}
	if page.Page != normalized.Page || page.PageSize != normalized.PageSize || page.Total < 0 {
		return model.AppPage{}, model.ErrAppInvalid
	}
	for _, item := range page.Items {
		if err := item.ValidatePersisted(); err != nil {
			return model.AppPage{}, model.ErrAppInvalid
		}
		if item.MerchantID != normalized.MerchantID || item.ShopID != normalized.ShopID {
			return model.AppPage{}, model.ErrAppInvalid
		}
	}
	return page, nil
}

func (p *PrivateApps) Create(ctx context.Context, command model.CreateAppCommand) (model.AppMutation, bool, error) {
	if p == nil || p.repository == nil {
		return model.AppMutation{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.AppMutation{}, false, err
	}
	return p.repository.CreateApp(ctx, normalized)
}

func (p *PrivateApps) ResetSecret(ctx context.Context, command model.ResetAppSecretCommand) (model.AppMutation, bool, error) {
	if p == nil || p.repository == nil {
		return model.AppMutation{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.AppMutation{}, false, err
	}
	return p.repository.ResetAppSecret(ctx, normalized)
}

func (p *PrivateApps) SetEnabled(ctx context.Context, command model.SetAppEnabledCommand) (model.App, bool, error) {
	if p == nil || p.repository == nil {
		return model.App{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.App{}, false, err
	}
	return p.repository.SetAppEnabled(ctx, normalized)
}
