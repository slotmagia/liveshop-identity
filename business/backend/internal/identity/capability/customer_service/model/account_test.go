package model

import (
	"errors"
	"testing"
)

func TestQueryNormalizeAllowsEmptyMerchantAndShop(t *testing.T) {
	value, err := (Query{}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if value.MerchantID != 0 || value.ShopID != 0 || value.Page != 1 || value.PageSize != 20 {
		t.Fatalf("normalized=%+v", value)
	}
}

func TestAccountNormalizesLegacyFields(t *testing.T) {
	value, err := (Account{Platform: " WhatsApp ", Account: " +15551234567 ", Nickname: " 客服 ", Status: StatusActive, Config: ` {"country_code":"US"} `}).NormalizeEditable()
	if err != nil {
		t.Fatal(err)
	}
	if value.Platform != "whatsapp" || value.Account != "+15551234567" || value.Nickname != "客服" || value.Config != `{"country_code":"US"}` {
		t.Fatalf("normalized=%+v", value)
	}
}

func TestAccountRejectsLegacyInvalidValues(t *testing.T) {
	valid := Account{Platform: "telegram_bot", Account: "support", Status: StatusDisabled}
	for name, mutate := range map[string]func(*Account){
		"platform": func(value *Account) { value.Platform = "Whats App" },
		"account":  func(value *Account) { value.Account = "" },
		"status":   func(value *Account) { value.Status = "UNKNOWN" },
		"config":   func(value *Account) { value.Config = "{" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if _, err := candidate.NormalizeEditable(); !errors.Is(err, ErrInvalid) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestCommandDigestBindsScopeVersionAndInput(t *testing.T) {
	base, err := (SaveCommand{CommandKey: "customer-service-test", ExpectedVersion: 1, Account: Account{
		ID: 8, MerchantID: 10, ShopID: 20, Platform: "telegram", Account: "support", Status: StatusActive,
	}}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.Account.ShopID++
	if base.RequestDigest() == changed.RequestDigest() {
		t.Fatal("shop scope was not bound into request digest")
	}
	changed = base
	changed.ExpectedVersion++
	if base.RequestDigest() == changed.RequestDigest() {
		t.Fatal("expected version was not bound into request digest")
	}
}
