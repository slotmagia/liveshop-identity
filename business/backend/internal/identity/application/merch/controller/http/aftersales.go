package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/aftersales"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type AftersaleQueryController struct{ service service.Merch }

func NewAftersaleQuery(s service.Merch) *AftersaleQueryController {
	return &AftersaleQueryController{service: s}
}

func (c *AftersaleQueryController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	value, err := c.service.Aftersales(ctx, appmodel.AftersaleQuery{
		CustomerSubject: request.CustomerSubject, Status: request.Status, Type: request.Type,
		Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRes{Page: value.Page, PageSize: value.PageSize, Total: value.Total, Items: []api.Aftersale{}}
	for _, item := range value.Items {
		out.Items = append(out.Items, wireAftersale(item))
	}
	return &out, nil
}

func (c *AftersaleQueryController) Get(ctx context.Context, request *api.GetReq) (*api.GetRes, error) {
	value, err := c.service.Aftersale(ctx, request.AftersaleId)
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.GetRes{Aftersale: wireAftersale(value)}, nil
}

type AftersaleWriteController struct{ service service.Merch }

func NewAftersaleWrite(s service.Merch) *AftersaleWriteController {
	return &AftersaleWriteController{service: s}
}

func (c *AftersaleWriteController) Review(ctx context.Context, request *api.ReviewReq) (*api.ReviewRes, error) {
	value, err := c.service.ReviewAftersale(ctx, appmodel.ReviewAftersale{
		AftersaleID: request.AftersaleId, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
		Status: request.Status, Amount: request.Amount, HandleNote: request.HandleNote,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.ReviewRes{Aftersale: wireAftersale(value.Aftersale), Replayed: value.Replayed}, nil
}

func (c *AftersaleWriteController) CreateReturn(ctx context.Context, request *api.CreateReturnReq) (*api.CreateReturnRes, error) {
	value, err := c.service.ReceiveAftersale(ctx, appmodel.ReceiveAftersale{
		AftersaleID: request.AftersaleId, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateReturnRes{Aftersale: wireAftersale(value.Aftersale), Replayed: value.Replayed}, nil
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
		ID: value.ID, CustomerSubject: value.CustomerSubject, OrderID: value.OrderID, PaymentNo: value.PaymentNo,
		Type: value.Type, RequestedAmount: value.RequestedAmount, Amount: value.Amount, Reason: value.Reason,
		Status: value.Status, ReturnStatus: value.ReturnStatus, HandleNote: value.HandleNote, Items: items,
		Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		ReviewedAt: value.ReviewedAt, ReceivedAt: value.ReceivedAt,
	}
}
