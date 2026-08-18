package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment"
	fulfillmentmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type stubShipmentRepo struct {
	query   fulfillmentmodel.ShipmentQuery
	page    fulfillmentmodel.ShipmentPage
	item    fulfillmentmodel.Shipment
	shipped fulfillmentmodel.ShipCommand
	traced  fulfillmentmodel.TraceCommand
	closed  fulfillmentmodel.CloseShipmentCommand
	err     error
}

func (s *stubShipmentRepo) ListShipments(_ context.Context, query fulfillmentmodel.ShipmentQuery) (fulfillmentmodel.ShipmentPage, error) {
	s.query = query
	if s.err != nil {
		return fulfillmentmodel.ShipmentPage{}, s.err
	}
	if s.page.Page == 0 {
		return fulfillmentmodel.ShipmentPage{Items: []fulfillmentmodel.Shipment{}, Page: query.Page, PageSize: query.PageSize}, nil
	}
	return s.page, nil
}

func (s *stubShipmentRepo) GetShipment(_ context.Context, _, _, _ int64) (fulfillmentmodel.Shipment, error) {
	if s.err != nil {
		return fulfillmentmodel.Shipment{}, s.err
	}
	return s.item, nil
}

func (s *stubShipmentRepo) Ship(_ context.Context, command fulfillmentmodel.ShipCommand) (fulfillmentmodel.Shipment, bool, error) {
	s.shipped = command
	if s.err != nil {
		return fulfillmentmodel.Shipment{}, false, s.err
	}
	return s.item, false, nil
}

func (s *stubShipmentRepo) AddTrace(_ context.Context, command fulfillmentmodel.TraceCommand) (fulfillmentmodel.Shipment, bool, error) {
	s.traced = command
	if s.err != nil {
		return fulfillmentmodel.Shipment{}, false, s.err
	}
	return s.item, false, nil
}

func (s *stubShipmentRepo) CloseShipment(_ context.Context, command fulfillmentmodel.CloseShipmentCommand) (fulfillmentmodel.Shipment, bool, error) {
	s.closed = command
	if s.err != nil {
		return fulfillmentmodel.Shipment{}, false, s.err
	}
	return s.item, false, nil
}

func merchShipmentLogic(repo *stubShipmentRepo) *Logic {
	return New(nil, nil, nil, nil, nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil, nil, nil, nil, nil, fulfillment.NewShipments(repo), nil)
}

func merchShipmentContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{MerchantID: 2001, ShopID: 3001})
}

func sampleMerchShipment(status fulfillmentmodel.ShipmentStatus, version uint64) fulfillmentmodel.Shipment {
	return fulfillmentmodel.Shipment{
		ID: 11, MerchantID: 2001, ShopID: 3001, OrderID: 8801, Carrier: "顺丰速运", TrackingNo: "SF1234567890",
		Status: status, Version: version,
		Traces:    []fulfillmentmodel.Trace{{OccurredAt: time.Unix(1, 0).UTC(), Node: "已揽收"}},
		CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(2, 0).UTC(),
	}
}

func TestShipmentsRequireShopContext(t *testing.T) {
	if _, err := merchShipmentLogic(&stubShipmentRepo{}).Shipments(context.Background(), appmodel.ShipmentQuery{}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestShipmentsUseSessionShopAndProjectRows(t *testing.T) {
	repo := &stubShipmentRepo{page: fulfillmentmodel.ShipmentPage{Page: 1, PageSize: 20, Total: 1, Items: []fulfillmentmodel.Shipment{
		sampleMerchShipment(fulfillmentmodel.ShipmentShipped, 1),
	}}}
	page, err := merchShipmentLogic(repo).Shipments(merchShipmentContext(), appmodel.ShipmentQuery{OrderID: 8801, Status: "SHIPPED", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if repo.query.MerchantID != 2001 || repo.query.ShopID != 3001 || repo.query.OrderID != 8801 || repo.query.Status != fulfillmentmodel.ShipmentShipped {
		t.Fatalf("query=%+v", repo.query)
	}
	if page.Total != 1 || page.Items[0].OrderID != 8801 || page.Items[0].Status != "SHIPPED" || page.Items[0].CreatedAt != "1970-01-01T00:00:01Z" {
		t.Fatalf("page=%+v", page)
	}
}

func TestShipmentsRejectClosedShop(t *testing.T) {
	_, err := merchShipmentLogic(&stubShipmentRepo{err: fulfillmentmodel.ErrNotFound}).Shipments(merchShipmentContext(), appmodel.ShipmentQuery{})
	if !errors.Is(err, fulfillmentmodel.ErrNotFound) {
		t.Fatalf("error=%v", err)
	}
}

func TestCreateShipmentUsesSessionShop(t *testing.T) {
	repo := &stubShipmentRepo{item: sampleMerchShipment(fulfillmentmodel.ShipmentShipped, 1)}
	value, err := merchShipmentLogic(repo).CreateShipment(merchShipmentContext(), appmodel.CreateShipment{
		CommandKey: "ship-0001", OrderID: 8801, Carrier: "顺丰速运", TrackingNo: "SF1234567890",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.shipped.MerchantID != 2001 || repo.shipped.ShopID != 3001 || repo.shipped.OrderID != 8801 {
		t.Fatalf("command=%+v", repo.shipped)
	}
	if value.Shipment.Status != "SHIPPED" || value.Replayed || value.Shipment.TrackingNo != "SF1234567890" {
		t.Fatalf("value=%+v", value)
	}
}

func TestCreateShipmentTraceUsesSessionShop(t *testing.T) {
	item := sampleMerchShipment(fulfillmentmodel.ShipmentShipped, 2)
	item.Traces = append(item.Traces, fulfillmentmodel.Trace{OccurredAt: time.Unix(2, 0).UTC(), Node: "运输中"})
	repo := &stubShipmentRepo{item: item}
	value, err := merchShipmentLogic(repo).CreateShipmentTrace(merchShipmentContext(), appmodel.CreateShipmentTrace{
		ShipmentID: 11, CommandKey: "trace-0001", ExpectedVersion: 1, Node: "运输中",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.traced.MerchantID != 2001 || repo.traced.ShopID != 3001 || repo.traced.ShipmentID != 11 || repo.traced.Node != "运输中" {
		t.Fatalf("command=%+v", repo.traced)
	}
	if value.Shipment.Version != 2 || len(value.Shipment.Traces) != 2 {
		t.Fatalf("value=%+v", value)
	}
}

func TestCloseShipmentUsesSessionShop(t *testing.T) {
	item := sampleMerchShipment(fulfillmentmodel.ShipmentDelivered, 2)
	repo := &stubShipmentRepo{item: item}
	value, err := merchShipmentLogic(repo).CloseShipment(merchShipmentContext(), appmodel.CloseShipment{
		ShipmentID: 11, CommandKey: "close-0001", ExpectedVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.closed.MerchantID != 2001 || repo.closed.ShopID != 3001 || repo.closed.ShipmentID != 11 {
		t.Fatalf("command=%+v", repo.closed)
	}
	if value.Shipment.Status != "DELIVERED" || value.Replayed {
		t.Fatalf("value=%+v", value)
	}
}
