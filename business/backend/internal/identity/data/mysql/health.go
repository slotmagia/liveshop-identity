// Package mysql implements the identity repository ports. It owns SQL,
// transactions and row mapping, and never sees a transport type.
package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
)

type HealthRepository struct{ db *sql.DB }

var _ biz.HealthRepository = (*HealthRepository)(nil)

// NewHealthRepository fails fast when no database is configured, so a process
// never starts with a silent in-memory fallback.
func NewHealthRepository(db *sql.DB) (*HealthRepository, error) {
	if db == nil {
		return nil, errors.New("identity: database is required")
	}
	return &HealthRepository{db: db}, nil
}

func (r *HealthRepository) Ready(ctx context.Context) error { return r.db.PingContext(ctx) }
