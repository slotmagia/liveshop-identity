// Package model owns the fulfillment capability facts, invariants, state machines, and domain errors.
package model


import (
	"crypto/sha256"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Status string
type TargetType string

const (
	StatusOpen     Status = "OPEN"
	StatusAccepted Status = "ACCEPTED"
	StatusRejected Status = "REJECTED"

	TargetOrder     TargetType = "ORDER"
	TargetAftersale TargetType = "AFTERSALE"
	TargetLive      TargetType = "LIVE"
	TargetProduct   TargetType = "PRODUCT"
	TargetOther     TargetType = "OTHER"
)

var (
	ErrUnavailable = errors.New("complaint repository unavailable")
	ErrNotFound    = errors.New("complaint not found")
	ErrInvalid     = errors.New("invalid complaint")
	ErrConflict    = errors.New("complaint version or status conflict")
	ErrIdempotency = errors.New("complaint command key was reused with different input")
)

type Complaint struct {
	ID              int64
	MerchantID      int64
	ShopID          int64
	CustomerSubject string
	TargetType      TargetType
	TargetID        int64
	ReasonCode      string
	Content         string
	Status          Status
	HandleNote      string
	Version         uint64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	HandledAt       *time.Time
}

func (c Complaint) ValidatePersisted() error {
	if c.ID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.Version == 0 || c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
		return ErrInvalid
	}
	if err := validateCustomerSubject(c.CustomerSubject); err != nil {
		return err
	}
	if !validTargetType(c.TargetType) || c.TargetID < 0 || !validStatus(c.Status) {
		return ErrInvalid
	}
	if len([]rune(c.ReasonCode)) == 0 || len([]rune(c.ReasonCode)) > 64 {
		return ErrInvalid
	}
	if len([]rune(c.Content)) == 0 || len([]rune(c.Content)) > 4000 {
		return ErrInvalid
	}
	noteRunes := []rune(c.HandleNote)
	switch c.Status {
	case StatusOpen:
		if c.HandleNote != "" || c.HandledAt != nil {
			return ErrInvalid
		}
	case StatusAccepted, StatusRejected:
		if len(noteRunes) == 0 || len(noteRunes) > 2000 || c.HandledAt == nil || c.HandledAt.IsZero() {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

type Query struct {
	MerchantID      int64
	ShopID          int64
	CustomerSubject string
	Status          Status
	TargetType      TargetType
	Page            int
	PageSize        int
}

func (q Query) Normalize() (Query, error) {
	q.CustomerSubject = strings.TrimSpace(q.CustomerSubject)
	q.Status = Status(strings.ToUpper(strings.TrimSpace(string(q.Status))))
	q.TargetType = TargetType(strings.ToUpper(strings.TrimSpace(string(q.TargetType))))
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	if q.MerchantID <= 0 || q.ShopID <= 0 {
		return q, ErrInvalid
	}
	if q.CustomerSubject != "" {
		if err := validateCustomerSubject(q.CustomerSubject); err != nil {
			return q, err
		}
	}
	if q.Status != "" && !validStatus(q.Status) {
		return q, ErrInvalid
	}
	if q.TargetType != "" && !validTargetType(q.TargetType) {
		return q, ErrInvalid
	}
	return q, nil
}

type Page struct {
	Items    []Complaint
	Page     int
	PageSize int
	Total    int64
}

type ReviewCommand struct {
	ComplaintID     int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
	Status          Status
	HandleNote      string
}

func (c ReviewCommand) Normalize() (ReviewCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.HandleNote = strings.TrimSpace(c.HandleNote)
	c.Status = Status(strings.ToUpper(strings.TrimSpace(string(c.Status))))
	if c.ComplaintID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 ||
		len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrInvalid
	}
	if c.Status != StatusAccepted && c.Status != StatusRejected {
		return c, ErrInvalid
	}
	noteRunes := []rune(c.HandleNote)
	if len(noteRunes) == 0 || len(noteRunes) > 2000 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c ReviewCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"REVIEW", c.CommandKey, strconv.FormatInt(c.ComplaintID, 10), strconv.FormatInt(c.MerchantID, 10),
		strconv.FormatInt(c.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10), string(c.Status), c.HandleNote,
	}, "\n")))
}

func validStatus(value Status) bool {
	switch value {
	case StatusOpen, StatusAccepted, StatusRejected:
		return true
	default:
		return false
	}
}

func validTargetType(value TargetType) bool {
	switch value {
	case TargetOrder, TargetAftersale, TargetLive, TargetProduct, TargetOther:
		return true
	default:
		return false
	}
}

func validateCustomerSubject(value string) error {
	if value == "" || len([]rune(value)) > 128 {
		return ErrInvalid
	}
	for _, r := range value {
		if unicode.IsSpace(r) {
			return ErrInvalid
		}
	}
	return nil
}
