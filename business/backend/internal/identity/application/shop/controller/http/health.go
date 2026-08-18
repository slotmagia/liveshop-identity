package http

import (
	"context"

	apihealth "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/api/http/v1/health"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type HealthController struct{ service service.Shop }

func NewHealth(application service.Shop) *HealthController {
	return &HealthController{service: application}
}

func (c *HealthController) Read(ctx context.Context, _ *apihealth.ReadReq) (*apihealth.ReadRes, error) {
	current, err := c.service.Health(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	return &apihealth.ReadRes{Status: string(current.Status)}, nil
}
