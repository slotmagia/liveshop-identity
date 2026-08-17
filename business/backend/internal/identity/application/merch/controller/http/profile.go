package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/profile"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type ProfileController struct{ service service.Merch }

func NewProfile(s service.Merch) *ProfileController { return &ProfileController{service: s} }

func (c *ProfileController) Get(ctx context.Context, _ *api.GetReq) (*api.GetRes, error) {
	value, err := c.service.Profile(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	res := api.GetRes(wireProfile(value))
	return &res, nil
}

func (c *ProfileController) Save(ctx context.Context, request *api.SaveReq) (*api.SaveRes, error) {
	value, err := c.service.SaveProfile(ctx, appmodel.SaveProfile{
		CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
		ExternalID: request.ExternalID, ContactName: request.ContactName, ContactPhone: request.ContactPhone,
		MarketingEmailOptIn: request.MarketingEmailOptIn, MarketingSMSOptIn: request.MarketingSMSOptIn,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.SaveRes{Profile: wireProfile(value.Profile), Replayed: value.Replayed}, nil
}

func wireProfile(value appmodel.Profile) api.Setting {
	return api.Setting{
		MerchantID: value.MerchantID, Name: value.Name, Account: value.Account, ExternalID: value.ExternalID,
		ContactName: value.ContactName, ContactPhone: value.ContactPhone,
		MarketingEmailOptIn: value.MarketingEmailOptIn, MarketingSMSOptIn: value.MarketingSMSOptIn,
		Status: value.Status, Version: value.Version, Owner: value.Owner,
	}
}
