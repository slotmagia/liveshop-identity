package service

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/appmodel"
)

type Shop interface {
	Health(ctx context.Context) (appmodel.Health, error)
	CreateLoginOTP(ctx context.Context, input appmodel.CreateLoginOTP) (appmodel.LoginOTP, error)
	CreateLogin(ctx context.Context, input appmodel.CreateLogin) (appmodel.Login, error)
}
