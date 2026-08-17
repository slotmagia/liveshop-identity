package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/privacy"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type PrivacyController struct{ service service.Merch }

func NewPrivacy(s service.Merch) *PrivacyController { return &PrivacyController{service: s} }

func (c *PrivacyController) Get(ctx context.Context, _ *api.GetReq) (*api.GetRes, error) {
	value, err := c.service.Privacy(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	res := api.GetRes(wirePrivacy(value))
	return &res, nil
}

func (c *PrivacyController) Save(ctx context.Context, request *api.SaveReq) (*api.SaveRes, error) {
	value, err := c.service.SavePrivacy(ctx, appmodel.SavePrivacy{
		CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
		CollectConsent: request.CollectConsent, MarketingConsent: request.MarketingConsent,
		CookieBanner: request.CookieBanner, DataRetentionDays: request.DataRetentionDays, ContactEmail: request.ContactEmail,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.SaveRes{Setting: wirePrivacy(value.Privacy), Replayed: value.Replayed}, nil
}

func wirePrivacy(value appmodel.Privacy) api.Setting {
	return api.Setting{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID,
		CollectConsent: value.CollectConsent, MarketingConsent: value.MarketingConsent, CookieBanner: value.CookieBanner,
		DataRetentionDays: value.DataRetentionDays, ContactEmail: value.ContactEmail, Version: value.Version,
		PlatformStatus: value.PlatformStatus, PlatformReasonPublic: value.PlatformReasonPublic, Editable: value.Editable,
	}
}
