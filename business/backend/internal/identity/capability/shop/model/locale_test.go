package model

import "testing"

func TestReplaceLanguagesRequiresDefaultInPublishedAndAllowedCatalog(t *testing.T) {
	command, err := ReplaceLanguagesCommand{
		MerchantID: 7, ShopID: 3001, CommandKey: "languages-0001", ExpectedVersion: 1,
		DefaultLocale: "en-US", PublishedLocales: []string{"zh-CN", "en-US"}, AllowedLocales: []string{"zh-CN", "en-US"},
	}.Normalize()
	if err != nil || command.DefaultLocale != "en-US" || len(command.PublishedLocales) != 2 {
		t.Fatalf("command=%+v err=%v", command, err)
	}
	if _, err := (ReplaceLanguagesCommand{
		MerchantID: 7, ShopID: 3001, CommandKey: "languages-0001", ExpectedVersion: 1,
		DefaultLocale: "en-US", PublishedLocales: []string{"zh-CN"}, AllowedLocales: []string{"zh-CN", "en-US"},
	}).Normalize(); err != ErrInvalid {
		t.Fatalf("missing default in published: %v", err)
	}
	if _, err := (ReplaceLanguagesCommand{
		MerchantID: 7, ShopID: 3001, CommandKey: "languages-0001", ExpectedVersion: 1,
		DefaultLocale: "fr-FR", PublishedLocales: []string{"fr-FR"}, AllowedLocales: []string{"zh-CN", "en-US"},
	}).Normalize(); err != ErrInvalid {
		t.Fatalf("unknown locale: %v", err)
	}
}
