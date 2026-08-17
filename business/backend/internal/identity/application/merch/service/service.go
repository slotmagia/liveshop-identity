// Package service declares the merch surface application boundary. Every
// transport of this surface depends on it and on nothing below it.
package service

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
)

type Merch interface {
	Health(ctx context.Context) (appmodel.Health, error)
	Account(ctx context.Context) (appmodel.Account, error)
	Profile(ctx context.Context) (appmodel.Profile, error)
	SaveProfile(ctx context.Context, input appmodel.SaveProfile) (appmodel.ProfileMutation, error)
	AccountSecurity(ctx context.Context) (appmodel.AccountSecurity, error)
	ChangeOwnCredential(ctx context.Context, input appmodel.ChangeOwnCredential) (appmodel.ChangeOwnCredentialResult, error)
	Directory(ctx context.Context) (appmodel.Directory, error)
	CreateUnit(ctx context.Context, input appmodel.CreateUnit) (appmodel.Mutation, error)
	CreateMember(ctx context.Context, input appmodel.CreateMember) (appmodel.Mutation, error)
	UpdateMember(ctx context.Context, input appmodel.UpdateMember) (appmodel.MemberMutation, error)
	ReplaceAccess(ctx context.Context, input appmodel.ReplaceAccess) (appmodel.Mutation, error)
	MemberOptions(ctx context.Context) (appmodel.MemberOptions, error)
	Members(ctx context.Context, query appmodel.MemberQuery) (appmodel.MemberPage, error)
	Member(ctx context.Context, subject string) (appmodel.MemberRecord, error)
	Permissions(context.Context) ([]appmodel.Permission, error)
	Roles(context.Context) ([]appmodel.Role, error)
	PutRole(context.Context, appmodel.PutRole) (appmodel.Role, error)
	PutRolePolicy(context.Context, appmodel.PutRolePolicy) (appmodel.Role, error)
	PutSubjectGrants(context.Context, appmodel.PutSubjectGrants) error
	ResetCredential(context.Context, appmodel.ResetCredential) (appmodel.ManagedCredential, error)
	Sessions(context.Context, string) ([]appmodel.ManagedSession, error)
	RevokeSessions(context.Context, appmodel.RevokeSessions) error
	AccountSessions(context.Context, appmodel.AccountSessionQuery) (appmodel.AccountSessionPage, error)
	RevokeAccountSession(context.Context, appmodel.RevokeAccountSession) (appmodel.RevokeAccountSessionResult, error)
	ChangeMemberStatus(context.Context, appmodel.ChangeMemberStatus) (appmodel.MemberStatusMutation, error)
	Privacy(context.Context) (appmodel.Privacy, error)
	SavePrivacy(context.Context, appmodel.SavePrivacy) (appmodel.PrivacyMutation, error)
	PolicyShops(context.Context) ([]appmodel.PolicyShop, error)
	Policies(context.Context, appmodel.PolicyQuery) (appmodel.PolicyPage, error)
	SavePolicy(context.Context, appmodel.SavePolicy) (appmodel.PolicyResult, error)
	PublishPolicy(context.Context, appmodel.PublishPolicy) (appmodel.PolicyResult, error)
	AppShops(context.Context) ([]appmodel.AppShop, error)
	AppScopes(context.Context) ([]appmodel.AppScope, error)
	Apps(context.Context, appmodel.AppQuery) (appmodel.AppPage, error)
	CreateApp(context.Context, appmodel.CreateApp) (appmodel.AppResult, error)
	ResetAppSecret(context.Context, appmodel.ResetAppSecret) (appmodel.AppResult, error)
	SetAppEnabled(context.Context, appmodel.SetAppEnabled) (appmodel.AppToggleResult, error)
	SubscriptionPlans(context.Context) (appmodel.SubscriptionPlans, error)
	Subscription(context.Context) (appmodel.SubscriptionCurrent, error)
	SubscriptionPayMethods(context.Context, int64) ([]appmodel.PayMethod, error)
	CreateSubscriptionOrder(context.Context, appmodel.CreateSubscriptionOrder) (appmodel.SubscriptionOrder, error)
	SubscriptionOrder(context.Context, string) (appmodel.SubscriptionOrder, error)
	SubscriptionOrders(context.Context, appmodel.SubscriptionOrderQuery) (appmodel.SubscriptionOrderPage, error)
	ConfirmSubscriptionOrder(context.Context, appmodel.ConfirmSubscriptionOrder) (appmodel.SubscriptionOrder, error)
	CloseSubscriptionOrder(context.Context, appmodel.CloseSubscriptionOrder) (appmodel.SubscriptionOrder, error)
	ShopCategories(context.Context) ([]appmodel.ShopCategoryOption, error)
	ManagedShops(context.Context, appmodel.ShopQuery) (appmodel.ShopPage, error)
	CurrentShop(context.Context) (appmodel.CurrentShop, error)
	CreateShop(context.Context, appmodel.CreateShop) (appmodel.ShopMutation, error)
	UpdateShop(context.Context, appmodel.UpdateShop) (appmodel.ShopMutation, error)
	SetShopEnabled(context.Context, appmodel.SetShopEnabled) (appmodel.ShopMutation, error)
	CloseShop(context.Context, appmodel.CloseShop) (appmodel.ShopMutation, error)
	RiskEvents(context.Context, appmodel.RiskEventQuery) (appmodel.RiskEventPage, error)
}
