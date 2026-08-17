package subscription

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
)

type AssignmentRepository interface {
	GetAssignment(context.Context, int64) (model.Assignment, error)
	Assign(context.Context, model.AssignCommand) (model.Assignment, bool, error)
}

type Assignments struct{ repository AssignmentRepository }

func NewAssignments(repository AssignmentRepository) *Assignments {
	return &Assignments{repository: repository}
}

func (a *Assignments) Get(ctx context.Context, merchantID int64) (model.Assignment, error) {
	if a == nil || a.repository == nil {
		return model.Assignment{}, model.ErrPlanInvalid
	}
	if merchantID <= 0 {
		return model.Assignment{}, model.ErrAssignmentInvalid
	}
	return a.repository.GetAssignment(ctx, merchantID)
}

func (a *Assignments) Assign(ctx context.Context, command model.AssignCommand) (model.Assignment, bool, error) {
	if a == nil || a.repository == nil {
		return model.Assignment{}, false, model.ErrPlanInvalid
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Assignment{}, false, err
	}
	return a.repository.Assign(ctx, normalized)
}
