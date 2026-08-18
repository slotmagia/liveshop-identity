package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment"
	fulfillmentmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance"
	governancemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
)

type stubShippingRepo struct {
	query      fulfillmentmodel.ShippingQuery
	rule       fulfillmentmodel.ShippingRule
	preset     fulfillmentmodel.ShippingPreset
	saveCalled bool
	err        error
}

func sampleMerchShippingRule() fulfillmentmodel.ShippingRule {
	return fulfillmentmodel.ShippingRule{
		ID: 11, MerchantID: 2001, ShopID: 3001, Name: "美国标准", Regions: "US",
		FeeFen: 800, FreeOverFen: 9900, MinDays: 3, MaxDays: 7, SortOrder: 1,
		Status: fulfillmentmodel.ShippingActive, Version: 1, CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(1, 0).UTC(),
	}
}

func sampleMerchShippingPreset() fulfillmentmodel.ShippingPreset {
	return fulfillmentmodel.ShippingPreset{
		ID: 21, MerchantID: 2001, ShopID: 3001, Name: "默认发货", IsDefault: true,
		ProductScope: fulfillmentmodel.ProductScopeAll, ProductIDs: []int64{}, OriginName: "洛杉矶仓",
		OriginRegionCode: "US-CA", OriginRegionName: "California", OriginCountryCode: "US", OriginCountryName: "United States",
		Status: fulfillmentmodel.ShippingActive, Version: 1, CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(1, 0).UTC(),
		Zones: []fulfillmentmodel.ShippingZone{{
			ID: 1, Name: "北美",
			Regions: []fulfillmentmodel.ShippingRegion{{RegionCode: "US", RegionName: "United States", CountryCode: "US", CountryName: "United States"}},
			Rates: []fulfillmentmodel.ShippingRate{{
				ID: 101, Name: "标准", TransitType: fulfillmentmodel.TransitStandard, MinDays: 3, MaxDays: 7, Status: fulfillmentmodel.ShippingActive,
			}},
		}},
	}
}

func (s *stubShippingRepo) ListRules(_ context.Context, query fulfillmentmodel.ShippingQuery) (fulfillmentmodel.ShippingRulePage, error) {
	s.query = query
	if s.err != nil {
		return fulfillmentmodel.ShippingRulePage{}, s.err
	}
	return fulfillmentmodel.ShippingRulePage{Items: []fulfillmentmodel.ShippingRule{s.rule}, Page: query.Page, PageSize: query.PageSize, Total: 1}, nil
}
func (s *stubShippingRepo) SaveRule(_ context.Context, command fulfillmentmodel.SaveShippingRuleCommand) (fulfillmentmodel.ShippingRule, bool, error) {
	s.saveCalled = true
	if s.err != nil {
		return fulfillmentmodel.ShippingRule{}, false, s.err
	}
	value := s.rule
	value.Name = command.Rule.Name
	return value, false, nil
}
func (s *stubShippingRepo) RetireRule(context.Context, fulfillmentmodel.RetireShippingCommand) (fulfillmentmodel.ShippingRule, bool, error) {
	s.saveCalled = true
	value := s.rule
	value.Status = fulfillmentmodel.ShippingRetired
	value.Version = 2
	return value, false, s.err
}
func (s *stubShippingRepo) ListPresets(_ context.Context, query fulfillmentmodel.ShippingQuery) (fulfillmentmodel.ShippingPresetPage, error) {
	s.query = query
	if s.err != nil {
		return fulfillmentmodel.ShippingPresetPage{}, s.err
	}
	return fulfillmentmodel.ShippingPresetPage{Items: []fulfillmentmodel.ShippingPreset{s.preset}, Page: query.Page, PageSize: query.PageSize, Total: 1}, nil
}
func (s *stubShippingRepo) GetPreset(context.Context, int64, int64, int64) (fulfillmentmodel.ShippingPreset, error) {
	return s.preset, s.err
}
func (s *stubShippingRepo) SavePreset(context.Context, fulfillmentmodel.SaveShippingPresetCommand) (fulfillmentmodel.ShippingPreset, bool, error) {
	s.saveCalled = true
	return s.preset, false, s.err
}
func (s *stubShippingRepo) SetPresetEnabled(context.Context, fulfillmentmodel.SetShippingPresetEnabledCommand) (fulfillmentmodel.ShippingPreset, bool, error) {
	s.saveCalled = true
	return s.preset, false, s.err
}
func (s *stubShippingRepo) RetirePreset(context.Context, fulfillmentmodel.RetireShippingCommand) (fulfillmentmodel.ShippingPreset, bool, error) {
	s.saveCalled = true
	value := s.preset
	value.Status = fulfillmentmodel.ShippingRetired
	value.IsDefault = false
	value.Version = 2
	return value, false, s.err
}

func merchShippingLogic(items []governancemodel.Capability) (*Logic, *stubShippingRepo) {
	repo := &stubShippingRepo{rule: sampleMerchShippingRule(), preset: sampleMerchShippingPreset()}
	return New(nil, nil, nil, nil, shop.NewDirectory(stubShopRepo{}), nil, nil, nil, merchant_governance.NewCapabilities(stubGovernanceRepo{items: items}), Subscription{}, nil, nil, nil, nil, nil, nil, nil, nil, fulfillment.NewShipping(repo)), repo
}

func TestShippingShopsRequiresMerchantContext(t *testing.T) {
	logic, _ := merchShippingLogic(nil)
	if _, err := logic.ShippingShops(context.Background()); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestShippingRulesRejectsForeignShopWhenSessionIsBound(t *testing.T) {
	logic, _ := merchShippingLogic(nil)
	if _, err := logic.ShippingRules(merchPolicyContext(), appmodel.ShippingQuery{ShopID: 4001}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestShippingRulesAttachesSyntheticActiveOverlay(t *testing.T) {
	logic, repo := merchShippingLogic(nil)
	page, err := logic.ShippingRules(merchPolicyContext(), appmodel.ShippingQuery{ShopID: 3001, Page: 1, PageSize: 20})
	if err != nil || page.Total != 1 || page.Items[0].PlatformStatus != "active" || page.Items[0].Name != "美国标准" || repo.query.ShopID != 3001 {
		t.Fatalf("page=%+v query=%+v err=%v", page, repo.query, err)
	}
}

func TestShippingWritesAreDeniedWhenOverlayRestricts(t *testing.T) {
	logic, repo := merchShippingLogic([]governancemodel.Capability{{
		ID: 9, MerchantID: 2001, ShopID: 3001, Module: "shipping", PlatformStatus: governancemodel.PlatformRestricted,
		PlatformReasonPublic: "平台限制配送", Version: 1,
	}})
	_, err := logic.CreateShippingRule(merchPolicyContext(), appmodel.SaveShippingRule{
		CommandKey: "rule-create-0001", ShopID: 3001, Name: "美国标准", Regions: "US", FeeFen: 800, MinDays: 3, MaxDays: 7,
	})
	if !errors.Is(err, fulfillmentmodel.ErrShippingRestricted) || repo.saveCalled {
		t.Fatalf("create error=%v called=%v", err, repo.saveCalled)
	}
	_, err = logic.RetireShippingRule(merchPolicyContext(), appmodel.RetireShipping{
		ID: 11, ShopID: 3001, CommandKey: "rule-retire-0001", ExpectedVersion: 1,
	})
	if !errors.Is(err, fulfillmentmodel.ErrShippingRestricted) {
		t.Fatalf("retire error=%v", err)
	}
}

func TestCreateShippingRuleUsesSessionMerchant(t *testing.T) {
	logic, _ := merchShippingLogic(nil)
	result, err := logic.CreateShippingRule(merchPolicyContext(), appmodel.SaveShippingRule{
		CommandKey: "rule-create-0002", ShopID: 3001, Name: "美国标准", Regions: "US", FeeFen: 800, MinDays: 3, MaxDays: 7,
	})
	if err != nil || result.Rule.MerchantID != 2001 || result.Rule.ShopID != 3001 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
