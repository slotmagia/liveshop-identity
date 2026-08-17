package mysql

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	model "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
)

func TestMerchantAssignmentIsIdempotentAndVersioned(t *testing.T) {
	_, database := integrationRepository(t)
	plans := NewPlanRepository(database)
	assignments := NewAssignmentRepository(database)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	code := "assign-" + suffix
	keyPrefix := "assign-test-" + suffix
	merchantID := time.Now().UnixNano() / 1000
	var planID int64
	t.Cleanup(func() {
		_, _ = database.Exec(`DELETE FROM subscription_merchant_assignment WHERE merchant_id=?`, merchantID)
		_, _ = database.Exec(`DELETE FROM subscription_merchant_assignment_command WHERE command_key LIKE ?`, keyPrefix+"%")
		if planID > 0 {
			_, _ = database.Exec(`DELETE FROM subscription_plan_permission WHERE plan_id=?`, planID)
			_, _ = database.Exec(`DELETE FROM subscription_plan_quota WHERE plan_id=?`, planID)
			_, _ = database.Exec(`DELETE FROM subscription_plan WHERE plan_id=?`, planID)
		}
		_, _ = database.Exec(`DELETE FROM subscription_plan_command WHERE command_key LIKE ?`, keyPrefix+"%")
	})

	saved, _, err := plans.SavePlan(context.Background(), mustAssignPlan(t, keyPrefix+"-plan", code))
	if err != nil {
		t.Fatal(err)
	}
	planID = saved.ID
	empty, err := assignments.GetAssignment(context.Background(), merchantID)
	if err != nil || empty.PlanID != 0 {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
	command, err := (model.AssignCommand{MerchantID: merchantID, CommandKey: keyPrefix + "-assign", PlanID: planID}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	assigned, replayed, err := assignments.Assign(context.Background(), command)
	if err != nil || replayed || assigned.PlanID != planID || assigned.Version != 1 {
		t.Fatalf("assigned=%+v replayed=%v err=%v", assigned, replayed, err)
	}
	repeated, replayed, err := assignments.Assign(context.Background(), command)
	if err != nil || !replayed || repeated.Version != assigned.Version {
		t.Fatalf("repeated=%+v replayed=%v err=%v", repeated, replayed, err)
	}
	changed := command
	expires := "2099-01-01T00:00:00Z"
	changed.ExpiresAt = &expires
	if _, _, err := assignments.Assign(context.Background(), changed); !errors.Is(err, model.ErrAssignmentIdempotency) {
		t.Fatalf("changed key error=%v", err)
	}
	conflict := command
	conflict.CommandKey = keyPrefix + "-conflict"
	conflict.ExpectedVersion = 99
	if _, _, err := assignments.Assign(context.Background(), conflict); !errors.Is(err, model.ErrAssignmentConflict) {
		t.Fatalf("conflict error=%v", err)
	}
}

func mustAssignPlan(t *testing.T, commandKey, code string) model.SavePlanCommand {
	t.Helper()
	command, err := (model.SavePlanCommand{CommandKey: commandKey, Plan: model.Plan{
		Code: code, Name: "指派套餐", Level: 1, PriceMinor: 0, DurationDays: 30, Status: model.PlanActive, Default: true, Sort: 1,
	}}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	return command
}
