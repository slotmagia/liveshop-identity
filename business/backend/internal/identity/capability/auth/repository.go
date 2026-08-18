// Package auth defines the auth capability use cases and repository ports.
package auth

import (
	"context"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/auth/model"
)

// Repository persists OTP challenges. Dispatch is not part of this port.
type Repository interface {
	CreatePending(context.Context, model.Record) (model.Record, error)
	Consume(ctx context.Context, command model.VerifyCommand, codeHash string, now time.Time) error
	Get(context.Context, string) (model.Record, error)
}

// Notifier sends a committed challenge through Platform notification.
type Notifier interface {
	Dispatch(ctx context.Context, message Dispatch) ([]model.Delivery, error)
}

type Dispatch struct {
	EventKey    string
	DeliveryKey string
	MerchantID  int64
	ShopID      int64
	Phone       string
	Email       string
	Variables   map[string]string
}
