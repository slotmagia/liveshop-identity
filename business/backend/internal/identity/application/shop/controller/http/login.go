package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/api/http/v1/login"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type LoginController struct{ service service.Shop }

func NewLogin(application service.Shop) *LoginController {
	return &LoginController{service: application}
}

func (c *LoginController) CreateOTP(ctx context.Context, request *api.CreateOTPReq) (*api.CreateOTPRes, error) {
	value, err := c.service.CreateLoginOTP(ctx, appmodel.CreateLoginOTP{ShopCode: request.ShopCode, Channel: request.Channel, Phone: request.Phone, Email: request.Email})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateOTPRes{
		ChallengeID:        value.ChallengeID,
		TTLSeconds:         value.TTLSeconds,
		ExpiresAt:          value.ExpiresAt,
		ResendAfterSeconds: value.ResendAfterSeconds,
		NextSendAt:         value.NextSendAt,
	}, nil
}

func (c *LoginController) Create(ctx context.Context, request *api.CreateReq) (*api.CreateRes, error) {
	value, err := c.service.CreateLogin(ctx, appmodel.CreateLogin{ShopCode: request.ShopCode, ChallengeID: request.ChallengeID, Code: request.Code})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateRes{ChallengeID: value.ChallengeID, Verified: value.Verified}, nil
}

func (c *LoginController) ListSMSRegions(ctx context.Context, request *api.ListSMSRegionsReq) (*api.ListSMSRegionsRes, error) {
	value, err := c.service.LoginSMSRegions(ctx, request.ShopCode)
	if err != nil {
		return nil, web.Failure(err)
	}
	response := &api.ListSMSRegionsRes{Items: make([]api.SMSRegion, 0, len(value.Items)), Unrestricted: value.Unrestricted}
	for _, item := range value.Items {
		response.Items = append(response.Items, api.SMSRegion{DialCode: item.DialCode, Name: item.Name, ISO2: item.ISO2, Emoji: item.Emoji})
	}
	return response, nil
}
