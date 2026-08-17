package capabilityendpoint

import (
	"encoding/json"
	"testing"

	"github.com/lvtuopen-ai/kernel-go/accessidentity"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
)

func TestIssueResponseMatchesHostRuntimeContract(t *testing.T) {
	value := issueResponse(biz.CapabilityGrant{Token: "capability", ExpiresIn: 60, RegistryRevision: 7, AuthorizationRevision: 8, EntitlementRevision: 9, Permissions: []string{"identity.directory.read"}}, accessidentity.Claims{MerchantID: 2, ShopID: 4})
	document, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(document, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["token"] != "capability" {
		t.Fatalf("Host token contract missing: %s", document)
	}
	if _, legacy := payload["moduleSession"]; legacy {
		t.Fatalf("legacy dual response leaked: %s", document)
	}
	tenant, ok := payload["tenant"].(map[string]any)
	if !ok || tenant["merchantId"] != float64(2) || tenant["shopId"] != float64(4) || tenant["appId"] != nil || tenant["commercialId"] != nil {
		t.Fatalf("Host tenant contract mismatch: %s", document)
	}
}
