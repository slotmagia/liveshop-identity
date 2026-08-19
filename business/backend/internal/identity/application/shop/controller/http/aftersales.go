package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/api/http/v1/aftersales"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type AftersaleController struct{ service service.Shop }

func NewAftersales(application service.Shop) *AftersaleController {
	return &AftersaleController{application}
}

func (c *AftersaleController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	page, err := c.service.Aftersales(ctx, appmodel.AftersaleQuery{
		Status: request.Status, Type: request.Type, Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := &api.ListRes{Items: make([]api.Aftersale, 0, len(page.Items)), Page: page.Page, PageSize: page.PageSize, Total: page.Total}
	for _, item := range page.Items {
		out.Items = append(out.Items, wireAftersale(item))
	}
	return out, nil
}

func (c *AftersaleController) Get(ctx context.Context, request *api.GetReq) (*api.GetRes, error) {
	value, err := c.service.Aftersale(ctx, request.AftersaleId)
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.GetRes{Aftersale: wireAftersale(value)}, nil
}

func wireAftersale(value appmodel.Aftersale) api.Aftersale {
	items := make([]api.AftersaleItem, 0, len(value.Items))
	for _, item := range value.Items {
		items = append(items, api.AftersaleItem{
			ID: item.ID, SKUID: item.SKUID, Title: item.Title, Quantity: item.Quantity,
			RefundAmount: item.RefundAmount, ReceivedQuantity: item.ReceivedQuantity,
		})
	}
	return api.Aftersale{
		ID: value.ID, OrderID: value.OrderID, PaymentNo: value.PaymentNo, Type: value.Type,
		RequestedAmount: value.RequestedAmount, Amount: value.Amount, Reason: value.Reason,
		Status: value.Status, ReturnStatus: value.ReturnStatus, HandleNote: value.HandleNote, Items: items,
		Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		ReviewedAt: value.ReviewedAt, ReceivedAt: value.ReceivedAt,
	}
}
