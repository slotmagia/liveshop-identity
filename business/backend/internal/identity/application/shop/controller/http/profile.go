package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/api/http/v1/profile"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type ProfileController struct{ service service.Shop }

func NewProfile(application service.Shop) *ProfileController { return &ProfileController{application} }

func (c *ProfileController) Get(ctx context.Context, _ *api.GetReq) (*api.GetRes, error) {
	value, err := c.service.Profile(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.GetRes{Subject: value.Subject, PrincipalType: value.PrincipalType, SignedIn: value.SignedIn, DisplayName: value.DisplayName}, nil
}
