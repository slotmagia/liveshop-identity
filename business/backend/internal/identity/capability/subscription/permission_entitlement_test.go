package subscription

import "testing"

func TestPermissionCommandNormalizationAndHash(t *testing.T) {
	command := ApplyPermissionEntitlementCommand{MerchantID: 9, CommandKey: "merchant-plan-0001", ExpectedRevision: 3, PermissionCodes: []string{"catalog.product.read", " trade.order.read ", "catalog.product.read"}}
	normalized, err := command.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if len(normalized.PermissionCodes) != 2 || normalized.PermissionCodes[0] != "catalog.product.read" || normalized.PermissionCodes[1] != "trade.order.read" {
		t.Fatalf("unexpected normalization: %#v", normalized.PermissionCodes)
	}
	reordered := command
	reordered.PermissionCodes = []string{"trade.order.read", "catalog.product.read"}
	reordered, _ = reordered.Normalize()
	if normalized.RequestDigest() != reordered.RequestDigest() {
		t.Fatal("equivalent permission sets must share a request hash")
	}
	spaced := command
	spaced.CommandKey = "  merchant-plan-0001  "
	spaced, _ = spaced.Normalize()
	if spaced.CommandKey != normalized.CommandKey || spaced.RequestDigest() != normalized.RequestDigest() {
		t.Fatal("command key whitespace must be normalized before hashing")
	}
	changed := normalized
	changed.ExpectedRevision++
	if normalized.RequestDigest() == changed.RequestDigest() {
		t.Fatal("expected revision must be part of the request hash")
	}
}

func TestEmptyPermissionSetIsExplicitSnapshot(t *testing.T) {
	command, err := (ApplyPermissionEntitlementCommand{MerchantID: 9, CommandKey: "empty-plan-0001"}).Normalize()
	if err != nil || command.PermissionCodes == nil {
		t.Fatalf("empty entitlement must be valid and normalized: %#v %v", command, err)
	}
	if PermissionSnapshotDigest(command.PermissionCodes) == ([32]byte{}) {
		t.Fatal("empty explicit snapshot still requires a nonzero digest")
	}
}
