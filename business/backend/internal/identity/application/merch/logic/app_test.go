package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance"
	governancemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type stubAppRepo struct {
	items        []shopmodel.App
	createCalled bool
	resetCalled  bool
	lastEnable   shopmodel.SetAppEnabledCommand
}

func sampleApp() shopmodel.App {
	return shopmodel.App{
		ID: 21, MerchantID: 2001, ShopID: 3001, Name: "订单同步",
		ClientID: "app_abcdefabcdefabcdefabcdef", SecretHint: "abcdef", Scopes: "orders:read,products:read",
		Status: shopmodel.AppActive, Version: 1, CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}
}

func (s stubAppRepo) ListApps(_ context.Context, query shopmodel.AppQuery) (shopmodel.AppPage, error) {
	return shopmodel.AppPage{Items: s.items, Page: query.Page, PageSize: query.PageSize, Total: int64(len(s.items))}, nil
}
func (s *stubAppRepo) CreateApp(context.Context, shopmodel.CreateAppCommand) (shopmodel.AppMutation, bool, error) {
	s.createCalled = true
	return shopmodel.AppMutation{App: sampleApp(), ClientSecret: "sec_once"}, false, nil
}
func (s *stubAppRepo) ResetAppSecret(context.Context, shopmodel.ResetAppSecretCommand) (shopmodel.AppMutation, bool, error) {
	s.resetCalled = true
	value := sampleApp()
	value.Version = 2
	value.SecretHint = "xyzxyz"
	return shopmodel.AppMutation{App: value, ClientSecret: "sec_rotated"}, false, nil
}
func (s *stubAppRepo) SetAppEnabled(_ context.Context, command shopmodel.SetAppEnabledCommand) (shopmodel.App, bool, error) {
	s.lastEnable = command
	value := sampleApp()
	if command.Enabled {
		value.Status = shopmodel.AppActive
	} else {
		value.Status = shopmodel.AppDisabled
	}
	value.Version = command.ExpectedVersion + 1
	return value, false, nil
}

func merchAppLogic(items []governancemodel.Capability) (*Logic, *stubAppRepo) {
	repo := &stubAppRepo{items: []shopmodel.App{sampleApp()}}
	return New(nil, nil, nil, nil, shop.NewDirectory(stubShopRepo{}), nil, nil, shop.NewPrivateApps(repo), merchant_governance.NewCapabilities(stubGovernanceRepo{items: items}), Subscription{}, nil, nil, nil), repo
}

func TestAppShopsRequiresMerchantContext(t *testing.T) {
	logic, _ := merchAppLogic(nil)
	if _, err := logic.AppShops(context.Background()); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestAppsRejectsForeignShopWhenSessionIsBound(t *testing.T) {
	logic, _ := merchAppLogic(nil)
	if _, err := logic.Apps(merchPolicyContext(), appmodel.AppQuery{ShopID: 4001}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestAppsAttachesSyntheticActiveOverlay(t *testing.T) {
	logic, _ := merchAppLogic(nil)
	page, err := logic.Apps(merchPolicyContext(), appmodel.AppQuery{ShopID: 3001, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].PlatformStatus != "active" || page.Items[0].Status != "ACTIVE" || page.PlatformStatus != "active" || page.Items[0].SecretHint != "abcdef" {
		t.Fatalf("page=%+v", page)
	}
}

func TestCreateAndResetAppAreDeniedWhenOverlayRestricts(t *testing.T) {
	logic, repo := merchAppLogic([]governancemodel.Capability{{
		ID: 8, MerchantID: 2001, ShopID: 3001, Module: "apps", PlatformStatus: governancemodel.PlatformRestricted,
		PlatformReasonPublic: "平台限制应用凭据", Version: 1,
	}})
	_, err := logic.CreateApp(merchPolicyContext(), appmodel.CreateApp{
		CommandKey: "app-create-0001", ShopID: 3001, Name: "订单同步", Scopes: "orders:read",
	})
	if !errors.Is(err, shopmodel.ErrAppRestricted) || repo.createCalled {
		t.Fatalf("create error=%v called=%v", err, repo.createCalled)
	}
	_, err = logic.ResetAppSecret(merchPolicyContext(), appmodel.ResetAppSecret{
		AppID: 21, ShopID: 3001, CommandKey: "app-reset-0001", ExpectedVersion: 1,
	})
	if !errors.Is(err, shopmodel.ErrAppRestricted) || repo.resetCalled {
		t.Fatalf("reset error=%v called=%v", err, repo.resetCalled)
	}
	_, err = logic.SetAppEnabled(merchPolicyContext(), appmodel.SetAppEnabled{
		AppID: 21, ShopID: 3001, CommandKey: "app-enable-0001", ExpectedVersion: 1, Enabled: true,
	})
	if !errors.Is(err, shopmodel.ErrAppRestricted) {
		t.Fatalf("enable error=%v", err)
	}
}

func TestDisableAppRemainsAllowedWhenOverlayRestricts(t *testing.T) {
	logic, repo := merchAppLogic([]governancemodel.Capability{{
		ID: 8, MerchantID: 2001, ShopID: 3001, Module: "apps", PlatformStatus: governancemodel.PlatformRestricted,
		PlatformReasonPublic: "平台限制应用凭据", Version: 1,
	}})
	result, err := logic.SetAppEnabled(merchPolicyContext(), appmodel.SetAppEnabled{
		AppID: 21, ShopID: 3001, CommandKey: "app-disable-0001", ExpectedVersion: 1, Enabled: false,
	})
	if err != nil || result.App.Status != "DISABLED" || result.App.PlatformStatus != "restricted" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if repo.lastEnable.Enabled || repo.lastEnable.AppID != 21 {
		t.Fatalf("disable command=%+v", repo.lastEnable)
	}
}

func TestAppScopesReturnsClosedCatalog(t *testing.T) {
	logic, _ := merchAppLogic(nil)
	values, err := logic.AppScopes(merchPolicyContext())
	if err != nil || len(values) != len(shopmodel.AppScopeCatalog()) || values[0].Code != "orders:read" {
		t.Fatalf("scopes=%+v err=%v", values, err)
	}
}
