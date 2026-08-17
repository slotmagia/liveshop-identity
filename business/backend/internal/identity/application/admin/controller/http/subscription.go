package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/api/http/v1/subscription"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type SubscriptionController struct{ service service.Admin }

func NewSubscription(service service.Admin) *SubscriptionController {
	return &SubscriptionController{service: service}
}

func (c *SubscriptionController) ListPlans(ctx context.Context, _ *api.ListPlansReq) (*api.ListPlansRes, error) {
	values, err := c.service.SubscriptionPlans(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListPlansRes, 0, len(values))
	for _, value := range values {
		out = append(out, wireSubscriptionPlan(value))
	}
	return &out, nil
}

func (c *SubscriptionController) CreatePlan(ctx context.Context, request *api.CreatePlanReq) (*api.SavePlanRes, error) {
	return c.save(ctx, request.CommandKey, request.ExpectedVersion, appmodel.SubscriptionPlan{
		Code: request.Code, Name: request.Name, Level: request.Level, PriceMinor: request.PriceMinor,
		DurationDays: request.DurationDays, Description: request.Description, Default: request.Default,
		Sort: request.Sort, Status: request.Status,
	})
}

func (c *SubscriptionController) UpdatePlan(ctx context.Context, request *api.UpdatePlanReq) (*api.SavePlanRes, error) {
	return c.save(ctx, request.CommandKey, request.ExpectedVersion, appmodel.SubscriptionPlan{
		ID: request.PlanID, Code: request.Code, Name: request.Name, Level: request.Level, PriceMinor: request.PriceMinor,
		DurationDays: request.DurationDays, Description: request.Description, Default: request.Default,
		Sort: request.Sort, Status: request.Status,
	})
}

func (c *SubscriptionController) save(ctx context.Context, commandKey string, expectedVersion uint64, plan appmodel.SubscriptionPlan) (*api.SavePlanRes, error) {
	result, err := c.service.SaveSubscriptionPlan(ctx, appmodel.SaveSubscriptionPlan{CommandKey: commandKey, ExpectedVersion: expectedVersion, Plan: plan})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.SavePlanRes{Plan: wireSubscriptionPlan(result.Plan), Replayed: result.Replayed}, nil
}

func (c *SubscriptionController) RetirePlan(ctx context.Context, request *api.RetirePlanReq) (*api.RetirePlanRes, error) {
	result, err := c.service.RetireSubscriptionPlan(ctx, appmodel.RetireSubscriptionPlan{PlanID: request.PlanID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion})
	if err != nil {
		return nil, web.Failure(err)
	}
	response := api.RetirePlanRes{Plan: wireSubscriptionPlan(result.Plan), Replayed: result.Replayed}
	return &response, nil
}

func (c *SubscriptionController) ListPermissions(ctx context.Context, _ *api.ListPermissionsReq) (*api.ListPermissionsRes, error) {
	values, err := c.service.SubscriptionPermissions(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListPermissionsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Permission{ModuleID: value.ModuleID, Code: value.Code, Name: value.Name, Resource: value.Resource, Action: value.Action, Description: value.Description, RegistryRevision: value.RegistryRevision})
	}
	return &out, nil
}

func (c *SubscriptionController) GetPlanPermissions(ctx context.Context, request *api.GetPlanPermissionsReq) (*api.GetPlanPermissionsRes, error) {
	value, err := c.service.SubscriptionPlanPolicy(ctx, request.PlanID)
	if err != nil {
		return nil, web.Failure(err)
	}
	response := api.GetPlanPermissionsRes(wireSubscriptionPlanPolicy(value))
	return &response, nil
}

func (c *SubscriptionController) PutPlanPermissions(ctx context.Context, request *api.PutPlanPermissionsReq) (*api.PutPlanPermissionsRes, error) {
	result, err := c.service.SaveSubscriptionPlanPolicy(ctx, appmodel.SaveSubscriptionPlanPolicy{
		CommandKey: request.CommandKey, ExpectedRevision: request.ExpectedRevision,
		Policy: appmodel.SubscriptionPlanPolicy{PlanID: request.PlanID, PermissionCodes: request.PermissionCodes, ProductLimit: request.ProductLimit},
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.PutPlanPermissionsRes{Policy: wireSubscriptionPlanPolicy(result.Policy), Replayed: result.Replayed}, nil
}

func wireSubscriptionPlan(value appmodel.SubscriptionPlan) api.Plan {
	return api.Plan{ID: value.ID, Code: value.Code, Name: value.Name, Level: value.Level, PriceMinor: value.PriceMinor,
		DurationDays: value.DurationDays, Description: value.Description, Default: value.Default, Sort: value.Sort,
		Status: value.Status, Version: value.Version}
}

func wireSubscriptionPlanPolicy(value appmodel.SubscriptionPlanPolicy) api.PlanPermissionPolicy {
	return api.PlanPermissionPolicy{PlanID: value.PlanID, PlanCode: value.PlanCode, PlanName: value.PlanName,
		PermissionCodes: value.PermissionCodes, ProductLimit: value.ProductLimit, Revision: value.Revision}
}
