package logic

import (
	"context"

	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	merchantmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

func (l *Logic) Profile(ctx context.Context) (appmodel.Profile, error) {
	merchantID, err := l.profileMerchant(ctx)
	if err != nil {
		return appmodel.Profile{}, err
	}
	if l.merchants == nil {
		return appmodel.Profile{}, merchantmodel.ErrUnavailable
	}
	value, err := l.merchants.Get(ctx, merchantID)
	if err != nil {
		return appmodel.Profile{}, err
	}
	if value.Status == merchantmodel.StatusClosed {
		return appmodel.Profile{}, merchantmodel.ErrClosed
	}
	return projectProfile(value, true), nil
}

func (l *Logic) SaveProfile(ctx context.Context, input appmodel.SaveProfile) (appmodel.ProfileMutation, error) {
	merchantID, err := l.profileMerchant(ctx)
	if err != nil {
		return appmodel.ProfileMutation{}, err
	}
	if l.merchants == nil {
		return appmodel.ProfileMutation{}, merchantmodel.ErrUnavailable
	}
	value, replayed, err := l.merchants.UpdateProfile(ctx, merchantmodel.UpdateProfileCommand{
		MerchantID: merchantID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
		ExternalID: input.ExternalID, ContactName: input.ContactName, ContactPhone: input.ContactPhone,
		MarketingEmailOptIn: input.MarketingEmailOptIn, MarketingSMSOptIn: input.MarketingSMSOptIn,
	})
	if err != nil {
		return appmodel.ProfileMutation{}, err
	}
	return appmodel.ProfileMutation{Profile: projectProfile(value, true), Replayed: replayed}, nil
}

func (l *Logic) profileMerchant(ctx context.Context) (int64, error) {
	claims := authctx.Caller(ctx)
	if claims.MerchantID <= 0 {
		return 0, model.ErrInvalidContext
	}
	if claims.PrincipalType != principal.TypeMerchantOwner {
		return 0, model.ErrProtectedOwner
	}
	return claims.MerchantID, nil
}

func projectProfile(value merchantmodel.Record, owner bool) appmodel.Profile {
	return appmodel.Profile{
		MerchantID: value.ID, Name: value.Name, Account: value.Account, ExternalID: value.ExternalID,
		ContactName: value.ContactName, ContactPhone: value.ContactPhone,
		MarketingEmailOptIn: value.MarketingEmailOptIn, MarketingSMSOptIn: value.MarketingSMSOptIn,
		Status: string(value.Status), Version: value.Version, Owner: owner,
	}
}
