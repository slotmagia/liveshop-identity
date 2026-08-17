package mysql

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	model "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
)

func TestPurchaseOrderIsIdempotentAndActivatesAssignment(t *testing.T) {
	_, database := integrationRepository(t)
	plans := NewPlanRepository(database)
	orders := NewOrderRepository(database)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	code := "buy-" + suffix
	keyPrefix := "buy-test-" + suffix
	merchantID := time.Now().UnixNano() / 1000
	var planID int64
	t.Cleanup(func() {
		_, _ = database.Exec(`DELETE FROM subscription_order WHERE merchant_id=?`, merchantID)
		_, _ = database.Exec(`DELETE FROM subscription_order_command WHERE command_key LIKE ?`, keyPrefix+"%")
		_, _ = database.Exec(`DELETE FROM subscription_merchant_assignment WHERE merchant_id=?`, merchantID)
		if planID > 0 {
			_, _ = database.Exec(`DELETE FROM subscription_plan_permission WHERE plan_id=?`, planID)
			_, _ = database.Exec(`DELETE FROM subscription_plan_quota WHERE plan_id=?`, planID)
			_, _ = database.Exec(`DELETE FROM subscription_plan WHERE plan_id=?`, planID)
		}
		_, _ = database.Exec(`DELETE FROM subscription_plan_command WHERE command_key LIKE ?`, keyPrefix+"%")
	})

	saved, _, err := plans.SavePlan(context.Background(), mustPaidPlan(t, keyPrefix+"-plan", code))
	if err != nil {
		t.Fatal(err)
	}
	planID = saved.ID
	create, err := (model.CreateOrderCommand{
		MerchantID: merchantID, CommandKey: keyPrefix + "-create", PlanID: planID,
		PlanCode: saved.Code, PlanName: saved.Name, PriceMinor: 9900, DurationDays: 30, ChannelCode: "mock",
	}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	order, replayed, err := orders.CreateOrder(context.Background(), create)
	if err != nil || replayed || order.Status != model.OrderPending || order.PriceMinor != 9900 {
		t.Fatalf("create=%+v replayed=%v err=%v", order, replayed, err)
	}
	repeated, replayed, err := orders.CreateOrder(context.Background(), create)
	if err != nil || !replayed || repeated.OrderNo != order.OrderNo {
		t.Fatalf("replay=%+v replayed=%v err=%v", repeated, replayed, err)
	}
	attached, err := orders.AttachPayment(context.Background(), model.AttachPaymentCommand{
		MerchantID: merchantID, OrderNo: order.OrderNo, PayNo: "PAY-1",
	})
	if err != nil || attached.PayNo != "PAY-1" {
		t.Fatalf("attach=%+v err=%v", attached, err)
	}
	activated, assignment, replayed, err := orders.Activate(context.Background(), model.ActivateOrderCommand{
		MerchantID: merchantID, CommandKey: keyPrefix + "-pay", OrderNo: order.OrderNo, Now: time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC),
	})
	if err != nil || replayed || activated.Status != model.OrderPaid || assignment.PlanID != planID || assignment.ExpiresAt == "" {
		t.Fatalf("activate=%+v assignment=%+v replayed=%v err=%v", activated, assignment, replayed, err)
	}
	again, assignment, replayed, err := orders.Activate(context.Background(), model.ActivateOrderCommand{
		MerchantID: merchantID, CommandKey: keyPrefix + "-pay", OrderNo: order.OrderNo,
	})
	if err != nil || !replayed || again.OrderNo != activated.OrderNo || assignment.PlanID != planID {
		t.Fatalf("activate replay=%+v assignment=%+v replayed=%v err=%v", again, assignment, replayed, err)
	}
	if _, err := orders.GetOrder(context.Background(), merchantID, "missing"); !errors.Is(err, model.ErrOrderNotFound) {
		t.Fatalf("missing error=%v", err)
	}
}

func TestPurchaseOrderListAndClosePending(t *testing.T) {
	_, database := integrationRepository(t)
	plans := NewPlanRepository(database)
	orders := NewOrderRepository(database)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	code := "bill-" + suffix
	keyPrefix := "bill-test-" + suffix
	merchantID := time.Now().UnixNano() / 1000
	var planID int64
	t.Cleanup(func() {
		_, _ = database.Exec(`DELETE FROM subscription_order WHERE merchant_id=?`, merchantID)
		_, _ = database.Exec(`DELETE FROM subscription_order_command WHERE command_key LIKE ?`, keyPrefix+"%")
		if planID > 0 {
			_, _ = database.Exec(`DELETE FROM subscription_plan_permission WHERE plan_id=?`, planID)
			_, _ = database.Exec(`DELETE FROM subscription_plan_quota WHERE plan_id=?`, planID)
			_, _ = database.Exec(`DELETE FROM subscription_plan WHERE plan_id=?`, planID)
		}
		_, _ = database.Exec(`DELETE FROM subscription_plan_command WHERE command_key LIKE ?`, keyPrefix+"%")
	})

	saved, _, err := plans.SavePlan(context.Background(), mustPaidPlan(t, keyPrefix+"-plan", code))
	if err != nil {
		t.Fatal(err)
	}
	planID = saved.ID
	create, err := (model.CreateOrderCommand{
		MerchantID: merchantID, CommandKey: keyPrefix + "-create", PlanID: planID,
		PlanCode: saved.Code, PlanName: saved.Name, PriceMinor: 9900, DurationDays: 30, ChannelCode: "mock",
	}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	order, _, err := orders.CreateOrder(context.Background(), create)
	if err != nil {
		t.Fatal(err)
	}
	page, err := orders.ListOrders(context.Background(), model.OrderQuery{MerchantID: merchantID, Page: 1, PageSize: 20})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].OrderNo != order.OrderNo || page.Items[0].CreatedAt == "" {
		t.Fatalf("list=%+v err=%v", page, err)
	}
	closed, replayed, err := orders.Close(context.Background(), model.CloseOrderCommand{
		MerchantID: merchantID, CommandKey: keyPrefix + "-close", OrderNo: order.OrderNo,
	})
	if err != nil || replayed || closed.Status != model.OrderCancelled {
		t.Fatalf("close=%+v replayed=%v err=%v", closed, replayed, err)
	}
	again, replayed, err := orders.Close(context.Background(), model.CloseOrderCommand{
		MerchantID: merchantID, CommandKey: keyPrefix + "-close", OrderNo: order.OrderNo,
	})
	if err != nil || !replayed || again.Status != model.OrderCancelled {
		t.Fatalf("close replay=%+v replayed=%v err=%v", again, replayed, err)
	}
	if _, err := orders.Activate(context.Background(), model.ActivateOrderCommand{
		MerchantID: merchantID, CommandKey: keyPrefix + "-pay", OrderNo: order.OrderNo,
	}); !errors.Is(err, model.ErrOrderConflict) {
		t.Fatalf("activate closed error=%v", err)
	}
	second, err := (model.CreateOrderCommand{
		MerchantID: merchantID, CommandKey: keyPrefix + "-create-2", PlanID: planID,
		PlanCode: saved.Code, PlanName: saved.Name, PriceMinor: 9900, DurationDays: 30, ChannelCode: "mock",
	}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	created, replayed, err := orders.CreateOrder(context.Background(), second)
	if err != nil || replayed || created.Status != model.OrderPending || created.OrderNo == order.OrderNo {
		t.Fatalf("reopen=%+v replayed=%v err=%v", created, replayed, err)
	}
}

func mustPaidPlan(t *testing.T, commandKey, code string) model.SavePlanCommand {
	t.Helper()
	command, err := (model.SavePlanCommand{CommandKey: commandKey, Plan: model.Plan{
		Code: code, Name: "付费套餐", Level: 2, PriceMinor: 9900, DurationDays: 30, Status: model.PlanActive, Sort: 2,
	}}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	return command
}
