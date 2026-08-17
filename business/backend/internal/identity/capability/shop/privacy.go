package shop

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type PrivacyRepository interface {
	GetPrivacy(context.Context, int64, int64) (model.Privacy, error)
	SavePrivacy(context.Context, model.SavePrivacyCommand) (model.Privacy, bool, error)
}

type PrivacySettings struct{ repository PrivacyRepository }

func NewPrivacySettings(repository PrivacyRepository) *PrivacySettings {
	return &PrivacySettings{repository: repository}
}

func (s *PrivacySettings) Get(ctx context.Context, merchantID, shopID int64) (model.Privacy, error) {
	if s == nil || s.repository == nil {
		return model.Privacy{}, model.ErrUnavailable
	}
	if merchantID <= 0 || shopID <= 0 {
		return model.Privacy{}, model.ErrPrivacyInvalid
	}
	value, err := s.repository.GetPrivacy(ctx, merchantID, shopID)
	if err != nil {
		return model.Privacy{}, err
	}
	if value.MerchantID != merchantID || value.ShopID != shopID {
		return model.Privacy{}, model.ErrPrivacyInvalid
	}
	if value.Version == 0 {
		if _, err := value.Normalize(); err != nil || value.ID != 0 {
			return model.Privacy{}, model.ErrPrivacyInvalid
		}
		return value, nil
	}
	if err := value.ValidatePersisted(); err != nil {
		return model.Privacy{}, model.ErrPrivacyInvalid
	}
	return value, nil
}

func (s *PrivacySettings) Save(ctx context.Context, command model.SavePrivacyCommand) (model.Privacy, bool, error) {
	if s == nil || s.repository == nil {
		return model.Privacy{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Privacy{}, false, err
	}
	return s.repository.SavePrivacy(ctx, normalized)
}
