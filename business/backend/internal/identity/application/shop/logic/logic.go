package logic

import (
	"context"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/auth"
	authmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/auth/model"
)

type Logic struct {
	health *biz.Health
	otp    *auth.OTP
}

var _ service.Shop = (*Logic)(nil)

func New(health *biz.Health, otp *auth.OTP) *Logic {
	return &Logic{health: health, otp: otp}
}

func (l *Logic) Health(ctx context.Context) (appmodel.Health, error) {
	if l.health == nil {
		return appmodel.Health{}, model.ErrUnavailable
	}
	current, err := l.health.Check(ctx)
	if err != nil {
		return appmodel.Health{}, err
	}
	return appmodel.Health{Status: current.Status}, nil
}

func (l *Logic) CreateLoginOTP(ctx context.Context, input appmodel.CreateLoginOTP) (appmodel.LoginOTP, error) {
	if l.otp == nil {
		return appmodel.LoginOTP{}, authmodel.ErrUnavailable
	}
	challenge, err := l.otp.Request(ctx, authmodel.RequestCommand{ShopCode: input.ShopCode, Phone: input.Phone, Email: input.Email})
	if err != nil {
		return appmodel.LoginOTP{}, err
	}
	return appmodel.LoginOTP{ChallengeID: challenge.ID, TTLSeconds: challenge.TTLSeconds, ExpiresAt: challenge.ExpiresAt.UTC().Format(time.RFC3339Nano)}, nil
}

func (l *Logic) CreateLogin(ctx context.Context, input appmodel.CreateLogin) (appmodel.Login, error) {
	if l.otp == nil {
		return appmodel.Login{}, authmodel.ErrUnavailable
	}
	challenge, err := l.otp.Verify(ctx, authmodel.VerifyCommand{ShopCode: input.ShopCode, ChallengeID: input.ChallengeID, Code: input.Code})
	if err != nil {
		return appmodel.Login{}, err
	}
	return appmodel.Login{ChallengeID: challenge.ID, Verified: true}, nil
}
