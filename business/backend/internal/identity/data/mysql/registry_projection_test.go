package mysql

import (
	"encoding/json"
	"testing"

	platformv1 "github.com/liveshop-platform/contracts/gen/go/platform/v1"
)

func TestCatalogReleaseUsesManifestWireNames(t *testing.T) {
	module := &platformv1.ActiveModuleCapability{ModuleId: "identity", ReleaseVersion: "1.2.3", ReleaseDigest: "sha256:x", Backend: &platformv1.Backend{Service: "identity", Origin: "http://identity"}, Permissions: []*platformv1.PermissionDefinition{{Code: "identity.directory.read", Name: "Read", Resource: "identity.directory", Action: "read"}}, HttpOperations: []*platformv1.HttpOperation{{OperationId: "identity.admin.directory.get", Surface: "admin", Method: "GET", Path: "/admin/identity/directory", Authentication: "module-session", RequiredPermissions: []string{"identity.directory.read"}, RequestFields: []*platformv1.CapabilityField{{Name: "organizationId", Location: "query", Type: "integer"}}, Responses: []*platformv1.CapabilityResponse{{Status: 200, Fields: []*platformv1.CapabilityField{{Name: "members", Type: "array"}}}}}}, Grpc: &platformv1.GrpcContract{Service: "identity.v1.Directory", Methods: []*platformv1.GrpcMethod{{Name: "Resolve", FullMethod: "/identity.v1.Directory/Resolve"}}}, Contributions: []*platformv1.ActiveContribution{{ContributionId: "identity.admin.directory", Surface: "admin", Artifact: &platformv1.Artifact{Type: "iframe", Entry: "http://identity-ui", Integrity: "sha256:x"}, Frontend: &platformv1.FrontendContract{Component: "IdentityPage", Actions: []*platformv1.FrontendAction{{ActionId: "open", Invocation: "event", Parameters: []*platformv1.CapabilityField{{Name: "id", Type: "string"}}}}}}}}
	document, err := catalogRelease(module)
	if err != nil {
		t.Fatal(err)
	}
	var release map[string]any
	if err := json.Unmarshal(document, &release); err != nil {
		t.Fatal(err)
	}
	backend := release["backend"].(map[string]any)
	route := backend["httpRoutes"].([]any)[0].(map[string]any)
	operation := route["operations"].([]any)[0].(map[string]any)
	if route["prefix"] != "/admin/identity" || operation["id"] != "identity.admin.directory.get" || operation["operationId"] != nil {
		t.Fatalf("HTTP Manifest contract mismatch: %s", document)
	}
	contribution := release["contributions"].([]any)[0].(map[string]any)
	action := contribution["frontend"].(map[string]any)["actions"].([]any)[0].(map[string]any)
	if contribution["id"] != "identity.admin.directory" || action["id"] != "open" || action["actionId"] != nil || contribution["artifact"].(map[string]any)["entry"] != "http://identity-ui" {
		t.Fatalf("contribution Manifest contract mismatch: %s", document)
	}
	if backend["grpc"].(map[string]any)["service"] != "identity.v1.Directory" {
		t.Fatalf("gRPC Manifest contract mismatch: %s", document)
	}
}

func TestCatalogReleasePreservesPublicRouteGroup(t *testing.T) {
	module := &platformv1.ActiveModuleCapability{
		ModuleId: "media", ReleaseVersion: "1.0.0", ReleaseDigest: "sha256:x",
		Backend: &platformv1.Backend{Service: "media", Origin: "http://media"},
		HttpOperations: []*platformv1.HttpOperation{
			{OperationId: "media.merch.list", Surface: "merch", Method: "GET", Path: "/merch/media/assets"},
			{OperationId: "media.public.content", Surface: "merch", Method: "GET", Path: "/public/media/assets/{id}"},
		},
	}
	document, err := catalogRelease(module)
	if err != nil {
		t.Fatal(err)
	}
	var release map[string]any
	if err := json.Unmarshal(document, &release); err != nil {
		t.Fatal(err)
	}
	routes := release["backend"].(map[string]any)["httpRoutes"].([]any)
	if len(routes) != 2 || routes[0].(map[string]any)["prefix"] != "/merch/media" || routes[1].(map[string]any)["prefix"] != "/public/media" {
		t.Fatalf("public route group was not preserved: %s", document)
	}
}

func TestRegistryProjectionAllowsExplicitGuestVisibleShopCapability(t *testing.T) {
	contribution := &platformv1.ActiveContribution{ContributionId: "catalog.shop.home", Surface: "shop"}
	route := &platformv1.AllowedRoute{Methods: []string{"GET"}, Prefix: "/shop/catalog/products"}
	if !completeContributionIdentity(contribution) || !completeAllowedRoute(contribution.Surface, route) {
		t.Fatal("guest-visible shop contribution was rejected")
	}
	contribution.Surface = "admin"
	if completeContributionIdentity(contribution) || completeAllowedRoute(contribution.Surface, route) {
		t.Fatal("protected control-surface capability accepted empty permissions")
	}
}
