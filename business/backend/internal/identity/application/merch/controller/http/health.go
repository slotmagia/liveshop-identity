// Package http adapts the merch HTTP contract to its application
// boundary. It performs no authorization and reaches no storage.
package http

import (
	"context"

	apihealth "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/health"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type HealthController struct{ service service.Merch }

func NewHealth(application service.Merch) *HealthController {
	return &HealthController{service: application}
}

func (c *HealthController) Read(ctx context.Context, _ *apihealth.ReadReq) (*apihealth.ReadRes, error) {
	current, err := c.service.Health(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	return &apihealth.ReadRes{Status: string(current.Status)}, nil
}
