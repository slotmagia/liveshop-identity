package fulfillment

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

type ShippingRepository interface {
	ListRules(context.Context, model.ShippingQuery) (model.ShippingRulePage, error)
	SaveRule(context.Context, model.SaveShippingRuleCommand) (model.ShippingRule, bool, error)
	RetireRule(context.Context, model.RetireShippingCommand) (model.ShippingRule, bool, error)
	ListPresets(context.Context, model.ShippingQuery) (model.ShippingPresetPage, error)
	GetPreset(context.Context, int64, int64, int64) (model.ShippingPreset, error)
	SavePreset(context.Context, model.SaveShippingPresetCommand) (model.ShippingPreset, bool, error)
	SetPresetEnabled(context.Context, model.SetShippingPresetEnabledCommand) (model.ShippingPreset, bool, error)
	RetirePreset(context.Context, model.RetireShippingCommand) (model.ShippingPreset, bool, error)
}

type Shipping struct{ repository ShippingRepository }

func NewShipping(repository ShippingRepository) *Shipping {
	return &Shipping{repository: repository}
}

func (s *Shipping) ListRules(ctx context.Context, query model.ShippingQuery) (model.ShippingRulePage, error) {
	if s == nil || s.repository == nil {
		return model.ShippingRulePage{}, model.ErrShippingUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.ShippingRulePage{}, err
	}
	page, err := s.repository.ListRules(ctx, normalized)
	if err != nil {
		return model.ShippingRulePage{}, err
	}
	if page.Page != normalized.Page || page.PageSize != normalized.PageSize || page.Total < 0 {
		return model.ShippingRulePage{}, model.ErrShippingInvalid
	}
	for _, item := range page.Items {
		if err := item.ValidatePersisted(); err != nil {
			return model.ShippingRulePage{}, model.ErrShippingInvalid
		}
		if item.MerchantID != normalized.MerchantID || item.ShopID != normalized.ShopID || item.Status == model.ShippingRetired {
			return model.ShippingRulePage{}, model.ErrShippingInvalid
		}
	}
	return page, nil
}

func (s *Shipping) SaveRule(ctx context.Context, command model.SaveShippingRuleCommand, create bool) (model.ShippingRule, bool, error) {
	if s == nil || s.repository == nil {
		return model.ShippingRule{}, false, model.ErrShippingUnavailable
	}
	normalized, err := command.Normalize(create)
	if err != nil {
		return model.ShippingRule{}, false, err
	}
	value, replayed, err := s.repository.SaveRule(ctx, normalized)
	if err != nil {
		return model.ShippingRule{}, false, err
	}
	if err := value.ValidatePersisted(); err != nil {
		return model.ShippingRule{}, false, model.ErrShippingInvalid
	}
	if value.MerchantID != normalized.Rule.MerchantID || value.ShopID != normalized.Rule.ShopID {
		return model.ShippingRule{}, false, model.ErrShippingInvalid
	}
	if !create && value.ID != normalized.Rule.ID {
		return model.ShippingRule{}, false, model.ErrShippingInvalid
	}
	return value, replayed, nil
}

func (s *Shipping) RetireRule(ctx context.Context, command model.RetireShippingCommand) (model.ShippingRule, bool, error) {
	if s == nil || s.repository == nil {
		return model.ShippingRule{}, false, model.ErrShippingUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.ShippingRule{}, false, err
	}
	value, replayed, err := s.repository.RetireRule(ctx, normalized)
	if err != nil {
		return model.ShippingRule{}, false, err
	}
	if err := value.ValidatePersisted(); err != nil || value.Status != model.ShippingRetired {
		return model.ShippingRule{}, false, model.ErrShippingInvalid
	}
	if value.ID != normalized.ID || value.MerchantID != normalized.MerchantID || value.ShopID != normalized.ShopID {
		return model.ShippingRule{}, false, model.ErrShippingInvalid
	}
	return value, replayed, nil
}

func (s *Shipping) ListPresets(ctx context.Context, query model.ShippingQuery) (model.ShippingPresetPage, error) {
	if s == nil || s.repository == nil {
		return model.ShippingPresetPage{}, model.ErrShippingUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.ShippingPresetPage{}, err
	}
	page, err := s.repository.ListPresets(ctx, normalized)
	if err != nil {
		return model.ShippingPresetPage{}, err
	}
	if page.Page != normalized.Page || page.PageSize != normalized.PageSize || page.Total < 0 {
		return model.ShippingPresetPage{}, model.ErrShippingInvalid
	}
	for _, item := range page.Items {
		if err := item.ValidatePersisted(); err != nil {
			return model.ShippingPresetPage{}, model.ErrShippingInvalid
		}
		if item.MerchantID != normalized.MerchantID || item.ShopID != normalized.ShopID || item.Status == model.ShippingRetired {
			return model.ShippingPresetPage{}, model.ErrShippingInvalid
		}
	}
	return page, nil
}

func (s *Shipping) GetPreset(ctx context.Context, merchantID, shopID, presetID int64) (model.ShippingPreset, error) {
	if s == nil || s.repository == nil {
		return model.ShippingPreset{}, model.ErrShippingUnavailable
	}
	if merchantID <= 0 || shopID <= 0 || presetID <= 0 {
		return model.ShippingPreset{}, model.ErrShippingInvalid
	}
	value, err := s.repository.GetPreset(ctx, merchantID, shopID, presetID)
	if err != nil {
		return model.ShippingPreset{}, err
	}
	if err := value.ValidatePersisted(); err != nil {
		return model.ShippingPreset{}, model.ErrShippingInvalid
	}
	if value.MerchantID != merchantID || value.ShopID != shopID || value.ID != presetID {
		return model.ShippingPreset{}, model.ErrShippingInvalid
	}
	if value.Status == model.ShippingRetired {
		return model.ShippingPreset{}, model.ErrShippingNotFound
	}
	return value, nil
}

func (s *Shipping) SavePreset(ctx context.Context, command model.SaveShippingPresetCommand, create bool) (model.ShippingPreset, bool, error) {
	if s == nil || s.repository == nil {
		return model.ShippingPreset{}, false, model.ErrShippingUnavailable
	}
	normalized, err := command.Normalize(create)
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	value, replayed, err := s.repository.SavePreset(ctx, normalized)
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	if err := value.ValidatePersisted(); err != nil {
		return model.ShippingPreset{}, false, model.ErrShippingInvalid
	}
	if value.MerchantID != normalized.Preset.MerchantID || value.ShopID != normalized.Preset.ShopID {
		return model.ShippingPreset{}, false, model.ErrShippingInvalid
	}
	if !create && value.ID != normalized.Preset.ID {
		return model.ShippingPreset{}, false, model.ErrShippingInvalid
	}
	return value, replayed, nil
}

func (s *Shipping) SetPresetEnabled(ctx context.Context, command model.SetShippingPresetEnabledCommand) (model.ShippingPreset, bool, error) {
	if s == nil || s.repository == nil {
		return model.ShippingPreset{}, false, model.ErrShippingUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	value, replayed, err := s.repository.SetPresetEnabled(ctx, normalized)
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	if err := value.ValidatePersisted(); err != nil {
		return model.ShippingPreset{}, false, model.ErrShippingInvalid
	}
	if value.ID != normalized.PresetID || value.MerchantID != normalized.MerchantID || value.ShopID != normalized.ShopID {
		return model.ShippingPreset{}, false, model.ErrShippingInvalid
	}
	return value, replayed, nil
}

func (s *Shipping) RetirePreset(ctx context.Context, command model.RetireShippingCommand) (model.ShippingPreset, bool, error) {
	if s == nil || s.repository == nil {
		return model.ShippingPreset{}, false, model.ErrShippingUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	value, replayed, err := s.repository.RetirePreset(ctx, normalized)
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	if err := value.ValidatePersisted(); err != nil || value.Status != model.ShippingRetired {
		return model.ShippingPreset{}, false, model.ErrShippingInvalid
	}
	if value.ID != normalized.ID || value.MerchantID != normalized.MerchantID || value.ShopID != normalized.ShopID {
		return model.ShippingPreset{}, false, model.ErrShippingInvalid
	}
	return value, replayed, nil
}
