package model

import (
	"crypto/sha256"
	"strconv"
	"strings"
	"time"
)

type ShipmentStatus string

const (
	ShipmentShipped   ShipmentStatus = "SHIPPED"
	ShipmentDelivered ShipmentStatus = "DELIVERED"
	MaxShipmentTraces                = 100
)

type Trace struct {
	OccurredAt time.Time `json:"occurredAt"`
	Node       string    `json:"node"`
}

type Shipment struct {
	ID         int64
	MerchantID int64
	ShopID     int64
	OrderID    int64
	Carrier    string
	TrackingNo string
	Status     ShipmentStatus
	Traces     []Trace
	Version    uint64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (s Shipment) ValidatePersisted() error {
	if s.ID <= 0 || s.MerchantID <= 0 || s.ShopID <= 0 || s.OrderID <= 0 || s.Version == 0 || s.CreatedAt.IsZero() || s.UpdatedAt.IsZero() {
		return ErrInvalid
	}
	if err := validateCarrierOrTracking(s.Carrier); err != nil {
		return err
	}
	if err := validateCarrierOrTracking(s.TrackingNo); err != nil {
		return err
	}
	if !validShipmentStatus(s.Status) {
		return ErrInvalid
	}
	if s.Traces == nil || len(s.Traces) > MaxShipmentTraces {
		return ErrInvalid
	}
	for _, item := range s.Traces {
		if item.OccurredAt.IsZero() {
			return ErrInvalid
		}
		if err := validateTraceNode(item.Node); err != nil {
			return err
		}
	}
	return nil
}

type ShipmentQuery struct {
	MerchantID int64
	ShopID     int64
	OrderID    int64
	Status     ShipmentStatus
	Page       int
	PageSize   int
}

func (q ShipmentQuery) Normalize() (ShipmentQuery, error) {
	q.Status = ShipmentStatus(strings.ToUpper(strings.TrimSpace(string(q.Status))))
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	if q.MerchantID <= 0 || q.ShopID <= 0 || q.OrderID < 0 {
		return q, ErrInvalid
	}
	if q.Status != "" && !validShipmentStatus(q.Status) {
		return q, ErrInvalid
	}
	return q, nil
}

type ShipmentPage struct {
	Items    []Shipment
	Page     int
	PageSize int
	Total    int64
}

type ShipCommand struct {
	MerchantID int64
	ShopID     int64
	CommandKey string
	OrderID    int64
	Carrier    string
	TrackingNo string
}

func (c ShipCommand) Normalize() (ShipCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.Carrier = strings.TrimSpace(c.Carrier)
	c.TrackingNo = strings.TrimSpace(c.TrackingNo)
	if c.MerchantID <= 0 || c.ShopID <= 0 || c.OrderID <= 0 || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrInvalid
	}
	if err := validateCarrierOrTracking(c.Carrier); err != nil {
		return c, err
	}
	if err := validateCarrierOrTracking(c.TrackingNo); err != nil {
		return c, err
	}
	return c, nil
}

func (c ShipCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"SHIP", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatInt(c.ShopID, 10),
		strconv.FormatInt(c.OrderID, 10), c.Carrier, c.TrackingNo,
	}, "\n")))
}

type TraceCommand struct {
	ShipmentID      int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
	Node            string
}

func (c TraceCommand) Normalize() (TraceCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.Node = strings.TrimSpace(c.Node)
	if c.ShipmentID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 ||
		len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrInvalid
	}
	if err := validateTraceNode(c.Node); err != nil {
		return c, err
	}
	return c, nil
}

func (c TraceCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"TRACE", c.CommandKey, strconv.FormatInt(c.ShipmentID, 10), strconv.FormatInt(c.MerchantID, 10),
		strconv.FormatInt(c.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10), c.Node,
	}, "\n")))
}

type CloseShipmentCommand struct {
	ShipmentID      int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
}

func (c CloseShipmentCommand) Normalize() (CloseShipmentCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.ShipmentID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 ||
		len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c CloseShipmentCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"CLOSE", c.CommandKey, strconv.FormatInt(c.ShipmentID, 10), strconv.FormatInt(c.MerchantID, 10),
		strconv.FormatInt(c.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}

func validShipmentStatus(value ShipmentStatus) bool {
	switch value {
	case ShipmentShipped, ShipmentDelivered:
		return true
	default:
		return false
	}
}

func validateCarrierOrTracking(value string) error {
	runes := []rune(value)
	if len(runes) < 1 || len(runes) > 64 {
		return ErrInvalid
	}
	return nil
}

func validateTraceNode(value string) error {
	runes := []rune(value)
	if len(runes) < 1 || len(runes) > 200 {
		return ErrInvalid
	}
	return nil
}
