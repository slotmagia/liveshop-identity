package model

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type ShippingStatus string
type ProductScope string
type TransitType string

const (
	ShippingActive   ShippingStatus = "ACTIVE"
	ShippingDisabled ShippingStatus = "DISABLED"
	ShippingRetired  ShippingStatus = "RETIRED"

	ProductScopeAll      ProductScope = "ALL"
	ProductScopeSelected ProductScope = "SELECTED"

	TransitStandard TransitType = "STANDARD"
	TransitExpress  TransitType = "EXPRESS"
	TransitEconomy  TransitType = "ECONOMY"

	ShippingNameMax    = 120
	ShippingRegionsMax = 2000
	ShippingMaxDays    = 365
	ShippingMaxZones   = 20
	ShippingMaxRates   = 10
)

var (
	ErrShippingUnavailable = errors.New("shipping repository unavailable")
	ErrShippingNotFound    = errors.New("shipping not found")
	ErrShippingInvalid     = errors.New("invalid shipping")
	ErrShippingConflict    = errors.New("shipping version or unique-key conflict")
	ErrShippingIdempotency = errors.New("shipping command key was reused with different input")
	ErrShippingRestricted  = errors.New("shipping is restricted by platform")
)

type ShippingRule struct {
	ID          int64
	MerchantID  int64
	ShopID      int64
	Name        string
	Regions     string
	FeeFen      int64
	FreeOverFen int64
	MinDays     int
	MaxDays     int
	SortOrder   int
	Status      ShippingStatus
	Version     uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ShippingRegion struct {
	RegionCode      string `json:"regionCode"`
	RegionName      string `json:"regionName"`
	CountryCode     string `json:"countryCode"`
	CountryName     string `json:"countryName"`
	SubdivisionCode string `json:"subdivisionCode"`
	SubdivisionName string `json:"subdivisionName"`
}

type ShippingRate struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	IsFree      bool           `json:"isFree"`
	PriceFen    int64          `json:"priceFen"`
	TransitType TransitType    `json:"transitType"`
	MinDays     int            `json:"minDays"`
	MaxDays     int            `json:"maxDays"`
	SortOrder   int            `json:"sortOrder"`
	Status      ShippingStatus `json:"status"`
}

type ShippingZone struct {
	ID        int64            `json:"id"`
	Name      string           `json:"name"`
	SortOrder int              `json:"sortOrder"`
	Regions   []ShippingRegion `json:"regions"`
	Rates     []ShippingRate   `json:"rates"`
}

type ShippingPreset struct {
	ID                    int64
	MerchantID            int64
	ShopID                int64
	Name                  string
	IsDefault             bool
	ProductScope          ProductScope
	ProductIDs            []int64
	OriginName            string
	OriginRegionCode      string
	OriginRegionName      string
	OriginCountryCode     string
	OriginCountryName     string
	OriginSubdivisionCode string
	OriginSubdivisionName string
	Status                ShippingStatus
	Zones                 []ShippingZone
	Version               uint64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type ShippingQuery struct {
	MerchantID int64
	ShopID     int64
	Status     ShippingStatus
	Page       int
	PageSize   int
}

type ShippingRulePage struct {
	Items    []ShippingRule
	Page     int
	PageSize int
	Total    int64
}

type ShippingPresetPage struct {
	Items    []ShippingPreset
	Page     int
	PageSize int
	Total    int64
}

type SaveShippingRuleCommand struct {
	CommandKey      string
	ExpectedVersion uint64
	Rule            ShippingRule
}

type RetireShippingCommand struct {
	ID              int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
}

type SaveShippingPresetCommand struct {
	CommandKey      string
	ExpectedVersion uint64
	Preset          ShippingPreset
}

type SetShippingPresetEnabledCommand struct {
	PresetID        int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
	Enabled         bool
}

func (q ShippingQuery) Normalize() (ShippingQuery, error) {
	q.Status = ShippingStatus(strings.ToUpper(strings.TrimSpace(string(q.Status))))
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	if q.MerchantID <= 0 || q.ShopID <= 0 {
		return q, ErrShippingInvalid
	}
	if q.Status != "" && q.Status != ShippingActive && q.Status != ShippingDisabled {
		return q, ErrShippingInvalid
	}
	return q, nil
}

func (r ShippingRule) NormalizeWrite(create bool) (ShippingRule, error) {
	r.Name = clipSpace(r.Name)
	r.Regions = clipSpace(r.Regions)
	r.Status = ShippingStatus(strings.ToUpper(strings.TrimSpace(string(r.Status))))
	if r.MerchantID <= 0 || r.ShopID <= 0 {
		return r, ErrShippingInvalid
	}
	if create && r.ID != 0 {
		return r, ErrShippingInvalid
	}
	if !create && r.ID <= 0 {
		return r, ErrShippingInvalid
	}
	if err := validateName(r.Name); err != nil {
		return r, err
	}
	if r.Regions == "" || len([]rune(r.Regions)) > ShippingRegionsMax {
		return r, ErrShippingInvalid
	}
	if r.FeeFen < 0 || r.FreeOverFen < 0 || !validDays(r.MinDays, r.MaxDays) {
		return r, ErrShippingInvalid
	}
	if r.Status == "" {
		r.Status = ShippingActive
	}
	if r.Status != ShippingActive && r.Status != ShippingDisabled {
		return r, ErrShippingInvalid
	}
	return r, nil
}

func (r ShippingRule) ValidatePersisted() error {
	if r.ID <= 0 || r.MerchantID <= 0 || r.ShopID <= 0 || r.Version == 0 || r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() {
		return ErrShippingInvalid
	}
	if err := validateName(r.Name); err != nil {
		return err
	}
	if r.Regions == "" || r.FeeFen < 0 || r.FreeOverFen < 0 || !validDays(r.MinDays, r.MaxDays) {
		return ErrShippingInvalid
	}
	if !validShippingStatus(r.Status) {
		return ErrShippingInvalid
	}
	return nil
}

func (p ShippingPreset) NormalizeWrite(create bool) (ShippingPreset, error) {
	p.Name = clipSpace(p.Name)
	p.OriginName = clipSpace(p.OriginName)
	p.OriginRegionCode = clipSpace(p.OriginRegionCode)
	p.OriginRegionName = clipSpace(p.OriginRegionName)
	p.OriginCountryCode = strings.ToUpper(clipSpace(p.OriginCountryCode))
	p.OriginCountryName = clipSpace(p.OriginCountryName)
	p.OriginSubdivisionCode = clipSpace(p.OriginSubdivisionCode)
	p.OriginSubdivisionName = clipSpace(p.OriginSubdivisionName)
	p.ProductScope = ProductScope(strings.ToUpper(strings.TrimSpace(string(p.ProductScope))))
	p.Status = ShippingStatus(strings.ToUpper(strings.TrimSpace(string(p.Status))))
	if p.MerchantID <= 0 || p.ShopID <= 0 {
		return p, ErrShippingInvalid
	}
	if create && p.ID != 0 {
		return p, ErrShippingInvalid
	}
	if !create && p.ID <= 0 {
		return p, ErrShippingInvalid
	}
	if err := validateName(p.Name); err != nil {
		return p, err
	}
	if err := validateName(p.OriginName); err != nil {
		return p, err
	}
	if err := validateName(p.OriginRegionName); err != nil {
		return p, err
	}
	if err := validateName(p.OriginCountryName); err != nil {
		return p, err
	}
	if !validCode(p.OriginRegionCode, 32) || !validCountry(p.OriginCountryCode) {
		return p, ErrShippingInvalid
	}
	if p.OriginSubdivisionCode != "" && !validCode(p.OriginSubdivisionCode, 32) {
		return p, ErrShippingInvalid
	}
	if len([]rune(p.OriginSubdivisionName)) > ShippingNameMax {
		return p, ErrShippingInvalid
	}
	if p.ProductScope == "" {
		p.ProductScope = ProductScopeAll
	}
	ids := uniquePositive(p.ProductIDs)
	switch p.ProductScope {
	case ProductScopeAll:
		if len(ids) != 0 {
			return p, ErrShippingInvalid
		}
		p.ProductIDs = []int64{}
	case ProductScopeSelected:
		if len(ids) == 0 {
			return p, ErrShippingInvalid
		}
		p.ProductIDs = ids
	default:
		return p, ErrShippingInvalid
	}
	if p.Status == "" {
		p.Status = ShippingActive
	}
	if p.Status != ShippingActive && p.Status != ShippingDisabled {
		return p, ErrShippingInvalid
	}
	zones, err := normalizeZones(p.Zones)
	if err != nil {
		return p, err
	}
	p.Zones = zones
	return p, nil
}

func (p ShippingPreset) ValidatePersisted() error {
	if p.ID <= 0 || p.MerchantID <= 0 || p.ShopID <= 0 || p.Version == 0 || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		return ErrShippingInvalid
	}
	if !validShippingStatus(p.Status) || !validCountry(p.OriginCountryCode) {
		return ErrShippingInvalid
	}
	if p.Status == ShippingRetired {
		return nil
	}
	if _, err := p.NormalizeWrite(false); err != nil {
		return err
	}
	return nil
}

func (c SaveShippingRuleCommand) Normalize(create bool) (SaveShippingRuleCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if !validCommandKey(c.CommandKey) {
		return c, ErrShippingInvalid
	}
	if !create && c.ExpectedVersion == 0 {
		return c, ErrShippingInvalid
	}
	rule, err := c.Rule.NormalizeWrite(create)
	if err != nil {
		return c, err
	}
	c.Rule = rule
	return c, nil
}

func (c SaveShippingRuleCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"SAVE-RULE", c.CommandKey, strconv.FormatInt(c.Rule.ID, 10), strconv.FormatInt(c.Rule.MerchantID, 10),
		strconv.FormatInt(c.Rule.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10), c.Rule.Name, c.Rule.Regions,
		strconv.FormatInt(c.Rule.FeeFen, 10), strconv.FormatInt(c.Rule.FreeOverFen, 10), strconv.Itoa(c.Rule.MinDays),
		strconv.Itoa(c.Rule.MaxDays), strconv.Itoa(c.Rule.SortOrder), string(c.Rule.Status),
	}, "\n")))
}

func (c SaveShippingPresetCommand) Normalize(create bool) (SaveShippingPresetCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if !validCommandKey(c.CommandKey) {
		return c, ErrShippingInvalid
	}
	if !create && c.ExpectedVersion == 0 {
		return c, ErrShippingInvalid
	}
	preset, err := c.Preset.NormalizeWrite(create)
	if err != nil {
		return c, err
	}
	c.Preset = preset
	return c, nil
}

func (c SaveShippingPresetCommand) RequestDigest() [32]byte {
	zones, _ := json.Marshal(c.Preset.Zones)
	ids, _ := json.Marshal(c.Preset.ProductIDs)
	defaultFlag := "0"
	if c.Preset.IsDefault {
		defaultFlag = "1"
	}
	return sha256.Sum256([]byte(strings.Join([]string{
		"SAVE-PRESET", c.CommandKey, strconv.FormatInt(c.Preset.ID, 10), strconv.FormatInt(c.Preset.MerchantID, 10),
		strconv.FormatInt(c.Preset.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10), c.Preset.Name, defaultFlag,
		string(c.Preset.ProductScope), string(ids), c.Preset.OriginName, c.Preset.OriginRegionCode, c.Preset.OriginRegionName,
		c.Preset.OriginCountryCode, c.Preset.OriginCountryName, c.Preset.OriginSubdivisionCode, c.Preset.OriginSubdivisionName,
		string(c.Preset.Status), string(zones),
	}, "\n")))
}

func (c RetireShippingCommand) Normalize() (RetireShippingCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.ID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 || !validCommandKey(c.CommandKey) {
		return c, ErrShippingInvalid
	}
	return c, nil
}

func (c RetireShippingCommand) RequestDigest(kind string) [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		kind, c.CommandKey, strconv.FormatInt(c.ID, 10), strconv.FormatInt(c.MerchantID, 10),
		strconv.FormatInt(c.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}

func (c SetShippingPresetEnabledCommand) Normalize() (SetShippingPresetEnabledCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.PresetID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 || !validCommandKey(c.CommandKey) {
		return c, ErrShippingInvalid
	}
	return c, nil
}

func (c SetShippingPresetEnabledCommand) RequestDigest() [32]byte {
	enabled := "0"
	if c.Enabled {
		enabled = "1"
	}
	return sha256.Sum256([]byte(strings.Join([]string{
		"SET-PRESET-ENABLED", c.CommandKey, strconv.FormatInt(c.PresetID, 10), strconv.FormatInt(c.MerchantID, 10),
		strconv.FormatInt(c.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10), enabled,
	}, "\n")))
}

func normalizeZones(values []ShippingZone) ([]ShippingZone, error) {
	if len(values) == 0 || len(values) > ShippingMaxZones {
		return nil, ErrShippingInvalid
	}
	out := make([]ShippingZone, 0, len(values))
	for i, zone := range values {
		zone.Name = clipSpace(zone.Name)
		if err := validateName(zone.Name); err != nil {
			return nil, err
		}
		if len(zone.Regions) == 0 || len(zone.Rates) == 0 || len(zone.Rates) > ShippingMaxRates {
			return nil, ErrShippingInvalid
		}
		regions := make([]ShippingRegion, 0, len(zone.Regions))
		for _, region := range zone.Regions {
			region.RegionCode = clipSpace(region.RegionCode)
			region.RegionName = clipSpace(region.RegionName)
			region.CountryCode = strings.ToUpper(clipSpace(region.CountryCode))
			region.CountryName = clipSpace(region.CountryName)
			region.SubdivisionCode = clipSpace(region.SubdivisionCode)
			region.SubdivisionName = clipSpace(region.SubdivisionName)
			if !validCode(region.RegionCode, 32) || !validCountry(region.CountryCode) {
				return nil, ErrShippingInvalid
			}
			if err := validateName(region.RegionName); err != nil {
				return nil, err
			}
			if err := validateName(region.CountryName); err != nil {
				return nil, err
			}
			regions = append(regions, region)
		}
		rates := make([]ShippingRate, 0, len(zone.Rates))
		for _, rate := range zone.Rates {
			rate.Name = clipSpace(rate.Name)
			rate.TransitType = TransitType(strings.ToUpper(strings.TrimSpace(string(rate.TransitType))))
			rate.Status = ShippingStatus(strings.ToUpper(strings.TrimSpace(string(rate.Status))))
			if err := validateName(rate.Name); err != nil {
				return nil, err
			}
			if rate.PriceFen < 0 || !validDays(rate.MinDays, rate.MaxDays) {
				return nil, ErrShippingInvalid
			}
			if rate.IsFree && rate.PriceFen != 0 {
				return nil, ErrShippingInvalid
			}
			if rate.TransitType != TransitStandard && rate.TransitType != TransitExpress && rate.TransitType != TransitEconomy {
				return nil, ErrShippingInvalid
			}
			if rate.Status == "" {
				rate.Status = ShippingActive
			}
			if rate.Status != ShippingActive && rate.Status != ShippingDisabled {
				return nil, ErrShippingInvalid
			}
			if rate.ID <= 0 {
				rate.ID = int64(i+1)*100 + int64(len(rates)+1)
			}
			rates = append(rates, rate)
		}
		if zone.ID <= 0 {
			zone.ID = int64(i + 1)
		}
		zone.Regions = regions
		zone.Rates = rates
		out = append(out, zone)
	}
	return out, nil
}

func validShippingStatus(value ShippingStatus) bool {
	return value == ShippingActive || value == ShippingDisabled || value == ShippingRetired
}

func validCommandKey(value string) bool {
	return len(value) >= 8 && len(value) <= 128
}

func validDays(minDays, maxDays int) bool {
	return minDays >= 0 && maxDays >= minDays && maxDays <= ShippingMaxDays
}

func validCountry(value string) bool {
	if len(value) != 2 {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func validCode(value string, max int) bool {
	if value == "" || len(value) > max {
		return false
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func validateName(value string) error {
	runes := []rune(value)
	if len(runes) == 0 || len(runes) > ShippingNameMax {
		return ErrShippingInvalid
	}
	return nil
}

func uniquePositive(values []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func clipSpace(value string) string {
	return strings.TrimSpace(value)
}
