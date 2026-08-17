package subscription

import (
	"context"
	"strings"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
)

type PlanRepository interface {
	ListPlans(context.Context) ([]model.Plan, error)
	SavePlan(context.Context, model.SavePlanCommand) (model.Plan, bool, error)
	RetirePlan(context.Context, model.RetirePlanCommand) (model.Plan, bool, error)
	GetPlanPolicy(context.Context, int64) (model.PlanPolicy, error)
	SavePlanPolicy(context.Context, model.SavePlanPolicyCommand) (model.PlanPolicy, bool, error)
}

func (p *Plans) Policy(ctx context.Context, planID int64) (model.PlanPolicy, error) {
	if planID <= 0 {
		return model.PlanPolicy{}, model.ErrPlanInvalid
	}
	return p.repository.GetPlanPolicy(ctx, planID)
}

func (p *Plans) SavePolicy(ctx context.Context, command model.SavePlanPolicyCommand) (model.PlanPolicy, bool, error) {
	normalized, err := command.Normalize()
	if err != nil {
		return model.PlanPolicy{}, false, err
	}
	return p.repository.SavePlanPolicy(ctx, normalized)
}

type Plans struct{ repository PlanRepository }

func NewPlans(repository PlanRepository) *Plans { return &Plans{repository: repository} }

func (p *Plans) List(ctx context.Context) ([]model.Plan, error) {
	return p.repository.ListPlans(ctx)
}

func (p *Plans) Save(ctx context.Context, command model.SavePlanCommand) (model.Plan, bool, error) {
	normalized, err := command.Normalize()
	if err != nil {
		return model.Plan{}, false, err
	}
	return p.repository.SavePlan(ctx, normalized)
}

func (p *Plans) Retire(ctx context.Context, command model.RetirePlanCommand) (model.Plan, bool, error) {
	if err := command.Validate(); err != nil {
		return model.Plan{}, false, err
	}
	command.CommandKey = strings.TrimSpace(command.CommandKey)
	return p.repository.RetirePlan(ctx, command)
}
