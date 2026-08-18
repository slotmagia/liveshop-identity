package logic

import (
	"context"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	fulfillmentmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

func (l *Logic) Shipments(ctx context.Context, query appmodel.ShipmentQuery) (appmodel.ShipmentPage, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.ShipmentPage{}, err
	}
	if l.shipments == nil {
		return appmodel.ShipmentPage{}, model.ErrUnavailable
	}
	page, err := l.shipments.List(ctx, fulfillmentmodel.ShipmentQuery{
		MerchantID: merchantID, ShopID: shopID, OrderID: query.OrderID,
		Status: fulfillmentmodel.ShipmentStatus(query.Status), Page: query.Page, PageSize: query.PageSize,
	})
	if err != nil {
		return appmodel.ShipmentPage{}, err
	}
	out := appmodel.ShipmentPage{
		Items: make([]appmodel.Shipment, 0, len(page.Items)), Page: page.Page, PageSize: page.PageSize, Total: page.Total,
	}
	for _, item := range page.Items {
		out.Items = append(out.Items, projectShipment(item))
	}
	return out, nil
}

func (l *Logic) Shipment(ctx context.Context, shipmentID int64) (appmodel.Shipment, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.Shipment{}, err
	}
	if l.shipments == nil {
		return appmodel.Shipment{}, model.ErrUnavailable
	}
	value, err := l.shipments.Get(ctx, merchantID, shopID, shipmentID)
	if err != nil {
		return appmodel.Shipment{}, err
	}
	return projectShipment(value), nil
}

func (l *Logic) CreateShipment(ctx context.Context, input appmodel.CreateShipment) (appmodel.ShipmentResult, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.ShipmentResult{}, err
	}
	if l.shipments == nil {
		return appmodel.ShipmentResult{}, model.ErrUnavailable
	}
	value, replayed, err := l.shipments.Ship(ctx, fulfillmentmodel.ShipCommand{
		MerchantID: merchantID, ShopID: shopID, CommandKey: input.CommandKey,
		OrderID: input.OrderID, Carrier: input.Carrier, TrackingNo: input.TrackingNo,
	})
	if err != nil {
		return appmodel.ShipmentResult{}, err
	}
	return appmodel.ShipmentResult{Shipment: projectShipment(value), Replayed: replayed}, nil
}

func (l *Logic) CreateShipmentTrace(ctx context.Context, input appmodel.CreateShipmentTrace) (appmodel.ShipmentResult, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.ShipmentResult{}, err
	}
	if l.shipments == nil {
		return appmodel.ShipmentResult{}, model.ErrUnavailable
	}
	value, replayed, err := l.shipments.AddTrace(ctx, fulfillmentmodel.TraceCommand{
		ShipmentID: input.ShipmentID, MerchantID: merchantID, ShopID: shopID, CommandKey: input.CommandKey,
		ExpectedVersion: input.ExpectedVersion, Node: input.Node,
	})
	if err != nil {
		return appmodel.ShipmentResult{}, err
	}
	return appmodel.ShipmentResult{Shipment: projectShipment(value), Replayed: replayed}, nil
}

func (l *Logic) CloseShipment(ctx context.Context, input appmodel.CloseShipment) (appmodel.ShipmentResult, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.ShipmentResult{}, err
	}
	if l.shipments == nil {
		return appmodel.ShipmentResult{}, model.ErrUnavailable
	}
	value, replayed, err := l.shipments.Close(ctx, fulfillmentmodel.CloseShipmentCommand{
		ShipmentID: input.ShipmentID, MerchantID: merchantID, ShopID: shopID, CommandKey: input.CommandKey,
		ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return appmodel.ShipmentResult{}, err
	}
	return appmodel.ShipmentResult{Shipment: projectShipment(value), Replayed: replayed}, nil
}

func projectShipment(item fulfillmentmodel.Shipment) appmodel.Shipment {
	out := appmodel.Shipment{
		ID: item.ID, OrderID: item.OrderID, Carrier: item.Carrier, TrackingNo: item.TrackingNo,
		Status: string(item.Status), Traces: make([]appmodel.ShipmentTrace, 0, len(item.Traces)), Version: item.Version,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	for _, trace := range item.Traces {
		out.Traces = append(out.Traces, appmodel.ShipmentTrace{
			OccurredAt: trace.OccurredAt.UTC().Format(time.RFC3339), Node: trace.Node,
		})
	}
	return out
}
