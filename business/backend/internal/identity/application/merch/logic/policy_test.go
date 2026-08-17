package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance"
	governancemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type stubShopRepo struct{}

func (stubShopRepo) ListShops(_ context.Context, merchantID int64) ([]shopmodel.Shop, error) {
	return []shopmodel.Shop{{
		ID: 3001, MerchantID: merchantID, Code: "local-shop", Name: "Local Shop",
		Currency: "CNY", Status: shopmodel.StatusActive, Version: 1,
	}}, nil
}
func (s stubShopRepo) ListShopsByMerchant(ctx context.Context, merchantID int64) ([]shopmodel.Shop, error) {
	return s.ListShops(ctx, merchantID)
}
func (s stubShopRepo) ListManagedShops(_ context.Context, query shopmodel.Query) (shopmodel.Page, error) {
	items, _ := s.ListShops(context.Background(), query.MerchantID)
	return shopmodel.Page{Items: items, Page: query.Page, PageSize: query.PageSize, Total: int64(len(items))}, nil
}
func (s stubShopRepo) GetManagedShop(ctx context.Context, merchantID, shopID int64) (shopmodel.Shop, error) {
	items, err := s.ListShops(ctx, merchantID)
	if err != nil {
		return shopmodel.Shop{}, err
	}
	for _, item := range items {
		if item.ID == shopID {
			return item, nil
		}
	}
	return shopmodel.Shop{}, shopmodel.ErrNotFound
}
func (stubShopRepo) CreateShop(_ context.Context, command shopmodel.CreateCommand) (shopmodel.Shop, bool, error) {
	return shopmodel.Shop{
		ID: 3002, MerchantID: command.MerchantID, Code: "shop-3002", Subdomain: command.Subdomain, Name: command.Name,
		DefaultLocale: command.DefaultLocale, Currency: command.Currency, CategoryCode: command.CategoryCode,
		Status: command.Status, Version: 1,
	}, false, nil
}
func (stubShopRepo) UpdateShop(_ context.Context, command shopmodel.UpdateCommand) (shopmodel.Shop, bool, error) {
	return shopmodel.Shop{
		ID: command.ShopID, MerchantID: command.MerchantID, Code: "local-shop", Subdomain: command.Subdomain,
		Name: command.Name, DefaultLocale: "zh-CN", Currency: "CNY", Status: shopmodel.StatusActive, Version: command.ExpectedVersion + 1,
	}, false, nil
}
func (stubShopRepo) SetShopEnabled(_ context.Context, command shopmodel.SetEnabledCommand) (shopmodel.Shop, bool, error) {
	status := shopmodel.StatusDisabled
	if command.Enabled {
		status = shopmodel.StatusActive
	}
	return shopmodel.Shop{
		ID: command.ShopID, MerchantID: command.MerchantID, Code: "local-shop", Name: "Local Shop",
		Currency: "CNY", Status: status, Version: command.ExpectedVersion + 1,
	}, false, nil
}
func (stubShopRepo) CloseShop(_ context.Context, command shopmodel.CloseCommand) (shopmodel.Shop, bool, error) {
	return shopmodel.Shop{
		ID: command.ShopID, MerchantID: command.MerchantID, Code: "local-shop", Name: "Local Shop",
		Currency: "CNY", Status: shopmodel.StatusClosed, Version: command.ExpectedVersion + 1,
	}, false, nil
}

type stubPolicyRepo struct {
	items []shopmodel.Policy
}

func (s stubPolicyRepo) ListPolicies(_ context.Context, query shopmodel.PolicyQuery) (shopmodel.PolicyPage, error) {
	return shopmodel.PolicyPage{Items: s.items, Page: query.Page, PageSize: query.PageSize, Total: int64(len(s.items))}, nil
}
func (stubPolicyRepo) SavePolicy(_ context.Context, command shopmodel.SavePolicyCommand) (shopmodel.Policy, bool, error) {
	status := shopmodel.PolicyDraft
	if command.Publish {
		status = shopmodel.PolicyPublished
	}
	return shopmodel.Policy{
		ID: 11, MerchantID: command.MerchantID, ShopID: command.ShopID, PolicyType: command.PolicyType,
		Title: command.Title, Content: command.Content, VersionNo: 1, Status: status, Version: 1, CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}, false, nil
}
func (stubPolicyRepo) PublishPolicy(_ context.Context, command shopmodel.PublishPolicyCommand) (shopmodel.Policy, bool, error) {
	return shopmodel.Policy{
		ID: command.PolicyID, MerchantID: command.MerchantID, ShopID: command.ShopID, PolicyType: shopmodel.PolicyTerms,
		Title: "服务条款", Content: "这是一份足够长的店铺服务条款正文。", VersionNo: 1, Status: shopmodel.PolicyPublished, Version: command.ExpectedVersion + 1,
		CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}, false, nil
}

func merchPolicyLogic(items []governancemodel.Capability) *Logic {
	return New(nil, nil, nil, nil, shop.NewDirectory(stubShopRepo{}), nil, shop.NewPolicies(stubPolicyRepo{
		items: []shopmodel.Policy{{
			ID: 11, MerchantID: 2001, ShopID: 3001, PolicyType: shopmodel.PolicyTerms, Title: "服务条款",
			Content: "这是一份足够长的店铺服务条款正文。", VersionNo: 1, Status: shopmodel.PolicyDraft, Version: 1,
			CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
		}},
	}), nil, merchant_governance.NewCapabilities(stubGovernanceRepo{items: items}), Subscription{}, nil, nil, nil)
}

func merchPolicyContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{MerchantID: 2001, ShopID: 3001})
}

func TestPolicyShopsRequiresMerchantContext(t *testing.T) {
	if _, err := merchPolicyLogic(nil).PolicyShops(context.Background()); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestPolicyShopsUsesSessionMerchant(t *testing.T) {
	values, err := merchPolicyLogic(nil).PolicyShops(merchPolicyContext())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].ShopID != 3001 || values[0].MerchantID != 2001 {
		t.Fatalf("shops=%+v", values)
	}
}

func TestPoliciesRejectsForeignShopWhenSessionIsBound(t *testing.T) {
	if _, err := merchPolicyLogic(nil).Policies(merchPolicyContext(), appmodel.PolicyQuery{ShopID: 4001}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestPoliciesAttachesSyntheticActiveOverlay(t *testing.T) {
	page, err := merchPolicyLogic(nil).Policies(merchPolicyContext(), appmodel.PolicyQuery{ShopID: 3001, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].PlatformStatus != "active" || page.Items[0].Status != "DRAFT" || page.PlatformStatus != "active" {
		t.Fatalf("page=%+v", page)
	}
}

func TestSavePolicyDraftRemainsAllowedWhenOverlayRestricts(t *testing.T) {
	logic := merchPolicyLogic([]governancemodel.Capability{{
		ID: 8, MerchantID: 2001, ShopID: 3001, Module: "policies", PlatformStatus: governancemodel.PlatformRestricted,
		PlatformReasonPublic: "平台限制政策发布", Version: 1,
	}})
	result, err := logic.SavePolicy(merchPolicyContext(), appmodel.SavePolicy{
		CommandKey: "policy-draft-0001", ShopID: 3001, PolicyType: "terms", Title: "服务条款", Content: "这是一份足够长的店铺服务条款正文。",
	})
	if err != nil || result.Policy.Status != "DRAFT" || result.Policy.PlatformStatus != "restricted" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPublishPolicyIsDeniedWhenOverlayRestricts(t *testing.T) {
	logic := merchPolicyLogic([]governancemodel.Capability{{
		ID: 8, MerchantID: 2001, ShopID: 3001, Module: "policies", PlatformStatus: governancemodel.PlatformRestricted,
		PlatformReasonPublic: "平台限制政策发布", Version: 1,
	}})
	_, err := logic.PublishPolicy(merchPolicyContext(), appmodel.PublishPolicy{
		PolicyID: 11, ShopID: 3001, CommandKey: "policy-publish-0001", ExpectedVersion: 1,
	})
	if !errors.Is(err, shopmodel.ErrPolicyRestricted) {
		t.Fatalf("error=%v", err)
	}
	_, err = logic.SavePolicy(merchPolicyContext(), appmodel.SavePolicy{
		CommandKey: "policy-publish-save-0001", ShopID: 3001, PolicyType: "terms", Title: "服务条款",
		Content: "这是一份足够长的店铺服务条款正文。", Publish: true,
	})
	if !errors.Is(err, shopmodel.ErrPolicyRestricted) {
		t.Fatalf("save-and-publish error=%v", err)
	}
}
