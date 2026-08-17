// Package service declares the admin surface application boundary. Every
// transport of this surface depends on it and on nothing below it.
package service

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/appmodel"
)

type Admin interface {
	Health(ctx context.Context) (appmodel.Health, error)
	Directory(ctx context.Context, input appmodel.DirectoryQuery) (appmodel.Directory, error)
	Permissions(context.Context) ([]appmodel.Permission, error)
	Roles(context.Context) ([]appmodel.Role, error)
	PutRole(context.Context, appmodel.PutRole) (appmodel.Role, error)
	PutRolePolicy(context.Context, appmodel.PutRolePolicy) (appmodel.Role, error)
	PutSubjectGrants(context.Context, appmodel.PutSubjectGrants) error
	Users(context.Context) ([]appmodel.ManagedUser, error)
	User(context.Context, string) (appmodel.ManagedUser, error)
	CreateOperator(context.Context, appmodel.CreateOperator) (appmodel.ManagedUser, error)
	ChangeUserStatus(context.Context, appmodel.ChangeUserStatus) (appmodel.ManagedUser, error)
	ResetCredential(context.Context, appmodel.ResetCredential) (appmodel.ManagedCredential, error)
	Sessions(context.Context, string) ([]appmodel.ManagedSession, error)
	RevokeSessions(context.Context, appmodel.RevokeSessions) error
	SubscriptionPlans(context.Context) ([]appmodel.SubscriptionPlan, error)
	SaveSubscriptionPlan(context.Context, appmodel.SaveSubscriptionPlan) (appmodel.SubscriptionPlanResult, error)
	RetireSubscriptionPlan(context.Context, appmodel.RetireSubscriptionPlan) (appmodel.SubscriptionPlanResult, error)
	SubscriptionPermissions(context.Context) ([]appmodel.Permission, error)
	SubscriptionPlanPolicy(context.Context, int64) (appmodel.SubscriptionPlanPolicy, error)
	SaveSubscriptionPlanPolicy(context.Context, appmodel.SaveSubscriptionPlanPolicy) (appmodel.SubscriptionPlanPolicyResult, error)
	ShopMerchants(context.Context) ([]appmodel.ShopMerchant, error)
	MerchantShops(context.Context, int64) ([]appmodel.ManagedShop, error)
	DirectoryShops(context.Context, int64) ([]appmodel.ManagedShop, error)
	ShopCategories(context.Context) ([]appmodel.ShopCategory, error)
	SaveShopCategory(context.Context, appmodel.SaveShopCategory) (appmodel.ShopCategoryResult, error)
	SetShopCategoryEnabled(context.Context, appmodel.SetShopCategoryEnabled) (appmodel.ShopCategoryResult, error)
	RetireShopCategory(context.Context, appmodel.RetireShopCategory) (appmodel.ShopCategoryResult, error)
	CustomerServiceAccounts(context.Context, appmodel.CustomerServiceQuery) (appmodel.CustomerServicePage, error)
	SaveCustomerServiceAccount(context.Context, appmodel.SaveCustomerServiceAccount) (appmodel.CustomerServiceAccountResult, error)
	DeleteCustomerServiceAccount(context.Context, appmodel.DeleteCustomerServiceAccount) (appmodel.CustomerServiceDeleteResult, error)
	GovernanceCatalog() []appmodel.GovernanceModule
	GovernanceCapabilities(context.Context, appmodel.GovernanceQuery) ([]appmodel.GovernanceCapability, error)
	GovernanceAudit(context.Context, appmodel.GovernanceQuery) ([]appmodel.GovernanceAuditItem, error)
	InterveneGovernance(context.Context, appmodel.InterveneGovernance) (appmodel.GovernanceCapabilityResult, error)
	Merchants(context.Context, appmodel.MerchantQuery) (appmodel.MerchantPage, error)
	CreateMerchant(context.Context, appmodel.CreateMerchant) (appmodel.CreateMerchantResult, error)
	UpdateMerchant(context.Context, appmodel.UpdateMerchant) (appmodel.MerchantResult, error)
	ResetMerchantPassword(context.Context, appmodel.ResetMerchantPassword) (bool, error)
	CloseMerchant(context.Context, appmodel.CloseMerchant) (appmodel.MerchantResult, error)
	MerchantSubscription(context.Context, int64) (appmodel.MerchantSubscription, error)
	AssignMerchantSubscription(context.Context, appmodel.AssignMerchantSubscription) (appmodel.MerchantSubscriptionResult, error)
	MerchantPaymentChannels(context.Context, int64, int64) (appmodel.MerchantPaymentChannels, error)
	PutMerchantPaymentChannels(context.Context, appmodel.PutMerchantPaymentChannels) (appmodel.MerchantPaymentChannels, error)
	MerchantSMSRegions(context.Context, int64, int64) (appmodel.MerchantSMSRegions, error)
	PutMerchantSMSRegions(context.Context, appmodel.PutMerchantSMSRegions) (appmodel.MerchantSMSRegions, error)
	MerchantLiveProviders(context.Context, int64) (appmodel.MerchantLiveProviders, error)
	PutMerchantLiveProviders(context.Context, appmodel.PutMerchantLiveProviders) (appmodel.MerchantLiveProviders, error)
}
