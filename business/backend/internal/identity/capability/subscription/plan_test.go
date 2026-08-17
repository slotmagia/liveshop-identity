package subscription

import (
	"context"
	"errors"
	"testing"

	model "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
)

type planRepositoryStub struct {
	saved       model.SavePlanCommand
	savedPolicy model.SavePlanPolicyCommand
	value       model.Plan
	policy      model.PlanPolicy
	err         error
}

func (s *planRepositoryStub) ListPlans(context.Context) ([]model.Plan, error) {
	return []model.Plan{s.value}, s.err
}
func (s *planRepositoryStub) SavePlan(_ context.Context, command model.SavePlanCommand) (model.Plan, bool, error) {
	s.saved = command
	return s.value, false, s.err
}
func (s *planRepositoryStub) RetirePlan(context.Context, model.RetirePlanCommand) (model.Plan, bool, error) {
	return s.value, false, s.err
}
func (s *planRepositoryStub) GetPlanPolicy(context.Context, int64) (model.PlanPolicy, error) {
	return s.policy, s.err
}
func (s *planRepositoryStub) SavePlanPolicy(_ context.Context, command model.SavePlanPolicyCommand) (model.PlanPolicy, bool, error) {
	s.savedPolicy = command
	return s.policy, false, s.err
}

func TestPlanPolicySaveNormalizesCompletePermissionSet(t *testing.T) {
	repository := &planRepositoryStub{policy: model.PlanPolicy{PlanID: 1, Revision: 2}}
	service := NewPlans(repository)
	limit := int64(50)
	_, _, err := service.SavePolicy(context.Background(), model.SavePlanPolicyCommand{
		CommandKey: "plan-policy-command-0001", ExpectedRevision: 1,
		Policy: model.PlanPolicy{PlanID: 1, ProductLimit: &limit,
			PermissionCodes: []string{"trade.order.read", " catalog.product.read ", "trade.order.read"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"catalog.product.read", "trade.order.read"}
	if len(repository.savedPolicy.Policy.PermissionCodes) != len(want) || repository.savedPolicy.Policy.PermissionCodes[0] != want[0] || repository.savedPolicy.Policy.PermissionCodes[1] != want[1] {
		t.Fatalf("normalized permissions = %#v", repository.savedPolicy.Policy.PermissionCodes)
	}
}

func TestPlanPolicySaveRejectsInvalidQuotaBeforeRepository(t *testing.T) {
	repository := &planRepositoryStub{}
	service := NewPlans(repository)
	zero := int64(0)
	_, _, err := service.SavePolicy(context.Background(), model.SavePlanPolicyCommand{CommandKey: "plan-policy-command-0002", ExpectedRevision: 1, Policy: model.PlanPolicy{PlanID: 1, ProductLimit: &zero}})
	if !errors.Is(err, model.ErrPlanInvalid) {
		t.Fatalf("error = %v", err)
	}
}

func TestPlanCommandDigestIsStableAndPayloadSensitive(t *testing.T) {
	command, err := (model.SavePlanCommand{CommandKey: "plan-command-0003", Plan: model.Plan{Code: "premium", Name: "高级版", Status: model.PlanActive}}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	changed := command
	changed.Plan.PriceMinor = 1
	if command.RequestDigest() == changed.RequestDigest() {
		t.Fatal("changed price must change the command digest")
	}
}

func TestPlanPolicyCommandDigestIsOrderIndependent(t *testing.T) {
	command, err := (model.SavePlanPolicyCommand{CommandKey: "plan-policy-command-0003", ExpectedRevision: 1, Policy: model.PlanPolicy{PlanID: 7, PermissionCodes: []string{"trade.order.read", "catalog.product.read"}}}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	reordered, _ := (model.SavePlanPolicyCommand{CommandKey: "plan-policy-command-0003", ExpectedRevision: 1, Policy: model.PlanPolicy{PlanID: 7, PermissionCodes: []string{"catalog.product.read", "trade.order.read"}}}).Normalize()
	if command.RequestDigest() != reordered.RequestDigest() {
		t.Fatal("equivalent permission sets must have one policy command digest")
	}
}
