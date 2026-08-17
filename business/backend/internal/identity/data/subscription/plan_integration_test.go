package mysql

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	model "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
)

func TestPlanWriteIsIdempotentVersionedAndTransactional(t *testing.T) {
	_, database := integrationRepository(t)
	repository := NewPlanRepository(database)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	code := "test-" + suffix
	commandPrefix := "plan-test-" + suffix
	var planID int64
	t.Cleanup(func() {
		tx, err := database.Begin()
		if err != nil {
			return
		}
		defer tx.Rollback()
		_, _ = tx.Exec(`DELETE FROM subscription_plan_command WHERE command_key LIKE ?`, commandPrefix+"%")
		_, _ = tx.Exec(`DELETE FROM subscription_plan_policy_command WHERE command_key LIKE ?`, commandPrefix+"%")
		if planID == 0 {
			_ = tx.QueryRow(`SELECT plan_id FROM subscription_plan WHERE code=?`, code).Scan(&planID)
		}
		if planID > 0 {
			_, _ = tx.Exec(`DELETE FROM subscription_plan_permission WHERE plan_id=?`, planID)
			_, _ = tx.Exec(`DELETE FROM subscription_plan_quota WHERE plan_id=?`, planID)
			_, _ = tx.Exec(`DELETE FROM subscription_plan WHERE plan_id=?`, planID)
		}
		_ = tx.Commit()
	})

	var permission string
	if err := database.QueryRow(`SELECT permission_code FROM identity_permission_projection WHERE active=1 ORDER BY permission_code LIMIT 1`).Scan(&permission); err != nil {
		t.Fatalf("active Registry permission required: %v", err)
	}
	create := model.SavePlanCommand{CommandKey: commandPrefix + "-create", Plan: model.Plan{
		Code: code, Name: "集成测试套餐", Status: model.PlanActive,
	}}
	created, replayed, err := repository.SavePlan(context.Background(), create)
	if err != nil || replayed || created.Version != 1 {
		t.Fatalf("create=%+v replayed=%v err=%v", created, replayed, err)
	}
	planID = created.ID
	replayedPlan, replayed, err := repository.SavePlan(context.Background(), create)
	if err != nil || !replayed || replayedPlan.ID != created.ID || replayedPlan.Version != 1 {
		t.Fatalf("replay=%+v replayed=%v err=%v", replayedPlan, replayed, err)
	}
	changed := create
	changed.Plan.Name = "changed"
	if _, _, err := repository.SavePlan(context.Background(), changed); !errors.Is(err, model.ErrPlanIdempotency) {
		t.Fatalf("changed replay error=%v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for index := range 2 {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			candidate := created
			candidate.Name = fmt.Sprintf("并发套餐 %d", index)
			_, _, err := repository.SavePlan(context.Background(), model.SavePlanCommand{CommandKey: fmt.Sprintf("%s-update-%d", commandPrefix, index), ExpectedVersion: 1, Plan: candidate})
			results <- err
		}(index)
	}
	close(start)
	group.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, model.ErrPlanConflict):
			conflicts++
		default:
			t.Fatalf("concurrent update error=%v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}

	plans, err := repository.ListPlans(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var current model.Plan
	for _, plan := range plans {
		if plan.ID == planID {
			current = plan
		}
	}
	if current.Version != 2 {
		t.Fatalf("current plan=%+v", current)
	}

	limit := int64(25)
	policyCommand := model.SavePlanPolicyCommand{CommandKey: commandPrefix + "-policy", ExpectedRevision: 1, Policy: model.PlanPolicy{
		PlanID: planID, PermissionCodes: []string{permission}, ProductLimit: &limit,
	}}
	policy, replayed, err := repository.SavePlanPolicy(context.Background(), policyCommand)
	if err != nil || replayed || policy.Revision != 2 || policy.ProductLimit == nil || *policy.ProductLimit != limit || len(policy.PermissionCodes) != 1 {
		t.Fatalf("policy=%+v replayed=%v err=%v", policy, replayed, err)
	}
	replayedPolicy, replayed, err := repository.SavePlanPolicy(context.Background(), policyCommand)
	if err != nil || !replayed || replayedPolicy.Revision != 2 {
		t.Fatalf("policy replay=%+v replayed=%v err=%v", replayedPolicy, replayed, err)
	}
	changedPolicy := policyCommand
	changedPolicy.Policy.PermissionCodes = []string{}
	if _, _, err := repository.SavePlanPolicy(context.Background(), changedPolicy); !errors.Is(err, model.ErrPlanIdempotency) {
		t.Fatalf("changed policy replay error=%v", err)
	}

	policyStart := make(chan struct{})
	policyResults := make(chan error, 2)
	for index := range 2 {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-policyStart
			candidateLimit := int64(30 + index)
			_, _, err := repository.SavePlanPolicy(context.Background(), model.SavePlanPolicyCommand{
				CommandKey: fmt.Sprintf("%s-policy-update-%d", commandPrefix, index), ExpectedRevision: 2,
				Policy: model.PlanPolicy{PlanID: planID, PermissionCodes: []string{permission}, ProductLimit: &candidateLimit},
			})
			policyResults <- err
		}(index)
	}
	close(policyStart)
	group.Wait()
	close(policyResults)
	policySuccesses, policyConflicts := 0, 0
	for err := range policyResults {
		switch {
		case err == nil:
			policySuccesses++
		case errors.Is(err, model.ErrPlanConflict):
			policyConflicts++
		default:
			t.Fatalf("concurrent policy update error=%v", err)
		}
	}
	if policySuccesses != 1 || policyConflicts != 1 {
		t.Fatalf("policy successes=%d conflicts=%d", policySuccesses, policyConflicts)
	}
	retired, replayed, err := repository.RetirePlan(context.Background(), model.RetirePlanCommand{PlanID: planID, CommandKey: commandPrefix + "-retire", ExpectedVersion: 2})
	if err != nil || replayed || retired.Status != model.PlanRetired || retired.Version != 3 {
		t.Fatalf("retired=%+v replayed=%v err=%v", retired, replayed, err)
	}
	var visible int
	if err := database.QueryRow(`SELECT COUNT(*) FROM subscription_plan WHERE plan_id=? AND status='RETIRED'`, planID).Scan(&visible); err != nil || visible != 1 {
		t.Fatalf("retired row count=%d err=%v", visible, err)
	}
}

func TestPlanRejectsInactivePermissionWithoutLeakingCommand(t *testing.T) {
	_, database := integrationRepository(t)
	repository := NewPlanRepository(database)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	code := "invalid-" + suffix
	createKey := "plan-invalid-create-" + suffix
	commandKey := "plan-invalid-permission-" + suffix
	created, _, err := repository.SavePlan(context.Background(), model.SavePlanCommand{CommandKey: createKey, Plan: model.Plan{Code: code, Name: "无效权限套餐", Status: model.PlanActive}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		tx, beginErr := database.Begin()
		if beginErr != nil {
			return
		}
		defer tx.Rollback()
		_, _ = tx.Exec(`DELETE FROM subscription_plan_policy_command WHERE command_key=?`, commandKey)
		_, _ = tx.Exec(`DELETE FROM subscription_plan_command WHERE command_key=?`, createKey)
		_, _ = tx.Exec(`DELETE FROM subscription_plan_permission WHERE plan_id=?`, created.ID)
		_, _ = tx.Exec(`DELETE FROM subscription_plan_quota WHERE plan_id=?`, created.ID)
		_, _ = tx.Exec(`DELETE FROM subscription_plan WHERE plan_id=?`, created.ID)
		_ = tx.Commit()
	})
	_, _, err = repository.SavePlanPolicy(context.Background(), model.SavePlanPolicyCommand{CommandKey: commandKey, ExpectedRevision: 1, Policy: model.PlanPolicy{
		PlanID: created.ID, PermissionCodes: []string{"missing.permission.read"}, ProductLimit: nil,
	}})
	if !errors.Is(err, model.ErrPlanPermissionInactive) {
		t.Fatalf("error=%v", err)
	}
	var commands, permissions int
	if err := database.QueryRow(`SELECT COUNT(*) FROM subscription_plan_policy_command WHERE command_key=?`, commandKey).Scan(&commands); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM subscription_plan_permission WHERE plan_id=?`, created.ID).Scan(&permissions); err != nil {
		t.Fatal(err)
	}
	if commands != 0 || permissions != 0 {
		t.Fatalf("failed command leaked commands=%d permissions=%d", commands, permissions)
	}
}
