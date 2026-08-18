package shippingdelivery

import "github.com/gogf/gf/v2/frame/g"

type Shop struct {
	ShopID     int64  `json:"shopId"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
}

type Rule struct {
	ID                   int64  `json:"id"`
	MerchantID           int64  `json:"merchantId"`
	ShopID               int64  `json:"shopId"`
	Name                 string `json:"name"`
	Regions              string `json:"regions"`
	FeeFen               int64  `json:"feeFen"`
	FreeOverFen          int64  `json:"freeOverFen"`
	MinDays              int    `json:"minDays"`
	MaxDays              int    `json:"maxDays"`
	SortOrder            int    `json:"sortOrder"`
	Status               string `json:"status"`
	Version              uint64 `json:"version"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
	PlatformStatus       string `json:"platformStatus"`
	PlatformReasonPublic string `json:"platformReasonPublic,omitempty"`
	Editable             bool   `json:"editable"`
}

type Region struct {
	RegionCode      string `json:"regionCode"`
	RegionName      string `json:"regionName"`
	CountryCode     string `json:"countryCode"`
	CountryName     string `json:"countryName"`
	SubdivisionCode string `json:"subdivisionCode,omitempty"`
	SubdivisionName string `json:"subdivisionName,omitempty"`
}

type Rate struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	IsFree      bool   `json:"isFree"`
	PriceFen    int64  `json:"priceFen"`
	TransitType string `json:"transitType"`
	MinDays     int    `json:"minDays"`
	MaxDays     int    `json:"maxDays"`
	SortOrder   int    `json:"sortOrder"`
	Status      string `json:"status"`
}

type Zone struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	SortOrder int      `json:"sortOrder"`
	Regions   []Region `json:"regions"`
	Rates     []Rate   `json:"rates"`
}

type Preset struct {
	ID                    int64   `json:"id"`
	MerchantID            int64   `json:"merchantId"`
	ShopID                int64   `json:"shopId"`
	Name                  string  `json:"name"`
	IsDefault             bool    `json:"isDefault"`
	ProductScope          string  `json:"productScope"`
	ProductIDs            []int64 `json:"productIds"`
	OriginName            string  `json:"originName"`
	OriginRegionCode      string  `json:"originRegionCode"`
	OriginRegionName      string  `json:"originRegionName"`
	OriginCountryCode     string  `json:"originCountryCode"`
	OriginCountryName     string  `json:"originCountryName"`
	OriginSubdivisionCode string  `json:"originSubdivisionCode,omitempty"`
	OriginSubdivisionName string  `json:"originSubdivisionName,omitempty"`
	Status                string  `json:"status"`
	Zones                 []Zone  `json:"zones"`
	Version               uint64  `json:"version"`
	CreatedAt             string  `json:"createdAt"`
	UpdatedAt             string  `json:"updatedAt"`
	PlatformStatus        string  `json:"platformStatus"`
	PlatformReasonPublic  string  `json:"platformReasonPublic,omitempty"`
	Editable              bool    `json:"editable"`
}

type ListShopsReq struct {
	g.Meta `path:"/shipping-delivery/shops" method:"get" tags:"Identity-merch"`
}
type ListShopsRes []Shop

type ListRulesReq struct {
	g.Meta   `path:"/shipping-delivery/rules" method:"get" tags:"Identity-merch"`
	ShopID   int64  `json:"shopId" in:"query"`
	Status   string `json:"status" in:"query"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"pageSize" in:"query" d:"20"`
}
type ListRulesRes struct {
	Items                []Rule `json:"items"`
	Page                 int    `json:"page"`
	PageSize             int    `json:"pageSize"`
	Total                int64  `json:"total"`
	PlatformStatus       string `json:"platformStatus"`
	PlatformReasonPublic string `json:"platformReasonPublic,omitempty"`
}

type CreateRuleReq struct {
	g.Meta      `path:"/shipping-delivery/rules" method:"post" tags:"Identity-merch"`
	CommandKey  string `json:"commandKey"`
	ShopID      int64  `json:"shopId"`
	Name        string `json:"name"`
	Regions     string `json:"regions"`
	FeeFen      int64  `json:"feeFen"`
	FreeOverFen int64  `json:"freeOverFen"`
	MinDays     int    `json:"minDays"`
	MaxDays     int    `json:"maxDays"`
	SortOrder   int    `json:"sortOrder"`
	Status      string `json:"status"`
}
type CreateRuleRes struct {
	Rule     Rule `json:"rule"`
	Replayed bool `json:"replayed"`
}

type UpdateRuleReq struct {
	g.Meta          `path:"/shipping-delivery/rules/{ruleId}" method:"put" tags:"Identity-merch"`
	RuleId          int64  `json:"ruleId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	ShopID          int64  `json:"shopId"`
	Name            string `json:"name"`
	Regions         string `json:"regions"`
	FeeFen          int64  `json:"feeFen"`
	FreeOverFen     int64  `json:"freeOverFen"`
	MinDays         int    `json:"minDays"`
	MaxDays         int    `json:"maxDays"`
	SortOrder       int    `json:"sortOrder"`
	Status          string `json:"status"`
}
type UpdateRuleRes = CreateRuleRes

type RetireRuleReq struct {
	g.Meta          `path:"/shipping-delivery/rules/{ruleId}/retire" method:"post" tags:"Identity-merch"`
	RuleId          int64  `json:"ruleId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	ShopID          int64  `json:"shopId"`
}
type RetireRuleRes = CreateRuleRes

type ListPresetsReq struct {
	g.Meta   `path:"/shipping-delivery/presets" method:"get" tags:"Identity-merch"`
	ShopID   int64  `json:"shopId" in:"query"`
	Status   string `json:"status" in:"query"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"pageSize" in:"query" d:"20"`
}
type ListPresetsRes struct {
	Items                []Preset `json:"items"`
	Page                 int      `json:"page"`
	PageSize             int      `json:"pageSize"`
	Total                int64    `json:"total"`
	PlatformStatus       string   `json:"platformStatus"`
	PlatformReasonPublic string   `json:"platformReasonPublic,omitempty"`
}

type GetPresetReq struct {
	g.Meta   `path:"/shipping-delivery/presets/{presetId}" method:"get" tags:"Identity-merch"`
	PresetId int64 `json:"presetId" in:"path"`
	ShopID   int64 `json:"shopId" in:"query"`
}
type GetPresetRes struct {
	Preset Preset `json:"preset"`
}

type CreatePresetReq struct {
	g.Meta                `path:"/shipping-delivery/presets" method:"post" tags:"Identity-merch"`
	CommandKey            string  `json:"commandKey"`
	ShopID                int64   `json:"shopId"`
	Name                  string  `json:"name"`
	IsDefault             bool    `json:"isDefault"`
	ProductScope          string  `json:"productScope"`
	ProductIDs            []int64 `json:"productIds"`
	OriginName            string  `json:"originName"`
	OriginRegionCode      string  `json:"originRegionCode"`
	OriginRegionName      string  `json:"originRegionName"`
	OriginCountryCode     string  `json:"originCountryCode"`
	OriginCountryName     string  `json:"originCountryName"`
	OriginSubdivisionCode string  `json:"originSubdivisionCode"`
	OriginSubdivisionName string  `json:"originSubdivisionName"`
	Status                string  `json:"status"`
	Zones                 []Zone  `json:"zones"`
}
type CreatePresetRes struct {
	Preset   Preset `json:"preset"`
	Replayed bool   `json:"replayed"`
}

type UpdatePresetReq struct {
	g.Meta                `path:"/shipping-delivery/presets/{presetId}" method:"put" tags:"Identity-merch"`
	PresetId              int64   `json:"presetId" in:"path"`
	CommandKey            string  `json:"commandKey"`
	ExpectedVersion       uint64  `json:"expectedVersion"`
	ShopID                int64   `json:"shopId"`
	Name                  string  `json:"name"`
	IsDefault             bool    `json:"isDefault"`
	ProductScope          string  `json:"productScope"`
	ProductIDs            []int64 `json:"productIds"`
	OriginName            string  `json:"originName"`
	OriginRegionCode      string  `json:"originRegionCode"`
	OriginRegionName      string  `json:"originRegionName"`
	OriginCountryCode     string  `json:"originCountryCode"`
	OriginCountryName     string  `json:"originCountryName"`
	OriginSubdivisionCode string  `json:"originSubdivisionCode"`
	OriginSubdivisionName string  `json:"originSubdivisionName"`
	Status                string  `json:"status"`
	Zones                 []Zone  `json:"zones"`
}
type UpdatePresetRes = CreatePresetRes

type EnablePresetReq struct {
	g.Meta          `path:"/shipping-delivery/presets/{presetId}/enable" method:"post" tags:"Identity-merch"`
	PresetId        int64  `json:"presetId" in:"path"`
	ShopID          int64  `json:"shopId"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type EnablePresetRes = CreatePresetRes

type DisablePresetReq struct {
	g.Meta          `path:"/shipping-delivery/presets/{presetId}/disable" method:"post" tags:"Identity-merch"`
	PresetId        int64  `json:"presetId" in:"path"`
	ShopID          int64  `json:"shopId"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type DisablePresetRes = CreatePresetRes

type RetirePresetReq struct {
	g.Meta          `path:"/shipping-delivery/presets/{presetId}/retire" method:"post" tags:"Identity-merch"`
	PresetId        int64  `json:"presetId" in:"path"`
	ShopID          int64  `json:"shopId"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type RetirePresetRes = CreatePresetRes
