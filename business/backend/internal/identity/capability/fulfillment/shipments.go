package fulfillment

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

// ShipmentRepository is implemented by the Identity data layer.
type ShipmentRepository interface {
	ListShipments(context.Context, model.ShipmentQuery) (model.ShipmentPage, error)
	GetShipment(context.Context, int64, int64, int64) (model.Shipment, error)
	Ship(context.Context, model.ShipCommand) (model.Shipment, bool, error)
	AddTrace(context.Context, model.TraceCommand) (model.Shipment, bool, error)
	CloseShipment(context.Context, model.CloseShipmentCommand) (model.Shipment, bool, error)
}

// Shipments is the capability application boundary for shop logistics.
type Shipments struct{ repository ShipmentRepository }

func NewShipments(repository ShipmentRepository) *Shipments {
	return &Shipments{repository: repository}
}

func (s *Shipments) List(ctx context.Context, query model.ShipmentQuery) (model.ShipmentPage, error) {
	if s == nil || s.repository == nil {
		return model.ShipmentPage{}, model.ErrUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.ShipmentPage{}, err
	}
	page, err := s.repository.ListShipments(ctx, normalized)
	if err != nil {
		return model.ShipmentPage{}, err
	}
	if page.Page != normalized.Page || page.PageSize != normalized.PageSize || page.Total < 0 {
		return model.ShipmentPage{}, model.ErrInvalid
	}
	for _, item := range page.Items {
		if err := item.ValidatePersisted(); err != nil {
			return model.ShipmentPage{}, model.ErrInvalid
		}
		if item.MerchantID != normalized.MerchantID || item.ShopID != normalized.ShopID {
			return model.ShipmentPage{}, model.ErrInvalid
		}
	}
	return page, nil
}

func (s *Shipments) Get(ctx context.Context, merchantID, shopID, shipmentID int64) (model.Shipment, error) {
	if s == nil || s.repository == nil {
		return model.Shipment{}, model.ErrUnavailable
	}
	if merchantID <= 0 || shopID <= 0 || shipmentID <= 0 {
		return model.Shipment{}, model.ErrInvalid
	}
	value, err := s.repository.GetShipment(ctx, merchantID, shopID, shipmentID)
	if err != nil {
		return model.Shipment{}, err
	}
	if err := value.ValidatePersisted(); err != nil {
		return model.Shipment{}, model.ErrInvalid
	}
	if value.MerchantID != merchantID || value.ShopID != shopID || value.ID != shipmentID {
		return model.Shipment{}, model.ErrInvalid
	}
	return value, nil
}

func (s *Shipments) Ship(ctx context.Context, command model.ShipCommand) (model.Shipment, bool, error) {
	if s == nil || s.repository == nil {
		return model.Shipment{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Shipment{}, false, err
	}
	value, replayed, err := s.repository.Ship(ctx, normalized)
	if err != nil {
		return model.Shipment{}, false, err
	}
	return s.persistedWrite(value, replayed, normalized.MerchantID, normalized.ShopID, 0)
}

func (s *Shipments) AddTrace(ctx context.Context, command model.TraceCommand) (model.Shipment, bool, error) {
	if s == nil || s.repository == nil {
		return model.Shipment{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Shipment{}, false, err
	}
	value, replayed, err := s.repository.AddTrace(ctx, normalized)
	if err != nil {
		return model.Shipment{}, false, err
	}
	return s.persistedWrite(value, replayed, normalized.MerchantID, normalized.ShopID, normalized.ShipmentID)
}

func (s *Shipments) Close(ctx context.Context, command model.CloseShipmentCommand) (model.Shipment, bool, error) {
	if s == nil || s.repository == nil {
		return model.Shipment{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Shipment{}, false, err
	}
	value, replayed, err := s.repository.CloseShipment(ctx, normalized)
	if err != nil {
		return model.Shipment{}, false, err
	}
	return s.persistedWrite(value, replayed, normalized.MerchantID, normalized.ShopID, normalized.ShipmentID)
}

func (s *Shipments) persistedWrite(value model.Shipment, replayed bool, merchantID, shopID, shipmentID int64) (model.Shipment, bool, error) {
	if err := value.ValidatePersisted(); err != nil {
		return model.Shipment{}, false, model.ErrInvalid
	}
	if value.MerchantID != merchantID || value.ShopID != shopID {
		return model.Shipment{}, false, model.ErrInvalid
	}
	if shipmentID > 0 && value.ID != shipmentID {
		return model.Shipment{}, false, model.ErrInvalid
	}
	return value, replayed, nil
}
