package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant"
	merchantmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type stubProfileRepo struct{ record merchantmodel.Record }

func (stubProfileRepo) ListMerchants(context.Context) ([]merchantmodel.Merchant, error) {
	return nil, nil
}
func (stubProfileRepo) ListMerchantPage(context.Context, merchantmodel.Query) (merchantmodel.Page, error) {
	return merchantmodel.Page{}, nil
}
func (s stubProfileRepo) GetMerchant(_ context.Context, merchantID int64) (merchantmodel.Record, error) {
	if merchantID != s.record.ID {
		return merchantmodel.Record{}, merchantmodel.ErrNotFound
	}
	return s.record, nil
}
func (stubProfileRepo) CreateMerchant(context.Context, merchantmodel.CreateCommand) (merchantmodel.CreateResult, bool, error) {
	return merchantmodel.CreateResult{}, false, nil
}
func (stubProfileRepo) UpdateMerchant(context.Context, merchantmodel.UpdateCommand) (merchantmodel.Record, bool, error) {
	return merchantmodel.Record{}, false, nil
}
func (s *stubProfileRepo) UpdateProfile(_ context.Context, command merchantmodel.UpdateProfileCommand) (merchantmodel.Record, bool, error) {
	s.record.ExternalID = command.ExternalID
	s.record.ContactName = command.ContactName
	s.record.ContactPhone = command.ContactPhone
	s.record.MarketingEmailOptIn = command.MarketingEmailOptIn
	s.record.MarketingSMSOptIn = command.MarketingSMSOptIn
	s.record.Version = command.ExpectedVersion + 1
	return s.record, false, nil
}
func (stubProfileRepo) ResetOwnerPassword(context.Context, merchantmodel.ResetPasswordCommand) (bool, error) {
	return false, nil
}
func (stubProfileRepo) CloseMerchant(context.Context, merchantmodel.CloseCommand) (merchantmodel.Record, bool, error) {
	return merchantmodel.Record{}, false, nil
}

func sampleProfileRecord() merchantmodel.Record {
	return merchantmodel.Record{
		ID: 2001, Name: "Local Merchant", Account: "merchant", ExternalID: "ext-2001",
		ContactName: "Ada", ContactPhone: "13800000000", Status: merchantmodel.StatusActive, Version: 1,
	}
}

func merchProfileLogic(record merchantmodel.Record) *Logic {
	return New(nil, nil, nil, nil, nil, nil, nil, nil, nil, Subscription{}, merchant.NewDirectory(&stubProfileRepo{record: record}), nil, nil, nil, nil, nil, nil, nil, nil)
}

func merchProfileOwnerContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{
		Subject: "owner-1", PrincipalType: principal.TypeMerchantOwner, MerchantID: 2001,
	})
}

func TestProfileRequiresMerchantContext(t *testing.T) {
	logic := merchProfileLogic(sampleProfileRecord())
	if _, err := logic.Profile(context.Background()); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestProfileRejectsStaffEvenWithMerchantContext(t *testing.T) {
	logic := merchProfileLogic(sampleProfileRecord())
	ctx := authctx.With(context.Background(), modulesession.Claims{
		Subject: "staff-1", PrincipalType: principal.TypeMerchantStaff, MerchantID: 2001,
	})
	if _, err := logic.Profile(ctx); !errors.Is(err, model.ErrProtectedOwner) {
		t.Fatalf("error=%v", err)
	}
	if _, err := logic.SaveProfile(ctx, appmodel.SaveProfile{CommandKey: "profile-save-0001", ExpectedVersion: 1, ContactName: "Ada"}); !errors.Is(err, model.ErrProtectedOwner) {
		t.Fatalf("save error=%v", err)
	}
}

func TestProfileReturnsOwnerSelfServiceFields(t *testing.T) {
	logic := merchProfileLogic(sampleProfileRecord())
	value, err := logic.Profile(merchProfileOwnerContext())
	if err != nil {
		t.Fatal(err)
	}
	if value.MerchantID != 2001 || value.Name != "Local Merchant" || value.Account != "merchant" || value.ExternalID != "ext-2001" || value.ContactName != "Ada" || !value.Owner || value.Version != 1 {
		t.Fatalf("profile=%+v", value)
	}
}

func TestSaveProfileUpdatesSelfServiceFieldsOnly(t *testing.T) {
	logic := merchProfileLogic(sampleProfileRecord())
	result, err := logic.SaveProfile(merchProfileOwnerContext(), appmodel.SaveProfile{
		CommandKey: "profile-save-0001", ExpectedVersion: 1, ExternalID: "ext-9",
		ContactName: "Bob", ContactPhone: "13900000000", MarketingEmailOptIn: true, MarketingSMSOptIn: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Replayed || result.Profile.Name != "Local Merchant" || result.Profile.Account != "merchant" || result.Profile.ExternalID != "ext-9" || result.Profile.ContactName != "Bob" || result.Profile.ContactPhone != "13900000000" || !result.Profile.MarketingEmailOptIn || !result.Profile.MarketingSMSOptIn || result.Profile.Version != 2 {
		t.Fatalf("result=%+v", result)
	}
}
