package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/api/http/v1/merchant"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type MerchantController struct{ service service.Admin }

func NewMerchants(service service.Admin) *MerchantController { return &MerchantController{service: service} }

func (c *MerchantController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	value, err := c.service.Merchants(ctx, appmodel.MerchantQuery{Keyword: request.Keyword, Status: request.Status, Page: request.Page, PageSize: request.PageSize})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRes{Items: []api.Merchant{}, Page: value.Page, PageSize: value.PageSize, Total: value.Total}
	for _, item := range value.Items {
		out.Items = append(out.Items, projectMerchant(item))
	}
	return &out, nil
}

func (c *MerchantController) Create(ctx context.Context, request *api.CreateReq) (*api.CreateRes, error) {
	value, err := c.service.CreateMerchant(ctx, appmodel.CreateMerchant{
		CommandKey: request.CommandKey, Account: request.Account, Password: request.Password,
		Name: request.Name, ContactName: request.ContactName, ContactPhone: request.ContactPhone,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateRes{Merchant: projectMerchant(value.Merchant), ShopID: value.ShopID, ShopCode: value.ShopCode, Account: value.Account, Replayed: value.Replayed}, nil
}

func (c *MerchantController) Update(ctx context.Context, request *api.UpdateReq) (*api.UpdateRes, error) {
	value, err := c.service.UpdateMerchant(ctx, appmodel.UpdateMerchant{
		MerchantID: request.MerchantID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
		Name: request.Name, Status: request.Status, ContactName: request.ContactName, ContactPhone: request.ContactPhone,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.UpdateRes{Merchant: projectMerchant(value.Merchant), Replayed: value.Replayed}, nil
}

func (c *MerchantController) ResetPassword(ctx context.Context, request *api.ResetPasswordReq) (*api.ResetPasswordRes, error) {
	replayed, err := c.service.ResetMerchantPassword(ctx, appmodel.ResetMerchantPassword{MerchantID: request.MerchantID, CommandKey: request.CommandKey, Password: request.Password})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.ResetPasswordRes{Replayed: replayed}, nil
}

func (c *MerchantController) Close(ctx context.Context, request *api.CloseReq) (*api.CloseRes, error) {
	value, err := c.service.CloseMerchant(ctx, appmodel.CloseMerchant{MerchantID: request.MerchantID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CloseRes{Merchant: projectMerchant(value.Merchant), Replayed: value.Replayed}, nil
}

func (c *MerchantController) ListShops(ctx context.Context, request *api.ListShopsReq) (*api.ListShopsRes, error) {
	values, err := c.service.MerchantShops(ctx, request.MerchantID)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListShopsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Shop{ShopID: value.ID, MerchantID: value.MerchantID, Name: value.Name, Code: value.Code, Status: value.Status})
	}
	return &out, nil
}

func (c *MerchantController) ListPlans(ctx context.Context, _ *api.ListPlansReq) (*api.ListPlansRes, error) {
	values, err := c.service.SubscriptionPlans(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListPlansRes, 0, len(values))
	for _, value := range values {
		if value.Status != "ACTIVE" {
			continue
		}
		out = append(out, api.Plan{ID: value.ID, Code: value.Code, Name: value.Name, DurationDays: value.DurationDays, Status: value.Status, Default: value.Default})
	}
	return &out, nil
}

func (c *MerchantController) GetSubscription(ctx context.Context, request *api.GetSubscriptionReq) (*api.GetSubscriptionRes, error) {
	value, err := c.service.MerchantSubscription(ctx, request.MerchantID)
	if err != nil {
		return nil, web.Failure(err)
	}
	response := api.GetSubscriptionRes(projectSubscription(value))
	return &response, nil
}

func (c *MerchantController) PutSubscription(ctx context.Context, request *api.PutSubscriptionReq) (*api.PutSubscriptionRes, error) {
	value, err := c.service.AssignMerchantSubscription(ctx, appmodel.AssignMerchantSubscription{
		MerchantID: request.MerchantID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, PlanID: request.PlanID,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.PutSubscriptionRes{Assignment: projectSubscription(value.Assignment), Replayed: value.Replayed}, nil
}

func (c *MerchantController) GetPaymentChannels(ctx context.Context, request *api.GetPaymentChannelsReq) (*api.GetPaymentChannelsRes, error) {
	value, err := c.service.MerchantPaymentChannels(ctx, request.MerchantID, request.ShopID)
	if err != nil {
		return nil, web.Failure(err)
	}
	response := api.GetPaymentChannelsRes(projectPayment(value))
	return &response, nil
}

func (c *MerchantController) PutPaymentChannels(ctx context.Context, request *api.PutPaymentChannelsReq) (*api.PutPaymentChannelsRes, error) {
	channels := make([]appmodel.MerchantPaymentGrant, 0, len(request.Channels))
	for _, item := range request.Channels {
		channels = append(channels, appmodel.MerchantPaymentGrant{ChannelCode: item.ChannelCode, Name: item.Name, Enabled: item.Enabled, Priority: item.Priority})
	}
	value, err := c.service.PutMerchantPaymentChannels(ctx, appmodel.PutMerchantPaymentChannels{
		MerchantID: request.MerchantID, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Channels: channels,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	response := api.PutPaymentChannelsRes(projectPayment(value))
	return &response, nil
}

func (c *MerchantController) GetSMSRegions(ctx context.Context, request *api.GetSMSRegionsReq) (*api.GetSMSRegionsRes, error) {
	value, err := c.service.MerchantSMSRegions(ctx, request.MerchantID, request.ShopID)
	if err != nil {
		return nil, web.Failure(err)
	}
	response := api.GetSMSRegionsRes(projectSMS(value))
	return &response, nil
}

func (c *MerchantController) PutSMSRegions(ctx context.Context, request *api.PutSMSRegionsReq) (*api.PutSMSRegionsRes, error) {
	value, err := c.service.PutMerchantSMSRegions(ctx, appmodel.PutMerchantSMSRegions{
		MerchantID: request.MerchantID, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, DialCodes: request.DialCodes,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	response := api.PutSMSRegionsRes(projectSMS(value))
	return &response, nil
}

func (c *MerchantController) GetLiveProviders(ctx context.Context, request *api.GetLiveProvidersReq) (*api.GetLiveProvidersRes, error) {
	value, err := c.service.MerchantLiveProviders(ctx, request.MerchantID)
	if err != nil {
		return nil, web.Failure(err)
	}
	response := api.GetLiveProvidersRes(projectLive(value))
	return &response, nil
}

func (c *MerchantController) PutLiveProviders(ctx context.Context, request *api.PutLiveProvidersReq) (*api.PutLiveProvidersRes, error) {
	providers := make([]appmodel.MerchantLiveAssignment, 0, len(request.Providers))
	for _, item := range request.Providers {
		providers = append(providers, appmodel.MerchantLiveAssignment{ProviderCode: item.ProviderCode, Name: item.Name, Enabled: item.Enabled, Default: item.Default})
	}
	value, err := c.service.PutMerchantLiveProviders(ctx, appmodel.PutMerchantLiveProviders{
		MerchantID: request.MerchantID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Providers: providers,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	response := api.PutLiveProvidersRes(projectLive(value))
	return &response, nil
}

func projectMerchant(value appmodel.ManagedMerchant) api.Merchant {
	return api.Merchant{MerchantID: value.ID, Name: value.Name, ExternalID: value.ExternalID, Account: value.Account,
		ContactName: value.ContactName, ContactPhone: value.ContactPhone, Status: value.Status, Version: value.Version,
		ShopID: value.ShopID, ShopCode: value.ShopCode}
}

func projectSubscription(value appmodel.MerchantSubscription) api.Subscription {
	return api.Subscription{MerchantID: value.MerchantID, PlanID: value.PlanID, PlanCode: value.PlanCode, PlanName: value.PlanName,
		ExpiresAt: value.ExpiresAt, Version: value.Version, DurationDays: value.DurationDays}
}

func projectSMS(value appmodel.MerchantSMSRegions) api.SMSRegions {
	out := api.SMSRegions{MerchantID: value.MerchantID, ShopID: value.ShopID, DialCodes: value.DialCodes, Unrestricted: value.Unrestricted, Regions: []api.SMSRegionOption{}, Version: value.Version}
	for _, item := range value.Regions {
		out.Regions = append(out.Regions, api.SMSRegionOption{DialCode: item.DialCode, Name: item.Name, ISO2: item.ISO2, Emoji: item.Emoji, Enabled: item.Enabled})
	}
	return out
}

func projectPayment(value appmodel.MerchantPaymentChannels) api.PaymentChannels {
	out := api.PaymentChannels{MerchantID: value.MerchantID, ShopID: value.ShopID, Channels: []api.PaymentGrant{}, Version: value.Version}
	for _, item := range value.Channels {
		out.Channels = append(out.Channels, api.PaymentGrant{ChannelCode: item.ChannelCode, Name: item.Name, Enabled: item.Enabled, Priority: item.Priority})
	}
	return out
}

func projectLive(value appmodel.MerchantLiveProviders) api.LiveProviders {
	out := api.LiveProviders{MerchantID: value.MerchantID, Providers: []api.LiveAssignment{}, Version: value.Version}
	for _, item := range value.Providers {
		out.Providers = append(out.Providers, api.LiveAssignment{ProviderCode: item.ProviderCode, Name: item.Name, Enabled: item.Enabled, Default: item.Default})
	}
	return out
}
