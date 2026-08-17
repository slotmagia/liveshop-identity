package model

import "testing"

func TestPrivacyNormalizeRejectsInvalidEmailAndRetention(t *testing.T) {
	base := Privacy{MerchantID: 10, ShopID: 20, CollectConsent: true, CookieBanner: true, DataRetentionDays: 365, ContactEmail: " Privacy@Example.com "}
	normalized, err := base.Normalize()
	if err != nil || normalized.ContactEmail != "privacy@example.com" {
		t.Fatalf("normalized=%+v err=%v", normalized, err)
	}
	invalidDays := base
	invalidDays.DataRetentionDays = 0
	if _, err := invalidDays.Normalize(); err != ErrPrivacyInvalid {
		t.Fatalf("days error=%v", err)
	}
	invalidEmail := base
	invalidEmail.ContactEmail = "not-an-email"
	if _, err := invalidEmail.Normalize(); err != ErrPrivacyInvalid {
		t.Fatalf("email error=%v", err)
	}
}

func TestPrivacyCommandDigestSeparatesPayloads(t *testing.T) {
	base := SavePrivacyCommand{
		CommandKey: "privacy-save-0001",
		Privacy:    Privacy{MerchantID: 10, ShopID: 20, CollectConsent: true, CookieBanner: true, DataRetentionDays: 365},
	}
	changed := base
	changed.Privacy.MarketingConsent = true
	if base.RequestDigest() == changed.RequestDigest() {
		t.Fatal("consent changes must not share a digest")
	}
}
