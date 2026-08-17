package model

import "testing"

func TestCatalogMatchesLegacyModules(t *testing.T) {
	got := map[string]string{}
	for _, item := range Catalog() {
		got[item.Key] = item.Label
	}
	want := map[string]string{
		"privacy": "隐私", "policies": "政策", "domains": "域名",
		"apps": "私有应用", "languages": "语言", "shipping": "配送",
	}
	if len(got) != len(want) {
		t.Fatalf("catalog size %d, want %d", len(got), len(want))
	}
	for key, label := range want {
		if got[key] != label || !ValidModule(key) {
			t.Fatalf("module %q = %q", key, got[key])
		}
	}
}

func TestInterveneRequiresPublicReasonWhenRestricting(t *testing.T) {
	base := InterveneCommand{
		CommandKey: "command-1", MerchantID: 1, ShopID: 2, Module: "privacy",
		PlatformStatus: PlatformRestricted, ReasonInternal: "abuse", Operator: "admin",
	}
	if _, err := base.Normalize(); err == nil {
		t.Fatal("restricted without public reason was accepted")
	}
	base.ReasonPublic = "violates policy"
	if _, err := base.Normalize(); err != nil {
		t.Fatal(err)
	}
	base.PlatformStatus = PlatformActive
	normalized, err := base.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if normalized.ReasonPublic != "" {
		t.Fatal("active restore kept a public reason")
	}
}

func TestQueryRequiresAuthoritativeShopScope(t *testing.T) {
	if _, err := (Query{MerchantID: 1}).Normalize(); err == nil {
		t.Fatal("merchant-only query was accepted")
	}
	if _, err := (Query{MerchantID: 1, ShopID: 2, Module: "unknown"}).Normalize(); err == nil {
		t.Fatal("unknown module was accepted")
	}
	if _, err := (Query{MerchantID: 1, ShopID: 2, Module: "shipping"}).Normalize(); err != nil {
		t.Fatal(err)
	}
}
