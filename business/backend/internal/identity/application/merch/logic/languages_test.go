package logic

import (
	"context"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

func TestLanguagesUsesSessionShop(t *testing.T) {
	value, err := merchShopLogic().Languages(merchShopSession(true))
	if err != nil || value.DefaultLocale != "zh-CN" || len(value.Items) == 0 {
		t.Fatalf("value=%+v err=%v", value, err)
	}
	if value.Items[0].Locale != shopmodel.SourceLocale || !value.Items[0].Published {
		t.Fatalf("source locale=%+v", value.Items[0])
	}
}

func TestUpdateLanguagesRejectsMissingShop(t *testing.T) {
	if _, err := merchShopLogic().UpdateLanguages(context.Background(), appmodel.UpdateLanguages{
		CommandKey: "languages-0001", ExpectedVersion: 1, DefaultLocale: "zh-CN", PublishedLocales: []string{"zh-CN"},
	}); err == nil {
		t.Fatal("expected missing shop context")
	}
}

func TestUpdateLanguagesPublishesDefault(t *testing.T) {
	value, err := merchShopLogic().UpdateLanguages(merchShopSession(true), appmodel.UpdateLanguages{
		CommandKey: "languages-0001", ExpectedVersion: 1, DefaultLocale: "en-US", PublishedLocales: []string{"zh-CN", "en-US"},
	})
	if err != nil || value.Languages.DefaultLocale != "en-US" || value.Languages.Version != 2 {
		t.Fatalf("value=%+v err=%v", value, err)
	}
}
