package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/subscription"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type SubscriptionQueryController struct{ service service.Merch }

func NewSubscriptionQuery(s service.Merch) *SubscriptionQueryController {
	return &SubscriptionQueryController{service: s}
}

func (c *SubscriptionQueryController) ListPlans(ctx context.Context, _ *api.ListPlansReq) (*api.ListPlansRes, error) {
	value, err := c.service.SubscriptionPlans(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListPlansRes{Items: []api.Plan{}, CurrentPlanID: value.CurrentPlanID}
	for _, item := range value.Items {
		out.Items = append(out.Items, wirePlan(item))
	}
	return &out, nil
}

func (c *SubscriptionQueryController) Get(ctx context.Context, _ *api.GetReq) (*api.GetRes, error) {
	value, err := c.service.Subscription(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	res := api.GetRes(wireCurrent(value))
	return &res, nil
}

func (c *SubscriptionQueryController) ListPayMethods(ctx context.Context, request *api.ListPayMethodsReq) (*api.ListPayMethodsRes, error) {
	values, err := c.service.SubscriptionPayMethods(ctx, request.PlanID)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListPayMethodsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.PayMethod{
			ChannelCode: value.ChannelCode, DisplayName: value.DisplayName, TypeCode: value.TypeCode, DriverKey: value.DriverKey,
		})
	}
	return &out, nil
}

func (c *SubscriptionQueryController) ListOrders(ctx context.Context, request *api.ListOrdersReq) (*api.ListOrdersRes, error) {
	value, err := c.service.SubscriptionOrders(ctx, appmodel.SubscriptionOrderQuery{
		Status: request.Status, Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListOrdersRes{Items: []api.Order{}, Page: value.Page, PageSize: value.PageSize, Total: value.Total, Owner: value.Owner}
	for _, item := range value.Items {
		out.Items = append(out.Items, wireOrder(item))
	}
	return &out, nil
}

type SubscriptionWriteController struct{ service service.Merch }

func NewSubscriptionWrite(s service.Merch) *SubscriptionWriteController {
	return &SubscriptionWriteController{service: s}
}

func (c *SubscriptionWriteController) Create(ctx context.Context, request *api.CreateOrderReq) (*api.CreateOrderRes, error) {
	value, err := c.service.CreateSubscriptionOrder(ctx, appmodel.CreateSubscriptionOrder{
		CommandKey: request.CommandKey, PlanID: request.PlanID, ChannelCode: request.ChannelCode,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	res := api.CreateOrderRes(wireOrder(value))
	return &res, nil
}

func (c *SubscriptionWriteController) GetOrder(ctx context.Context, request *api.GetOrderReq) (*api.GetOrderRes, error) {
	value, err := c.service.SubscriptionOrder(ctx, request.OrderNo)
	if err != nil {
		return nil, web.Failure(err)
	}
	res := api.GetOrderRes(wireOrder(value))
	return &res, nil
}

func (c *SubscriptionWriteController) Confirm(ctx context.Context, request *api.ConfirmOrderReq) (*api.ConfirmOrderRes, error) {
	value, err := c.service.ConfirmSubscriptionOrder(ctx, appmodel.ConfirmSubscriptionOrder{
		CommandKey: request.CommandKey, OrderNo: request.OrderNo,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	res := api.ConfirmOrderRes(wireOrder(value))
	return &res, nil
}

func (c *SubscriptionWriteController) Close(ctx context.Context, request *api.CloseOrderReq) (*api.CloseOrderRes, error) {
	value, err := c.service.CloseSubscriptionOrder(ctx, appmodel.CloseSubscriptionOrder{
		CommandKey: request.CommandKey, OrderNo: request.OrderNo,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	res := api.CloseOrderRes(wireOrder(value))
	return &res, nil
}

func wirePlan(value appmodel.SubscriptionPlan) api.Plan {
	names := value.PermissionNames
	if names == nil {
		names = []string{}
	}
	return api.Plan{
		ID: value.ID, Code: value.Code, Name: value.Name, Level: value.Level, PriceMinor: value.PriceMinor,
		DurationDays: value.DurationDays, Description: value.Description, Default: value.Default, Current: value.Current,
		Buyable: value.Buyable, ProductLimit: value.ProductLimit, PermissionNames: names,
	}
}

func wireCurrent(value appmodel.SubscriptionCurrent) api.Current {
	names := value.PermissionNames
	if names == nil {
		names = []string{}
	}
	return api.Current{
		MerchantID: value.MerchantID, PlanID: value.PlanID, PlanCode: value.PlanCode, PlanName: value.PlanName,
		ExpiresAt: value.ExpiresAt, Version: value.Version, ProductLimit: value.ProductLimit,
		QuotaConfigured: value.QuotaConfigured, PermissionNames: names,
	}
}

func wireOrder(value appmodel.SubscriptionOrder) api.Order {
	payload := value.Payload
	if payload == nil {
		payload = map[string]string{}
	}
	return api.Order{
		OrderNo: value.OrderNo, PlanID: value.PlanID, PlanCode: value.PlanCode, PlanName: value.PlanName,
		PriceMinor: value.PriceMinor, DurationDays: value.DurationDays, Status: value.Status, PayNo: value.PayNo,
		ChannelCode: value.ChannelCode, DriverKey: value.DriverKey, PayStatus: value.PayStatus, Payload: payload,
		Activated: value.Activated, ExpiresAt: value.ExpiresAt, CreatedAt: value.CreatedAt, PaidAt: value.PaidAt, Replayed: value.Replayed,
	}
}
