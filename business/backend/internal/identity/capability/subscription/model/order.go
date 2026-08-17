package model

import (
	"crypto/sha256"
	"strconv"
	"strings"
	"time"
)

type OrderStatus string

const (
	OrderPending   OrderStatus = "PENDING"
	OrderPaid      OrderStatus = "PAID"
	OrderCancelled OrderStatus = "CANCELLED"
)

type Order struct {
	OrderNo      string
	MerchantID   int64
	PlanID       int64
	PlanCode     string
	PlanName     string
	PriceMinor   int64
	DurationDays int
	Status       OrderStatus
	PayNo        string
	ChannelCode  string
	Version      uint64
	PaidAt       string
	CreatedAt    string
}

type OrderQuery struct {
	MerchantID int64
	Status     OrderStatus
	Page       int
	PageSize   int
}

type OrderPage struct {
	Items    []Order
	Page     int
	PageSize int
	Total    int64
}

func (o Order) Paid() bool { return o.Status == OrderPaid }

type CreateOrderCommand struct {
	MerchantID   int64
	CommandKey   string
	PlanID       int64
	PlanCode     string
	PlanName     string
	PriceMinor   int64
	DurationDays int
	ChannelCode  string
}

func (c CreateOrderCommand) Normalize() (CreateOrderCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.PlanCode = strings.TrimSpace(c.PlanCode)
	c.PlanName = strings.TrimSpace(c.PlanName)
	c.ChannelCode = strings.TrimSpace(c.ChannelCode)
	if c.MerchantID <= 0 || c.PlanID <= 0 || c.PriceMinor <= 0 || c.DurationDays < 0 {
		return c, ErrOrderNotBuyable
	}
	if len(c.CommandKey) < 8 || len(c.CommandKey) > 128 || c.PlanCode == "" || c.PlanName == "" || c.ChannelCode == "" {
		return c, ErrOrderInvalid
	}
	return c, nil
}

func (c CreateOrderCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"CREATE_ORDER", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatInt(c.PlanID, 10),
		c.PlanCode, c.PlanName, strconv.FormatInt(c.PriceMinor, 10), strconv.Itoa(c.DurationDays), c.ChannelCode,
	}, "\n")))
}

type AttachPaymentCommand struct {
	MerchantID int64
	OrderNo    string
	PayNo      string
}

func (c AttachPaymentCommand) Normalize() (AttachPaymentCommand, error) {
	c.OrderNo = strings.TrimSpace(c.OrderNo)
	c.PayNo = strings.TrimSpace(c.PayNo)
	if c.MerchantID <= 0 || c.OrderNo == "" || c.PayNo == "" {
		return c, ErrOrderInvalid
	}
	return c, nil
}

type ActivateOrderCommand struct {
	MerchantID int64
	CommandKey string
	OrderNo    string
	Now        time.Time
}

func (c ActivateOrderCommand) Normalize() (ActivateOrderCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.OrderNo = strings.TrimSpace(c.OrderNo)
	if c.MerchantID <= 0 || c.OrderNo == "" || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrOrderInvalid
	}
	if c.Now.IsZero() {
		c.Now = time.Now().UTC()
	}
	return c, nil
}

func (c ActivateOrderCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"ACTIVATE_ORDER", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), c.OrderNo,
	}, "\n")))
}

type CloseOrderCommand struct {
	MerchantID int64
	CommandKey string
	OrderNo    string
}

func (c CloseOrderCommand) Normalize() (CloseOrderCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.OrderNo = strings.TrimSpace(c.OrderNo)
	if c.MerchantID <= 0 || c.OrderNo == "" || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrOrderInvalid
	}
	return c, nil
}

func (c CloseOrderCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"CLOSE_ORDER", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), c.OrderNo,
	}, "\n")))
}

func (q OrderQuery) Normalize() (OrderQuery, error) {
	if q.MerchantID <= 0 {
		return q, ErrOrderInvalid
	}
	q.Status = OrderStatus(strings.TrimSpace(string(q.Status)))
	if q.Status != "" && q.Status != OrderPending && q.Status != OrderPaid && q.Status != OrderCancelled {
		return q, ErrOrderInvalid
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	return q, nil
}

func Buyable(plan Plan, current Assignment) error {
	if plan.ID <= 0 || plan.Status != PlanActive || plan.PriceMinor <= 0 {
		return ErrOrderNotBuyable
	}
	if current.PlanID == plan.ID && plan.DurationDays == 0 {
		return ErrOrderNotBuyable
	}
	return nil
}

func RenewExpiresAt(current Assignment, planID int64, durationDays int, now time.Time) *string {
	if durationDays <= 0 {
		return nil
	}
	base := now.UTC()
	if current.PlanID == planID {
		if expires, err := parseExpires(current.ExpiresAt); err == nil && expires.After(now) {
			base = expires.UTC()
		}
	}
	text := base.AddDate(0, 0, durationDays).Format(time.RFC3339Nano)
	return &text
}

func parseExpires(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, ErrAssignmentInvalid
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}
