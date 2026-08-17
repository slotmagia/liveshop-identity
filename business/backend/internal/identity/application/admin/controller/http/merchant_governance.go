package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/api/http/v1/merchantgovernance"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type MerchantGovernanceController struct{ service service.Admin }

func NewMerchantGovernance(service service.Admin) *MerchantGovernanceController {
	return &MerchantGovernanceController{service: service}
}

func (c *MerchantGovernanceController) Catalog(_ context.Context, _ *api.CatalogReq) (*api.CatalogRes, error) {
	values := c.service.GovernanceCatalog()
	out := make(api.CatalogRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Module{Key: value.Key, Label: value.Label})
	}
	return &out, nil
}

func (c *MerchantGovernanceController) ListMerchants(ctx context.Context, _ *api.ListMerchantsReq) (*api.ListMerchantsRes, error) {
	values, err := c.service.ShopMerchants(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListMerchantsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Merchant{MerchantID: value.ID, Name: value.Name, Status: value.Status})
	}
	return &out, nil
}

func (c *MerchantGovernanceController) ListShops(ctx context.Context, request *api.ListShopsReq) (*api.ListShopsRes, error) {
	values, err := c.service.MerchantShops(ctx, request.MerchantID)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListShopsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Shop{ShopID: value.ID, MerchantID: value.MerchantID,
			Name: value.Name, Code: value.Code, Status: value.Status})
	}
	return &out, nil
}

func (c *MerchantGovernanceController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	values, err := c.service.GovernanceCapabilities(ctx, appmodel.GovernanceQuery{MerchantID: request.MerchantID, ShopID: request.ShopID, Module: request.Module})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListRes, 0, len(values))
	for _, value := range values {
		out = append(out, projectGovernanceAPI(value))
	}
	return &out, nil
}

func (c *MerchantGovernanceController) Audit(ctx context.Context, request *api.AuditReq) (*api.AuditRes, error) {
	values, err := c.service.GovernanceAudit(ctx, appmodel.GovernanceQuery{MerchantID: request.MerchantID, ShopID: request.ShopID, Module: request.Module})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.AuditRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.AuditItem{
			ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID,
			Module: value.Module, CapabilityID: value.CapabilityID, Action: value.Action, Operator: value.Operator,
			ReasonInternal: value.ReasonInternal, ReasonPublic: value.ReasonPublic, CreatedAt: value.CreatedAt,
		})
	}
	return &out, nil
}

func (c *MerchantGovernanceController) Intervene(ctx context.Context, request *api.InterveneReq) (*api.InterveneRes, error) {
	value, err := c.service.InterveneGovernance(ctx, appmodel.InterveneGovernance{
		CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, MerchantID: request.MerchantID,
		ShopID: request.ShopID, Module: request.Module, PlatformStatus: request.PlatformStatus,
		ReasonInternal: request.ReasonInternal, ReasonPublic: request.ReasonPublic,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.InterveneRes{Capability: projectGovernanceAPI(value.Capability), Replayed: value.Replayed}, nil
}

func projectGovernanceAPI(value appmodel.GovernanceCapability) api.Capability {
	return api.Capability{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID,
		Module: value.Module, ModuleLabel: value.ModuleLabel, Name: value.Name, MerchantStatus: value.MerchantStatus,
		PlatformStatus: value.PlatformStatus, PlatformReasonPublic: value.PlatformReasonPublic, Version: value.Version,
		UpdatedBy: value.UpdatedBy, UpdatedAt: value.UpdatedAt,
	}
}
