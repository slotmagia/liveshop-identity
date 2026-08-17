package model

import (
	"errors"
	"testing"
)

func TestShopResolutionCurrencyContract(t *testing.T) {
	base := ShopResolution{
		Context:  ShopContext{MerchantID: 7, ShopID: 101},
		Currency: "USD",
		Status:   StatusActive,
		Version:  1,
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid resolution rejected: %v", err)
	}
	for _, invalid := range []string{"", "usd", "EU", "US12", "中元A"} {
		t.Run(invalid, func(t *testing.T) {
			candidate := base
			candidate.Currency = invalid
			if err := candidate.Validate(); !errors.Is(err, ErrInvalidShopCurrency) {
				t.Fatalf("currency %q error = %v, want invalid shop currency", invalid, err)
			}
		})
	}
}

func TestShopResolutionRejectsInvalidIdentityFacts(t *testing.T) {
	candidate := ShopResolution{Currency: "USD", Status: StatusActive, Version: 1}
	if err := candidate.Validate(); !errors.Is(err, ErrInvalidContext) {
		t.Fatalf("error = %v, want invalid context", err)
	}
}
