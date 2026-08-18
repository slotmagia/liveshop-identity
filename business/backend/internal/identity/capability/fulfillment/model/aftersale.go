package model

import (
	"crypto/sha256"
	"errors"
	"strconv"
	"strings"
	"time"
)

type AftersaleStatus string
type AftersaleType string
type ReturnStatus string

const (
	AftersalePending  AftersaleStatus = "PENDING"
	AftersaleApproved AftersaleStatus = "APPROVED"
	AftersaleRejected AftersaleStatus = "REJECTED"
	AftersaleRefunded AftersaleStatus = "REFUNDED"
	AftersaleClosed   AftersaleStatus = "CLOSED"

	AftersaleRefundOnly   AftersaleType = "REFUND_ONLY"
	AftersaleReturnRefund AftersaleType = "RETURN_REFUND"

	ReturnNotRequired ReturnStatus = "NOT_REQUIRED"
	ReturnPending     ReturnStatus = "PENDING"
	ReturnReceived    ReturnStatus = "RECEIVED"
)

var (
	ErrAftersaleUnavailable = errors.New("aftersale repository unavailable")
	ErrAftersaleNotFound    = errors.New("aftersale not found")
	ErrAftersaleInvalid     = errors.New("invalid aftersale")
	ErrAftersaleConflict    = errors.New("aftersale version or status conflict")
	ErrAftersaleIdempotency = errors.New("aftersale command key was reused with different input")
)

type AftersaleItem struct {
	ID               int64
	SKUID            int64
	Title            string
	Quantity         int64
	RefundAmount     int64
	ReceivedQuantity int64
}

type Aftersale struct {
	ID              int64
	MerchantID      int64
	ShopID          int64
	CustomerSubject string
	OrderID         int64
	PaymentNo       string
	Type            AftersaleType
	RequestedAmount int64
	Amount          int64
	Reason          string
	Status          AftersaleStatus
	ReturnStatus    ReturnStatus
	HandleNote      string
	Items           []AftersaleItem
	Version         uint64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ReviewedAt      *time.Time
	ReceivedAt      *time.Time
}

func (a Aftersale) ValidatePersisted() error {
	if a.ID <= 0 || a.MerchantID <= 0 || a.ShopID <= 0 || a.OrderID <= 0 || a.Version == 0 ||
		a.CreatedAt.IsZero() || a.UpdatedAt.IsZero() {
		return ErrAftersaleInvalid
	}
	if err := validateCustomerSubject(a.CustomerSubject); err != nil {
		return ErrAftersaleInvalid
	}
	if len(a.PaymentNo) > 64 || !validAftersaleType(a.Type) || !validAftersaleStatus(a.Status) ||
		!validReturnStatus(a.ReturnStatus) {
		return ErrAftersaleInvalid
	}
	reasonRunes := []rune(strings.TrimSpace(a.Reason))
	if len(reasonRunes) == 0 || len(reasonRunes) > 255 || a.RequestedAmount < 1 || a.Amount < 1 ||
		a.Amount > a.RequestedAmount {
		return ErrAftersaleInvalid
	}
	if a.Type == AftersaleRefundOnly && a.ReturnStatus != ReturnNotRequired {
		return ErrAftersaleInvalid
	}
	if a.Type == AftersaleReturnRefund && a.ReturnStatus == ReturnNotRequired {
		return ErrAftersaleInvalid
	}
	noteRunes := []rune(a.HandleNote)
	switch a.Status {
	case AftersalePending:
		if a.HandleNote != "" || a.ReviewedAt != nil {
			return ErrAftersaleInvalid
		}
	case AftersaleApproved, AftersaleRejected, AftersaleRefunded, AftersaleClosed:
		if len(noteRunes) == 0 || len(noteRunes) > 2000 || a.ReviewedAt == nil || a.ReviewedAt.IsZero() {
			return ErrAftersaleInvalid
		}
	default:
		return ErrAftersaleInvalid
	}
	if a.ReturnStatus != ReturnReceived && a.ReceivedAt != nil {
		return ErrAftersaleInvalid
	}
	if a.ReturnStatus == ReturnReceived && (a.ReceivedAt == nil || a.ReceivedAt.IsZero()) {
		return ErrAftersaleInvalid
	}
	for _, item := range a.Items {
		if item.SKUID <= 0 || item.Quantity <= 0 || item.RefundAmount < 0 || item.ReceivedQuantity < 0 ||
			item.ReceivedQuantity > item.Quantity || len([]rune(item.Title)) > 200 {
			return ErrAftersaleInvalid
		}
	}
	return nil
}

type AftersaleQuery struct {
	MerchantID      int64
	ShopID          int64
	CustomerSubject string
	Status          AftersaleStatus
	Type            AftersaleType
	Page            int
	PageSize        int
}

func (q AftersaleQuery) Normalize() (AftersaleQuery, error) {
	q.CustomerSubject = strings.TrimSpace(q.CustomerSubject)
	q.Status = AftersaleStatus(strings.ToUpper(strings.TrimSpace(string(q.Status))))
	q.Type = AftersaleType(strings.ToUpper(strings.TrimSpace(string(q.Type))))
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	if q.MerchantID <= 0 || q.ShopID <= 0 {
		return q, ErrAftersaleInvalid
	}
	if q.CustomerSubject != "" {
		if err := validateCustomerSubject(q.CustomerSubject); err != nil {
			return q, ErrAftersaleInvalid
		}
	}
	if q.Status != "" && !validAftersaleStatus(q.Status) {
		return q, ErrAftersaleInvalid
	}
	if q.Type != "" && !validAftersaleType(q.Type) {
		return q, ErrAftersaleInvalid
	}
	return q, nil
}

type AftersalePage struct {
	Items    []Aftersale
	Page     int
	PageSize int
	Total    int64
}

type ReviewAftersaleCommand struct {
	AftersaleID     int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
	Status          AftersaleStatus
	Amount          int64
	HandleNote      string
}

func (c ReviewAftersaleCommand) Normalize() (ReviewAftersaleCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.HandleNote = strings.TrimSpace(c.HandleNote)
	c.Status = AftersaleStatus(strings.ToUpper(strings.TrimSpace(string(c.Status))))
	if c.AftersaleID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 ||
		len(c.CommandKey) < 8 || len(c.CommandKey) > 128 || c.Amount < 0 {
		return c, ErrAftersaleInvalid
	}
	if c.Status != AftersaleApproved && c.Status != AftersaleRejected {
		return c, ErrAftersaleInvalid
	}
	noteRunes := []rune(c.HandleNote)
	if len(noteRunes) == 0 || len(noteRunes) > 2000 {
		return c, ErrAftersaleInvalid
	}
	return c, nil
}

func (c ReviewAftersaleCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"REVIEW", c.CommandKey, strconv.FormatInt(c.AftersaleID, 10), strconv.FormatInt(c.MerchantID, 10),
		strconv.FormatInt(c.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10), string(c.Status),
		strconv.FormatInt(c.Amount, 10), c.HandleNote,
	}, "\n")))
}

type ReceiveAftersaleCommand struct {
	AftersaleID     int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
}

func (c ReceiveAftersaleCommand) Normalize() (ReceiveAftersaleCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.AftersaleID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 ||
		len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrAftersaleInvalid
	}
	return c, nil
}

func (c ReceiveAftersaleCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"RETURN", c.CommandKey, strconv.FormatInt(c.AftersaleID, 10), strconv.FormatInt(c.MerchantID, 10),
		strconv.FormatInt(c.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}

func validAftersaleStatus(value AftersaleStatus) bool {
	switch value {
	case AftersalePending, AftersaleApproved, AftersaleRejected, AftersaleRefunded, AftersaleClosed:
		return true
	default:
		return false
	}
}

func validAftersaleType(value AftersaleType) bool {
	switch value {
	case AftersaleRefundOnly, AftersaleReturnRefund:
		return true
	default:
		return false
	}
}

func validReturnStatus(value ReturnStatus) bool {
	switch value {
	case ReturnNotRequired, ReturnPending, ReturnReceived:
		return true
	default:
		return false
	}
}
