package app

// This is the routing and authorization test every surface must have. It starts
// a real HTTP server and sends real requests: binding, middleware order and the
// response envelope only exist at that level, so calling a controller directly
// would prove nothing about whether the endpoint is reachable or protected.
//
// Copy the table below when you add a capability. A capability without a denied
// case is untested: the interesting failure is not "does it work" but "can the
// wrong caller reach it".

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"

	adminrouter "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/router"
	merchrouter "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/router"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service"
	customerservicemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant"
	merchantmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance"
	governancemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/risk"
	riskmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/risk/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
	subscriptionmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/middleware"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/config"
)

const healthPermission = "identity.directory.read"

// surfaces is every surface this module mounts, so a new one is covered the
// moment it is added to the composition root.
var surfaces = []struct {
	name   string
	prefix string
}{
	{name: adminrouter.Surface, prefix: adminrouter.Prefix},
	{name: merchrouter.Surface, prefix: merchrouter.Prefix},
}

// stubHealth stands in for the repository port. Routing and authorization must
// be testable without a database, otherwise these tests stop being run.
type stubHealth struct{ err error }

func (s stubHealth) Ready(context.Context) error { return s.err }

type stubIdentityDirectory struct{}

func (stubIdentityDirectory) ResolvePrincipalContext(_ context.Context, subject string) (model.PrincipalContext, error) {
	if subject == "" {
		subject = "tester"
	}
	return model.PrincipalContext{
		Subject: model.Subject{
			ID: subject, Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff,
			DisplayName: "测试员工", Status: model.StatusActive, Version: 1,
		},
		Organization: model.Organization{
			ID: 1, Type: model.OrganizationMerchant, MerchantID: 2001, Name: "Local Merchant", Status: model.StatusActive, Version: 1,
		},
		Member: model.WorkforceMember{
			ID: 11, OrganizationID: 1, MerchantID: 2001, Subject: subject, Type: model.MemberStaff, Status: model.MemberActive, AccessVersion: 1,
		},
		AvailableShops: []model.ShopContext{{MerchantID: 2001, ShopID: 3001}},
	}, nil
}
func (stubIdentityDirectory) ValidateActiveSession(context.Context, string, string, model.SelectedContext, uint64) error {
	return nil
}
func (stubIdentityDirectory) ResolveShopByID(context.Context, int64) (model.ShopResolution, error) {
	return model.ShopResolution{Context: model.ShopContext{MerchantID: 2001, ShopID: 3001}, Currency: "CNY", Status: model.StatusActive, Version: 1}, nil
}
func (stubIdentityDirectory) ListOrganizationSubtree(context.Context, int64, int64) ([]int64, uint64, error) {
	return nil, 0, nil
}
func (stubIdentityDirectory) BatchGetSubjects(context.Context, []string) ([]model.Subject, error) {
	return nil, nil
}
func (stubIdentityDirectory) ResolveLegacySubjects(context.Context, principal.Realm, []int64) ([]model.Subject, error) {
	return nil, nil
}
func (stubIdentityDirectory) ListOrganizationDirectory(context.Context, int64, int64) (biz.OrganizationDirectory, error) {
	return biz.OrganizationDirectory{
		Organization: model.Organization{ID: 1, Type: model.OrganizationMerchant, MerchantID: 2001, Name: "Local Merchant", Status: model.StatusActive, Version: 1},
		Units:        []biz.OrganizationUnit{{ID: 1, Name: "总部", Status: model.StatusActive, Version: 1}},
		Members: []biz.MemberDirectoryItem{{
			Member:      model.WorkforceMember{ID: 11, OrganizationID: 1, MerchantID: 2001, Subject: "tester", Type: model.MemberStaff, Status: model.MemberActive, AccessVersion: 1},
			DisplayName: "测试员工", PrincipalType: principal.TypeMerchantStaff, SubjectStatus: model.StatusActive, SubjectVersion: 1,
			Credential: biz.ManagedCredential{Identifier: "tester", Status: "ACTIVE", Version: 1},
			ShopIDs:    []int64{3001},
		}},
		Shops: []biz.ShopDirectoryItem{{
			Context: model.ShopContext{MerchantID: 2001, ShopID: 3001}, Name: "Local Shop", Code: "local-shop", Status: model.StatusActive, Version: 1,
		}},
	}, nil
}
func (stubIdentityDirectory) CreateOrganizationUnit(context.Context, biz.CreateOrganizationUnit) (biz.OrganizationUnitResult, error) {
	return biz.OrganizationUnitResult{}, nil
}
func (stubIdentityDirectory) ProvisionMember(context.Context, biz.ProvisionMember) (biz.ProvisionMemberResult, error) {
	return biz.ProvisionMemberResult{}, nil
}
func (stubIdentityDirectory) ReplaceMemberAccess(context.Context, biz.ReplaceMemberAccess) (biz.ProvisionMemberResult, error) {
	return biz.ProvisionMemberResult{}, nil
}
func (stubIdentityDirectory) UpdateMember(context.Context, biz.UpdateMember) (biz.UpdateMemberResult, error) {
	return biz.UpdateMemberResult{}, nil
}

type stubUsers struct{ currentErr error }

type stubPlans struct{}

type stubMerchantDirectory struct{}

func (stubMerchantDirectory) ListMerchants(context.Context) ([]merchantmodel.Merchant, error) {
	return []merchantmodel.Merchant{{ID: 2001, Name: "Local Merchant", Status: merchantmodel.StatusActive, Version: 1}}, nil
}
func (stubMerchantDirectory) ListMerchantPage(context.Context, merchantmodel.Query) (merchantmodel.Page, error) {
	return merchantmodel.Page{Items: []merchantmodel.Record{{ID: 2001, Name: "Local Merchant", Account: "merchant", Status: merchantmodel.StatusActive, Version: 1}}, Page: 1, PageSize: 20, Total: 1}, nil
}
func (stubMerchantDirectory) GetMerchant(_ context.Context, merchantID int64) (merchantmodel.Record, error) {
	if merchantID != 2001 {
		return merchantmodel.Record{}, merchantmodel.ErrNotFound
	}
	return merchantmodel.Record{
		ID: 2001, Name: "Local Merchant", Account: "merchant", ExternalID: "ext-2001",
		ContactName: "Ada", ContactPhone: "13800000000", Status: merchantmodel.StatusActive, Version: 1,
	}, nil
}
func (stubMerchantDirectory) CreateMerchant(context.Context, merchantmodel.CreateCommand) (merchantmodel.CreateResult, bool, error) {
	return merchantmodel.CreateResult{}, false, nil
}
func (stubMerchantDirectory) UpdateMerchant(context.Context, merchantmodel.UpdateCommand) (merchantmodel.Record, bool, error) {
	return merchantmodel.Record{}, false, nil
}
func (stubMerchantDirectory) UpdateProfile(_ context.Context, command merchantmodel.UpdateProfileCommand) (merchantmodel.Record, bool, error) {
	return merchantmodel.Record{
		ID: command.MerchantID, Name: "Local Merchant", Account: "merchant", ExternalID: command.ExternalID,
		ContactName: command.ContactName, ContactPhone: command.ContactPhone,
		MarketingEmailOptIn: command.MarketingEmailOptIn, MarketingSMSOptIn: command.MarketingSMSOptIn,
		Status: merchantmodel.StatusActive, Version: command.ExpectedVersion + 1,
	}, false, nil
}
func (stubMerchantDirectory) ResetOwnerPassword(context.Context, merchantmodel.ResetPasswordCommand) (bool, error) {
	return false, nil
}
func (stubMerchantDirectory) CloseMerchant(context.Context, merchantmodel.CloseCommand) (merchantmodel.Record, bool, error) {
	return merchantmodel.Record{}, false, nil
}

type stubAssignments struct{}

func (stubAssignments) GetAssignment(context.Context, int64) (subscriptionmodel.Assignment, error) {
	return subscriptionmodel.Assignment{MerchantID: 2001}, nil
}
func (stubAssignments) Assign(context.Context, subscriptionmodel.AssignCommand) (subscriptionmodel.Assignment, bool, error) {
	return subscriptionmodel.Assignment{}, false, nil
}

type stubOrders struct{}

func (stubOrders) CreateOrder(context.Context, subscriptionmodel.CreateOrderCommand) (subscriptionmodel.Order, bool, error) {
	return subscriptionmodel.Order{}, false, nil
}
func (stubOrders) GetOrder(context.Context, int64, string) (subscriptionmodel.Order, error) {
	return subscriptionmodel.Order{}, subscriptionmodel.ErrOrderNotFound
}
func (stubOrders) ListOrders(context.Context, subscriptionmodel.OrderQuery) (subscriptionmodel.OrderPage, error) {
	return subscriptionmodel.OrderPage{}, nil
}
func (stubOrders) AttachPayment(context.Context, subscriptionmodel.AttachPaymentCommand) (subscriptionmodel.Order, error) {
	return subscriptionmodel.Order{}, subscriptionmodel.ErrOrderNotFound
}
func (stubOrders) Activate(context.Context, subscriptionmodel.ActivateOrderCommand) (subscriptionmodel.Order, subscriptionmodel.Assignment, bool, error) {
	return subscriptionmodel.Order{}, subscriptionmodel.Assignment{}, false, subscriptionmodel.ErrOrderNotFound
}
func (stubOrders) Close(context.Context, subscriptionmodel.CloseOrderCommand) (subscriptionmodel.Order, bool, error) {
	return subscriptionmodel.Order{}, false, subscriptionmodel.ErrOrderNotFound
}

type stubShopDirectory struct{}

func (stubShopDirectory) ListShops(_ context.Context, merchantID int64) ([]shopmodel.Shop, error) {
	id := merchantID
	if id <= 0 {
		id = 2001
	}
	return []shopmodel.Shop{{ID: 3001, MerchantID: id, Code: "local-shop", Name: "Local Shop", Currency: "CNY", Status: shopmodel.StatusActive, Version: 1}}, nil
}
func (s stubShopDirectory) ListShopsByMerchant(ctx context.Context, merchantID int64) ([]shopmodel.Shop, error) {
	return s.ListShops(ctx, merchantID)
}
func (s stubShopDirectory) ListManagedShops(_ context.Context, query shopmodel.Query) (shopmodel.Page, error) {
	items, _ := s.ListShops(context.Background(), query.MerchantID)
	return shopmodel.Page{Items: items, Page: query.Page, PageSize: query.PageSize, Total: int64(len(items))}, nil
}
func (s stubShopDirectory) GetManagedShop(ctx context.Context, merchantID, shopID int64) (shopmodel.Shop, error) {
	items, err := s.ListShops(ctx, merchantID)
	if err != nil {
		return shopmodel.Shop{}, err
	}
	for _, item := range items {
		if item.ID == shopID {
			return item, nil
		}
	}
	return shopmodel.Shop{}, shopmodel.ErrNotFound
}
func (stubShopDirectory) CreateShop(_ context.Context, command shopmodel.CreateCommand) (shopmodel.Shop, bool, error) {
	status := command.Status
	if status == "" {
		status = shopmodel.StatusActive
	}
	return shopmodel.Shop{
		ID: 3002, MerchantID: command.MerchantID, Code: "shop-3002", Subdomain: command.Subdomain, Name: command.Name,
		DefaultLocale: "zh-CN", Currency: command.Currency, CategoryCode: command.CategoryCode, Status: status, Version: 1,
	}, false, nil
}
func (stubShopDirectory) UpdateShop(_ context.Context, command shopmodel.UpdateCommand) (shopmodel.Shop, bool, error) {
	return shopmodel.Shop{
		ID: command.ShopID, MerchantID: command.MerchantID, Code: "local-shop", Subdomain: command.Subdomain,
		Name: command.Name, DefaultLocale: "zh-CN", Currency: "CNY", Status: shopmodel.StatusActive, Version: command.ExpectedVersion + 1,
	}, false, nil
}
func (stubShopDirectory) SetShopEnabled(_ context.Context, command shopmodel.SetEnabledCommand) (shopmodel.Shop, bool, error) {
	status := shopmodel.StatusDisabled
	if command.Enabled {
		status = shopmodel.StatusActive
	}
	return shopmodel.Shop{
		ID: command.ShopID, MerchantID: command.MerchantID, Code: "local-shop", Name: "Local Shop",
		Currency: "CNY", Status: status, Version: command.ExpectedVersion + 1,
	}, false, nil
}
func (stubShopDirectory) CloseShop(_ context.Context, command shopmodel.CloseCommand) (shopmodel.Shop, bool, error) {
	return shopmodel.Shop{
		ID: command.ShopID, MerchantID: command.MerchantID, Code: "local-shop", Name: "Local Shop",
		Currency: "CNY", Status: shopmodel.StatusClosed, Version: command.ExpectedVersion + 1,
	}, false, nil
}

type stubShopCategories struct{}

type stubPrivacy struct{}

func (stubPrivacy) GetPrivacy(_ context.Context, merchantID, shopID int64) (shopmodel.Privacy, error) {
	return shopmodel.DefaultPrivacy(merchantID, shopID), nil
}
func (stubPrivacy) SavePrivacy(_ context.Context, command shopmodel.SavePrivacyCommand) (shopmodel.Privacy, bool, error) {
	value := command.Privacy
	value.ID = 1
	value.Version = command.ExpectedVersion + 1
	return value, false, nil
}

type stubPolicies struct{}

func (stubPolicies) ListPolicies(_ context.Context, query shopmodel.PolicyQuery) (shopmodel.PolicyPage, error) {
	return shopmodel.PolicyPage{Items: []shopmodel.Policy{}, Page: query.Page, PageSize: query.PageSize}, nil
}
func (stubPolicies) SavePolicy(context.Context, shopmodel.SavePolicyCommand) (shopmodel.Policy, bool, error) {
	return shopmodel.Policy{}, false, nil
}
func (stubPolicies) PublishPolicy(context.Context, shopmodel.PublishPolicyCommand) (shopmodel.Policy, bool, error) {
	return shopmodel.Policy{}, false, nil
}

type stubApps struct{}

func (stubApps) ListApps(_ context.Context, query shopmodel.AppQuery) (shopmodel.AppPage, error) {
	return shopmodel.AppPage{Items: []shopmodel.App{}, Page: query.Page, PageSize: query.PageSize}, nil
}
func (stubApps) CreateApp(context.Context, shopmodel.CreateAppCommand) (shopmodel.AppMutation, bool, error) {
	return shopmodel.AppMutation{}, false, nil
}
func (stubApps) ResetAppSecret(context.Context, shopmodel.ResetAppSecretCommand) (shopmodel.AppMutation, bool, error) {
	return shopmodel.AppMutation{}, false, nil
}
func (stubApps) SetAppEnabled(context.Context, shopmodel.SetAppEnabledCommand) (shopmodel.App, bool, error) {
	return shopmodel.App{}, false, nil
}

type stubCustomerService struct{}

func (stubCustomerService) List(context.Context, customerservicemodel.Query) (customerservicemodel.Page, error) {
	return customerservicemodel.Page{Items: []customerservicemodel.Account{}, Page: 1, PageSize: 20, Total: 0}, nil
}
func (stubCustomerService) Save(context.Context, customerservicemodel.SaveCommand) (customerservicemodel.Account, bool, error) {
	return customerservicemodel.Account{}, false, nil
}
func (stubCustomerService) Delete(context.Context, customerservicemodel.DeleteCommand) (customerservicemodel.DeleteResult, bool, error) {
	return customerservicemodel.DeleteResult{}, false, nil
}

type stubRiskEvents struct{}

func (stubRiskEvents) List(context.Context, riskmodel.Query) (riskmodel.Page, error) {
	return riskmodel.Page{Page: 1, PageSize: 20, Total: 1, Items: []riskmodel.Event{{
		ID: 1, MerchantID: 2001, ShopID: 3001, VisitorID: "v-1001", Nickname: "Ada", RoomID: 9001, Reason: "spam",
		ScoreBefore: 10, ScoreAfterDecay: 10, ScoreDelta: 20, ScoreAfter: 30, CurrentScore: 30,
		CurrentLevel: riskmodel.LevelLow, VisitorStatus: riskmodel.StatusWatch, CreatedAt: time.Unix(1, 0).UTC(),
	}}}, nil
}

type stubMerchantGovernance struct{}

func (stubMerchantGovernance) List(context.Context, governancemodel.Query) (governancemodel.Page, error) {
	return governancemodel.Page{Items: []governancemodel.Capability{}}, nil
}
func (stubMerchantGovernance) Audit(context.Context, governancemodel.AuditQuery) ([]governancemodel.AuditItem, error) {
	return []governancemodel.AuditItem{}, nil
}
func (stubMerchantGovernance) Intervene(context.Context, governancemodel.InterveneCommand) (governancemodel.Capability, bool, error) {
	return governancemodel.Capability{}, false, nil
}

func (stubShopCategories) ListCategories(context.Context) ([]shopmodel.Category, error) {
	return []shopmodel.Category{{ID: 1, Code: "apparel", Name: "服装服饰", Icon: "👗", Sort: 1, Status: shopmodel.CategoryActive, Version: 1}}, nil
}
func (stubShopCategories) SaveCategory(context.Context, shopmodel.SaveCategoryCommand) (shopmodel.Category, bool, error) {
	return shopmodel.Category{}, false, nil
}
func (stubShopCategories) SetCategoryEnabled(context.Context, shopmodel.SetCategoryEnabledCommand) (shopmodel.Category, bool, error) {
	return shopmodel.Category{}, false, nil
}
func (stubShopCategories) RetireCategory(context.Context, shopmodel.RetireCategoryCommand) (shopmodel.Category, bool, error) {
	return shopmodel.Category{}, false, nil
}

func (stubPlans) ListPlans(context.Context) ([]subscriptionmodel.Plan, error) {
	return []subscriptionmodel.Plan{{ID: 1, Code: "free", Name: "免费版", Status: subscriptionmodel.PlanActive, Default: true, Version: 1}}, nil
}
func (stubPlans) SavePlan(context.Context, subscriptionmodel.SavePlanCommand) (subscriptionmodel.Plan, bool, error) {
	return subscriptionmodel.Plan{}, false, nil
}
func (stubPlans) RetirePlan(context.Context, subscriptionmodel.RetirePlanCommand) (subscriptionmodel.Plan, bool, error) {
	return subscriptionmodel.Plan{}, false, nil
}
func (stubPlans) GetPlanPolicy(context.Context, int64) (subscriptionmodel.PlanPolicy, error) {
	return subscriptionmodel.PlanPolicy{PlanID: 1, PlanCode: "free", PlanName: "免费版", Revision: 1, PermissionCodes: []string{}}, nil
}
func (stubPlans) SavePlanPolicy(context.Context, subscriptionmodel.SavePlanPolicyCommand) (subscriptionmodel.PlanPolicy, bool, error) {
	return subscriptionmodel.PlanPolicy{}, false, nil
}

func (stubUsers) ListUsers(context.Context, biz.UserScope) ([]biz.ManagedUser, error) {
	return []biz.ManagedUser{{Subject: model.Subject{ID: "managed-user", DisplayName: "Managed User"}}}, nil
}
func (stubUsers) ListMembers(context.Context, biz.MemberQuery) (biz.MemberPage, error) {
	return biz.MemberPage{Page: 1, PageSize: 20}, nil
}
func (stubUsers) GetUser(context.Context, biz.UserScope, string) (biz.ManagedUser, error) {
	return biz.ManagedUser{}, nil
}
func (stubUsers) OwnAccount(context.Context, biz.UserScope, string) (biz.ManagedUser, error) {
	return biz.ManagedUser{
		Subject:        model.Subject{ID: "tester", DisplayName: "Tester", PrincipalType: principal.TypeMerchantStaff, Status: model.StatusActive, Version: 1},
		Member:         model.WorkforceMember{ID: 1, OrganizationID: 1, MerchantID: 2001, Type: model.MemberStaff, Status: model.MemberActive, AccessVersion: 1, Subject: "tester"},
		Credential:     biz.ManagedCredential{ID: 1, Version: 1, Kind: "USERNAME", Identifier: "tester", Status: model.StatusActive},
		ActiveSessions: 1,
	}, nil
}
func (stubUsers) CreatePlatformOperator(context.Context, biz.CreateOperator) (biz.ManagedUser, error) {
	return biz.ManagedUser{}, nil
}
func (stubUsers) ChangeUserStatus(context.Context, biz.ChangeUserStatus) (biz.ManagedUser, error) {
	return biz.ManagedUser{}, nil
}
func (stubUsers) ResetCredential(context.Context, biz.ResetCredential) (biz.ManagedCredential, error) {
	return biz.ManagedCredential{}, nil
}
func (stubUsers) ChangeOwnCredential(context.Context, biz.ChangeOwnCredential) (biz.ChangeOwnCredentialResult, error) {
	return biz.ChangeOwnCredentialResult{
		Credential:      biz.ManagedCredential{ID: 1, Version: 2, Kind: "USERNAME", Identifier: "tester", Status: model.StatusActive},
		RevokedSessions: 0,
		CurrentRetained: true,
	}, nil
}
func (stubUsers) ListSessions(context.Context, biz.UserScope, string) ([]biz.ManagedSession, error) {
	now := time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
	return []biz.ManagedSession{
		{ID: "session-test", DeviceName: "Test Browser", IPAddress: "127.0.0.1", Status: "ACTIVE", CreatedAt: now, LastRefreshedAt: now, ExpiresAt: now},
		{ID: "managed-session", DeviceName: "Other Device", IPAddress: "10.0.0.8", Status: "ACTIVE", CreatedAt: now, LastRefreshedAt: now, ExpiresAt: now},
	}, nil
}
func (s stubUsers) ListOwnSessions(ctx context.Context, scope biz.UserScope, subject string) ([]biz.ManagedSession, error) {
	return s.ListSessions(ctx, scope, subject)
}
func (stubUsers) RevokeSessions(context.Context, biz.RevokeSessions) error    { return nil }
func (stubUsers) RevokeOwnSessions(context.Context, biz.RevokeSessions) error { return nil }
func (s stubUsers) ValidateCurrentAuthorization(context.Context, modulesession.Claims) error {
	return s.currentErr
}

func TestHealthRequiresAuthorizedModuleSession(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)

	for _, surface := range surfaces {
		endpoint := base + surface.prefix + "/health"

		t.Run(surface.name+"/granted", func(t *testing.T) {
			token := sign(t, issuer, surface.name, surface.prefix, healthPermission)
			status, body := call(t, endpoint, surface.name, token)
			if status != http.StatusOK {
				t.Fatalf("状态 %d：%s", status, body)
			}
			if !strings.Contains(body, string(model.StatusActive)) {
				t.Fatalf("响应未包含健康状态：%s", body)
			}
		})

		t.Run(surface.name+"/no session", func(t *testing.T) {
			if status, body := call(t, endpoint, surface.name, ""); status != http.StatusUnauthorized {
				t.Fatalf("无凭证时状态 %d：%s", status, body)
			}
		})

		t.Run(surface.name+"/permission not granted", func(t *testing.T) {
			token := sign(t, issuer, surface.name, surface.prefix)
			if status, body := call(t, endpoint, surface.name, token); status != http.StatusForbidden {
				t.Fatalf("缺权限时状态 %d：%s", status, body)
			}
		})

		t.Run(surface.name+"/route not in scope", func(t *testing.T) {
			// A session whose contribution was granted another prefix must not
			// reach this one, even though the module and surface match.
			token := sign(t, issuer, surface.name, surface.prefix+"/somewhere-else", healthPermission)
			if status, body := call(t, endpoint, surface.name, token); status != http.StatusForbidden {
				t.Fatalf("越权路由时状态 %d：%s", status, body)
			}
		})

		t.Run(surface.name+"/surface header mismatch", func(t *testing.T) {
			token := sign(t, issuer, surface.name, surface.prefix, healthPermission)
			if status, body := call(t, endpoint, "other", token); status != http.StatusForbidden {
				t.Fatalf("surface 头不符时状态 %d：%s", status, body)
			}
		})
	}
}

func TestAdminUserAndSessionPermissionsAreIndependent(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	for _, test := range []struct {
		name, path, prefix, permission string
		want                           int
	}{
		{name: "user granted", path: "/admin/identity/users", prefix: "/admin/identity/users", permission: "identity.user.manage", want: http.StatusOK},
		{name: "user missing", path: "/admin/identity/users", prefix: "/admin/identity/users", permission: "identity.session.manage", want: http.StatusForbidden},
		{name: "session granted", path: "/admin/identity/users/managed-user/sessions", prefix: "/admin/identity/users", permission: "identity.session.manage", want: http.StatusOK},
		{name: "user permission cannot reach sessions", path: "/admin/identity/users/managed-user/sessions", prefix: "/admin/identity/users", permission: "identity.user.manage", want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			token := sign(t, issuer, "admin", test.prefix, test.permission)
			status, body := call(t, base+test.path, "admin", token)
			if status != test.want {
				t.Fatalf("status=%d want=%d body=%s", status, test.want, body)
			}
		})
	}
}

func TestAdminSubscriptionRequiresDedicatedPermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/admin/identity/subscription/plans"
	granted := sign(t, issuer, "admin", "/admin/identity/subscription", "identity.subscription.manage")
	if status, body := call(t, endpoint, "admin", granted); status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, body)
	}
	policyEndpoint := base + "/admin/identity/subscription/plans/1/permissions"
	if status, body := call(t, policyEndpoint, "admin", granted); status != http.StatusOK {
		t.Fatalf("plan policy granted status=%d body=%s", status, body)
	}
	wrong := sign(t, issuer, "admin", "/admin/identity/subscription", "identity.authorization.manage")
	if status, body := call(t, endpoint, "admin", wrong); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, body)
	}
	outOfScope := sign(t, issuer, "admin", "/admin/identity/users", "identity.subscription.manage")
	if status, body := call(t, endpoint, "admin", outOfScope); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, body)
	}
}

func TestAdminShopsRequireDedicatedReadPermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	for _, endpoint := range []string{
		base + "/admin/identity/shops/merchants",
		base + "/admin/identity/shops?merchantId=2001",
	} {
		granted := sign(t, issuer, "admin", "/admin/identity/shops", "identity.shop.read")
		if status, body := call(t, endpoint, "admin", granted); status != http.StatusOK {
			t.Fatalf("granted endpoint=%s status=%d body=%s", endpoint, status, body)
		}
		wrong := sign(t, issuer, "admin", "/admin/identity/shops", "identity.directory.read")
		if status, body := call(t, endpoint, "admin", wrong); status != http.StatusForbidden {
			t.Fatalf("wrong permission endpoint=%s status=%d body=%s", endpoint, status, body)
		}
		outOfScope := sign(t, issuer, "admin", "/admin/identity/subscription", "identity.shop.read")
		if status, body := call(t, endpoint, "admin", outOfScope); status != http.StatusForbidden {
			t.Fatalf("out of scope endpoint=%s status=%d body=%s", endpoint, status, body)
		}
	}
}

func TestAdminShopCategoriesRequireDedicatedManagePermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/admin/identity/shop-categories"
	granted := sign(t, issuer, "admin", "/admin/identity/shop-categories", "identity.shop-category.manage")
	if status, body := call(t, endpoint, "admin", granted); status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, body)
	}
	wrong := sign(t, issuer, "admin", "/admin/identity/shop-categories", "identity.shop.read")
	if status, body := call(t, endpoint, "admin", wrong); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, body)
	}
	outOfScope := sign(t, issuer, "admin", "/admin/identity/shops", "identity.shop-category.manage")
	if status, body := call(t, endpoint, "admin", outOfScope); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, body)
	}
}

func TestAdminCustomerServiceRequiresDedicatedManagePermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/admin/identity/customer-accounts?merchantId=2001&shopId=3001&page=1&pageSize=20"
	granted := sign(t, issuer, "admin", "/admin/identity/customer-accounts", "identity.customer-account.manage")
	if status, body := call(t, endpoint, "admin", granted); status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, body)
	}
	wrong := sign(t, issuer, "admin", "/admin/identity/customer-accounts", "identity.shop.read")
	if status, body := call(t, endpoint, "admin", wrong); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, body)
	}
	outOfScope := sign(t, issuer, "admin", "/admin/identity/shops", "identity.customer-account.manage")
	if status, body := call(t, endpoint, "admin", outOfScope); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, body)
	}
}

func TestAdminMerchantsRequireDedicatedManagePermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/admin/identity/merchants?page=1&pageSize=20"
	granted := sign(t, issuer, "admin", "/admin/identity/merchants", "identity.merchant.manage")
	if status, body := call(t, endpoint, "admin", granted); status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, body)
	}
	wrong := sign(t, issuer, "admin", "/admin/identity/merchants", "identity.shop.read")
	if status, body := call(t, endpoint, "admin", wrong); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, body)
	}
	outOfScope := sign(t, issuer, "admin", "/admin/identity/shops", "identity.merchant.manage")
	if status, body := call(t, endpoint, "admin", outOfScope); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, body)
	}
}

func TestAdminMerchantGovernanceRequiresDedicatedManagePermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/admin/identity/merchant-governance?merchantId=2001&shopId=3001"
	granted := sign(t, issuer, "admin", "/admin/identity/merchant-governance", "identity.merchant-governance.manage")
	if status, body := call(t, endpoint, "admin", granted); status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, body)
	}
	wrong := sign(t, issuer, "admin", "/admin/identity/merchant-governance", "identity.customer-account.manage")
	if status, body := call(t, endpoint, "admin", wrong); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, body)
	}
	outOfScope := sign(t, issuer, "admin", "/admin/identity/customer-accounts", "identity.merchant-governance.manage")
	if status, body := call(t, endpoint, "admin", outOfScope); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, body)
	}
}

func TestMerchPoliciesRequireDedicatedReadPermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	for _, endpoint := range []string{
		base + "/merch/identity/policies/shops",
		base + "/merch/identity/policies?shopId=3001&page=1&pageSize=20",
	} {
		granted := sign(t, issuer, "merch", "/merch/identity/policies", "identity.policy.read")
		if status, body := call(t, endpoint, "merch", granted); status != http.StatusOK {
			t.Fatalf("granted %s status=%d body=%s", endpoint, status, body)
		}
		wrong := sign(t, issuer, "merch", "/merch/identity/policies", "identity.privacy.manage")
		if status, body := call(t, endpoint, "merch", wrong); status != http.StatusForbidden {
			t.Fatalf("wrong permission %s status=%d body=%s", endpoint, status, body)
		}
		outOfScope := sign(t, issuer, "merch", "/merch/identity/privacy", "identity.policy.read")
		if status, body := call(t, endpoint, "merch", outOfScope); status != http.StatusForbidden {
			t.Fatalf("out of scope %s status=%d body=%s", endpoint, status, body)
		}
	}
}

func TestMerchAppsRequireDedicatedReadPermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	for _, endpoint := range []string{
		base + "/merch/identity/apps/shops",
		base + "/merch/identity/apps/scopes",
		base + "/merch/identity/apps?shopId=3001&page=1&pageSize=20",
	} {
		granted := sign(t, issuer, "merch", "/merch/identity/apps", "identity.app.read")
		if status, body := call(t, endpoint, "merch", granted); status != http.StatusOK {
			t.Fatalf("granted %s status=%d body=%s", endpoint, status, body)
		}
		wrong := sign(t, issuer, "merch", "/merch/identity/apps", "identity.policy.read")
		if status, body := call(t, endpoint, "merch", wrong); status != http.StatusForbidden {
			t.Fatalf("wrong permission %s status=%d body=%s", endpoint, status, body)
		}
		outOfScope := sign(t, issuer, "merch", "/merch/identity/policies", "identity.app.read")
		if status, body := call(t, endpoint, "merch", outOfScope); status != http.StatusForbidden {
			t.Fatalf("out of scope %s status=%d body=%s", endpoint, status, body)
		}
	}
}

func TestMerchSubscriptionRequiresDedicatedReadPermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	for _, endpoint := range []string{
		base + "/merch/identity/subscription/plans",
		base + "/merch/identity/subscription",
		base + "/merch/identity/subscription/pay-methods",
	} {
		granted := sign(t, issuer, "merch", "/merch/identity/subscription", "identity.subscription.read")
		if status, body := call(t, endpoint, "merch", granted); status != http.StatusOK {
			t.Fatalf("granted %s status=%d body=%s", endpoint, status, body)
		}
		wrong := sign(t, issuer, "merch", "/merch/identity/subscription", "identity.app.read")
		if status, body := call(t, endpoint, "merch", wrong); status != http.StatusForbidden {
			t.Fatalf("wrong permission %s status=%d body=%s", endpoint, status, body)
		}
		outOfScope := sign(t, issuer, "merch", "/merch/identity/apps", "identity.subscription.read")
		if status, body := call(t, endpoint, "merch", outOfScope); status != http.StatusForbidden {
			t.Fatalf("out of scope %s status=%d body=%s", endpoint, status, body)
		}
	}
}

func TestMerchShopsRequireDedicatedReadPermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	for _, endpoint := range []string{
		base + "/merch/identity/shops?page=1&pageSize=20",
		base + "/merch/identity/shops/categories",
	} {
		granted := sign(t, issuer, "merch", "/merch/identity/shops", "identity.shop.read")
		if status, body := call(t, endpoint, "merch", granted); status != http.StatusOK {
			t.Fatalf("granted %s status=%d body=%s", endpoint, status, body)
		}
		wrong := sign(t, issuer, "merch", "/merch/identity/shops", "identity.app.read")
		if status, body := call(t, endpoint, "merch", wrong); status != http.StatusForbidden {
			t.Fatalf("wrong permission %s status=%d body=%s", endpoint, status, body)
		}
		outOfScope := sign(t, issuer, "merch", "/merch/identity/apps", "identity.shop.read")
		if status, body := call(t, endpoint, "merch", outOfScope); status != http.StatusForbidden {
			t.Fatalf("out of scope %s status=%d body=%s", endpoint, status, body)
		}
	}
}

func TestMerchCurrentShopRequiresSessionShopAndReadPermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/merch/identity/shops/current"
	granted := signMerchShop(t, issuer, "/merch/identity/shops", 2001, 3001, "identity.shop.read")
	if status, body := call(t, endpoint, "merch", granted); status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, body)
	}
	wrong := signMerchShop(t, issuer, "/merch/identity/shops", 2001, 3001, "identity.app.read")
	if status, body := call(t, endpoint, "merch", wrong); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, body)
	}
	outOfScope := signMerchShop(t, issuer, "/merch/identity/apps", 2001, 3001, "identity.shop.read")
	if status, body := call(t, endpoint, "merch", outOfScope); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, body)
	}
	missingShop := signMerchShop(t, issuer, "/merch/identity/shops", 2001, 0, "identity.shop.read")
	if status, body := call(t, endpoint, "merch", missingShop); status != http.StatusForbidden {
		t.Fatalf("missing shop status=%d body=%s", status, body)
	}
	unknownShop := signMerchShop(t, issuer, "/merch/identity/shops", 2001, 9999, "identity.shop.read")
	if status, body := call(t, endpoint, "merch", unknownShop); status != http.StatusNotFound {
		t.Fatalf("unknown shop status=%d body=%s", status, body)
	}
}

func TestMerchShopWritesRequireOwnerAndManagePermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/merch/identity/shops"
	body := `{"commandKey":"shop-create-0001","name":"二店","subdomain":"second-shop","currency":"CNY"}`
	granted := signMerchOwner(t, issuer, "/merch/identity/shops", 2001, "identity.shop.manage")
	if status, payload := callJSON(t, http.MethodPost, endpoint, "merch", granted, body); status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, payload)
	}
	staff := signMerchShop(t, issuer, "/merch/identity/shops", 2001, 0, "identity.shop.manage")
	if status, payload := callJSON(t, http.MethodPost, endpoint, "merch", staff, body); status != http.StatusForbidden {
		t.Fatalf("staff status=%d body=%s", status, payload)
	}
	wrong := signMerchOwner(t, issuer, "/merch/identity/shops", 2001, "identity.shop.read")
	if status, payload := callJSON(t, http.MethodPost, endpoint, "merch", wrong, body); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, payload)
	}
	outOfScope := signMerchOwner(t, issuer, "/merch/identity/apps", 2001, "identity.shop.manage")
	if status, payload := callJSON(t, http.MethodPost, endpoint, "merch", outOfScope, body); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, payload)
	}
}

func TestMerchPrivacyRequiresDedicatedPermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/merch/identity/privacy"
	granted := signMerchShop(t, issuer, "/merch/identity/privacy", 2001, 3001, "identity.privacy.manage")
	if status, body := call(t, endpoint, "merch", granted); status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, body)
	}
	wrong := signMerchShop(t, issuer, "/merch/identity/privacy", 2001, 3001, "identity.organization.read")
	if status, body := call(t, endpoint, "merch", wrong); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, body)
	}
	outOfScope := signMerchShop(t, issuer, "/merch/identity/directory", 2001, 3001, "identity.privacy.manage")
	if status, body := call(t, endpoint, "merch", outOfScope); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, body)
	}
	missingShop := signMerchShop(t, issuer, "/merch/identity/privacy", 2001, 0, "identity.privacy.manage")
	if status, body := call(t, endpoint, "merch", missingShop); status != http.StatusForbidden {
		t.Fatalf("missing shop status=%d body=%s", status, body)
	}
}

func TestMerchProfileRequiresDedicatedPermission(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/merch/identity/profile"
	granted := signMerchOwner(t, issuer, "/merch/identity/profile", 2001, "identity.profile.manage")
	status, body := call(t, endpoint, "merch", granted)
	if status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, body)
	}
	if !strings.Contains(body, `"name":"Local Merchant"`) || !strings.Contains(body, `"account":"merchant"`) || !strings.Contains(body, `"externalId":"ext-2001"`) {
		t.Fatalf("granted body missing profile: %s", body)
	}
	staff := signMerchShop(t, issuer, "/merch/identity/profile", 2001, 0, "identity.profile.manage")
	if status, body := call(t, endpoint, "merch", staff); status != http.StatusForbidden {
		t.Fatalf("staff status=%d body=%s", status, body)
	}
	wrong := signMerchOwner(t, issuer, "/merch/identity/profile", 2001, "identity.organization.read")
	if status, body := call(t, endpoint, "merch", wrong); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, body)
	}
	outOfScope := signMerchOwner(t, issuer, "/merch/identity/account", 2001, "identity.profile.manage")
	if status, body := call(t, endpoint, "merch", outOfScope); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, body)
	}
}

func TestMerchAccountRequiresOrganizationRead(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/merch/identity/account"
	granted := signMerchShop(t, issuer, "/merch/identity/account", 2001, 3001, "identity.organization.read")
	status, body := call(t, endpoint, "merch", granted)
	if status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, body)
	}
	if !strings.Contains(body, `"account":"tester"`) || !strings.Contains(body, "Local Merchant") {
		t.Fatalf("granted body missing account overview: %s", body)
	}
	wrong := signMerchShop(t, issuer, "/merch/identity/account", 2001, 3001, "identity.privacy.manage")
	if status, body := call(t, endpoint, "merch", wrong); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, body)
	}
	outOfScope := signMerchShop(t, issuer, "/merch/identity/privacy", 2001, 3001, "identity.organization.read")
	if status, body := call(t, endpoint, "merch", outOfScope); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, body)
	}
}

func TestMerchAccountSecurityRequiresOrganizationRead(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/merch/identity/account/security"
	granted := signMerchShop(t, issuer, "/merch/identity/account/security", 2001, 3001, "identity.organization.read")
	status, body := call(t, endpoint, "merch", granted)
	if status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, body)
	}
	if !strings.Contains(body, `"account":"tester"`) || !strings.Contains(body, `"version":1`) {
		t.Fatalf("granted body missing account security: %s", body)
	}
	wrong := signMerchShop(t, issuer, "/merch/identity/account/security", 2001, 3001, "identity.privacy.manage")
	if status, body := call(t, endpoint, "merch", wrong); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, body)
	}
	outOfScope := signMerchShop(t, issuer, "/merch/identity/privacy", 2001, 3001, "identity.organization.read")
	if status, body := call(t, endpoint, "merch", outOfScope); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, body)
	}

	update := base + "/merch/identity/account/credentials"
	payload := `{"commandKey":"change-own","expectedVersion":1,"oldPassword":"password-old","password":"password-new"}`
	putGranted := signMerchShop(t, issuer, "/merch/identity/account/credentials", 2001, 3001, "identity.organization.read")
	status, body = callJSON(t, http.MethodPut, update, "merch", putGranted, payload)
	if status != http.StatusOK {
		t.Fatalf("granted put status=%d body=%s", status, body)
	}
	if !strings.Contains(body, `"currentRetained":true`) || !strings.Contains(body, `"version":2`) {
		t.Fatalf("granted put body missing credential result: %s", body)
	}
	putWrong := signMerchShop(t, issuer, "/merch/identity/account/credentials", 2001, 3001, "identity.privacy.manage")
	if status, body := callJSON(t, http.MethodPut, update, "merch", putWrong, payload); status != http.StatusForbidden {
		t.Fatalf("wrong permission put status=%d body=%s", status, body)
	}
	putOutOfScope := signMerchShop(t, issuer, "/merch/identity/account/security", 2001, 3001, "identity.organization.read")
	if status, body := callJSON(t, http.MethodPut, update, "merch", putOutOfScope, payload); status != http.StatusForbidden {
		t.Fatalf("out of scope put status=%d body=%s", status, body)
	}
}

func TestMerchAccountSessionsRequiresOrganizationRead(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/merch/identity/account/sessions"
	granted := signMerchShop(t, issuer, "/merch/identity/account/sessions", 2001, 3001, "identity.organization.read")
	status, body := call(t, endpoint, "merch", granted)
	if status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, body)
	}
	if !strings.Contains(body, `"id":"session-test"`) || !strings.Contains(body, `"current":true`) {
		t.Fatalf("granted body missing current session: %s", body)
	}
	wrong := signMerchShop(t, issuer, "/merch/identity/account/sessions", 2001, 3001, "identity.privacy.manage")
	if status, body := call(t, endpoint, "merch", wrong); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, body)
	}
	outOfScope := signMerchShop(t, issuer, "/merch/identity/privacy", 2001, 3001, "identity.organization.read")
	if status, body := call(t, endpoint, "merch", outOfScope); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, body)
	}

	revoke, err := http.NewRequest(http.MethodPost, endpoint+"/managed-session/revoke", strings.NewReader(`{"idempotencyKey":"device-1","operationId":"device-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	revoke.Header.Set("Authorization", "Bearer "+granted)
	revoke.Header.Set("X-Liveshop-Surface", "merch")
	revoke.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(revoke)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	revokeBody, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("revoke status=%d body=%s", response.StatusCode, revokeBody)
	}
	if !strings.Contains(string(revokeBody), `"currentRevoked":false`) {
		t.Fatalf("revoke body=%s", revokeBody)
	}
}

func TestMerchRiskEventsRequiresRiskRead(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServer(t, keys)
	endpoint := base + "/merch/identity/risk-events"
	granted := signMerchShop(t, issuer, "/merch/identity/risk-events", 2001, 3001, "identity.risk.read")
	status, body := call(t, endpoint, "merch", granted)
	if status != http.StatusOK {
		t.Fatalf("granted status=%d body=%s", status, body)
	}
	if !strings.Contains(body, `"visitorId":"v-1001"`) || !strings.Contains(body, `"reason":"spam"`) {
		t.Fatalf("granted body missing risk event: %s", body)
	}
	wrong := signMerchShop(t, issuer, "/merch/identity/risk-events", 2001, 3001, "identity.organization.read")
	if status, body := call(t, endpoint, "merch", wrong); status != http.StatusForbidden {
		t.Fatalf("wrong permission status=%d body=%s", status, body)
	}
	outOfScope := signMerchShop(t, issuer, "/merch/identity/privacy", 2001, 3001, "identity.risk.read")
	if status, body := call(t, endpoint, "merch", outOfScope); status != http.StatusForbidden {
		t.Fatalf("out of scope status=%d body=%s", status, body)
	}
}

func TestMutationRejectsStaleCapabilityAuthorization(t *testing.T) {
	issuer, keys := testKeys(t)
	base := startServerWithUsers(t, keys, stubUsers{currentErr: model.ErrAuthorizationDenied})
	token := sign(t, issuer, "admin", "/admin/identity/users", "identity.user.manage")
	request, err := http.NewRequest(http.MethodPost, base+"/admin/identity/users", strings.NewReader(`{"idempotencyKey":"key"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Liveshop-Surface", "admin")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("stale mutation status=%d body=%s", response.StatusCode, body)
	}
}

// startServer assembles the process the way main does, minus the database, and
// returns its base URL.
func startServer(t *testing.T, keys map[string]string) string {
	return startServerWithUsers(t, keys, stubUsers{})
}
func startServerWithUsers(t *testing.T, keys map[string]string, users stubUsers) string {
	t.Helper()
	verifier, err := modulesession.NewVerifier(keys, "liveshop-identity", middleware.Audience)
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(&Dependencies{
		Config:             config.Config{Server: config.Server{HTTP: "127.0.0.1:0"}},
		Health:             biz.NewHealth(stubHealth{}),
		Directory:          biz.NewDirectory(stubIdentityDirectory{}),
		Users:              biz.NewUserLifecycle(users),
		Plans:              subscription.NewPlans(stubPlans{}),
		Merchants:          merchant.NewDirectory(stubMerchantDirectory{}),
		Shops:              shop.NewDirectory(stubShopDirectory{}),
		ShopCategories:     shop.NewCategories(stubShopCategories{}),
		Privacy:            shop.NewPrivacySettings(stubPrivacy{}),
		Policies:           shop.NewPolicies(stubPolicies{}),
		Apps:               shop.NewPrivateApps(stubApps{}),
		CustomerService:    customer_service.NewAccounts(stubCustomerService{}),
		RiskEvents:         risk.NewEvents(stubRiskEvents{}),
		MerchantGovernance: merchant_governance.NewCapabilities(stubMerchantGovernance{}),
		Assignments:        subscription.NewAssignments(stubAssignments{}),
		Orders:             subscription.NewOrders(stubOrders{}),
		ModuleSessions:     verifier,
	})

	ctx, cancel := context.WithCancel(context.Background())
	failures := make(chan error, 1)
	go func() { failures <- server.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-failures
	})

	deadline := time.Now().Add(5 * time.Second)
	for server.Port() <= 0 {
		select {
		case err := <-failures:
			t.Fatalf("服务未能启动：%v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("服务未在 5 秒内监听")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", server.Port())
}

func testKeys(t *testing.T) (*modulesession.Issuer, map[string]string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	issuer, err := modulesession.NewIssuer(encode(private), "test-key", "liveshop-identity")
	if err != nil {
		t.Fatal(err)
	}
	return issuer, map[string]string{"test-key": encode(public)}
}

// sign mints the session the Gateway would issue for one contribution.
func sign(t *testing.T, issuer *modulesession.Issuer, surface, prefix string, permissions ...string) string {
	return signSession(t, issuer, surface, prefix, 1, 0, permissions...)
}

func signMerchShop(t *testing.T, issuer *modulesession.Issuer, prefix string, merchantID, shopID int64, permissions ...string) string {
	return signSession(t, issuer, "merch", prefix, merchantID, shopID, permissions...)
}

func signMerchOwner(t *testing.T, issuer *modulesession.Issuer, prefix string, merchantID int64, permissions ...string) string {
	t.Helper()
	token, err := issuer.Sign(modulesession.Claims{
		Subject:               "tester",
		SessionID:             "session-test",
		Realm:                 principal.RealmMerchant,
		PrincipalType:         principal.TypeMerchantOwner,
		ModuleID:              middleware.ModuleID,
		ModuleVersion:         "0.1.0",
		Surface:               "merch",
		ContributionID:        "test-contribution",
		MerchantID:            merchantID,
		OrganizationID:        1,
		AuthorizationRevision: 1,
		RegistryRevision:      1,
		EntitlementRevision:   1,
		IdentityVersion:       1,
		OrganizationVersion:   1,
		ContextVersion:        1,
		AllowedRoutes:         []modulesession.RouteScope{{Methods: []string{http.MethodGet, http.MethodPost, http.MethodPut}, Prefix: prefix}},
		Permissions:           permissions,
	}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func signSession(t *testing.T, issuer *modulesession.Issuer, surface, prefix string, merchantID, shopID int64, permissions ...string) string {
	t.Helper()
	token, err := issuer.Sign(modulesession.Claims{
		Subject:               "tester",
		SessionID:             "session-test",
		Realm:                 map[string]principal.Realm{"admin": principal.RealmPlatform, "merch": principal.RealmMerchant}[surface],
		PrincipalType:         map[string]principal.Type{"admin": principal.TypePlatformOperator, "merch": principal.TypeMerchantStaff}[surface],
		ModuleID:              middleware.ModuleID,
		ModuleVersion:         "0.1.0",
		Surface:               surface,
		ContributionID:        "test-contribution",
		MerchantID:            merchantID,
		ShopID:                shopID,
		OrganizationID:        1,
		AuthorizationRevision: 1,
		RegistryRevision:      1,
		EntitlementRevision:   1,
		IdentityVersion:       1,
		OrganizationVersion:   1,
		ContextVersion:        1,
		AllowedRoutes:         []modulesession.RouteScope{{Methods: []string{http.MethodGet, http.MethodPost, http.MethodPut}, Prefix: prefix}},
		Permissions:           permissions,
	}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func call(t *testing.T, endpoint, surface, token string) (int, string) {
	return callJSON(t, http.MethodGet, endpoint, surface, token, "")
}

func callJSON(t *testing.T, method, endpoint, surface, token, body string) (int, string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request, err := http.NewRequest(method, endpoint, reader)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	request.Header.Set("X-Liveshop-Surface", surface)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(payload)
}
