package http

import (
	"context"
	"time"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/policy"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type PolicyQueryController struct{ service service.Merch }

func NewPolicyQuery(s service.Merch) *PolicyQueryController {
	return &PolicyQueryController{service: s}
}

func (c *PolicyQueryController) ListShops(ctx context.Context, _ *api.ListShopsReq) (*api.ListShopsRes, error) {
	values, err := c.service.PolicyShops(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListShopsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Shop{
			ShopID: value.ShopID, MerchantID: value.MerchantID, Name: value.Name, Code: value.Code, Status: value.Status,
		})
	}
	return &out, nil
}

func (c *PolicyQueryController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	value, err := c.service.Policies(ctx, appmodel.PolicyQuery{
		ShopID: request.ShopID, PolicyType: request.PolicyType, Status: request.Status,
		Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRes{
		Page: value.Page, PageSize: value.PageSize, Total: value.Total, Items: []api.Policy{},
		PlatformStatus: value.PlatformStatus, PlatformReasonPublic: value.PlatformReason,
	}
	for _, item := range value.Items {
		out.Items = append(out.Items, wirePolicy(item))
	}
	return &out, nil
}

type PolicyWriteController struct{ service service.Merch }

func NewPolicyWrite(s service.Merch) *PolicyWriteController {
	return &PolicyWriteController{service: s}
}

func (c *PolicyWriteController) Create(ctx context.Context, request *api.CreateReq) (*api.CreateRes, error) {
	value, err := c.service.SavePolicy(ctx, appmodel.SavePolicy{
		CommandKey: request.CommandKey, ShopID: request.ShopID, PolicyType: request.PolicyType,
		Title: request.Title, Content: request.Content, Publish: request.Publish,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateRes{Policy: wirePolicy(value.Policy), Replayed: value.Replayed}, nil
}

func (c *PolicyWriteController) Publish(ctx context.Context, request *api.PublishReq) (*api.PublishRes, error) {
	value, err := c.service.PublishPolicy(ctx, appmodel.PublishPolicy{
		PolicyID: request.PolicyId, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.PublishRes{Policy: wirePolicy(value.Policy), Replayed: value.Replayed}, nil
}

func wirePolicy(value appmodel.Policy) api.Policy {
	return api.Policy{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID,
		PolicyType: value.PolicyType, Title: value.Title, Content: value.Content,
		VersionNo: value.VersionNo, Status: value.Status, Version: value.Version,
		PublishedAt: formatPolicyTimePtr(value.PublishedAt), CreatedAt: formatPolicyTime(value.CreatedAt),
		UpdatedAt: formatPolicyTime(value.UpdatedAt), PlatformStatus: value.PlatformStatus,
		PlatformReasonPublic: value.PlatformReason, Editable: value.Status == "DRAFT" && (value.PlatformStatus == "" || value.PlatformStatus == "active"),
	}
}

func formatPolicyTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func formatPolicyTimePtr(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	formatted := formatPolicyTime(*value)
	return &formatted
}
