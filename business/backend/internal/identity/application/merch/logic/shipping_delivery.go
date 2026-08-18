package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	fulfillmentmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
	governancemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
)

func (l *Logic) ShippingShops(ctx context.Context) ([]appmodel.ShippingShop, error) {
	values, err := l.PolicyShops(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.ShippingShop, 0, len(values))
	for _, value := range values {
		out = append(out, appmodel.ShippingShop{
			ShopID: value.ShopID, MerchantID: value.MerchantID, Name: value.Name, Code: value.Code, Status: value.Status,
		})
	}
	return out, nil
}

func (l *Logic) ShippingRules(ctx context.Context, input appmodel.ShippingQuery) (appmodel.ShippingRulePage, error) {
	merchantID, shopID, overlay, err := l.prepareShippingRead(ctx, input.ShopID)
	if err != nil {
		return appmodel.ShippingRulePage{}, err
	}
	page, err := l.shipping.ListRules(ctx, fulfillmentmodel.ShippingQuery{
		MerchantID: merchantID, ShopID: shopID, Status: fulfillmentmodel.ShippingStatus(input.Status),
		Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return appmodel.ShippingRulePage{}, err
	}
	out := appmodel.ShippingRulePage{
		Page: page.Page, PageSize: page.PageSize, Total: page.Total, Items: []appmodel.ShippingRule{},
		PlatformStatus: string(overlay.PlatformStatus), PlatformReasonPublic: overlay.PlatformReasonPublic,
	}
	if out.PlatformStatus == "" {
		out.PlatformStatus = string(governancemodel.PlatformActive)
	}
	for _, item := range page.Items {
		out.Items = append(out.Items, l.projectShippingRule(item, overlay))
	}
	return out, nil
}

func (l *Logic) CreateShippingRule(ctx context.Context, input appmodel.SaveShippingRule) (appmodel.ShippingRuleResult, error) {
	return l.saveShippingRule(ctx, input, true)
}

func (l *Logic) UpdateShippingRule(ctx context.Context, input appmodel.SaveShippingRule) (appmodel.ShippingRuleResult, error) {
	return l.saveShippingRule(ctx, input, false)
}

func (l *Logic) saveShippingRule(ctx context.Context, input appmodel.SaveShippingRule, create bool) (appmodel.ShippingRuleResult, error) {
	merchantID, shopID, overlay, err := l.prepareShippingWrite(ctx, input.ShopID)
	if err != nil {
		return appmodel.ShippingRuleResult{}, err
	}
	value, replayed, err := l.shipping.SaveRule(ctx, fulfillmentmodel.SaveShippingRuleCommand{
		CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
		Rule: fulfillmentmodel.ShippingRule{
			ID: input.ID, MerchantID: merchantID, ShopID: shopID, Name: input.Name, Regions: input.Regions,
			FeeFen: input.FeeFen, FreeOverFen: input.FreeOverFen, MinDays: input.MinDays, MaxDays: input.MaxDays,
			SortOrder: input.SortOrder, Status: fulfillmentmodel.ShippingStatus(input.Status),
		},
	}, create)
	if err != nil {
		return appmodel.ShippingRuleResult{}, err
	}
	return appmodel.ShippingRuleResult{Rule: l.projectShippingRule(value, overlay), Replayed: replayed}, nil
}

func (l *Logic) RetireShippingRule(ctx context.Context, input appmodel.RetireShipping) (appmodel.ShippingRuleResult, error) {
	merchantID, shopID, overlay, err := l.prepareShippingWrite(ctx, input.ShopID)
	if err != nil {
		return appmodel.ShippingRuleResult{}, err
	}
	value, replayed, err := l.shipping.RetireRule(ctx, fulfillmentmodel.RetireShippingCommand{
		ID: input.ID, MerchantID: merchantID, ShopID: shopID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return appmodel.ShippingRuleResult{}, err
	}
	return appmodel.ShippingRuleResult{Rule: l.projectShippingRule(value, overlay), Replayed: replayed}, nil
}

func (l *Logic) ShippingPresets(ctx context.Context, input appmodel.ShippingQuery) (appmodel.ShippingPresetPage, error) {
	merchantID, shopID, overlay, err := l.prepareShippingRead(ctx, input.ShopID)
	if err != nil {
		return appmodel.ShippingPresetPage{}, err
	}
	page, err := l.shipping.ListPresets(ctx, fulfillmentmodel.ShippingQuery{
		MerchantID: merchantID, ShopID: shopID, Status: fulfillmentmodel.ShippingStatus(input.Status),
		Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return appmodel.ShippingPresetPage{}, err
	}
	out := appmodel.ShippingPresetPage{
		Page: page.Page, PageSize: page.PageSize, Total: page.Total, Items: []appmodel.ShippingPreset{},
		PlatformStatus: string(overlay.PlatformStatus), PlatformReasonPublic: overlay.PlatformReasonPublic,
	}
	if out.PlatformStatus == "" {
		out.PlatformStatus = string(governancemodel.PlatformActive)
	}
	for _, item := range page.Items {
		out.Items = append(out.Items, l.projectShippingPreset(item, overlay))
	}
	return out, nil
}

func (l *Logic) ShippingPreset(ctx context.Context, shopID, presetID int64) (appmodel.ShippingPreset, error) {
	merchantID, resolvedShopID, overlay, err := l.prepareShippingRead(ctx, shopID)
	if err != nil {
		return appmodel.ShippingPreset{}, err
	}
	value, err := l.shipping.GetPreset(ctx, merchantID, resolvedShopID, presetID)
	if err != nil {
		return appmodel.ShippingPreset{}, err
	}
	return l.projectShippingPreset(value, overlay), nil
}

func (l *Logic) CreateShippingPreset(ctx context.Context, input appmodel.SaveShippingPreset) (appmodel.ShippingPresetResult, error) {
	return l.saveShippingPreset(ctx, input, true)
}

func (l *Logic) UpdateShippingPreset(ctx context.Context, input appmodel.SaveShippingPreset) (appmodel.ShippingPresetResult, error) {
	return l.saveShippingPreset(ctx, input, false)
}

func (l *Logic) saveShippingPreset(ctx context.Context, input appmodel.SaveShippingPreset, create bool) (appmodel.ShippingPresetResult, error) {
	merchantID, shopID, overlay, err := l.prepareShippingWrite(ctx, input.ShopID)
	if err != nil {
		return appmodel.ShippingPresetResult{}, err
	}
	value, replayed, err := l.shipping.SavePreset(ctx, fulfillmentmodel.SaveShippingPresetCommand{
		CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
		Preset: fulfillmentmodel.ShippingPreset{
			ID: input.ID, MerchantID: merchantID, ShopID: shopID, Name: input.Name, IsDefault: input.IsDefault,
			ProductScope: fulfillmentmodel.ProductScope(input.ProductScope), ProductIDs: input.ProductIDs,
			OriginName: input.OriginName, OriginRegionCode: input.OriginRegionCode, OriginRegionName: input.OriginRegionName,
			OriginCountryCode: input.OriginCountryCode, OriginCountryName: input.OriginCountryName,
			OriginSubdivisionCode: input.OriginSubdivisionCode, OriginSubdivisionName: input.OriginSubdivisionName,
			Status: fulfillmentmodel.ShippingStatus(input.Status), Zones: projectInboundZones(input.Zones),
		},
	}, create)
	if err != nil {
		return appmodel.ShippingPresetResult{}, err
	}
	return appmodel.ShippingPresetResult{Preset: l.projectShippingPreset(value, overlay), Replayed: replayed}, nil
}

func (l *Logic) SetShippingPresetEnabled(ctx context.Context, input appmodel.SetShippingPresetEnabled) (appmodel.ShippingPresetResult, error) {
	merchantID, shopID, overlay, err := l.prepareShippingWrite(ctx, input.ShopID)
	if err != nil {
		return appmodel.ShippingPresetResult{}, err
	}
	value, replayed, err := l.shipping.SetPresetEnabled(ctx, fulfillmentmodel.SetShippingPresetEnabledCommand{
		PresetID: input.PresetID, MerchantID: merchantID, ShopID: shopID, CommandKey: input.CommandKey,
		ExpectedVersion: input.ExpectedVersion, Enabled: input.Enabled,
	})
	if err != nil {
		return appmodel.ShippingPresetResult{}, err
	}
	return appmodel.ShippingPresetResult{Preset: l.projectShippingPreset(value, overlay), Replayed: replayed}, nil
}

func (l *Logic) RetireShippingPreset(ctx context.Context, input appmodel.RetireShipping) (appmodel.ShippingPresetResult, error) {
	merchantID, shopID, overlay, err := l.prepareShippingWrite(ctx, input.ShopID)
	if err != nil {
		return appmodel.ShippingPresetResult{}, err
	}
	value, replayed, err := l.shipping.RetirePreset(ctx, fulfillmentmodel.RetireShippingCommand{
		ID: input.ID, MerchantID: merchantID, ShopID: shopID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return appmodel.ShippingPresetResult{}, err
	}
	return appmodel.ShippingPresetResult{Preset: l.projectShippingPreset(value, overlay), Replayed: replayed}, nil
}

func (l *Logic) prepareShippingRead(ctx context.Context, requestedShopID int64) (int64, int64, governancemodel.Capability, error) {
	merchantID, shopID, err := l.appShopID(ctx, requestedShopID)
	if err != nil {
		return 0, 0, governancemodel.Capability{}, err
	}
	if l.shipping == nil {
		return 0, 0, governancemodel.Capability{}, model.ErrUnavailable
	}
	overlay, err := l.shippingOverlay(ctx, merchantID, shopID)
	if err != nil {
		return 0, 0, governancemodel.Capability{}, err
	}
	return merchantID, shopID, overlay, nil
}

func (l *Logic) prepareShippingWrite(ctx context.Context, requestedShopID int64) (int64, int64, governancemodel.Capability, error) {
	merchantID, shopID, overlay, err := l.prepareShippingRead(ctx, requestedShopID)
	if err != nil {
		return 0, 0, governancemodel.Capability{}, err
	}
	if overlay.PlatformStatus != "" && overlay.PlatformStatus != governancemodel.PlatformActive {
		return 0, 0, governancemodel.Capability{}, fmt.Errorf("%w: %s", fulfillmentmodel.ErrShippingRestricted, overlay.PlatformReasonPublic)
	}
	return merchantID, shopID, overlay, nil
}

func (l *Logic) shippingOverlay(ctx context.Context, merchantID, shopID int64) (governancemodel.Capability, error) {
	if l.merchantGovernance == nil {
		return governancemodel.Capability{PlatformStatus: governancemodel.PlatformActive}, nil
	}
	page, err := l.merchantGovernance.List(ctx, governancemodel.Query{MerchantID: merchantID, ShopID: shopID, Module: "shipping"})
	if err != nil {
		return governancemodel.Capability{}, err
	}
	if len(page.Items) == 0 {
		return governancemodel.Capability{MerchantID: merchantID, ShopID: shopID, Module: "shipping", PlatformStatus: governancemodel.PlatformActive}, nil
	}
	return page.Items[0], nil
}

func (l *Logic) projectShippingRule(value fulfillmentmodel.ShippingRule, overlay governancemodel.Capability) appmodel.ShippingRule {
	status := string(overlay.PlatformStatus)
	if status == "" {
		status = string(governancemodel.PlatformActive)
	}
	return appmodel.ShippingRule{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID, Name: value.Name, Regions: value.Regions,
		FeeFen: value.FeeFen, FreeOverFen: value.FreeOverFen, MinDays: value.MinDays, MaxDays: value.MaxDays,
		SortOrder: value.SortOrder, Status: string(value.Status), Version: value.Version,
		CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339Nano),
		PlatformStatus: status, PlatformReasonPublic: overlay.PlatformReasonPublic,
		Editable: overlay.PlatformStatus == "" || overlay.PlatformStatus == governancemodel.PlatformActive,
	}
}

func (l *Logic) projectShippingPreset(value fulfillmentmodel.ShippingPreset, overlay governancemodel.Capability) appmodel.ShippingPreset {
	status := string(overlay.PlatformStatus)
	if status == "" {
		status = string(governancemodel.PlatformActive)
	}
	return appmodel.ShippingPreset{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID, Name: value.Name, IsDefault: value.IsDefault,
		ProductScope: string(value.ProductScope), ProductIDs: value.ProductIDs, OriginName: value.OriginName,
		OriginRegionCode: value.OriginRegionCode, OriginRegionName: value.OriginRegionName, OriginCountryCode: value.OriginCountryCode,
		OriginCountryName: value.OriginCountryName, OriginSubdivisionCode: value.OriginSubdivisionCode,
		OriginSubdivisionName: value.OriginSubdivisionName, Status: string(value.Status), Zones: projectOutboundZones(value.Zones),
		Version: value.Version, CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339Nano),
		PlatformStatus: status, PlatformReasonPublic: overlay.PlatformReasonPublic,
		Editable: overlay.PlatformStatus == "" || overlay.PlatformStatus == governancemodel.PlatformActive,
	}
}

func projectOutboundZones(values []fulfillmentmodel.ShippingZone) []appmodel.ShippingZone {
	out := make([]appmodel.ShippingZone, 0, len(values))
	for _, zone := range values {
		item := appmodel.ShippingZone{ID: zone.ID, Name: zone.Name, SortOrder: zone.SortOrder, Regions: []appmodel.ShippingRegion{}, Rates: []appmodel.ShippingRate{}}
		for _, region := range zone.Regions {
			item.Regions = append(item.Regions, appmodel.ShippingRegion{
				RegionCode: region.RegionCode, RegionName: region.RegionName, CountryCode: region.CountryCode,
				CountryName: region.CountryName, SubdivisionCode: region.SubdivisionCode, SubdivisionName: region.SubdivisionName,
			})
		}
		for _, rate := range zone.Rates {
			item.Rates = append(item.Rates, appmodel.ShippingRate{
				ID: rate.ID, Name: rate.Name, IsFree: rate.IsFree, PriceFen: rate.PriceFen, TransitType: string(rate.TransitType),
				MinDays: rate.MinDays, MaxDays: rate.MaxDays, SortOrder: rate.SortOrder, Status: string(rate.Status),
			})
		}
		out = append(out, item)
	}
	return out
}

func projectInboundZones(values []appmodel.ShippingZone) []fulfillmentmodel.ShippingZone {
	out := make([]fulfillmentmodel.ShippingZone, 0, len(values))
	for _, zone := range values {
		item := fulfillmentmodel.ShippingZone{ID: zone.ID, Name: zone.Name, SortOrder: zone.SortOrder}
		for _, region := range zone.Regions {
			item.Regions = append(item.Regions, fulfillmentmodel.ShippingRegion{
				RegionCode: region.RegionCode, RegionName: region.RegionName, CountryCode: region.CountryCode,
				CountryName: region.CountryName, SubdivisionCode: region.SubdivisionCode, SubdivisionName: region.SubdivisionName,
			})
		}
		for _, rate := range zone.Rates {
			item.Rates = append(item.Rates, fulfillmentmodel.ShippingRate{
				ID: rate.ID, Name: rate.Name, IsFree: rate.IsFree, PriceFen: rate.PriceFen, TransitType: fulfillmentmodel.TransitType(rate.TransitType),
				MinDays: rate.MinDays, MaxDays: rate.MaxDays, SortOrder: rate.SortOrder, Status: fulfillmentmodel.ShippingStatus(rate.Status),
			})
		}
		out = append(out, item)
	}
	return out
}
