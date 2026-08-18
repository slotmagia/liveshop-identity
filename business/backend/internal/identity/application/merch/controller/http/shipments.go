package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/shipments"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type ShipmentQueryController struct{ service service.Merch }

func NewShipmentQuery(s service.Merch) *ShipmentQueryController {
	return &ShipmentQueryController{service: s}
}

func (c *ShipmentQueryController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	value, err := c.service.Shipments(ctx, appmodel.ShipmentQuery{
		OrderID: request.OrderId, Status: request.Status, Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRes{Page: value.Page, PageSize: value.PageSize, Total: value.Total, Items: []api.Shipment{}}
	for _, item := range value.Items {
		out.Items = append(out.Items, wireShipment(item))
	}
	return &out, nil
}

func (c *ShipmentQueryController) Get(ctx context.Context, request *api.GetReq) (*api.GetRes, error) {
	value, err := c.service.Shipment(ctx, request.ShipmentId)
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.GetRes{Shipment: wireShipment(value)}, nil
}

type ShipmentWriteController struct{ service service.Merch }

func NewShipmentWrite(s service.Merch) *ShipmentWriteController {
	return &ShipmentWriteController{service: s}
}

func (c *ShipmentWriteController) Create(ctx context.Context, request *api.CreateReq) (*api.CreateRes, error) {
	value, err := c.service.CreateShipment(ctx, appmodel.CreateShipment{
		CommandKey: request.CommandKey, OrderID: request.OrderId, Carrier: request.Carrier, TrackingNo: request.TrackingNo,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateRes{Shipment: wireShipment(value.Shipment), Replayed: value.Replayed}, nil
}

func (c *ShipmentWriteController) CreateTrace(ctx context.Context, request *api.CreateTraceReq) (*api.CreateTraceRes, error) {
	value, err := c.service.CreateShipmentTrace(ctx, appmodel.CreateShipmentTrace{
		ShipmentID: request.ShipmentId, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Node: request.Node,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateTraceRes{Shipment: wireShipment(value.Shipment), Replayed: value.Replayed}, nil
}

func (c *ShipmentWriteController) Close(ctx context.Context, request *api.CloseReq) (*api.CloseRes, error) {
	value, err := c.service.CloseShipment(ctx, appmodel.CloseShipment{
		ShipmentID: request.ShipmentId, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CloseRes{Shipment: wireShipment(value.Shipment), Replayed: value.Replayed}, nil
}

func wireShipment(value appmodel.Shipment) api.Shipment {
	traces := make([]api.Trace, 0, len(value.Traces))
	for _, item := range value.Traces {
		traces = append(traces, api.Trace{OccurredAt: item.OccurredAt, Node: item.Node})
	}
	return api.Shipment{
		ID: value.ID, OrderID: value.OrderID, Carrier: value.Carrier, TrackingNo: value.TrackingNo,
		Status: value.Status, Traces: traces, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}
