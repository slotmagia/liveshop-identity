package authendpoint

import (
	"testing"

	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/config"
)

func TestRefreshCookieIsRealmSpecificAndSurfaceBound(t *testing.T) {
	endpoint := &Endpoint{settings: config.AccessIdentity{PlatformRefreshCookie: "platform_refresh", MerchantRefreshCookie: "merchant_refresh", CustomerRefreshCookie: "customer_refresh"}}
	if endpoint.cookieName(principal.RealmPlatform) == endpoint.cookieName(principal.RealmMerchant) {
		t.Fatal("platform and merchant refresh cookies collide")
	}
	tests := map[string]principal.Realm{"admin": principal.RealmPlatform, "merch": principal.RealmMerchant, "shop": principal.RealmCustomer, "live": principal.RealmCustomer}
	for surface, expected := range tests {
		got, ok := realmForSurface(surface)
		if !ok || got != expected {
			t.Fatalf("realmForSurface(%q) = %q,%v; want %q,true", surface, got, ok, expected)
		}
	}
	if _, ok := realmForSurface(""); ok {
		t.Fatal("missing trusted surface selected a refresh cookie")
	}
	if realmMatchesSurface(principal.RealmPlatform, "merch") || realmMatchesSurface(principal.RealmMerchant, "admin") {
		t.Fatal("cross-realm login surface was accepted")
	}
	if !guestSurface("shop") || !guestSurface("live") || guestSurface("admin") || guestSurface("merch") {
		t.Fatal("guest issuance surface boundary is incorrect")
	}
}
