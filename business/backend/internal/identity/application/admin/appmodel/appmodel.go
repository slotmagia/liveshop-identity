// Package appmodel holds the transport-neutral input and output of the
// admin surface. No transport DTO may appear below this boundary.
package appmodel

import "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"

type Health struct {
	Status model.Status
}
type DirectoryQuery struct{ OrganizationID, MerchantID int64 }
type Directory struct {
	Organization Organization       `json:"organization"`
	Units        []OrganizationUnit `json:"units"`
	Members      []Member           `json:"members"`
	Shops        []Shop             `json:"shops"`
}
type Organization struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Version    uint64 `json:"version"`
}
type OrganizationUnit struct {
	ID       int64  `json:"id"`
	ParentID int64  `json:"parentId"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Version  uint64 `json:"version"`
}
type Member struct {
	ID             int64   `json:"id"`
	OrganizationID int64   `json:"organizationId"`
	MerchantID     int64   `json:"merchantId"`
	Subject        string  `json:"subject"`
	DisplayName    string  `json:"displayName"`
	Type           string  `json:"type"`
	Status         string  `json:"status"`
	PrincipalType  string  `json:"principalType"`
	AccessVersion  uint64  `json:"accessVersion"`
	UnitIDs        []int64 `json:"unitIds"`
	ShopIDs        []int64 `json:"shopIds"`
}
type Shop struct {
	ID         int64  `json:"id"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
	Version    uint64 `json:"version"`
}
type PutRole struct {
	RoleID, ExpectedVersion int64
	Code, Name, Status      string
}
type PutRolePolicy struct {
	RoleID, ExpectedVersion int64
	Permissions             []string
	Scopes                  []ScopeRule
}
type ScopeRule struct {
	Resource     string
	Type         string
	ReferenceIDs []string
}
type Permission struct {
	ModuleID, Code, Name, Resource, Action, Description string
	RegistryRevision                                    uint64
}
type Role struct {
	ID         int64
	Code       string
	Name       string
	Status     string
	SystemRole bool
	Version    int64
}
type PutSubjectGrants struct {
	Subject       string
	RoleIDs       []int64
	OperationID   string
	AccessVersion uint64
}

type ManagedCredential struct {
	ID, Version              uint64
	Kind, Identifier, Status string
}
type ManagedSession struct{ ID, DeviceName, IPAddress, UserAgent, Status, CreatedAt, LastRefreshedAt, ExpiresAt string }
type ManagedUser struct {
	Subject, DisplayName, Realm, PrincipalType, SubjectStatus, MemberType, MemberStatus string
	SubjectVersion, AccessVersion                                                       uint64
	MemberID, OrganizationID                                                            int64
	Credential                                                                          ManagedCredential
	RoleIDs                                                                             []int64
	ActiveSessions                                                                      int
}
type CreateOperator struct {
	IdempotencyKey, OperationID, DisplayName, Username, Password string
	RoleIDs                                                      []int64
}
type ChangeUserStatus struct {
	Subject, IdempotencyKey, OperationID, Status   string
	ExpectedIdentityVersion, ExpectedAccessVersion uint64
}
type ResetCredential struct {
	Subject, IdempotencyKey, OperationID, Password string
	CredentialID, ExpectedCredentialVersion        uint64
}
type RevokeSessions struct{ Subject, SessionID, IdempotencyKey, OperationID string }

type SubscriptionPlan struct {
	ID           int64  `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Level        int    `json:"level"`
	PriceMinor   int64  `json:"priceMinor"`
	DurationDays int    `json:"durationDays"`
	Description  string `json:"description"`
	Default      bool   `json:"default"`
	Sort         int    `json:"sort"`
	Status       string `json:"status"`
	Version      uint64 `json:"version"`
}
type SaveSubscriptionPlan struct {
	CommandKey      string
	ExpectedVersion uint64
	Plan            SubscriptionPlan
}
type RetireSubscriptionPlan struct {
	PlanID          int64
	CommandKey      string
	ExpectedVersion uint64
}
type SubscriptionPlanResult struct {
	Plan     SubscriptionPlan `json:"plan"`
	Replayed bool             `json:"replayed"`
}
type SubscriptionPlanPolicy struct {
	PlanID          int64    `json:"planID"`
	PlanCode        string   `json:"planCode"`
	PlanName        string   `json:"planName"`
	PermissionCodes []string `json:"permissionCodes"`
	ProductLimit    *int64   `json:"productLimit"`
	Revision        uint64   `json:"revision"`
}
type SaveSubscriptionPlanPolicy struct {
	CommandKey       string
	ExpectedRevision uint64
	Policy           SubscriptionPlanPolicy
}
type SubscriptionPlanPolicyResult struct {
	Policy   SubscriptionPlanPolicy `json:"policy"`
	Replayed bool                   `json:"replayed"`
}

type ShopMerchant struct {
	ID         int64  `json:"merchantId"`
	Name       string `json:"name"`
	ExternalID string `json:"externalId"`
	Status     string `json:"status"`
	Version    uint64 `json:"version"`
}

type ManagedShop struct {
	ID            int64  `json:"shopId"`
	MerchantID    int64  `json:"merchantId"`
	Code          string `json:"code"`
	Subdomain     string `json:"subdomain"`
	Name          string `json:"name"`
	DefaultLocale string `json:"defaultLocale"`
	Currency      string `json:"currency"`
	Status        string `json:"status"`
	Version       uint64 `json:"version"`
}

type ShopCategory struct {
	ID            int64  `json:"id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	Icon          string `json:"icon"`
	Sort          int    `json:"sort"`
	Status        string `json:"status"`
	Version       uint64 `json:"version"`
	UsedShopCount int64  `json:"usedShopCount"`
}
type SaveShopCategory struct {
	CommandKey      string
	ExpectedVersion uint64
	Category        ShopCategory
}
type SetShopCategoryEnabled struct {
	CategoryID      int64
	CommandKey      string
	ExpectedVersion uint64
	Enabled         bool
}
type RetireShopCategory struct {
	CategoryID      int64
	CommandKey      string
	ExpectedVersion uint64
}
type ShopCategoryResult struct {
	Category ShopCategory `json:"category"`
	Replayed bool         `json:"replayed"`
}

type CustomerServiceQuery struct {
	MerchantID int64
	ShopID     int64
	Platform   string
	Account    string
	Status     string
	Page       int
	PageSize   int
}
type CustomerServiceAccount struct {
	ID           int64  `json:"id"`
	MerchantID   int64  `json:"merchantId"`
	ShopID       int64  `json:"shopId"`
	Platform     string `json:"platform"`
	Account      string `json:"account"`
	Nickname     string `json:"nickname"`
	Status       string `json:"status"`
	Config       string `json:"config"`
	Remark       string `json:"remark"`
	Version      uint64 `json:"version"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}
type CustomerServicePage struct {
	Items    []CustomerServiceAccount `json:"items"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"pageSize"`
	Total    int64                    `json:"total"`
}
type SaveCustomerServiceAccount struct {
	CommandKey      string
	ExpectedVersion uint64
	Account         CustomerServiceAccount
}
type CustomerServiceAccountResult struct {
	Account  CustomerServiceAccount `json:"account"`
	Replayed bool                   `json:"replayed"`
}
type DeleteCustomerServiceAccount struct {
	AccountID       int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
}
type CustomerServiceDeleteResult struct {
	ID       int64  `json:"id"`
	Deleted  bool   `json:"deleted"`
	Version  uint64 `json:"version"`
	Replayed bool   `json:"replayed"`
}

type GovernanceModule struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}
type GovernanceQuery struct {
	MerchantID int64
	ShopID     int64
	Module     string
}
type GovernanceCapability struct {
	ID                   int64  `json:"id"`
	MerchantID           int64  `json:"merchantId"`
	ShopID               int64  `json:"shopId"`
	Module               string `json:"module"`
	ModuleLabel          string `json:"moduleLabel"`
	Name                 string `json:"name"`
	MerchantStatus       string `json:"merchantStatus"`
	PlatformStatus       string `json:"platformStatus"`
	PlatformReasonPublic string `json:"platformReasonPublic"`
	Version              uint64 `json:"version"`
	UpdatedBy            string `json:"updatedBy"`
	UpdatedAt            string `json:"updatedAt"`
}
type GovernanceAuditItem struct {
	ID             int64  `json:"id"`
	MerchantID     int64  `json:"merchantId"`
	ShopID         int64  `json:"shopId"`
	Module         string `json:"module"`
	CapabilityID   int64  `json:"capabilityId"`
	Action         string `json:"action"`
	Operator       string `json:"operator"`
	ReasonInternal string `json:"reasonInternal"`
	ReasonPublic   string `json:"reasonPublic"`
	CreatedAt      string `json:"createdAt"`
}
type InterveneGovernance struct {
	CommandKey      string
	ExpectedVersion uint64
	MerchantID      int64
	ShopID          int64
	Module          string
	PlatformStatus  string
	ReasonInternal  string
	ReasonPublic    string
}
type GovernanceCapabilityResult struct {
	Capability GovernanceCapability `json:"capability"`
	Replayed   bool                 `json:"replayed"`
}

type MerchantQuery struct {
	Keyword  string
	Status   string
	Page     int
	PageSize int
}
type ManagedMerchant struct {
	ID           int64  `json:"merchantId"`
	Name         string `json:"name"`
	ExternalID   string `json:"externalId"`
	Account      string `json:"account"`
	ContactName  string `json:"contactName"`
	ContactPhone string `json:"contactPhone"`
	Status       string `json:"status"`
	Version      uint64 `json:"version"`
	ShopID       int64  `json:"shopId"`
	ShopCode     string `json:"shopCode"`
}
type MerchantPage struct {
	Items    []ManagedMerchant `json:"items"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
	Total    int64             `json:"total"`
}
type CreateMerchant struct {
	CommandKey   string
	Account      string
	Password     string
	Name         string
	ContactName  string
	ContactPhone string
}
type CreateMerchantResult struct {
	Merchant ManagedMerchant `json:"merchant"`
	ShopID   int64           `json:"shopId"`
	ShopCode string          `json:"shopCode"`
	Account  string          `json:"account"`
	Replayed bool            `json:"replayed"`
}
type UpdateMerchant struct {
	MerchantID      int64
	CommandKey      string
	ExpectedVersion uint64
	Name            string
	Status          string
	ContactName     string
	ContactPhone    string
}
type MerchantResult struct {
	Merchant ManagedMerchant `json:"merchant"`
	Replayed bool            `json:"replayed"`
}
type ResetMerchantPassword struct {
	MerchantID int64
	CommandKey string
	Password   string
}
type CloseMerchant struct {
	MerchantID      int64
	CommandKey      string
	ExpectedVersion uint64
}
type MerchantSubscription struct {
	MerchantID   int64   `json:"merchantId"`
	PlanID       int64   `json:"planId"`
	PlanCode     string  `json:"planCode"`
	PlanName     string  `json:"planName"`
	ExpiresAt    string  `json:"expiresAt"`
	Version      uint64  `json:"version"`
	DurationDays int     `json:"durationDays"`
}
type AssignMerchantSubscription struct {
	MerchantID      int64
	CommandKey      string
	ExpectedVersion uint64
	PlanID          int64
}
type MerchantSubscriptionResult struct {
	Assignment MerchantSubscription `json:"assignment"`
	Replayed   bool                 `json:"replayed"`
}
type MerchantPaymentChannels struct {
	MerchantID int64                  `json:"merchantId"`
	ShopID     int64                  `json:"shopId"`
	Channels   []MerchantPaymentGrant `json:"channels"`
	Version    int64                  `json:"version"`
}
type MerchantPaymentGrant struct {
	ChannelCode string `json:"channelCode"`
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`
}
type PutMerchantPaymentChannels struct {
	MerchantID      int64                  `json:"merchantId"`
	ShopID          int64                  `json:"shopId"`
	CommandKey      string                 `json:"commandKey"`
	ExpectedVersion int64                  `json:"expectedVersion"`
	Channels        []MerchantPaymentGrant `json:"channels"`
}
type MerchantSMSRegionOption struct {
	DialCode string `json:"dialCode"`
	Name     string `json:"name"`
	ISO2     string `json:"iso2"`
	Emoji    string `json:"emoji"`
	Enabled  bool   `json:"enabled"`
}
type MerchantSMSRegions struct {
	MerchantID   int64                      `json:"merchantId"`
	ShopID       int64                      `json:"shopId"`
	DialCodes    []string                   `json:"dialCodes"`
	Unrestricted bool                       `json:"unrestricted"`
	Regions      []MerchantSMSRegionOption  `json:"regions"`
	Version      int64                      `json:"version"`
}
type PutMerchantSMSRegions struct {
	MerchantID      int64    `json:"merchantId"`
	ShopID          int64    `json:"shopId"`
	CommandKey      string   `json:"commandKey"`
	ExpectedVersion int64    `json:"expectedVersion"`
	DialCodes       []string `json:"dialCodes"`
}
type MerchantLiveProviders struct {
	MerchantID int64                    `json:"merchantId"`
	Providers  []MerchantLiveAssignment `json:"providers"`
	Version    int64                    `json:"version"`
}
type MerchantLiveAssignment struct {
	ProviderCode string `json:"providerCode"`
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	Default      bool   `json:"default"`
}
type PutMerchantLiveProviders struct {
	MerchantID      int64                    `json:"merchantId"`
	CommandKey      string                   `json:"commandKey"`
	ExpectedVersion int64                    `json:"expectedVersion"`
	Providers       []MerchantLiveAssignment `json:"providers"`
}
