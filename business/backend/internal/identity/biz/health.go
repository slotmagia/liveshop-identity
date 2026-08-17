// Package biz owns identity use cases and the ports they depend on.
package biz

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

// HealthRepository is the port every storage adapter must satisfy.
type HealthRepository interface {
	Ready(ctx context.Context) error
}

type Health struct{ repository HealthRepository }

func NewHealth(repository HealthRepository) *Health { return &Health{repository: repository} }

func (a *Health) Check(ctx context.Context) (model.Health, error) {
	if a.repository == nil {
		return model.Health{}, model.ErrUnavailable
	}
	if err := a.repository.Ready(ctx); err != nil {
		return model.Health{}, err
	}
	return model.Health{Status: model.StatusActive}, nil
}
