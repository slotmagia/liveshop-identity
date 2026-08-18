package fulfillment

import (
	"context"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

type shipmentRepositoryStub struct {
	query    model.ShipmentQuery
	page     model.ShipmentPage
	item     model.Shipment
	shipped  model.ShipCommand
	traced   model.TraceCommand
	closed   model.CloseShipmentCommand
	replayed bool
	listErr  error
	getErr   error
	writeErr error
}

func (s *shipmentRepositoryStub) ListShipments(_ context.Context, query model.ShipmentQuery) (model.ShipmentPage, error) {
	s.query = query
	if s.listErr != nil {
		return model.ShipmentPage{}, s.listErr
	}
	if s.page.Page == 0 {
		return model.ShipmentPage{Items: []model.Shipment{}, Page: query.Page, PageSize: query.PageSize}, nil
	}
	return s.page, nil
}

func (s *shipmentRepositoryStub) GetShipment(_ context.Context, _, _, _ int64) (model.Shipment, error) {
	if s.getErr != nil {
		return model.Shipment{}, s.getErr
	}
	return s.item, nil
}

func (s *shipmentRepositoryStub) Ship(_ context.Context, command model.ShipCommand) (model.Shipment, bool, error) {
	s.shipped = command
	if s.writeErr != nil {
		return model.Shipment{}, false, s.writeErr
	}
	return s.item, s.replayed, nil
}

func (s *shipmentRepositoryStub) AddTrace(_ context.Context, command model.TraceCommand) (model.Shipment, bool, error) {
	s.traced = command
	if s.writeErr != nil {
		return model.Shipment{}, false, s.writeErr
	}
	return s.item, s.replayed, nil
}

func (s *shipmentRepositoryStub) CloseShipment(_ context.Context, command model.CloseShipmentCommand) (model.Shipment, bool, error) {
	s.closed = command
	if s.writeErr != nil {
		return model.Shipment{}, false, s.writeErr
	}
	return s.item, s.replayed, nil
}

func sampleShipment() model.Shipment {
	return model.Shipment{
		ID: 11, MerchantID: 2001, ShopID: 3001, OrderID: 8801, Carrier: "顺丰速运", TrackingNo: "SF1234567890",
		Status: model.ShipmentShipped, Version: 1,
		Traces:    []model.Trace{{OccurredAt: time.Unix(1, 0).UTC(), Node: "已揽收"}},
		CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(1, 0).UTC(),
	}
}

func TestShipmentListRequiresShopScope(t *testing.T) {
	if _, err := NewShipments(&shipmentRepositoryStub{}).List(context.Background(), model.ShipmentQuery{Page: 1, PageSize: 20}); err != model.ErrInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestShipmentListPreservesShopScopeAndDefaultPage(t *testing.T) {
	item := sampleShipment()
	repository := &shipmentRepositoryStub{page: model.ShipmentPage{Page: 1, PageSize: 20, Total: 1, Items: []model.Shipment{item}}}
	page, err := NewShipments(repository).List(context.Background(), model.ShipmentQuery{MerchantID: 2001, ShopID: 3001})
	if err != nil || page.Total != 1 || repository.query.MerchantID != 2001 || repository.query.ShopID != 3001 || repository.query.Page != 1 || repository.query.PageSize != 20 {
		t.Fatalf("page=%+v query=%+v err=%v", page, repository.query, err)
	}
}

func TestShipmentListRejectsForeignShopRows(t *testing.T) {
	item := sampleShipment()
	item.ShopID = 4001
	repository := &shipmentRepositoryStub{page: model.ShipmentPage{Page: 1, PageSize: 20, Total: 1, Items: []model.Shipment{item}}}
	if _, err := NewShipments(repository).List(context.Background(), model.ShipmentQuery{MerchantID: 2001, ShopID: 3001, Page: 1, PageSize: 20}); err != model.ErrInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestShipmentGetUnavailableWithoutRepository(t *testing.T) {
	if _, err := NewShipments(nil).Get(context.Background(), 2001, 3001, 11); err != model.ErrUnavailable {
		t.Fatalf("error=%v", err)
	}
}

func TestShipRejectsEmptyCarrier(t *testing.T) {
	_, _, err := NewShipments(&shipmentRepositoryStub{item: sampleShipment()}).Ship(context.Background(), model.ShipCommand{
		MerchantID: 2001, ShopID: 3001, CommandKey: "ship-0001", OrderID: 8801, Carrier: " ", TrackingNo: "SF1234567890",
	})
	if err != model.ErrInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestShipReturnsPersistedResult(t *testing.T) {
	item := sampleShipment()
	repository := &shipmentRepositoryStub{item: item}
	value, replayed, err := NewShipments(repository).Ship(context.Background(), model.ShipCommand{
		MerchantID: 2001, ShopID: 3001, CommandKey: "ship-0001", OrderID: 8801, Carrier: "顺丰速运", TrackingNo: "SF1234567890",
	})
	if err != nil || replayed || value.ID != 11 || repository.shipped.OrderID != 8801 {
		t.Fatalf("value=%+v replayed=%v err=%v command=%+v", value, replayed, err, repository.shipped)
	}
}

func TestAddTraceReturnsPersistedResult(t *testing.T) {
	item := sampleShipment()
	item.Version = 2
	item.Traces = append(item.Traces, model.Trace{OccurredAt: time.Unix(2, 0).UTC(), Node: "运输中"})
	item.UpdatedAt = time.Unix(2, 0).UTC()
	repository := &shipmentRepositoryStub{item: item}
	value, replayed, err := NewShipments(repository).AddTrace(context.Background(), model.TraceCommand{
		ShipmentID: 11, MerchantID: 2001, ShopID: 3001, CommandKey: "trace-0001", ExpectedVersion: 1, Node: "运输中",
	})
	if err != nil || replayed || value.Version != 2 || repository.traced.Node != "运输中" {
		t.Fatalf("value=%+v replayed=%v err=%v command=%+v", value, replayed, err, repository.traced)
	}
}

func TestCloseReturnsPersistedResult(t *testing.T) {
	item := sampleShipment()
	item.Status = model.ShipmentDelivered
	item.Version = 2
	item.UpdatedAt = time.Unix(2, 0).UTC()
	repository := &shipmentRepositoryStub{item: item}
	value, replayed, err := NewShipments(repository).Close(context.Background(), model.CloseShipmentCommand{
		ShipmentID: 11, MerchantID: 2001, ShopID: 3001, CommandKey: "close-0001", ExpectedVersion: 1,
	})
	if err != nil || replayed || value.Status != model.ShipmentDelivered || repository.closed.ShipmentID != 11 {
		t.Fatalf("value=%+v replayed=%v err=%v command=%+v", value, replayed, err, repository.closed)
	}
}
