package subscription

import (
	"errors"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
)

func TestBuyableRejectsFreeCurrentAndDisabledPlans(t *testing.T) {
	activePaid := model.Plan{ID: 2, Status: model.PlanActive, PriceMinor: 9900, DurationDays: 30}
	if err := model.Buyable(activePaid, model.Assignment{}); err != nil {
		t.Fatal(err)
	}
	if err := model.Buyable(model.Plan{ID: 1, Status: model.PlanActive, PriceMinor: 0, DurationDays: 30}, model.Assignment{}); !errors.Is(err, model.ErrOrderNotBuyable) {
		t.Fatalf("free error=%v", err)
	}
	if err := model.Buyable(model.Plan{ID: 2, Status: model.PlanDisabled, PriceMinor: 9900, DurationDays: 30}, model.Assignment{}); !errors.Is(err, model.ErrOrderNotBuyable) {
		t.Fatalf("disabled error=%v", err)
	}
	if err := model.Buyable(activePaid, model.Assignment{PlanID: 2}); err != nil {
		t.Fatalf("timed current should renew: %v", err)
	}
	if err := model.Buyable(model.Plan{ID: 2, Status: model.PlanActive, PriceMinor: 9900, DurationDays: 0}, model.Assignment{PlanID: 2}); !errors.Is(err, model.ErrOrderNotBuyable) {
		t.Fatalf("forever current error=%v", err)
	}
}

func TestCloseOrderCommandRejectsBlankKeys(t *testing.T) {
	if _, err := (model.CloseOrderCommand{MerchantID: 1, CommandKey: "short", OrderNo: "SUB1"}).Normalize(); !errors.Is(err, model.ErrOrderInvalid) {
		t.Fatalf("short key error=%v", err)
	}
	normalized, err := (model.CloseOrderCommand{MerchantID: 7, CommandKey: "close-key-01", OrderNo: " SUB-1 "}).Normalize()
	if err != nil || normalized.OrderNo != "SUB-1" {
		t.Fatalf("normalized=%+v err=%v", normalized, err)
	}
}

func TestOrderQueryNormalizesPageAndRejectsUnknownStatus(t *testing.T) {
	if _, err := (model.OrderQuery{MerchantID: 7, Status: "OPEN"}).Normalize(); !errors.Is(err, model.ErrOrderInvalid) {
		t.Fatalf("status error=%v", err)
	}
	normalized, err := (model.OrderQuery{MerchantID: 7}).Normalize()
	if err != nil || normalized.Page != 1 || normalized.PageSize != 20 {
		t.Fatalf("defaults=%+v err=%v", normalized, err)
	}
}

func TestRenewExpiresAtExtendsSamePlanAndReplacesOtherPlan(t *testing.T) {
	now := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	current := model.Assignment{PlanID: 2, ExpiresAt: now.Add(10 * 24 * time.Hour).Format(time.RFC3339Nano)}
	same := model.RenewExpiresAt(current, 2, 30, now)
	if same == nil {
		t.Fatal("expected expiry")
	}
	got, err := time.Parse(time.RFC3339Nano, *same)
	if err != nil || !got.Equal(now.Add(40 * 24 * time.Hour)) {
		t.Fatalf("same-plan expiry=%v err=%v", *same, err)
	}
	other := model.RenewExpiresAt(current, 3, 30, now)
	got, err = time.Parse(time.RFC3339Nano, *other)
	if err != nil || !got.Equal(now.Add(30 * 24 * time.Hour)) {
		t.Fatalf("other-plan expiry=%v err=%v", *other, err)
	}
	if model.RenewExpiresAt(current, 2, 0, now) != nil {
		t.Fatal("forever should clear expiry")
	}
}
