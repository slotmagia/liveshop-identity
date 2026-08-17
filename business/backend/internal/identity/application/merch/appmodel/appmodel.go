// Package appmodel holds the transport-neutral input and output of the
// merch surface. No transport DTO may appear below this boundary.
package appmodel

import (
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

type Health struct {
	Status model.Status
}

type Directory struct {
	Organization Organization       `json:"organization"`
	Units        []OrganizationUnit `json:"units"`
	Members      []Member           `json:"members"`
	Shops        []Shop             `json:"shops"`
}
type Organization struct {
	ID         int64                  `json:"id"`
	Type       model.OrganizationType `json:"type"`
	MerchantID int64                  `json:"merchantId"`
	Name       string                 `json:"name"`
	Status     model.Status           `json:"status"`
	Version    uint64                 `json:"version"`
}
type OrganizationUnit struct {
	ID       int64  `json:"id"`
	ParentID int64  `json:"parentId"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Version  uint64 `json:"version"`
}
type Member struct {
	ID             int64             `json:"id"`
	Subject        string            `json:"subject"`
	DisplayName    string            `json:"displayName"`
	Type           string            `json:"type"`
	Status         string            `json:"status"`
	PrincipalType  string            `json:"principalType"`
	AccessVersion  uint64            `json:"accessVersion"`
	SubjectStatus  string            `json:"subjectStatus"`
	SubjectVersion uint64            `json:"subjectVersion"`
	Credential     ManagedCredential `json:"credential"`
	UnitIDs        []int64           `json:"unitIds"`
	ShopIDs        []int64           `json:"shopIds"`
}
type Shop struct {
	ID         int64  `json:"id"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
	Version    uint64 `json:"version"`
}

type Account struct {
	Subject         string              `json:"subject"`
	DisplayName     string              `json:"displayName"`
	Account         string              `json:"account"`
	PrincipalType   string              `json:"principalType"`
	Owner           bool                `json:"owner"`
	Status          string              `json:"status"`
	Merchant        AccountMerchant     `json:"merchant"`
	CurrentShopID   int64               `json:"currentShopId"`
	Shops           []Shop              `json:"shops"`
	Subscription    SubscriptionCurrent `json:"subscription"`
	PermissionNames []string            `json:"permissionNames"`
	Organization    AccountOrganization `json:"organization"`
}

type AccountMerchant struct {
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Status     string `json:"status"`
}

type AccountOrganization struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	UnitCount   int    `json:"unitCount"`
	MemberCount int    `json:"memberCount"`
	ShopCount   int    `json:"shopCount"`
}

type AccountSecurity struct {
	Subject        string            `json:"subject"`
	DisplayName    string            `json:"displayName"`
	Account        string            `json:"account"`
	PrincipalType  string            `json:"principalType"`
	Owner          bool              `json:"owner"`
	Status         string            `json:"status"`
	Credential     ManagedCredential `json:"credential"`
	ActiveSessions int               `json:"activeSessions"`
}

type ChangeOwnCredential struct {
	CommandKey      string
	ExpectedVersion uint64
	OldPassword     string
	Password        string
}

type ChangeOwnCredentialResult struct {
	Credential      ManagedCredential `json:"credential"`
	RevokedSessions int64             `json:"revokedSessions"`
	CurrentRetained bool              `json:"currentRetained"`
	Replayed        bool              `json:"replayed"`
}
type CreateUnit struct {
	IdempotencyKey   string
	UnitID, ParentID int64
	Name             string
	ExpectedVersion  uint64
}
type CreateMember struct {
	IdempotencyKey, OperationID, DisplayName, MemberType, Username, Password string
	UnitIDs, ShopIDs, RoleIDs                                                []int64
}
type UpdateMember struct {
	Subject, IdempotencyKey, OperationID, DisplayName, MemberType string
	ExpectedIdentityVersion, ExpectedAccessVersion                uint64
	UnitIDs, ShopIDs, RoleIDs                                     []int64
}
type ReplaceAccess struct {
	IdempotencyKey, OperationID string
	MemberID                    int64
	ExpectedAccessVersion       uint64
	UnitIDs, ShopIDs            []int64
	MemberType                  string
}
type Mutation struct {
	MemberID                     int64
	Subject, Status, OperationID string
	Version                      uint64
}
type MemberRecord struct {
	ID             int64             `json:"id"`
	Subject        string            `json:"subject"`
	DisplayName    string            `json:"displayName"`
	Type           string            `json:"type"`
	Status         string            `json:"status"`
	MemberStatus   string            `json:"memberStatus"`
	PrincipalType  string            `json:"principalType"`
	AccessVersion  uint64            `json:"accessVersion"`
	SubjectVersion uint64            `json:"subjectVersion"`
	Credential     ManagedCredential `json:"credential"`
	RoleIDs        []int64           `json:"roleIds"`
	UnitIDs        []int64           `json:"unitIds"`
	ShopIDs        []int64           `json:"shopIds"`
	ActiveSessions int               `json:"activeSessions"`
}
type MemberQuery struct {
	Keyword    string
	MemberType string
	Status     string
	ShopID     int64
	Page       int
	PageSize   int
}
type MemberPage struct {
	Items    []MemberRecord `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Total    int64          `json:"total"`
}
type MemberOptions struct {
	Shops []Shop             `json:"shops"`
	Roles []Role             `json:"roles"`
	Units []OrganizationUnit `json:"units"`
}
type MemberMutation struct {
	MemberID        int64  `json:"memberId"`
	Subject         string `json:"subject"`
	Status          string `json:"status"`
	OperationID     string `json:"operationId"`
	IdentityVersion uint64 `json:"identityVersion"`
	AccessVersion   uint64 `json:"accessVersion"`
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
type AccountSession struct {
	ID, DeviceName, IPAddress, UserAgent, Status, CreatedAt, LastRefreshedAt, ExpiresAt string
	Current                                                                             bool
}
type AccountSessionQuery struct {
	Status   string
	Page     int
	PageSize int
}
type AccountSessionPage struct {
	Items    []AccountSession
	Page     int
	PageSize int
	Total    int64
}
type RevokeAccountSession struct {
	SessionID, IdempotencyKey, OperationID string
}
type RevokeAccountSessionResult struct {
	CurrentRevoked bool
}
type ResetCredential struct {
	Subject, IdempotencyKey, OperationID, Password string
	CredentialID, ExpectedCredentialVersion        uint64
}
type RevokeSessions struct{ Subject, SessionID, IdempotencyKey, OperationID string }
type ChangeMemberStatus struct {
	Subject, IdempotencyKey, OperationID, Status   string
	ExpectedIdentityVersion, ExpectedAccessVersion uint64
}
type MemberStatusMutation struct {
	Subject, Status                string
	IdentityVersion, AccessVersion uint64
}

type PolicyShop struct {
	ShopID     int64  `json:"shopId"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
}

type PolicyQuery struct {
	ShopID     int64
	PolicyType string
	Status     string
	Page       int
	PageSize   int
}

type Policy struct {
	ID             int64      `json:"id"`
	MerchantID     int64      `json:"merchantId"`
	ShopID         int64      `json:"shopId"`
	PolicyType     string     `json:"policyType"`
	Title          string     `json:"title"`
	Content        string     `json:"content"`
	VersionNo      int        `json:"versionNo"`
	Status         string     `json:"status"`
	Version        uint64     `json:"version"`
	PublishedAt    *time.Time `json:"publishedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	PlatformStatus string     `json:"platformStatus"`
	PlatformReason string     `json:"platformReasonPublic"`
}

type PolicyPage struct {
	Items          []Policy `json:"items"`
	Page           int      `json:"page"`
	PageSize       int      `json:"pageSize"`
	Total          int64    `json:"total"`
	PlatformStatus string   `json:"platformStatus"`
	PlatformReason string   `json:"platformReasonPublic"`
}

type SavePolicy struct {
	CommandKey string
	ShopID     int64
	PolicyType string
	Title      string
	Content    string
	Publish    bool
}

type PublishPolicy struct {
	PolicyID        int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
}

type PolicyResult struct {
	Policy   Policy `json:"policy"`
	Replayed bool   `json:"replayed"`
}

type Privacy struct {
	ID                   int64  `json:"id,omitempty"`
	MerchantID           int64  `json:"merchantId"`
	ShopID               int64  `json:"shopId"`
	CollectConsent       bool   `json:"collectConsent"`
	MarketingConsent     bool   `json:"marketingConsent"`
	CookieBanner         bool   `json:"cookieBanner"`
	DataRetentionDays    int    `json:"dataRetentionDays"`
	ContactEmail         string `json:"contactEmail"`
	Version              uint64 `json:"version"`
	PlatformStatus       string `json:"platformStatus"`
	PlatformReasonPublic string `json:"platformReasonPublic,omitempty"`
	Editable             bool   `json:"editable"`
}

type SavePrivacy struct {
	CommandKey        string
	ExpectedVersion   uint64
	CollectConsent    bool
	MarketingConsent  bool
	CookieBanner      bool
	DataRetentionDays int
	ContactEmail      string
}

type PrivacyMutation struct {
	Privacy  Privacy
	Replayed bool
}

type AppShop struct {
	ShopID     int64  `json:"shopId"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
}

type AppScope struct {
	Code  string `json:"code"`
	Group string `json:"group"`
	Label string `json:"label"`
}

type AppQuery struct {
	ShopID   int64
	Status   string
	Page     int
	PageSize int
}

type App struct {
	ID             int64     `json:"id"`
	MerchantID     int64     `json:"merchantId"`
	ShopID         int64     `json:"shopId"`
	Name           string    `json:"name"`
	ClientID       string    `json:"clientId"`
	SecretHint     string    `json:"secretHint"`
	Scopes         string    `json:"scopes"`
	Status         string    `json:"status"`
	Version        uint64    `json:"version"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	PlatformStatus string    `json:"platformStatus"`
	PlatformReason string    `json:"platformReasonPublic"`
	Editable       bool      `json:"editable"`
}

type AppPage struct {
	Items          []App  `json:"items"`
	Page           int    `json:"page"`
	PageSize       int    `json:"pageSize"`
	Total          int64  `json:"total"`
	PlatformStatus string `json:"platformStatus"`
	PlatformReason string `json:"platformReasonPublic"`
}

type CreateApp struct {
	CommandKey string
	ShopID     int64
	Name       string
	Scopes     string
}

type ResetAppSecret struct {
	AppID           int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
}

type SetAppEnabled struct {
	AppID           int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
	Enabled         bool
}

type AppResult struct {
	App          App    `json:"app"`
	ClientSecret string `json:"clientSecret,omitempty"`
	Replayed     bool   `json:"replayed"`
}

type AppToggleResult struct {
	App      App  `json:"app"`
	Replayed bool `json:"replayed"`
}

type SubscriptionPlan struct {
	ID              int64    `json:"id"`
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	Level           int      `json:"level"`
	PriceMinor      int64    `json:"priceMinor"`
	DurationDays    int      `json:"durationDays"`
	Description     string   `json:"description"`
	Default         bool     `json:"default"`
	Current         bool     `json:"current"`
	Buyable         bool     `json:"buyable"`
	ProductLimit    *int64   `json:"productLimit"`
	PermissionNames []string `json:"permissionNames"`
}

type SubscriptionPlans struct {
	Items         []SubscriptionPlan `json:"items"`
	CurrentPlanID int64              `json:"currentPlanId"`
}

type SubscriptionCurrent struct {
	MerchantID      int64    `json:"merchantId"`
	PlanID          int64    `json:"planId"`
	PlanCode        string   `json:"planCode"`
	PlanName        string   `json:"planName"`
	ExpiresAt       string   `json:"expiresAt"`
	Version         uint64   `json:"version"`
	ProductLimit    *int64   `json:"productLimit"`
	QuotaConfigured bool     `json:"quotaConfigured"`
	PermissionNames []string `json:"permissionNames"`
}

type PayMethod struct {
	ChannelCode string `json:"channelCode"`
	DisplayName string `json:"displayName"`
	TypeCode    string `json:"typeCode"`
	DriverKey   string `json:"driverKey"`
}

type ChargePayment struct {
	OrderNo     string
	AmountMinor int64
	ChannelCode string
}

type ChargeResult struct {
	PayNo     string
	DriverKey string
	PayStatus string
	Paid      bool
	Payload   map[string]string
}

type PaymentStatus struct {
	PayNo     string
	Status    string
	Paid      bool
	DriverKey string
	Payload   map[string]string
}

type CreateSubscriptionOrder struct {
	CommandKey  string
	PlanID      int64
	ChannelCode string
}

type SubscriptionOrder struct {
	OrderNo      string            `json:"orderNo"`
	PlanID       int64             `json:"planId"`
	PlanCode     string            `json:"planCode"`
	PlanName     string            `json:"planName"`
	PriceMinor   int64             `json:"priceMinor"`
	DurationDays int               `json:"durationDays"`
	Status       string            `json:"status"`
	PayNo        string            `json:"payNo"`
	ChannelCode  string            `json:"channelCode"`
	DriverKey    string            `json:"driverKey"`
	PayStatus    string            `json:"payStatus"`
	Payload      map[string]string `json:"payload"`
	Activated    bool              `json:"activated"`
	ExpiresAt    string            `json:"expiresAt"`
	CreatedAt    string            `json:"createdAt"`
	PaidAt       string            `json:"paidAt"`
	Replayed     bool              `json:"replayed"`
}

type ConfirmSubscriptionOrder struct {
	CommandKey string
	OrderNo    string
}

type SubscriptionOrderQuery struct {
	Status   string
	Page     int
	PageSize int
}

type SubscriptionOrderPage struct {
	Items    []SubscriptionOrder `json:"items"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
	Total    int64               `json:"total"`
	Owner    bool                `json:"owner"`
}

type CloseSubscriptionOrder struct {
	CommandKey string
	OrderNo    string
}

type Profile struct {
	MerchantID          int64  `json:"merchantId"`
	Name                string `json:"name"`
	Account             string `json:"account"`
	ExternalID          string `json:"externalId"`
	ContactName         string `json:"contactName"`
	ContactPhone        string `json:"contactPhone"`
	MarketingEmailOptIn bool   `json:"marketingEmailOptIn"`
	MarketingSMSOptIn   bool   `json:"marketingSmsOptIn"`
	Status              string `json:"status"`
	Version             uint64 `json:"version"`
	Owner               bool   `json:"owner"`
}

type SaveProfile struct {
	CommandKey          string
	ExpectedVersion     uint64
	ExternalID          string
	ContactName         string
	ContactPhone        string
	MarketingEmailOptIn bool
	MarketingSMSOptIn   bool
}

type ProfileMutation struct {
	Profile  Profile
	Replayed bool
}

type ManagedShop struct {
	ShopID        int64  `json:"shopId"`
	MerchantID    int64  `json:"merchantId"`
	Code          string `json:"code"`
	Subdomain     string `json:"subdomain"`
	Name          string `json:"name"`
	DefaultLocale string `json:"defaultLocale"`
	Currency      string `json:"currency"`
	CategoryCode  string `json:"categoryCode"`
	Status        string `json:"status"`
	Version       uint64 `json:"version"`
}

type ShopCategoryOption struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type ShopQuery struct {
	Keyword  string
	Status   string
	Page     int
	PageSize int
}

type ShopPage struct {
	Items    []ManagedShop
	Page     int
	PageSize int
	Total    int64
	Owner    bool
}

type CurrentShop struct {
	Shop  ManagedShop
	Owner bool
}

type CreateShop struct {
	CommandKey   string
	Name         string
	Subdomain    string
	Currency     string
	CategoryCode string
	Status       string
}

type UpdateShop struct {
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
	Name            string
	Subdomain       string
}

type SetShopEnabled struct {
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
	Enabled         bool
}

type CloseShop struct {
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
}

type ShopMutation struct {
	Shop     ManagedShop
	Replayed bool
}

type RiskEventQuery struct {
	VisitorID     string
	RoomID        int64
	Reason        string
	VisitorStatus string
	Page          int
	PageSize      int
}

type RiskEvent struct {
	ID              int64
	VisitorID       string
	Nickname        string
	RoomID          int64
	Reason          string
	ScoreBefore     int
	ScoreAfterDecay int
	ScoreDelta      int
	ScoreAfter      int
	CurrentScore    int
	CurrentLevel    string
	VisitorStatus   string
	CreatedAt       string
}

type RiskEventPage struct {
	Items    []RiskEvent
	Page     int
	PageSize int
	Total    int64
}
