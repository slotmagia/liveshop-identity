package shop

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type stubPrivacyRepository struct {
	saved model.SavePrivacyCommand
	value model.Privacy
}

func (s *stubPrivacyRepository) GetPrivacy(context.Context, int64, int64) (model.Privacy, error) {
	return s.value, nil
}
func (s *stubPrivacyRepository) SavePrivacy(_ context.Context, command model.SavePrivacyCommand) (model.Privacy, bool, error) {
	s.saved = command
	return command.Privacy, false, nil
}

func TestPrivacySaveNormalizesEmail(t *testing.T) {
	repository := &stubPrivacyRepository{}
	service := NewPrivacySettings(repository)
	_, _, err := service.Save(context.Background(), model.SavePrivacyCommand{
		CommandKey: "privacy-save-0001",
		Privacy: model.Privacy{
			MerchantID: 10, ShopID: 20, CollectConsent: true, CookieBanner: true,
			DataRetentionDays: 30, ContactEmail: " DPO@Shop.Example ",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.saved.Privacy.ContactEmail != "dpo@shop.example" {
		t.Fatalf("normalized=%+v", repository.saved.Privacy)
	}
}

func TestPrivacyGetRejectsCrossShopResult(t *testing.T) {
	repository := &stubPrivacyRepository{value: model.Privacy{
		ID: 1, MerchantID: 99, ShopID: 20, CollectConsent: true, CookieBanner: true, DataRetentionDays: 365, Version: 1,
	}}
	_, err := NewPrivacySettings(repository).Get(context.Background(), 10, 20)
	if !errors.Is(err, model.ErrPrivacyInvalid) {
		t.Fatalf("error=%v", err)
	}
}
