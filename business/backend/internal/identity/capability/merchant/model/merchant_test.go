package model

import (
	"errors"
	"testing"
)

func TestCreateCommandAcceptsUsernameOrEmail(t *testing.T) {
	t.Parallel()
	accepted, err := (CreateCommand{CommandKey: "command-1", Account: "Lvtu@sufeipay.com", Password: "password1", Name: "Lvtu Open"}).Normalize()
	if err != nil {
		t.Fatalf("email account: %v", err)
	}
	if accepted.Account != "lvtu@sufeipay.com" {
		t.Fatalf("account=%q", accepted.Account)
	}
	username, err := (CreateCommand{CommandKey: "command-1", Account: "Owner_01", Password: "password1", Name: "Demo"}).Normalize()
	if err != nil || username.Account != "owner_01" {
		t.Fatalf("username=%+v err=%v", username, err)
	}
}

func TestCreateCommandRejectsInvalidAccount(t *testing.T) {
	t.Parallel()
	for _, account := range []string{"1bad", "ab", "owner@", "owner@bad", "吕途", "a@b.c"} {
		if _, err := (CreateCommand{CommandKey: "command-1", Account: account, Password: "password1", Name: "Demo"}).Normalize(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("account %q error=%v", account, err)
		}
	}
}

func TestUpdateProfileCommandRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	if _, err := (UpdateProfileCommand{MerchantID: 2001, CommandKey: "short", ExpectedVersion: 1}).Normalize(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short command key error=%v", err)
	}
	if _, err := (UpdateProfileCommand{MerchantID: 2001, CommandKey: "command-4", ContactName: "Ada"}).Normalize(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing version error=%v", err)
	}
	longID := make([]rune, 65)
	for i := range longID {
		longID[i] = 'a'
	}
	if _, err := (UpdateProfileCommand{MerchantID: 2001, CommandKey: "command-4", ExpectedVersion: 1, ExternalID: string(longID)}).Normalize(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized external id error=%v", err)
	}
	accepted, err := (UpdateProfileCommand{
		MerchantID: 2001, CommandKey: "command-4", ExpectedVersion: 1,
		ExternalID: " ext-1 ", ContactName: " Ada ", ContactPhone: " 13800000000 ",
		MarketingEmailOptIn: true,
	}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if accepted.ExternalID != "ext-1" || accepted.ContactName != "Ada" || accepted.ContactPhone != "13800000000" || !accepted.MarketingEmailOptIn || accepted.MarketingSMSOptIn {
		t.Fatalf("normalized=%+v", accepted)
	}
}
