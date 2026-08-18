package fulfillment

import (
	"context"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

type shippingRepositoryStub struct {
	query      model.ShippingQuery
	rulePage   model.ShippingRulePage
	presetPage model.ShippingPresetPage
	rule       model.ShippingRule
	preset     model.ShippingPreset
	saveRule   model.SaveShippingRuleCommand
	savePreset model.SaveShippingPresetCommand
	enabled    model.SetShippingPresetEnabledCommand
	replayed   bool
	listErr    error
	saveErr    error
}

func (s *shippingRepositoryStub) ListRules(_ context.Context, query model.ShippingQuery) (model.ShippingRulePage, error) {
	s.query = query
	if s.listErr != nil {
		return model.ShippingRulePage{}, s.listErr
	}
	if s.rulePage.Page == 0 {
		return model.ShippingRulePage{Items: []model.ShippingRule{}, Page: query.Page, PageSize: query.PageSize}, nil
	}
	return s.rulePage, nil
}
func (s *shippingRepositoryStub) SaveRule(_ context.Context, command model.SaveShippingRuleCommand) (model.ShippingRule, bool, error) {
	s.saveRule = command
	if s.saveErr != nil {
		return model.ShippingRule{}, false, s.saveErr
	}
	return s.rule, s.replayed, nil
}
func (s *shippingRepositoryStub) RetireRule(context.Context, model.RetireShippingCommand) (model.ShippingRule, bool, error) {
	return s.rule, s.replayed, s.saveErr
}
func (s *shippingRepositoryStub) ListPresets(_ context.Context, query model.ShippingQuery) (model.ShippingPresetPage, error) {
	s.query = query
	if s.listErr != nil {
		return model.ShippingPresetPage{}, s.listErr
	}
	if s.presetPage.Page == 0 {
		return model.ShippingPresetPage{Items: []model.ShippingPreset{}, Page: query.Page, PageSize: query.PageSize}, nil
	}
	return s.presetPage, nil
}
func (s *shippingRepositoryStub) GetPreset(context.Context, int64, int64, int64) (model.ShippingPreset, error) {
	return s.preset, nil
}
func (s *shippingRepositoryStub) SavePreset(_ context.Context, command model.SaveShippingPresetCommand) (model.ShippingPreset, bool, error) {
	s.savePreset = command
	if s.saveErr != nil {
		return model.ShippingPreset{}, false, s.saveErr
	}
	return s.preset, s.replayed, nil
}
func (s *shippingRepositoryStub) SetPresetEnabled(_ context.Context, command model.SetShippingPresetEnabledCommand) (model.ShippingPreset, bool, error) {
	s.enabled = command
	return s.preset, s.replayed, s.saveErr
}
func (s *shippingRepositoryStub) RetirePreset(context.Context, model.RetireShippingCommand) (model.ShippingPreset, bool, error) {
	return s.preset, s.replayed, s.saveErr
}

func sampleShippingRule() model.ShippingRule {
	return model.ShippingRule{
		ID: 11, MerchantID: 2001, ShopID: 3001, Name: "美国标准", Regions: "US",
		FeeFen: 800, FreeOverFen: 9900, MinDays: 3, MaxDays: 7, SortOrder: 1,
		Status: model.ShippingActive, Version: 1, CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(1, 0).UTC(),
	}
}

func sampleShippingPreset() model.ShippingPreset {
	return model.ShippingPreset{
		ID: 21, MerchantID: 2001, ShopID: 3001, Name: "默认发货", IsDefault: true,
		ProductScope: model.ProductScopeAll, ProductIDs: []int64{}, OriginName: "洛杉矶仓",
		OriginRegionCode: "US-CA", OriginRegionName: "California", OriginCountryCode: "US", OriginCountryName: "United States",
		Status: model.ShippingActive, Version: 1, CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(1, 0).UTC(),
		Zones: []model.ShippingZone{{
			ID: 1, Name: "北美", SortOrder: 0,
			Regions: []model.ShippingRegion{{RegionCode: "US", RegionName: "United States", CountryCode: "US", CountryName: "United States"}},
			Rates: []model.ShippingRate{{
				ID: 101, Name: "标准", TransitType: model.TransitStandard, MinDays: 3, MaxDays: 7, Status: model.ShippingActive,
			}},
		}},
	}
}

func TestShippingListRequiresShopScope(t *testing.T) {
	if _, err := NewShipping(&shippingRepositoryStub{}).ListRules(context.Background(), model.ShippingQuery{Page: 1, PageSize: 20}); err != model.ErrShippingInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestShippingListRejectsRetiredFilter(t *testing.T) {
	if _, err := NewShipping(&shippingRepositoryStub{}).ListRules(context.Background(), model.ShippingQuery{
		MerchantID: 2001, ShopID: 3001, Status: model.ShippingRetired,
	}); err != model.ErrShippingInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestShippingListPreservesShopScope(t *testing.T) {
	item := sampleShippingRule()
	repository := &shippingRepositoryStub{rulePage: model.ShippingRulePage{Page: 1, PageSize: 20, Total: 1, Items: []model.ShippingRule{item}}}
	page, err := NewShipping(repository).ListRules(context.Background(), model.ShippingQuery{MerchantID: 2001, ShopID: 3001})
	if err != nil || page.Total != 1 || repository.query.MerchantID != 2001 || repository.query.ShopID != 3001 {
		t.Fatalf("page=%+v query=%+v err=%v", page, repository.query, err)
	}
}

func TestShippingSaveRuleReturnsPersistedResult(t *testing.T) {
	item := sampleShippingRule()
	repository := &shippingRepositoryStub{rule: item}
	value, replayed, err := NewShipping(repository).SaveRule(context.Background(), model.SaveShippingRuleCommand{
		CommandKey: "rule-save-0001", Rule: model.ShippingRule{
			MerchantID: 2001, ShopID: 3001, Name: "美国标准", Regions: "US", FeeFen: 800, FreeOverFen: 9900, MinDays: 3, MaxDays: 7, SortOrder: 1,
		},
	}, true)
	if err != nil || replayed || value.ID != 11 || repository.saveRule.Rule.Name != "美国标准" {
		t.Fatalf("value=%+v replayed=%v err=%v command=%+v", value, replayed, err, repository.saveRule)
	}
}

func TestShippingGetPresetRejectsRetired(t *testing.T) {
	item := sampleShippingPreset()
	item.Status = model.ShippingRetired
	_, err := NewShipping(&shippingRepositoryStub{preset: item}).GetPreset(context.Background(), 2001, 3001, 21)
	if err != model.ErrShippingNotFound {
		t.Fatalf("error=%v", err)
	}
}
