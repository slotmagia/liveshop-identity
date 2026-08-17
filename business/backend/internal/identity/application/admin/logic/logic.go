// Package logic implements the admin surface boundary. It returns domain
// errors; each transport owns its own projection of them.
package logic

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/compose"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service"
	customerservicemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance"
	governancemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant"
	merchantmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
	subscriptionmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type Logic struct {
	health          *biz.Health
	directory       *biz.Directory
	authorization   *biz.AuthorizationService
	users           *biz.UserLifecycle
	plans           *subscription.Plans
	merchants       *merchant.Directory
	shops           *shop.Directory
	shopCategories  *shop.Categories
	customerService      *customer_service.Accounts
	merchantGovernance   *merchant_governance.Capabilities
	assignments          *subscription.Assignments
	permissionPlans      *subscription.PermissionEntitlements
	quotas               *subscription.Quotas
	grants               compose.Grants
}

var _ service.Admin = (*Logic)(nil)

func New(health *biz.Health, directory *biz.Directory, authorization *biz.AuthorizationService, users *biz.UserLifecycle, plans *subscription.Plans, merchants *merchant.Directory, shops *shop.Directory, shopCategories *shop.Categories, customerService *customer_service.Accounts, merchantGovernance *merchant_governance.Capabilities, assignments *subscription.Assignments, permissionPlans *subscription.PermissionEntitlements, quotas *subscription.Quotas, grants compose.Grants) *Logic {
	if grants == nil {
		grants = compose.Unavailable{}
	}
	return &Logic{health: health, directory: directory, authorization: authorization, users: users, plans: plans, merchants: merchants, shops: shops, shopCategories: shopCategories, customerService: customerService, merchantGovernance: merchantGovernance, assignments: assignments, permissionPlans: permissionPlans, quotas: quotas, grants: grants}
}

func (l *Logic) CustomerServiceAccounts(ctx context.Context, input appmodel.CustomerServiceQuery) (appmodel.CustomerServicePage, error) {
	var status *customerservicemodel.Status
	if input.Status != "" {
		value := customerservicemodel.Status(input.Status)
		status = &value
	}
	value, err := l.customerService.List(ctx, customerservicemodel.Query{MerchantID: input.MerchantID, ShopID: input.ShopID,
		Platform: input.Platform, Account: input.Account, Status: status, Page: input.Page, PageSize: input.PageSize})
	if err != nil {
		return appmodel.CustomerServicePage{}, err
	}
	result := appmodel.CustomerServicePage{Page: value.Page, PageSize: value.PageSize, Total: value.Total, Items: []appmodel.CustomerServiceAccount{}}
	for _, item := range value.Items {
		result.Items = append(result.Items, projectCustomerServiceAccount(item))
	}
	return result, nil
}

func (l *Logic) SaveCustomerServiceAccount(ctx context.Context, input appmodel.SaveCustomerServiceAccount) (appmodel.CustomerServiceAccountResult, error) {
	value, replayed, err := l.customerService.Save(ctx, customerservicemodel.SaveCommand{CommandKey: input.CommandKey,
		ExpectedVersion: input.ExpectedVersion, Account: customerservicemodel.Account{ID: input.Account.ID,
			MerchantID: input.Account.MerchantID, ShopID: input.Account.ShopID, Platform: input.Account.Platform,
			Account: input.Account.Account, Nickname: input.Account.Nickname, Status: customerservicemodel.Status(input.Account.Status),
			Config: input.Account.Config, Remark: input.Account.Remark}})
	return appmodel.CustomerServiceAccountResult{Account: projectCustomerServiceAccount(value), Replayed: replayed}, err
}

func (l *Logic) DeleteCustomerServiceAccount(ctx context.Context, input appmodel.DeleteCustomerServiceAccount) (appmodel.CustomerServiceDeleteResult, error) {
	value, replayed, err := l.customerService.Delete(ctx, customerservicemodel.DeleteCommand{AccountID: input.AccountID,
		MerchantID: input.MerchantID, ShopID: input.ShopID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion})
	return appmodel.CustomerServiceDeleteResult{ID: value.ID, Deleted: value.Deleted, Version: value.Version, Replayed: replayed}, err
}

func (l *Logic) GovernanceCatalog() []appmodel.GovernanceModule {
	if l.merchantGovernance == nil {
		return nil
	}
	out := make([]appmodel.GovernanceModule, 0, len(l.merchantGovernance.Catalog()))
	for _, item := range l.merchantGovernance.Catalog() {
		out = append(out, appmodel.GovernanceModule{Key: item.Key, Label: item.Label})
	}
	return out
}

func (l *Logic) GovernanceCapabilities(ctx context.Context, input appmodel.GovernanceQuery) ([]appmodel.GovernanceCapability, error) {
	if l.merchantGovernance == nil {
		return nil, governancemodel.ErrUnavailable
	}
	value, err := l.merchantGovernance.List(ctx, governancemodel.Query{MerchantID: input.MerchantID, ShopID: input.ShopID, Module: input.Module})
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.GovernanceCapability, 0, len(value.Items))
	for _, item := range value.Items {
		out = append(out, projectGovernanceCapability(item))
	}
	return out, nil
}

func (l *Logic) GovernanceAudit(ctx context.Context, input appmodel.GovernanceQuery) ([]appmodel.GovernanceAuditItem, error) {
	if l.merchantGovernance == nil {
		return nil, governancemodel.ErrUnavailable
	}
	values, err := l.merchantGovernance.Audit(ctx, governancemodel.AuditQuery{MerchantID: input.MerchantID, ShopID: input.ShopID, Module: input.Module, Limit: 50})
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.GovernanceAuditItem, 0, len(values))
	for _, item := range values {
		out = append(out, appmodel.GovernanceAuditItem{
			ID: item.ID, MerchantID: item.MerchantID, ShopID: item.ShopID,
			Module: item.Module, CapabilityID: item.CapabilityID, Action: item.Action, Operator: item.Operator,
			ReasonInternal: item.ReasonInternal, ReasonPublic: item.ReasonPublic, CreatedAt: item.CreatedAt.Format(time.RFC3339Nano),
		})
	}
	return out, nil
}

func (l *Logic) InterveneGovernance(ctx context.Context, input appmodel.InterveneGovernance) (appmodel.GovernanceCapabilityResult, error) {
	if l.merchantGovernance == nil {
		return appmodel.GovernanceCapabilityResult{}, governancemodel.ErrUnavailable
	}
	value, replayed, err := l.merchantGovernance.Intervene(ctx, governancemodel.InterveneCommand{
		CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion, MerchantID: input.MerchantID, ShopID: input.ShopID,
		Module: input.Module, PlatformStatus: governancemodel.PlatformStatus(input.PlatformStatus),
		ReasonInternal: input.ReasonInternal, ReasonPublic: input.ReasonPublic, Operator: authctx.Caller(ctx).Subject,
	})
	return appmodel.GovernanceCapabilityResult{Capability: projectGovernanceCapability(value), Replayed: replayed}, err
}

func projectGovernanceCapability(value governancemodel.Capability) appmodel.GovernanceCapability {
	updatedAt := ""
	if !value.UpdatedAt.IsZero() {
		updatedAt = value.UpdatedAt.Format(time.RFC3339Nano)
	}
	return appmodel.GovernanceCapability{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID,
		Module: value.Module, ModuleLabel: value.ModuleLabel, Name: value.Name, MerchantStatus: string(value.MerchantStatus),
		PlatformStatus: string(value.PlatformStatus), PlatformReasonPublic: value.PlatformReasonPublic, Version: value.Version,
		UpdatedBy: value.UpdatedBy, UpdatedAt: updatedAt,
	}
}

func projectCustomerServiceAccount(value customerservicemodel.Account) appmodel.CustomerServiceAccount {
	return appmodel.CustomerServiceAccount{ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID,
		Platform: value.Platform, Account: value.Account, Nickname: value.Nickname,
		Status: string(value.Status), Config: value.Config, Remark: value.Remark, Version: value.Version,
		CreatedAt: value.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: value.UpdatedAt.Format(time.RFC3339Nano)}
}

func (l *Logic) ShopCategories(ctx context.Context) ([]appmodel.ShopCategory, error) {
	values, err := l.shopCategories.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.ShopCategory, 0, len(values))
	for _, value := range values {
		out = append(out, projectShopCategory(value))
	}
	return out, nil
}

func (l *Logic) SaveShopCategory(ctx context.Context, input appmodel.SaveShopCategory) (appmodel.ShopCategoryResult, error) {
	value, replayed, err := l.shopCategories.Save(ctx, shopmodel.SaveCategoryCommand{
		CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
		Category: shopmodel.Category{ID: input.Category.ID, Code: input.Category.Code, Name: input.Category.Name,
			Icon: input.Category.Icon, Sort: input.Category.Sort, Status: shopmodel.CategoryStatus(input.Category.Status)},
	})
	return appmodel.ShopCategoryResult{Category: projectShopCategory(value), Replayed: replayed}, err
}

func (l *Logic) SetShopCategoryEnabled(ctx context.Context, input appmodel.SetShopCategoryEnabled) (appmodel.ShopCategoryResult, error) {
	value, replayed, err := l.shopCategories.SetEnabled(ctx, shopmodel.SetCategoryEnabledCommand{
		CategoryID: input.CategoryID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion, Enabled: input.Enabled,
	})
	return appmodel.ShopCategoryResult{Category: projectShopCategory(value), Replayed: replayed}, err
}

func (l *Logic) RetireShopCategory(ctx context.Context, input appmodel.RetireShopCategory) (appmodel.ShopCategoryResult, error) {
	value, replayed, err := l.shopCategories.Retire(ctx, shopmodel.RetireCategoryCommand{
		CategoryID: input.CategoryID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
	})
	return appmodel.ShopCategoryResult{Category: projectShopCategory(value), Replayed: replayed}, err
}

func projectShopCategory(value shopmodel.Category) appmodel.ShopCategory {
	return appmodel.ShopCategory{ID: value.ID, Code: value.Code, Name: value.Name, Icon: value.Icon, Sort: value.Sort,
		Status: string(value.Status), Version: value.Version, UsedShopCount: value.UsedShopCount}
}

func (l *Logic) ShopMerchants(ctx context.Context) ([]appmodel.ShopMerchant, error) {
	values, err := l.merchants.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.ShopMerchant, 0, len(values))
	for _, value := range values {
		out = append(out, appmodel.ShopMerchant{ID: value.ID, Name: value.Name, ExternalID: value.ExternalID, Status: string(value.Status), Version: value.Version})
	}
	return out, nil
}

func (l *Logic) MerchantShops(ctx context.Context, merchantID int64) ([]appmodel.ManagedShop, error) {
	values, err := l.shops.ListByMerchant(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	return projectManagedShops(values), nil
}

func (l *Logic) DirectoryShops(ctx context.Context, merchantID int64) ([]appmodel.ManagedShop, error) {
	values, err := l.shops.List(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	return projectManagedShops(values), nil
}

func projectManagedShops(values []shopmodel.Shop) []appmodel.ManagedShop {
	out := make([]appmodel.ManagedShop, 0, len(values))
	for _, value := range values {
		out = append(out, appmodel.ManagedShop{ID: value.ID, MerchantID: value.MerchantID,
			Code: value.Code, Subdomain: value.Subdomain, Name: value.Name,
			DefaultLocale: value.DefaultLocale, Currency: value.Currency, Status: string(value.Status), Version: value.Version})
	}
	return out
}

func (l *Logic) SubscriptionPlans(ctx context.Context) ([]appmodel.SubscriptionPlan, error) {
	if l.plans == nil {
		return nil, model.ErrUnavailable
	}
	values, err := l.plans.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.SubscriptionPlan, 0, len(values))
	for _, value := range values {
		out = append(out, projectSubscriptionPlan(value))
	}
	return out, nil
}

func (l *Logic) SaveSubscriptionPlan(ctx context.Context, input appmodel.SaveSubscriptionPlan) (appmodel.SubscriptionPlanResult, error) {
	if l.plans == nil {
		return appmodel.SubscriptionPlanResult{}, model.ErrUnavailable
	}
	value, replayed, err := l.plans.Save(ctx, subscriptionmodel.SavePlanCommand{
		CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion, Plan: subscriptionmodel.Plan{
			ID: input.Plan.ID, Code: input.Plan.Code, Name: input.Plan.Name, Level: input.Plan.Level,
			PriceMinor: input.Plan.PriceMinor, DurationDays: input.Plan.DurationDays, Description: input.Plan.Description,
			Default: input.Plan.Default, Sort: input.Plan.Sort, Status: subscriptionmodel.PlanStatus(input.Plan.Status),
		},
	})
	return appmodel.SubscriptionPlanResult{Plan: projectSubscriptionPlan(value), Replayed: replayed}, err
}

func (l *Logic) RetireSubscriptionPlan(ctx context.Context, input appmodel.RetireSubscriptionPlan) (appmodel.SubscriptionPlanResult, error) {
	if l.plans == nil {
		return appmodel.SubscriptionPlanResult{}, model.ErrUnavailable
	}
	value, replayed, err := l.plans.Retire(ctx, subscriptionmodel.RetirePlanCommand{PlanID: input.PlanID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion})
	return appmodel.SubscriptionPlanResult{Plan: projectSubscriptionPlan(value), Replayed: replayed}, err
}

func (l *Logic) SubscriptionPermissions(ctx context.Context) ([]appmodel.Permission, error) {
	return l.Permissions(ctx)
}

func (l *Logic) SubscriptionPlanPolicy(ctx context.Context, planID int64) (appmodel.SubscriptionPlanPolicy, error) {
	if l.plans == nil {
		return appmodel.SubscriptionPlanPolicy{}, model.ErrUnavailable
	}
	value, err := l.plans.Policy(ctx, planID)
	return projectSubscriptionPlanPolicy(value), err
}

func (l *Logic) SaveSubscriptionPlanPolicy(ctx context.Context, input appmodel.SaveSubscriptionPlanPolicy) (appmodel.SubscriptionPlanPolicyResult, error) {
	if l.plans == nil {
		return appmodel.SubscriptionPlanPolicyResult{}, model.ErrUnavailable
	}
	value, replayed, err := l.plans.SavePolicy(ctx, subscriptionmodel.SavePlanPolicyCommand{
		CommandKey: input.CommandKey, ExpectedRevision: input.ExpectedRevision,
		Policy: subscriptionmodel.PlanPolicy{PlanID: input.Policy.PlanID, PermissionCodes: input.Policy.PermissionCodes, ProductLimit: input.Policy.ProductLimit},
	})
	return appmodel.SubscriptionPlanPolicyResult{Policy: projectSubscriptionPlanPolicy(value), Replayed: replayed}, err
}

func projectSubscriptionPlan(value subscriptionmodel.Plan) appmodel.SubscriptionPlan {
	return appmodel.SubscriptionPlan{ID: value.ID, Code: value.Code, Name: value.Name, Level: value.Level,
		PriceMinor: value.PriceMinor, DurationDays: value.DurationDays, Description: value.Description,
		Default: value.Default, Sort: value.Sort, Status: string(value.Status), Version: value.Version}
}

func projectSubscriptionPlanPolicy(value subscriptionmodel.PlanPolicy) appmodel.SubscriptionPlanPolicy {
	return appmodel.SubscriptionPlanPolicy{PlanID: value.PlanID, PlanCode: value.PlanCode, PlanName: value.PlanName,
		PermissionCodes: value.PermissionCodes, ProductLimit: value.ProductLimit, Revision: value.Revision}
}

func (l *Logic) userScope(ctx context.Context) biz.UserScope {
	return biz.UserScope{OrganizationID: authctx.Caller(ctx).OrganizationID}
}
func (l *Logic) Users(ctx context.Context) ([]appmodel.ManagedUser, error) {
	values, err := l.users.List(ctx, l.userScope(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.ManagedUser, 0, len(values))
	for _, v := range values {
		out = append(out, projectManagedUser(v))
	}
	return out, nil
}
func (l *Logic) User(ctx context.Context, subject string) (appmodel.ManagedUser, error) {
	v, err := l.users.Detail(ctx, l.userScope(ctx), subject)
	return projectManagedUser(v), err
}
func (l *Logic) CreateOperator(ctx context.Context, input appmodel.CreateOperator) (appmodel.ManagedUser, error) {
	claims := authctx.Caller(ctx)
	digest := sha256.Sum256([]byte("platform:" + fmt.Sprint(claims.OrganizationID) + ":operator:" + input.OperationID))
	subject := "sub_" + base64.RawURLEncoding.EncodeToString(digest[:18])
	v, err := l.users.CreateOperator(ctx, biz.CreateOperator{IdempotencyKey: input.IdempotencyKey, OperationID: input.OperationID, Subject: subject, DisplayName: input.DisplayName, Username: input.Username, Password: input.Password, OrganizationID: claims.OrganizationID, RoleIDs: input.RoleIDs})
	return projectManagedUser(v), err
}
func (l *Logic) ChangeUserStatus(ctx context.Context, input appmodel.ChangeUserStatus) (appmodel.ManagedUser, error) {
	claims := authctx.Caller(ctx)
	v, err := l.users.ChangeStatus(ctx, biz.ChangeUserStatus{IdempotencyKey: input.IdempotencyKey, OperationID: input.OperationID, Subject: input.Subject, ActorSubject: claims.Subject, Scope: l.userScope(ctx), ExpectedIdentityVersion: input.ExpectedIdentityVersion, ExpectedAccessVersion: input.ExpectedAccessVersion, Target: model.Status(input.Status)})
	return projectManagedUser(v), err
}
func (l *Logic) ResetCredential(ctx context.Context, input appmodel.ResetCredential) (appmodel.ManagedCredential, error) {
	claims := authctx.Caller(ctx)
	v, err := l.users.ResetCredential(ctx, biz.ResetCredential{IdempotencyKey: input.IdempotencyKey, OperationID: input.OperationID, Subject: input.Subject, ActorSubject: claims.Subject, Scope: l.userScope(ctx), CredentialID: input.CredentialID, ExpectedCredentialVersion: input.ExpectedCredentialVersion, Password: input.Password})
	return appmodel.ManagedCredential{ID: v.ID, Version: v.Version, Kind: v.Kind, Identifier: v.Identifier, Status: string(v.Status)}, err
}
func (l *Logic) Sessions(ctx context.Context, subject string) ([]appmodel.ManagedSession, error) {
	values, err := l.users.Sessions(ctx, l.userScope(ctx), subject)
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.ManagedSession, 0, len(values))
	for _, v := range values {
		out = append(out, appmodel.ManagedSession(v))
	}
	return out, nil
}
func (l *Logic) RevokeSessions(ctx context.Context, input appmodel.RevokeSessions) error {
	claims := authctx.Caller(ctx)
	return l.users.RevokeSessions(ctx, biz.RevokeSessions{IdempotencyKey: input.IdempotencyKey, OperationID: input.OperationID, Subject: input.Subject, ActorSubject: claims.Subject, SessionID: input.SessionID, Scope: l.userScope(ctx)})
}
func projectManagedUser(v biz.ManagedUser) appmodel.ManagedUser {
	return appmodel.ManagedUser{Subject: v.Subject.ID, DisplayName: v.Subject.DisplayName, Realm: v.Subject.Realm.String(), PrincipalType: v.Subject.PrincipalType.String(), SubjectStatus: string(v.Subject.Status), SubjectVersion: v.Subject.Version, MemberID: v.Member.ID, OrganizationID: v.Member.OrganizationID, MemberType: string(v.Member.Type), MemberStatus: string(v.Member.Status), AccessVersion: v.Member.AccessVersion, Credential: appmodel.ManagedCredential{ID: v.Credential.ID, Version: v.Credential.Version, Kind: v.Credential.Kind, Identifier: v.Credential.Identifier, Status: string(v.Credential.Status)}, RoleIDs: v.RoleIDs, ActiveSessions: v.ActiveSessions}
}
func (l *Logic) domain(ctx context.Context) model.AuthorizationDomain {
	organizationID := authctx.Caller(ctx).OrganizationID
	return model.AuthorizationDomain{Type: model.AuthorizationPlatform, ID: organizationID, OrganizationID: organizationID}
}
func (l *Logic) Permissions(ctx context.Context) ([]appmodel.Permission, error) {
	permissions, err := l.authorization.Permissions(ctx, l.domain(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.Permission, 0, len(permissions))
	for _, permission := range permissions {
		out = append(out, appmodel.Permission{ModuleID: permission.ModuleID, Code: permission.Code, Name: permission.Name, Resource: permission.Resource, Action: permission.Action, Description: permission.Description, RegistryRevision: permission.RegistryRevision})
	}
	return out, nil
}
func (l *Logic) Roles(ctx context.Context) ([]appmodel.Role, error) {
	roles, err := l.authorization.Roles(ctx, l.domain(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.Role, 0, len(roles))
	for _, role := range roles {
		out = append(out, projectRole(role))
	}
	return out, nil
}
func (l *Logic) PutRole(ctx context.Context, x appmodel.PutRole) (appmodel.Role, error) {
	role, err := l.authorization.PutRole(ctx, l.domain(ctx), model.Role{ID: x.RoleID, Code: x.Code, Name: x.Name, Status: x.Status}, x.ExpectedVersion)
	return projectRole(role), err
}
func (l *Logic) PutRolePolicy(ctx context.Context, x appmodel.PutRolePolicy) (appmodel.Role, error) {
	scopes := make([]model.ScopeRule, 0, len(x.Scopes))
	for _, scope := range x.Scopes {
		scopes = append(scopes, model.ScopeRule{Resource: scope.Resource, Type: scope.Type, ReferenceIDs: scope.ReferenceIDs})
	}
	role, err := l.authorization.SetRolePolicy(ctx, l.domain(ctx), x.RoleID, x.ExpectedVersion, model.RolePolicy{Permissions: x.Permissions, Scopes: scopes})
	return projectRole(role), err
}

func projectRole(role model.Role) appmodel.Role {
	return appmodel.Role{ID: role.ID, Code: role.Code, Name: role.Name, Status: role.Status, SystemRole: role.SystemRole, Version: role.Version}
}
func (l *Logic) PutSubjectGrants(ctx context.Context, x appmodel.PutSubjectGrants) error {
	return l.authorization.ReplaceSubjectGrants(ctx, l.domain(ctx), x.Subject, x.RoleIDs, x.OperationID, x.AccessVersion)
}

func (l *Logic) Health(ctx context.Context) (appmodel.Health, error) {
	if l.health == nil {
		return appmodel.Health{}, model.ErrUnavailable
	}
	current, err := l.health.Check(ctx)
	if err != nil {
		return appmodel.Health{}, err
	}
	return appmodel.Health{Status: current.Status}, nil
}
func (l *Logic) Directory(ctx context.Context, input appmodel.DirectoryQuery) (appmodel.Directory, error) {
	if l.directory == nil {
		return appmodel.Directory{}, model.ErrUnavailable
	}
	claims := authctx.Caller(ctx)
	scope := authctx.Scope(ctx, "identity.directory")
	allowed := scope.Type == modulesession.DataScopeAll || input.OrganizationID == claims.OrganizationID
	for _, ref := range scope.ReferenceIDs {
		if ref == fmt.Sprintf("merchant:%d", input.MerchantID) || ref == fmt.Sprintf("organization:%d", input.OrganizationID) {
			allowed = true
		}
	}
	if !allowed {
		return appmodel.Directory{}, model.ErrInvalidContext
	}
	value, err := l.directory.OrganizationDirectory(ctx, input.OrganizationID, input.MerchantID)
	if err != nil {
		return appmodel.Directory{}, err
	}
	result := appmodel.Directory{Organization: appmodel.Organization{
		ID: value.Organization.ID, Type: string(value.Organization.Type),
		MerchantID: value.Organization.MerchantID, Name: value.Organization.Name,
		Status: string(value.Organization.Status), Version: value.Organization.Version,
	}}
	for _, unit := range value.Units {
		result.Units = append(result.Units, appmodel.OrganizationUnit{ID: unit.ID, ParentID: unit.ParentID, Name: unit.Name, Status: string(unit.Status), Version: unit.Version})
	}
	for _, member := range value.Members {
		result.Members = append(result.Members, appmodel.Member{
			ID: member.Member.ID, OrganizationID: member.Member.OrganizationID,
			MerchantID: member.Member.MerchantID, Subject: member.Member.Subject,
			DisplayName: member.DisplayName, Type: string(member.Member.Type),
			Status: string(member.Member.Status), PrincipalType: member.PrincipalType.String(),
			AccessVersion: member.Member.AccessVersion,
			UnitIDs:       member.UnitIDs, ShopIDs: member.ShopIDs,
		})
	}
	for _, shop := range value.Shops {
		result.Shops = append(result.Shops, appmodel.Shop{
			ID: shop.Context.ShopID, MerchantID: shop.Context.MerchantID,
			Name: shop.Name, Code: shop.Code, Status: string(shop.Status), Version: shop.Version,
		})
	}
	return result, nil
}

func (l *Logic) Merchants(ctx context.Context, input appmodel.MerchantQuery) (appmodel.MerchantPage, error) {
	if l.merchants == nil {
		return appmodel.MerchantPage{}, merchantmodel.ErrUnavailable
	}
	value, err := l.merchants.Page(ctx, merchantmodel.Query{Keyword: input.Keyword, Status: input.Status, Page: input.Page, PageSize: input.PageSize})
	if err != nil {
		return appmodel.MerchantPage{}, err
	}
	out := appmodel.MerchantPage{Items: []appmodel.ManagedMerchant{}, Page: value.Page, PageSize: value.PageSize, Total: value.Total}
	for _, item := range value.Items {
		out.Items = append(out.Items, projectManagedMerchant(item))
	}
	return out, nil
}

func (l *Logic) CreateMerchant(ctx context.Context, input appmodel.CreateMerchant) (appmodel.CreateMerchantResult, error) {
	if l.merchants == nil {
		return appmodel.CreateMerchantResult{}, merchantmodel.ErrUnavailable
	}
	value, replayed, err := l.merchants.Create(ctx, merchantmodel.CreateCommand{
		CommandKey: input.CommandKey, Account: input.Account, Password: input.Password,
		Name: input.Name, ContactName: input.ContactName, ContactPhone: input.ContactPhone,
	})
	if err != nil {
		return appmodel.CreateMerchantResult{}, err
	}
	if !replayed {
		if err := l.ensureMerchantPermissionEntitlement(ctx, value.Merchant.ID, input.CommandKey); err != nil {
			return appmodel.CreateMerchantResult{}, err
		}
	}
	return appmodel.CreateMerchantResult{Merchant: projectManagedMerchant(value.Merchant), ShopID: value.ShopID, ShopCode: value.ShopCode, Account: value.Account, Replayed: replayed}, nil
}

func (l *Logic) ensureMerchantPermissionEntitlement(ctx context.Context, merchantID int64, commandKey string) error {
	if l.permissionPlans == nil {
		return nil
	}
	if l.plans != nil {
		plans, err := l.plans.List(ctx)
		if err != nil {
			return err
		}
		for _, plan := range plans {
			if plan.Default && plan.Status == subscriptionmodel.PlanActive {
				return l.applyPlanEntitlements(ctx, merchantID, commandKey, plan.ID)
			}
		}
	}
	_, _, err := l.permissionPlans.Apply(ctx, subscription.ApplyPermissionEntitlementCommand{
		MerchantID: merchantID, CommandKey: commandKey + ":permissions",
	})
	return err
}

func (l *Logic) UpdateMerchant(ctx context.Context, input appmodel.UpdateMerchant) (appmodel.MerchantResult, error) {
	if l.merchants == nil {
		return appmodel.MerchantResult{}, merchantmodel.ErrUnavailable
	}
	value, replayed, err := l.merchants.Update(ctx, merchantmodel.UpdateCommand{
		MerchantID: input.MerchantID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
		Name: input.Name, Status: merchantmodel.Status(input.Status), ContactName: input.ContactName, ContactPhone: input.ContactPhone,
	})
	return appmodel.MerchantResult{Merchant: projectManagedMerchant(value), Replayed: replayed}, err
}

func (l *Logic) ResetMerchantPassword(ctx context.Context, input appmodel.ResetMerchantPassword) (bool, error) {
	if l.merchants == nil {
		return false, merchantmodel.ErrUnavailable
	}
	return l.merchants.ResetPassword(ctx, merchantmodel.ResetPasswordCommand{MerchantID: input.MerchantID, CommandKey: input.CommandKey, Password: input.Password})
}

func (l *Logic) CloseMerchant(ctx context.Context, input appmodel.CloseMerchant) (appmodel.MerchantResult, error) {
	if l.merchants == nil {
		return appmodel.MerchantResult{}, merchantmodel.ErrUnavailable
	}
	value, replayed, err := l.merchants.Close(ctx, merchantmodel.CloseCommand{MerchantID: input.MerchantID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion})
	return appmodel.MerchantResult{Merchant: projectManagedMerchant(value), Replayed: replayed}, err
}

func (l *Logic) MerchantSubscription(ctx context.Context, merchantID int64) (appmodel.MerchantSubscription, error) {
	if l.assignments == nil {
		return appmodel.MerchantSubscription{}, subscriptionmodel.ErrAssignmentInvalid
	}
	value, err := l.assignments.Get(ctx, merchantID)
	if err != nil {
		return appmodel.MerchantSubscription{}, err
	}
	return projectMerchantSubscription(value), nil
}

func (l *Logic) AssignMerchantSubscription(ctx context.Context, input appmodel.AssignMerchantSubscription) (appmodel.MerchantSubscriptionResult, error) {
	if l.assignments == nil || l.plans == nil {
		return appmodel.MerchantSubscriptionResult{}, subscriptionmodel.ErrAssignmentInvalid
	}
	plan, err := l.plans.List(ctx)
	if err != nil {
		return appmodel.MerchantSubscriptionResult{}, err
	}
	var selected subscriptionmodel.Plan
	for _, item := range plan {
		if item.ID == input.PlanID {
			selected = item
			break
		}
	}
	if selected.ID == 0 || selected.Status != subscriptionmodel.PlanActive {
		return appmodel.MerchantSubscriptionResult{}, subscriptionmodel.ErrPlanNotFound
	}
	expires := merchantmodel.ExpiresAt(selected.DurationDays, time.Now())
	var expiresText *string
	if expires != nil {
		text := expires.Format(time.RFC3339Nano)
		expiresText = &text
	}
	value, replayed, err := l.assignments.Assign(ctx, subscriptionmodel.AssignCommand{
		MerchantID: input.MerchantID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
		PlanID: input.PlanID, ExpiresAt: expiresText,
	})
	if err != nil {
		return appmodel.MerchantSubscriptionResult{}, err
	}
	if !replayed {
		if err := l.applyPlanEntitlements(ctx, input.MerchantID, input.CommandKey, selected.ID); err != nil {
			return appmodel.MerchantSubscriptionResult{}, err
		}
	}
	return appmodel.MerchantSubscriptionResult{Assignment: projectMerchantSubscription(value), Replayed: replayed}, nil
}

func (l *Logic) applyPlanEntitlements(ctx context.Context, merchantID int64, commandKey string, planID int64) error {
	policy, err := l.plans.Policy(ctx, planID)
	if err != nil {
		return err
	}
	if l.permissionPlans != nil {
		if _, _, err := l.permissionPlans.Apply(ctx, subscription.ApplyPermissionEntitlementCommand{
			MerchantID: merchantID, CommandKey: commandKey + ":permissions", PermissionCodes: policy.PermissionCodes,
		}); err != nil {
			return err
		}
	}
	if l.quotas != nil {
		if _, _, err := l.quotas.Apply(ctx, subscription.ApplyQuotaCommand{
			MerchantID: merchantID, CommandKey: commandKey + ":quota", Code: subscription.CatalogProductsQuota,
			Limit: policy.ProductLimit, EffectiveFrom: time.Now().UTC(),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (l *Logic) MerchantPaymentChannels(ctx context.Context, merchantID, shopID int64) (appmodel.MerchantPaymentChannels, error) {
	if err := l.assertMerchantShop(ctx, merchantID, shopID); err != nil {
		return appmodel.MerchantPaymentChannels{}, err
	}
	return l.grants.PaymentChannels(ctx, merchantID, shopID)
}
func (l *Logic) PutMerchantPaymentChannels(ctx context.Context, input appmodel.PutMerchantPaymentChannels) (appmodel.MerchantPaymentChannels, error) {
	if err := l.assertMerchantShop(ctx, input.MerchantID, input.ShopID); err != nil {
		return appmodel.MerchantPaymentChannels{}, err
	}
	return l.grants.PutPaymentChannels(ctx, input)
}
func (l *Logic) MerchantSMSRegions(ctx context.Context, merchantID, shopID int64) (appmodel.MerchantSMSRegions, error) {
	if err := l.assertMerchantShop(ctx, merchantID, shopID); err != nil {
		return appmodel.MerchantSMSRegions{}, err
	}
	return l.grants.SMSRegions(ctx, merchantID, shopID)
}
func (l *Logic) PutMerchantSMSRegions(ctx context.Context, input appmodel.PutMerchantSMSRegions) (appmodel.MerchantSMSRegions, error) {
	if err := l.assertMerchantShop(ctx, input.MerchantID, input.ShopID); err != nil {
		return appmodel.MerchantSMSRegions{}, err
	}
	return l.grants.PutSMSRegions(ctx, input)
}
func (l *Logic) MerchantLiveProviders(ctx context.Context, merchantID int64) (appmodel.MerchantLiveProviders, error) {
	if err := l.assertMerchant(ctx, merchantID); err != nil {
		return appmodel.MerchantLiveProviders{}, err
	}
	return l.grants.LiveProviders(ctx, merchantID)
}
func (l *Logic) PutMerchantLiveProviders(ctx context.Context, input appmodel.PutMerchantLiveProviders) (appmodel.MerchantLiveProviders, error) {
	if err := l.assertMerchant(ctx, input.MerchantID); err != nil {
		return appmodel.MerchantLiveProviders{}, err
	}
	return l.grants.PutLiveProviders(ctx, input)
}

func (l *Logic) assertMerchant(ctx context.Context, merchantID int64) error {
	if merchantID <= 0 {
		return merchantmodel.ErrInvalid
	}
	shops, err := l.shops.ListByMerchant(ctx, merchantID)
	if err != nil {
		return err
	}
	if len(shops) == 0 {
		return merchantmodel.ErrNotFound
	}
	return nil
}

func (l *Logic) assertMerchantShop(ctx context.Context, merchantID, shopID int64) error {
	if shopID <= 0 {
		return merchantmodel.ErrInvalid
	}
	shops, err := l.shops.ListByMerchant(ctx, merchantID)
	if err != nil {
		return err
	}
	for _, shop := range shops {
		if shop.ID == shopID {
			return nil
		}
	}
	return merchantmodel.ErrNotFound
}

func projectManagedMerchant(value merchantmodel.Record) appmodel.ManagedMerchant {
	return appmodel.ManagedMerchant{
		ID: value.ID, Name: value.Name, ExternalID: value.ExternalID, Account: value.Account,
		ContactName: value.ContactName, ContactPhone: value.ContactPhone, Status: string(value.Status),
		Version: value.Version, ShopID: value.ShopID, ShopCode: value.ShopCode,
	}
}

func projectMerchantSubscription(value subscriptionmodel.Assignment) appmodel.MerchantSubscription {
	return appmodel.MerchantSubscription{
		MerchantID: value.MerchantID, PlanID: value.PlanID, PlanCode: value.PlanCode, PlanName: value.PlanName,
		ExpiresAt: value.ExpiresAt, Version: value.Version,
	}
}
