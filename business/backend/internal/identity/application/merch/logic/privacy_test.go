package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance"
	governancemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type stubPrivacyRepo struct{}

func (stubPrivacyRepo) GetPrivacy(_ context.Context, merchantID, shopID int64) (shopmodel.Privacy, error) {
	return shopmodel.DefaultPrivacy(merchantID, shopID), nil
}
func (stubPrivacyRepo) SavePrivacy(_ context.Context, command shopmodel.SavePrivacyCommand) (shopmodel.Privacy, bool, error) {
	value := command.Privacy
	value.ID = 1
	value.Version = 1
	return value, false, nil
}

type stubGovernanceRepo struct{ items []governancemodel.Capability }

func (s stubGovernanceRepo) List(context.Context, governancemodel.Query) (governancemodel.Page, error) {
	return governancemodel.Page{Items: s.items}, nil
}
func (stubGovernanceRepo) Audit(context.Context, governancemodel.AuditQuery) ([]governancemodel.AuditItem, error) {
	return nil, nil
}
func (stubGovernanceRepo) Intervene(context.Context, governancemodel.InterveneCommand) (governancemodel.Capability, bool, error) {
	return governancemodel.Capability{}, false, nil
}

func merchPrivacyContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{MerchantID: 2001, ShopID: 3001})
}

func TestPrivacyRequiresShopContext(t *testing.T) {
	logic := New(nil, nil, nil, nil, nil, shop.NewPrivacySettings(stubPrivacyRepo{}), nil, nil, merchant_governance.NewCapabilities(stubGovernanceRepo{}), Subscription{}, nil, nil, nil)
	if _, err := logic.Privacy(context.Background()); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestPrivacyReturnsDefaultsWhenOverlayIsUnset(t *testing.T) {
	logic := New(nil, nil, nil, nil, nil, shop.NewPrivacySettings(stubPrivacyRepo{}), nil, nil, merchant_governance.NewCapabilities(stubGovernanceRepo{}), Subscription{}, nil, nil, nil)
	value, err := logic.Privacy(merchPrivacyContext())
	if err != nil {
		t.Fatal(err)
	}
	if value.MerchantID != 2001 || value.ShopID != 3001 || value.Version != 0 || !value.CollectConsent || !value.CookieBanner || value.MarketingConsent || !value.Editable || value.PlatformStatus != "active" {
		t.Fatalf("privacy=%+v", value)
	}
}

func TestSavePrivacyIsDeniedWhenPlatformOverlayRestricts(t *testing.T) {
	logic := New(nil, nil, nil, nil, nil, shop.NewPrivacySettings(stubPrivacyRepo{}), nil, nil, merchant_governance.NewCapabilities(stubGovernanceRepo{items: []governancemodel.Capability{{
		ID: 9, MerchantID: 2001, ShopID: 3001, Module: "privacy", PlatformStatus: governancemodel.PlatformRestricted,
		PlatformReasonPublic: "平台限制该店铺隐私设置", Version: 1,
	}}}), Subscription{}, nil, nil, nil)
	value, err := logic.Privacy(merchPrivacyContext())
	if err != nil || value.Editable || value.PlatformStatus != "restricted" {
		t.Fatalf("privacy=%+v err=%v", value, err)
	}
	_, err = logic.SavePrivacy(merchPrivacyContext(), appmodel.SavePrivacy{
		CommandKey: "privacy-save-0001", CollectConsent: true, CookieBanner: true, DataRetentionDays: 365,
	})
	if !errors.Is(err, shopmodel.ErrPrivacyRestricted) {
		t.Fatalf("error=%v", err)
	}
}
