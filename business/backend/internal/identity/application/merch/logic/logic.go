// Package logic implements the merch surface boundary. It returns domain
// errors; each transport owns its own projection of them.
package logic

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/compose"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance"
	governancemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/risk"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type Subscription struct {
	Plans       *subscription.Plans
	Assignments *subscription.Assignments
	Permissions *subscription.PermissionEntitlements
	Quotas      *subscription.Quotas
	Orders      *subscription.Orders
	Payments    PaymentCollector
}

type PaymentCollector interface {
	Methods(ctx context.Context, amountMinor int64) ([]appmodel.PayMethod, error)
	Charge(ctx context.Context, input appmodel.ChargePayment) (appmodel.ChargeResult, error)
	Status(ctx context.Context, payNo string) (appmodel.PaymentStatus, error)
	ConfirmWallet(ctx context.Context, payNo string) (appmodel.PaymentStatus, error)
}

type Logic struct {
	health             *biz.Health
	directory          *biz.Directory
	authorization      *biz.AuthorizationService
	users              *biz.UserLifecycle
	merchants          *merchant.Directory
	shops              *shop.Directory
	privacy            *shop.PrivacySettings
	policies           *shop.Policies
	apps               *shop.PrivateApps
	merchantGovernance *merchant_governance.Capabilities
	plans              *subscription.Plans
	assignments        *subscription.Assignments
	permissionPlans    *subscription.PermissionEntitlements
	quotas             *subscription.Quotas
	orders             *subscription.Orders
	payments           PaymentCollector
	categories         *shop.Categories
	riskEvents         *risk.Events
	customerService    *customer_service.Accounts
	complaints         *fulfillment.Complaints
	aftersales         *fulfillment.Aftersales
	shipments          *fulfillment.Shipments
	shipping           *fulfillment.Shipping
	domains            *shop.CustomDomains
	grants             compose.Grants
}

var _ service.Merch = (*Logic)(nil)

func New(health *biz.Health, directory *biz.Directory, authorization *biz.AuthorizationService, users *biz.UserLifecycle, shops *shop.Directory, privacy *shop.PrivacySettings, policies *shop.Policies, apps *shop.PrivateApps, merchantGovernance *merchant_governance.Capabilities, catalog Subscription, merchants *merchant.Directory, categories *shop.Categories, riskEvents *risk.Events, customerService *customer_service.Accounts, complaints *fulfillment.Complaints, domains *shop.CustomDomains, aftersales *fulfillment.Aftersales, shipments *fulfillment.Shipments, shipping *fulfillment.Shipping) *Logic {
	return &Logic{
		health: health, directory: directory, authorization: authorization, users: users, merchants: merchants, shops: shops,
		privacy: privacy, policies: policies, apps: apps, merchantGovernance: merchantGovernance, categories: categories,
		riskEvents: riskEvents, customerService: customerService, complaints: complaints, aftersales: aftersales, shipments: shipments, shipping: shipping, domains: domains,
		plans: catalog.Plans, assignments: catalog.Assignments, permissionPlans: catalog.Permissions,
		quotas: catalog.Quotas, orders: catalog.Orders, payments: catalog.Payments,
	}
}

func (l *Logic) UseGrants(grants compose.Grants) {
	if l != nil {
		l.grants = grants
	}
}
func (l *Logic) userScope(ctx context.Context) biz.UserScope {
	claims := authctx.Caller(ctx)
	return biz.UserScope{OrganizationID: claims.OrganizationID, MerchantID: claims.MerchantID}
}
func (l *Logic) ResetCredential(ctx context.Context, input appmodel.ResetCredential) (appmodel.ManagedCredential, error) {
	claims := authctx.Caller(ctx)
	if claims.PrincipalType != principal.TypeMerchantOwner {
		return appmodel.ManagedCredential{}, model.ErrProtectedOwner
	}
	v, err := l.users.ResetCredential(ctx, biz.ResetCredential{IdempotencyKey: input.IdempotencyKey, OperationID: input.OperationID, Subject: input.Subject, ActorSubject: claims.Subject, Scope: l.userScope(ctx), CredentialID: input.CredentialID, ExpectedCredentialVersion: input.ExpectedCredentialVersion, Password: input.Password})
	return appmodel.ManagedCredential{ID: v.ID, Version: v.Version, Kind: v.Kind, Identifier: v.Identifier, Status: string(v.Status)}, err
}
func (l *Logic) Sessions(ctx context.Context, subject string) ([]appmodel.ManagedSession, error) {
	if authctx.Caller(ctx).PrincipalType != principal.TypeMerchantOwner {
		return nil, model.ErrProtectedOwner
	}
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
	if claims.PrincipalType != principal.TypeMerchantOwner {
		return model.ErrProtectedOwner
	}
	return l.users.RevokeSessions(ctx, biz.RevokeSessions{IdempotencyKey: input.IdempotencyKey, OperationID: input.OperationID, Subject: input.Subject, ActorSubject: claims.Subject, SessionID: input.SessionID, Scope: l.userScope(ctx)})
}
func (l *Logic) ChangeMemberStatus(ctx context.Context, input appmodel.ChangeMemberStatus) (appmodel.MemberStatusMutation, error) {
	claims := authctx.Caller(ctx)
	if claims.PrincipalType != principal.TypeMerchantOwner {
		return appmodel.MemberStatusMutation{}, model.ErrProtectedOwner
	}
	v, err := l.users.ChangeStatus(ctx, biz.ChangeUserStatus{IdempotencyKey: input.IdempotencyKey, OperationID: input.OperationID, Subject: input.Subject, ActorSubject: claims.Subject, Scope: l.userScope(ctx), ExpectedIdentityVersion: input.ExpectedIdentityVersion, ExpectedAccessVersion: input.ExpectedAccessVersion, Target: model.Status(input.Status)})
	return appmodel.MemberStatusMutation{Subject: v.Subject.ID, Status: string(v.Subject.Status), IdentityVersion: v.Subject.Version, AccessVersion: v.Member.AccessVersion}, err
}
func (l *Logic) domain(ctx context.Context) model.AuthorizationDomain {
	claims := authctx.Caller(ctx)
	return model.AuthorizationDomain{Type: model.AuthorizationMerchant, ID: claims.MerchantID, OrganizationID: claims.OrganizationID}
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
	if authctx.Caller(ctx).PrincipalType != principal.TypeMerchantOwner {
		return appmodel.Role{}, model.ErrProtectedOwner
	}
	role, err := l.authorization.PutRole(ctx, l.domain(ctx), model.Role{ID: x.RoleID, Code: x.Code, Name: x.Name, Status: x.Status}, x.ExpectedVersion)
	return projectRole(role), err
}
func (l *Logic) PutRolePolicy(ctx context.Context, x appmodel.PutRolePolicy) (appmodel.Role, error) {
	if authctx.Caller(ctx).PrincipalType != principal.TypeMerchantOwner {
		return appmodel.Role{}, model.ErrProtectedOwner
	}
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
	if authctx.Caller(ctx).PrincipalType != principal.TypeMerchantOwner {
		return model.ErrProtectedOwner
	}
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

func (l *Logic) Directory(ctx context.Context) (appmodel.Directory, error) {
	claims := authctx.Caller(ctx)
	if l.directory == nil {
		return appmodel.Directory{}, model.ErrUnavailable
	}
	value, err := l.directory.OrganizationDirectory(ctx, claims.OrganizationID, claims.MerchantID)
	if err != nil {
		return appmodel.Directory{}, err
	}
	return projectDirectory(value), nil
}
func (l *Logic) CreateUnit(ctx context.Context, input appmodel.CreateUnit) (appmodel.Mutation, error) {
	claims := authctx.Caller(ctx)
	if l.directory == nil {
		return appmodel.Mutation{}, model.ErrUnavailable
	}
	result, err := l.directory.CreateOrganizationUnit(ctx, biz.CreateOrganizationUnit{IdempotencyKey: input.IdempotencyKey, OrganizationID: claims.OrganizationID, UnitID: input.UnitID, ParentUnitID: input.ParentID, Name: input.Name, ExpectedVersion: input.ExpectedVersion})
	if err != nil {
		return appmodel.Mutation{}, err
	}
	return appmodel.Mutation{Version: result.Version}, nil
}
func (l *Logic) CreateMember(ctx context.Context, input appmodel.CreateMember) (appmodel.Mutation, error) {
	claims := authctx.Caller(ctx)
	if claims.PrincipalType != principal.TypeMerchantOwner {
		return appmodel.Mutation{}, model.ErrProtectedOwner
	}
	if len(input.Password) < 8 || len(input.RoleIDs) == 0 {
		return appmodel.Mutation{}, model.ErrConflict
	}
	memberType := model.MemberType(input.MemberType)
	pt := model.ExpectedPrincipalType(memberType)
	kind := model.AssignmentOperate
	namespace := "GLOBAL"
	if memberType == model.MemberAnchor {
		kind = model.AssignmentAnchor
		namespace = "SHOP"
	}
	if memberType == model.MemberStaff && len(input.ShopIDs) == 0 {
		return appmodel.Mutation{}, model.ErrInvalidAssignment
	}
	if memberType == model.MemberAnchor && len(input.ShopIDs) != 1 {
		return appmodel.Mutation{}, model.ErrInvalidAssignment
	}
	result, err := l.directory.ProvisionMember(ctx, biz.ProvisionMember{OperationID: input.OperationID, IdempotencyKey: input.IdempotencyKey, Subject: stableSubject(input.OperationID, claims.MerchantID), Realm: principal.RealmMerchant, PrincipalType: pt, DisplayName: input.DisplayName, OrganizationID: claims.OrganizationID, MerchantID: claims.MerchantID, MemberType: memberType, OrganizationUnitIDs: input.UnitIDs, ShopIDs: input.ShopIDs, AssignmentKind: kind, CredentialKind: "USERNAME", CredentialNamespace: namespace, NormalizedIdentifier: input.Username, Password: input.Password, RoleIDs: input.RoleIDs})
	if err != nil {
		return appmodel.Mutation{}, err
	}
	return appmodel.Mutation{MemberID: result.MemberID, Subject: result.Subject, Status: string(result.Status), OperationID: result.OperationID, Version: result.AccessVersion}, nil
}
func (l *Logic) UpdateMember(ctx context.Context, input appmodel.UpdateMember) (appmodel.MemberMutation, error) {
	claims := authctx.Caller(ctx)
	if claims.PrincipalType != principal.TypeMerchantOwner {
		return appmodel.MemberMutation{}, model.ErrProtectedOwner
	}
	memberType := model.MemberType(input.MemberType)
	kind := model.AssignmentOperate
	if memberType == model.MemberAnchor {
		kind = model.AssignmentAnchor
	}
	result, err := l.directory.UpdateMember(ctx, biz.UpdateMember{OperationID: input.OperationID, IdempotencyKey: input.IdempotencyKey, Subject: input.Subject, MerchantID: claims.MerchantID, DisplayName: input.DisplayName, MemberType: memberType, ExpectedIdentityVersion: input.ExpectedIdentityVersion, ExpectedAccessVersion: input.ExpectedAccessVersion, OrganizationUnitIDs: input.UnitIDs, ShopIDs: input.ShopIDs, RoleIDs: input.RoleIDs, AssignmentKind: kind})
	if err != nil {
		return appmodel.MemberMutation{}, err
	}
	return appmodel.MemberMutation{MemberID: result.MemberID, Subject: result.Subject, Status: string(result.Status), OperationID: result.OperationID, IdentityVersion: result.IdentityVersion, AccessVersion: result.AccessVersion}, nil
}
func (l *Logic) MemberOptions(ctx context.Context) (appmodel.MemberOptions, error) {
	if authctx.Caller(ctx).PrincipalType != principal.TypeMerchantOwner {
		return appmodel.MemberOptions{}, model.ErrProtectedOwner
	}
	directory, err := l.Directory(ctx)
	if err != nil {
		return appmodel.MemberOptions{}, err
	}
	roles, err := l.Roles(ctx)
	if err != nil {
		return appmodel.MemberOptions{}, err
	}
	assignable := make([]appmodel.Role, 0, len(roles))
	for _, role := range roles {
		if role.Status == "ACTIVE" && !role.SystemRole {
			assignable = append(assignable, role)
		}
	}
	return appmodel.MemberOptions{Shops: directory.Shops, Roles: assignable, Units: directory.Units}, nil
}
func (l *Logic) Members(ctx context.Context, query appmodel.MemberQuery) (appmodel.MemberPage, error) {
	if authctx.Caller(ctx).PrincipalType != principal.TypeMerchantOwner {
		return appmodel.MemberPage{}, model.ErrProtectedOwner
	}
	page, err := l.users.ListMembers(ctx, biz.MemberQuery{Scope: l.userScope(ctx), Keyword: query.Keyword, MemberType: query.MemberType, Status: query.Status, ShopID: query.ShopID, Page: query.Page, PageSize: query.PageSize})
	if err != nil {
		return appmodel.MemberPage{}, err
	}
	out := appmodel.MemberPage{Items: make([]appmodel.MemberRecord, 0, len(page.Items)), Page: page.Page, PageSize: page.PageSize, Total: page.Total}
	for _, item := range page.Items {
		out.Items = append(out.Items, projectMember(item))
	}
	return out, nil
}
func (l *Logic) Member(ctx context.Context, subject string) (appmodel.MemberRecord, error) {
	if authctx.Caller(ctx).PrincipalType != principal.TypeMerchantOwner {
		return appmodel.MemberRecord{}, model.ErrProtectedOwner
	}
	value, err := l.users.Detail(ctx, l.userScope(ctx), subject)
	if err != nil {
		return appmodel.MemberRecord{}, err
	}
	return projectMember(value), nil
}
func projectMember(value biz.ManagedUser) appmodel.MemberRecord {
	return appmodel.MemberRecord{
		ID: value.Member.ID, Subject: value.Subject.ID, DisplayName: value.Subject.DisplayName,
		Type: string(value.Member.Type), Status: string(value.Subject.Status), MemberStatus: string(value.Member.Status),
		PrincipalType: value.Subject.PrincipalType.String(), AccessVersion: value.Member.AccessVersion, SubjectVersion: value.Subject.Version,
		Credential: appmodel.ManagedCredential{ID: value.Credential.ID, Version: value.Credential.Version, Kind: value.Credential.Kind, Identifier: value.Credential.Identifier, Status: string(value.Credential.Status)},
		RoleIDs:    value.RoleIDs, UnitIDs: value.UnitIDs, ShopIDs: value.ShopIDs, ActiveSessions: value.ActiveSessions,
	}
}
func (l *Logic) ReplaceAccess(ctx context.Context, input appmodel.ReplaceAccess) (appmodel.Mutation, error) {
	claims := authctx.Caller(ctx)
	if claims.PrincipalType != principal.TypeMerchantOwner {
		return appmodel.Mutation{}, model.ErrProtectedOwner
	}
	kind := model.AssignmentOperate
	if input.MemberType == string(model.MemberAnchor) {
		kind = model.AssignmentAnchor
	}
	result, err := l.directory.ReplaceMemberAccess(ctx, biz.ReplaceMemberAccess{OperationID: input.OperationID, IdempotencyKey: input.IdempotencyKey, MemberID: input.MemberID, ExpectedAccessVersion: input.ExpectedAccessVersion, OrganizationUnitIDs: input.UnitIDs, ShopIDs: input.ShopIDs, AssignmentKind: kind})
	if err != nil {
		return appmodel.Mutation{}, err
	}
	return appmodel.Mutation{MemberID: result.MemberID, Status: string(result.Status), OperationID: result.OperationID, Version: result.AccessVersion}, nil
}
func stableSubject(operationID string, merchantID int64) string {
	value := sha256.Sum256([]byte(fmt.Sprintf("merchant:%d:member:%s", merchantID, operationID)))
	return "sub_" + base64.RawURLEncoding.EncodeToString(value[:18])
}
func projectDirectory(value biz.OrganizationDirectory) appmodel.Directory {
	result := appmodel.Directory{Organization: appmodel.Organization{
		ID: value.Organization.ID, Type: value.Organization.Type,
		MerchantID: value.Organization.MerchantID, Name: value.Organization.Name,
		Status: value.Organization.Status, Version: value.Organization.Version,
	}}
	for _, v := range value.Units {
		result.Units = append(result.Units, appmodel.OrganizationUnit{ID: v.ID, ParentID: v.ParentID, Name: v.Name, Status: string(v.Status), Version: v.Version})
	}
	for _, v := range value.Members {
		result.Members = append(result.Members, appmodel.Member{ID: v.Member.ID, Subject: v.Member.Subject, DisplayName: v.DisplayName, Type: string(v.Member.Type), Status: string(v.Member.Status), PrincipalType: v.PrincipalType.String(), AccessVersion: v.Member.AccessVersion, SubjectStatus: string(v.SubjectStatus), SubjectVersion: v.SubjectVersion, Credential: appmodel.ManagedCredential{ID: v.Credential.ID, Version: v.Credential.Version, Kind: v.Credential.Kind, Identifier: v.Credential.Identifier, Status: string(v.Credential.Status)}, UnitIDs: v.UnitIDs, ShopIDs: v.ShopIDs})
	}
	for _, v := range value.Shops {
		result.Shops = append(result.Shops, appmodel.Shop{ID: v.Context.ShopID, MerchantID: v.Context.MerchantID, Name: v.Name, Code: v.Code, Status: string(v.Status), Version: v.Version})
	}
	return result
}

func (l *Logic) shopScope(ctx context.Context) (int64, int64, error) {
	claims := authctx.Caller(ctx)
	if claims.MerchantID <= 0 || claims.ShopID <= 0 {
		return 0, 0, model.ErrInvalidContext
	}
	return claims.MerchantID, claims.ShopID, nil
}

func (l *Logic) privacyOverlay(ctx context.Context, merchantID, shopID int64) (governancemodel.Capability, error) {
	if l.merchantGovernance == nil {
		return governancemodel.Capability{PlatformStatus: governancemodel.PlatformActive}, nil
	}
	page, err := l.merchantGovernance.List(ctx, governancemodel.Query{MerchantID: merchantID, ShopID: shopID, Module: "privacy"})
	if err != nil {
		return governancemodel.Capability{}, err
	}
	if len(page.Items) == 0 {
		return governancemodel.Capability{MerchantID: merchantID, ShopID: shopID, Module: "privacy", PlatformStatus: governancemodel.PlatformActive}, nil
	}
	return page.Items[0], nil
}

func (l *Logic) projectPrivacy(value shopmodel.Privacy, overlay governancemodel.Capability) appmodel.Privacy {
	status := string(overlay.PlatformStatus)
	if status == "" {
		status = string(governancemodel.PlatformActive)
	}
	return appmodel.Privacy{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID,
		CollectConsent: value.CollectConsent, MarketingConsent: value.MarketingConsent, CookieBanner: value.CookieBanner,
		DataRetentionDays: value.DataRetentionDays, ContactEmail: value.ContactEmail, Version: value.Version,
		PlatformStatus: status, PlatformReasonPublic: overlay.PlatformReasonPublic,
		Editable: overlay.PlatformStatus == "" || overlay.PlatformStatus == governancemodel.PlatformActive,
	}
}

func (l *Logic) Privacy(ctx context.Context) (appmodel.Privacy, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.Privacy{}, err
	}
	if l.privacy == nil {
		return appmodel.Privacy{}, model.ErrUnavailable
	}
	value, err := l.privacy.Get(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.Privacy{}, err
	}
	overlay, err := l.privacyOverlay(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.Privacy{}, err
	}
	return l.projectPrivacy(value, overlay), nil
}

func (l *Logic) SavePrivacy(ctx context.Context, input appmodel.SavePrivacy) (appmodel.PrivacyMutation, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.PrivacyMutation{}, err
	}
	if l.privacy == nil {
		return appmodel.PrivacyMutation{}, model.ErrUnavailable
	}
	overlay, err := l.privacyOverlay(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.PrivacyMutation{}, err
	}
	if overlay.PlatformStatus != "" && overlay.PlatformStatus != governancemodel.PlatformActive {
		return appmodel.PrivacyMutation{}, fmt.Errorf("%w: %s", shopmodel.ErrPrivacyRestricted, overlay.PlatformReasonPublic)
	}
	value, replayed, err := l.privacy.Save(ctx, shopmodel.SavePrivacyCommand{
		CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
		Privacy: shopmodel.Privacy{
			MerchantID: merchantID, ShopID: shopID, CollectConsent: input.CollectConsent,
			MarketingConsent: input.MarketingConsent, CookieBanner: input.CookieBanner,
			DataRetentionDays: input.DataRetentionDays, ContactEmail: input.ContactEmail,
		},
	})
	if err != nil {
		return appmodel.PrivacyMutation{}, err
	}
	return appmodel.PrivacyMutation{Privacy: l.projectPrivacy(value, overlay), Replayed: replayed}, nil
}

func (l *Logic) policyShopID(ctx context.Context, requested int64) (int64, int64, error) {
	claims := authctx.Caller(ctx)
	if claims.MerchantID <= 0 {
		return 0, 0, model.ErrInvalidContext
	}
	shopID := requested
	if shopID <= 0 {
		shopID = claims.ShopID
	}
	if shopID <= 0 {
		return 0, 0, shopmodel.ErrPolicyInvalid
	}
	if claims.ShopID > 0 && shopID != claims.ShopID {
		return 0, 0, model.ErrInvalidContext
	}
	return claims.MerchantID, shopID, nil
}

func (l *Logic) policyOverlay(ctx context.Context, merchantID, shopID int64) (governancemodel.Capability, error) {
	if l.merchantGovernance == nil {
		return governancemodel.Capability{PlatformStatus: governancemodel.PlatformActive}, nil
	}
	page, err := l.merchantGovernance.List(ctx, governancemodel.Query{MerchantID: merchantID, ShopID: shopID, Module: "policies"})
	if err != nil {
		return governancemodel.Capability{}, err
	}
	if len(page.Items) == 0 {
		return governancemodel.Capability{MerchantID: merchantID, ShopID: shopID, Module: "policies", PlatformStatus: governancemodel.PlatformActive}, nil
	}
	return page.Items[0], nil
}

func (l *Logic) projectPolicy(value shopmodel.Policy, overlay governancemodel.Capability) appmodel.Policy {
	status := string(overlay.PlatformStatus)
	if status == "" {
		status = string(governancemodel.PlatformActive)
	}
	return appmodel.Policy{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID,
		PolicyType: string(value.PolicyType), Title: value.Title, Content: value.Content,
		VersionNo: value.VersionNo, Status: string(value.Status), Version: value.Version,
		PublishedAt: value.PublishedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		PlatformStatus: status, PlatformReason: overlay.PlatformReasonPublic,
	}
}

func (l *Logic) requirePolicyPublishable(overlay governancemodel.Capability) error {
	if overlay.PlatformStatus != "" && overlay.PlatformStatus != governancemodel.PlatformActive {
		return fmt.Errorf("%w: %s", shopmodel.ErrPolicyRestricted, overlay.PlatformReasonPublic)
	}
	return nil
}

func (l *Logic) PolicyShops(ctx context.Context) ([]appmodel.PolicyShop, error) {
	claims := authctx.Caller(ctx)
	if claims.MerchantID <= 0 {
		return nil, model.ErrInvalidContext
	}
	if l.shops == nil {
		return nil, model.ErrUnavailable
	}
	values, err := l.shops.ListByMerchant(ctx, claims.MerchantID)
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.PolicyShop, 0, len(values))
	for _, value := range values {
		if claims.ShopID > 0 && value.ID != claims.ShopID {
			continue
		}
		out = append(out, appmodel.PolicyShop{
			ShopID: value.ID, MerchantID: value.MerchantID, Name: value.Name, Code: value.Code, Status: string(value.Status),
		})
	}
	return out, nil
}

func (l *Logic) Policies(ctx context.Context, input appmodel.PolicyQuery) (appmodel.PolicyPage, error) {
	merchantID, shopID, err := l.policyShopID(ctx, input.ShopID)
	if err != nil {
		return appmodel.PolicyPage{}, err
	}
	if l.policies == nil {
		return appmodel.PolicyPage{}, model.ErrUnavailable
	}
	page, err := l.policies.List(ctx, shopmodel.PolicyQuery{
		MerchantID: merchantID, ShopID: shopID, PolicyType: shopmodel.PolicyType(input.PolicyType),
		Status: shopmodel.PolicyStatus(input.Status), Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return appmodel.PolicyPage{}, err
	}
	overlay, err := l.policyOverlay(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.PolicyPage{}, err
	}
	out := appmodel.PolicyPage{
		Page: page.Page, PageSize: page.PageSize, Total: page.Total, Items: []appmodel.Policy{},
		PlatformStatus: string(overlay.PlatformStatus), PlatformReason: overlay.PlatformReasonPublic,
	}
	if out.PlatformStatus == "" {
		out.PlatformStatus = string(governancemodel.PlatformActive)
	}
	for _, item := range page.Items {
		out.Items = append(out.Items, l.projectPolicy(item, overlay))
	}
	return out, nil
}

func (l *Logic) SavePolicy(ctx context.Context, input appmodel.SavePolicy) (appmodel.PolicyResult, error) {
	merchantID, shopID, err := l.policyShopID(ctx, input.ShopID)
	if err != nil {
		return appmodel.PolicyResult{}, err
	}
	if l.policies == nil {
		return appmodel.PolicyResult{}, model.ErrUnavailable
	}
	overlay, err := l.policyOverlay(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.PolicyResult{}, err
	}
	if input.Publish {
		if err := l.requirePolicyPublishable(overlay); err != nil {
			return appmodel.PolicyResult{}, err
		}
	}
	value, replayed, err := l.policies.Save(ctx, shopmodel.SavePolicyCommand{
		CommandKey: input.CommandKey, MerchantID: merchantID, ShopID: shopID,
		PolicyType: shopmodel.PolicyType(input.PolicyType), Title: input.Title, Content: input.Content, Publish: input.Publish,
	})
	if err != nil {
		return appmodel.PolicyResult{}, err
	}
	return appmodel.PolicyResult{Policy: l.projectPolicy(value, overlay), Replayed: replayed}, nil
}

func (l *Logic) PublishPolicy(ctx context.Context, input appmodel.PublishPolicy) (appmodel.PolicyResult, error) {
	merchantID, shopID, err := l.policyShopID(ctx, input.ShopID)
	if err != nil {
		return appmodel.PolicyResult{}, err
	}
	if l.policies == nil {
		return appmodel.PolicyResult{}, model.ErrUnavailable
	}
	overlay, err := l.policyOverlay(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.PolicyResult{}, err
	}
	if err := l.requirePolicyPublishable(overlay); err != nil {
		return appmodel.PolicyResult{}, err
	}
	value, replayed, err := l.policies.Publish(ctx, shopmodel.PublishPolicyCommand{
		PolicyID: input.PolicyID, MerchantID: merchantID, ShopID: shopID,
		CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return appmodel.PolicyResult{}, err
	}
	return appmodel.PolicyResult{Policy: l.projectPolicy(value, overlay), Replayed: replayed}, nil
}

func (l *Logic) appShopID(ctx context.Context, requested int64) (int64, int64, error) {
	claims := authctx.Caller(ctx)
	if claims.MerchantID <= 0 {
		return 0, 0, model.ErrInvalidContext
	}
	shopID := requested
	if shopID <= 0 {
		shopID = claims.ShopID
	}
	if shopID <= 0 {
		return 0, 0, shopmodel.ErrAppInvalid
	}
	if claims.ShopID > 0 && shopID != claims.ShopID {
		return 0, 0, model.ErrInvalidContext
	}
	return claims.MerchantID, shopID, nil
}

func (l *Logic) appOverlay(ctx context.Context, merchantID, shopID int64) (governancemodel.Capability, error) {
	if l.merchantGovernance == nil {
		return governancemodel.Capability{PlatformStatus: governancemodel.PlatformActive}, nil
	}
	page, err := l.merchantGovernance.List(ctx, governancemodel.Query{MerchantID: merchantID, ShopID: shopID, Module: "apps"})
	if err != nil {
		return governancemodel.Capability{}, err
	}
	if len(page.Items) == 0 {
		return governancemodel.Capability{MerchantID: merchantID, ShopID: shopID, Module: "apps", PlatformStatus: governancemodel.PlatformActive}, nil
	}
	return page.Items[0], nil
}

func (l *Logic) requireAppWritable(overlay governancemodel.Capability) error {
	if overlay.PlatformStatus != "" && overlay.PlatformStatus != governancemodel.PlatformActive {
		return fmt.Errorf("%w: %s", shopmodel.ErrAppRestricted, overlay.PlatformReasonPublic)
	}
	return nil
}

func (l *Logic) projectApp(value shopmodel.App, overlay governancemodel.Capability) appmodel.App {
	status := string(overlay.PlatformStatus)
	if status == "" {
		status = string(governancemodel.PlatformActive)
	}
	editable := overlay.PlatformStatus == "" || overlay.PlatformStatus == governancemodel.PlatformActive
	return appmodel.App{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID, Name: value.Name, ClientID: value.ClientID,
		SecretHint: value.SecretHint, Scopes: value.Scopes, Status: string(value.Status), Version: value.Version,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, PlatformStatus: status, PlatformReason: overlay.PlatformReasonPublic,
		Editable: editable,
	}
}

func (l *Logic) AppShops(ctx context.Context) ([]appmodel.AppShop, error) {
	values, err := l.PolicyShops(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.AppShop, 0, len(values))
	for _, value := range values {
		out = append(out, appmodel.AppShop{
			ShopID: value.ShopID, MerchantID: value.MerchantID, Name: value.Name, Code: value.Code, Status: value.Status,
		})
	}
	return out, nil
}

func (l *Logic) AppScopes(context.Context) ([]appmodel.AppScope, error) {
	catalog := shopmodel.AppScopeCatalog()
	out := make([]appmodel.AppScope, 0, len(catalog))
	for _, item := range catalog {
		out = append(out, appmodel.AppScope{Code: item.Code, Group: item.Group, Label: item.Label})
	}
	return out, nil
}

func (l *Logic) Apps(ctx context.Context, input appmodel.AppQuery) (appmodel.AppPage, error) {
	merchantID, shopID, err := l.appShopID(ctx, input.ShopID)
	if err != nil {
		return appmodel.AppPage{}, err
	}
	if l.apps == nil {
		return appmodel.AppPage{}, model.ErrUnavailable
	}
	page, err := l.apps.List(ctx, shopmodel.AppQuery{
		MerchantID: merchantID, ShopID: shopID, Status: shopmodel.AppStatus(input.Status),
		Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return appmodel.AppPage{}, err
	}
	overlay, err := l.appOverlay(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.AppPage{}, err
	}
	out := appmodel.AppPage{
		Page: page.Page, PageSize: page.PageSize, Total: page.Total, Items: []appmodel.App{},
		PlatformStatus: string(overlay.PlatformStatus), PlatformReason: overlay.PlatformReasonPublic,
	}
	if out.PlatformStatus == "" {
		out.PlatformStatus = string(governancemodel.PlatformActive)
	}
	for _, item := range page.Items {
		out.Items = append(out.Items, l.projectApp(item, overlay))
	}
	return out, nil
}

func (l *Logic) CreateApp(ctx context.Context, input appmodel.CreateApp) (appmodel.AppResult, error) {
	merchantID, shopID, err := l.appShopID(ctx, input.ShopID)
	if err != nil {
		return appmodel.AppResult{}, err
	}
	if l.apps == nil {
		return appmodel.AppResult{}, model.ErrUnavailable
	}
	overlay, err := l.appOverlay(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.AppResult{}, err
	}
	if err := l.requireAppWritable(overlay); err != nil {
		return appmodel.AppResult{}, err
	}
	value, replayed, err := l.apps.Create(ctx, shopmodel.CreateAppCommand{
		CommandKey: input.CommandKey, MerchantID: merchantID, ShopID: shopID, Name: input.Name, Scopes: input.Scopes,
	})
	if err != nil {
		return appmodel.AppResult{}, err
	}
	return appmodel.AppResult{App: l.projectApp(value.App, overlay), ClientSecret: value.ClientSecret, Replayed: replayed}, nil
}

func (l *Logic) ResetAppSecret(ctx context.Context, input appmodel.ResetAppSecret) (appmodel.AppResult, error) {
	merchantID, shopID, err := l.appShopID(ctx, input.ShopID)
	if err != nil {
		return appmodel.AppResult{}, err
	}
	if l.apps == nil {
		return appmodel.AppResult{}, model.ErrUnavailable
	}
	overlay, err := l.appOverlay(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.AppResult{}, err
	}
	if err := l.requireAppWritable(overlay); err != nil {
		return appmodel.AppResult{}, err
	}
	value, replayed, err := l.apps.ResetSecret(ctx, shopmodel.ResetAppSecretCommand{
		AppID: input.AppID, MerchantID: merchantID, ShopID: shopID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return appmodel.AppResult{}, err
	}
	return appmodel.AppResult{App: l.projectApp(value.App, overlay), ClientSecret: value.ClientSecret, Replayed: replayed}, nil
}

func (l *Logic) SetAppEnabled(ctx context.Context, input appmodel.SetAppEnabled) (appmodel.AppToggleResult, error) {
	merchantID, shopID, err := l.appShopID(ctx, input.ShopID)
	if err != nil {
		return appmodel.AppToggleResult{}, err
	}
	if l.apps == nil {
		return appmodel.AppToggleResult{}, model.ErrUnavailable
	}
	overlay, err := l.appOverlay(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.AppToggleResult{}, err
	}
	if input.Enabled {
		if err := l.requireAppWritable(overlay); err != nil {
			return appmodel.AppToggleResult{}, err
		}
	}
	value, replayed, err := l.apps.SetEnabled(ctx, shopmodel.SetAppEnabledCommand{
		AppID: input.AppID, MerchantID: merchantID, ShopID: shopID, CommandKey: input.CommandKey,
		ExpectedVersion: input.ExpectedVersion, Enabled: input.Enabled,
	})
	if err != nil {
		return appmodel.AppToggleResult{}, err
	}
	return appmodel.AppToggleResult{App: l.projectApp(value, overlay), Replayed: replayed}, nil
}
