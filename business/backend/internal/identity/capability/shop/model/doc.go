// Package model owns the shop capability facts, invariants, state machines, and domain errors.
package model

import "errors"

var (
	ErrUnavailable       = errors.New("shop directory unavailable")
	ErrInvalidMerchantID = errors.New("merchant id is required")
	ErrInvalidShop       = errors.New("invalid shop directory item")
)

type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusDisabled Status = "DISABLED"
)

type Shop struct {
	ID            int64
	MerchantID    int64
	Code          string
	Subdomain     string
	Name          string
	DefaultLocale string
	Currency      string
	CategoryCode  string
	Status        Status
	Version       uint64
}

func (s Shop) Validate() error {
	if s.ID <= 0 || s.MerchantID <= 0 || s.Code == "" || s.Name == "" ||
		s.Currency == "" || s.Version == 0 || (s.Status != StatusActive && s.Status != StatusDisabled) {
		return ErrInvalidShop
	}
	return nil
}
