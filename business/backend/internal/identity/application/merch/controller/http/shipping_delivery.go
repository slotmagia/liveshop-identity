package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/shippingdelivery"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type ShippingQueryController struct{ service service.Merch }

func NewShippingQuery(s service.Merch) *ShippingQueryController {
	return &ShippingQueryController{service: s}
}

func (c *ShippingQueryController) ListShops(ctx context.Context, _ *api.ListShopsReq) (*api.ListShopsRes, error) {
	values, err := c.service.ShippingShops(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListShopsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Shop{
			ShopID: value.ShopID, MerchantID: value.MerchantID, Name: value.Name, Code: value.Code, Status: value.Status,
		})
	}
	return &out, nil
}

func (c *ShippingQueryController) ListRules(ctx context.Context, request *api.ListRulesReq) (*api.ListRulesRes, error) {
	value, err := c.service.ShippingRules(ctx, appmodel.ShippingQuery{
		ShopID: request.ShopID, Status: request.Status, Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRulesRes{
		Page: value.Page, PageSize: value.PageSize, Total: value.Total, Items: []api.Rule{},
		PlatformStatus: value.PlatformStatus, PlatformReasonPublic: value.PlatformReasonPublic,
	}
	for _, item := range value.Items {
		out.Items = append(out.Items, wireShippingRule(item))
	}
	return &out, nil
}

func (c *ShippingQueryController) ListPresets(ctx context.Context, request *api.ListPresetsReq) (*api.ListPresetsRes, error) {
	value, err := c.service.ShippingPresets(ctx, appmodel.ShippingQuery{
		ShopID: request.ShopID, Status: request.Status, Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListPresetsRes{
		Page: value.Page, PageSize: value.PageSize, Total: value.Total, Items: []api.Preset{},
		PlatformStatus: value.PlatformStatus, PlatformReasonPublic: value.PlatformReasonPublic,
	}
	for _, item := range value.Items {
		out.Items = append(out.Items, wireShippingPreset(item))
	}
	return &out, nil
}

func (c *ShippingQueryController) GetPreset(ctx context.Context, request *api.GetPresetReq) (*api.GetPresetRes, error) {
	value, err := c.service.ShippingPreset(ctx, request.ShopID, request.PresetId)
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.GetPresetRes{Preset: wireShippingPreset(value)}, nil
}

type ShippingWriteController struct{ service service.Merch }

func NewShippingWrite(s service.Merch) *ShippingWriteController {
	return &ShippingWriteController{service: s}
}

func (c *ShippingWriteController) CreateRule(ctx context.Context, request *api.CreateRuleReq) (*api.CreateRuleRes, error) {
	value, err := c.service.CreateShippingRule(ctx, appmodel.SaveShippingRule{
		CommandKey: request.CommandKey, ShopID: request.ShopID, Name: request.Name, Regions: request.Regions,
		FeeFen: request.FeeFen, FreeOverFen: request.FreeOverFen, MinDays: request.MinDays, MaxDays: request.MaxDays,
		SortOrder: request.SortOrder, Status: request.Status,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateRuleRes{Rule: wireShippingRule(value.Rule), Replayed: value.Replayed}, nil
}

func (c *ShippingWriteController) UpdateRule(ctx context.Context, request *api.UpdateRuleReq) (*api.UpdateRuleRes, error) {
	value, err := c.service.UpdateShippingRule(ctx, appmodel.SaveShippingRule{
		ID: request.RuleId, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, ShopID: request.ShopID,
		Name: request.Name, Regions: request.Regions, FeeFen: request.FeeFen, FreeOverFen: request.FreeOverFen,
		MinDays: request.MinDays, MaxDays: request.MaxDays, SortOrder: request.SortOrder, Status: request.Status,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.UpdateRuleRes{Rule: wireShippingRule(value.Rule), Replayed: value.Replayed}, nil
}

func (c *ShippingWriteController) RetireRule(ctx context.Context, request *api.RetireRuleReq) (*api.RetireRuleRes, error) {
	value, err := c.service.RetireShippingRule(ctx, appmodel.RetireShipping{
		ID: request.RuleId, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.RetireRuleRes{Rule: wireShippingRule(value.Rule), Replayed: value.Replayed}, nil
}

func (c *ShippingWriteController) CreatePreset(ctx context.Context, request *api.CreatePresetReq) (*api.CreatePresetRes, error) {
	value, err := c.service.CreateShippingPreset(ctx, inboundPreset(0, request.CommandKey, 0, request.ShopID, request.Name, request.IsDefault, request.ProductScope, request.ProductIDs, request.OriginName, request.OriginRegionCode, request.OriginRegionName, request.OriginCountryCode, request.OriginCountryName, request.OriginSubdivisionCode, request.OriginSubdivisionName, request.Status, request.Zones))
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreatePresetRes{Preset: wireShippingPreset(value.Preset), Replayed: value.Replayed}, nil
}

func (c *ShippingWriteController) UpdatePreset(ctx context.Context, request *api.UpdatePresetReq) (*api.UpdatePresetRes, error) {
	value, err := c.service.UpdateShippingPreset(ctx, inboundPreset(request.PresetId, request.CommandKey, request.ExpectedVersion, request.ShopID, request.Name, request.IsDefault, request.ProductScope, request.ProductIDs, request.OriginName, request.OriginRegionCode, request.OriginRegionName, request.OriginCountryCode, request.OriginCountryName, request.OriginSubdivisionCode, request.OriginSubdivisionName, request.Status, request.Zones))
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.UpdatePresetRes{Preset: wireShippingPreset(value.Preset), Replayed: value.Replayed}, nil
}

func (c *ShippingWriteController) EnablePreset(ctx context.Context, request *api.EnablePresetReq) (*api.EnablePresetRes, error) {
	value, err := c.service.SetShippingPresetEnabled(ctx, appmodel.SetShippingPresetEnabled{
		PresetID: request.PresetId, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Enabled: true,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.EnablePresetRes{Preset: wireShippingPreset(value.Preset), Replayed: value.Replayed}, nil
}

func (c *ShippingWriteController) DisablePreset(ctx context.Context, request *api.DisablePresetReq) (*api.DisablePresetRes, error) {
	value, err := c.service.SetShippingPresetEnabled(ctx, appmodel.SetShippingPresetEnabled{
		PresetID: request.PresetId, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Enabled: false,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.DisablePresetRes{Preset: wireShippingPreset(value.Preset), Replayed: value.Replayed}, nil
}

func (c *ShippingWriteController) RetirePreset(ctx context.Context, request *api.RetirePresetReq) (*api.RetirePresetRes, error) {
	value, err := c.service.RetireShippingPreset(ctx, appmodel.RetireShipping{
		ID: request.PresetId, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.RetirePresetRes{Preset: wireShippingPreset(value.Preset), Replayed: value.Replayed}, nil
}

func inboundPreset(id int64, commandKey string, expectedVersion uint64, shopID int64, name string, isDefault bool, productScope string, productIDs []int64, originName, originRegionCode, originRegionName, originCountryCode, originCountryName, originSubdivisionCode, originSubdivisionName, status string, zones []api.Zone) appmodel.SaveShippingPreset {
	return appmodel.SaveShippingPreset{
		ID: id, CommandKey: commandKey, ExpectedVersion: expectedVersion, ShopID: shopID, Name: name, IsDefault: isDefault,
		ProductScope: productScope, ProductIDs: productIDs, OriginName: originName, OriginRegionCode: originRegionCode,
		OriginRegionName: originRegionName, OriginCountryCode: originCountryCode, OriginCountryName: originCountryName,
		OriginSubdivisionCode: originSubdivisionCode, OriginSubdivisionName: originSubdivisionName, Status: status,
		Zones: inboundZones(zones),
	}
}

func inboundZones(values []api.Zone) []appmodel.ShippingZone {
	out := make([]appmodel.ShippingZone, 0, len(values))
	for _, zone := range values {
		item := appmodel.ShippingZone{ID: zone.ID, Name: zone.Name, SortOrder: zone.SortOrder}
		for _, region := range zone.Regions {
			item.Regions = append(item.Regions, appmodel.ShippingRegion{
				RegionCode: region.RegionCode, RegionName: region.RegionName, CountryCode: region.CountryCode,
				CountryName: region.CountryName, SubdivisionCode: region.SubdivisionCode, SubdivisionName: region.SubdivisionName,
			})
		}
		for _, rate := range zone.Rates {
			item.Rates = append(item.Rates, appmodel.ShippingRate{
				ID: rate.ID, Name: rate.Name, IsFree: rate.IsFree, PriceFen: rate.PriceFen, TransitType: rate.TransitType,
				MinDays: rate.MinDays, MaxDays: rate.MaxDays, SortOrder: rate.SortOrder, Status: rate.Status,
			})
		}
		out = append(out, item)
	}
	return out
}

func wireShippingRule(value appmodel.ShippingRule) api.Rule {
	return api.Rule{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID, Name: value.Name, Regions: value.Regions,
		FeeFen: value.FeeFen, FreeOverFen: value.FreeOverFen, MinDays: value.MinDays, MaxDays: value.MaxDays,
		SortOrder: value.SortOrder, Status: value.Status, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		PlatformStatus: value.PlatformStatus, PlatformReasonPublic: value.PlatformReasonPublic, Editable: value.Editable,
	}
}

func wireShippingPreset(value appmodel.ShippingPreset) api.Preset {
	return api.Preset{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID, Name: value.Name, IsDefault: value.IsDefault,
		ProductScope: value.ProductScope, ProductIDs: value.ProductIDs, OriginName: value.OriginName,
		OriginRegionCode: value.OriginRegionCode, OriginRegionName: value.OriginRegionName, OriginCountryCode: value.OriginCountryCode,
		OriginCountryName: value.OriginCountryName, OriginSubdivisionCode: value.OriginSubdivisionCode,
		OriginSubdivisionName: value.OriginSubdivisionName, Status: value.Status, Zones: outboundZones(value.Zones),
		Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		PlatformStatus: value.PlatformStatus, PlatformReasonPublic: value.PlatformReasonPublic, Editable: value.Editable,
	}
}

func outboundZones(values []appmodel.ShippingZone) []api.Zone {
	out := make([]api.Zone, 0, len(values))
	for _, zone := range values {
		item := api.Zone{ID: zone.ID, Name: zone.Name, SortOrder: zone.SortOrder, Regions: []api.Region{}, Rates: []api.Rate{}}
		for _, region := range zone.Regions {
			item.Regions = append(item.Regions, api.Region{
				RegionCode: region.RegionCode, RegionName: region.RegionName, CountryCode: region.CountryCode,
				CountryName: region.CountryName, SubdivisionCode: region.SubdivisionCode, SubdivisionName: region.SubdivisionName,
			})
		}
		for _, rate := range zone.Rates {
			item.Rates = append(item.Rates, api.Rate{
				ID: rate.ID, Name: rate.Name, IsFree: rate.IsFree, PriceFen: rate.PriceFen, TransitType: rate.TransitType,
				MinDays: rate.MinDays, MaxDays: rate.MaxDays, SortOrder: rate.SortOrder, Status: rate.Status,
			})
		}
		out = append(out, item)
	}
	return out
}
