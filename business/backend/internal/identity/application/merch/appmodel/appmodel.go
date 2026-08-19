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

type DomainShop struct {
	ShopID     int64  `json:"shopId"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
}

type DomainQuery struct {
	ShopID   int64
	Scene    string
	Status   string
	Page     int
	PageSize int
}

type Domain struct {
	ID             int64     `json:"id"`
	MerchantID     int64     `json:"merchantId"`
	ShopID         int64     `json:"shopId"`
	Host           string    `json:"host"`
	Scene          string    `json:"scene"`
	Status         string    `json:"status"`
	IsPrimary      bool      `json:"isPrimary"`
	TxtName        string    `json:"txtName"`
	TxtValue       string    `json:"txtValue"`
	CnameTarget    string    `json:"cnameTarget,omitempty"`
	Version        uint64    `json:"version"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	PlatformStatus string    `json:"platformStatus"`
	PlatformReason string    `json:"platformReasonPublic"`
	Editable       bool      `json:"editable"`
}

type DomainPage struct {
	Items          []Domain `json:"items"`
	Page           int      `json:"page"`
	PageSize       int      `json:"pageSize"`
	Total          int64    `json:"total"`
	CnameTarget    string   `json:"cnameTarget,omitempty"`
	PlatformStatus string   `json:"platformStatus"`
	PlatformReason string   `json:"platformReasonPublic"`
}

type CreateDomain struct {
	CommandKey string
	ShopID     int64
	Host       string
	Scene      string
}

type DomainWrite struct {
	DomainID        int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
	Scene           string
}

type DomainResult struct {
	Domain   Domain `json:"domain"`
	Replayed bool   `json:"replayed"`
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

type ComplaintQuery struct {
	CustomerSubject string
	Status          string
	TargetType      string
	Page            int
	PageSize        int
}

type Complaint struct {
	ID              int64
	CustomerSubject string
	TargetType      string
	TargetID        int64
	ReasonCode      string
	Content         string
	Status          string
	HandleNote      string
	Version         uint64
	CreatedAt       string
	UpdatedAt       string
	HandledAt       *string
}

type ComplaintPage struct {
	Items    []Complaint
	Page     int
	PageSize int
	Total    int64
}

type ReviewComplaint struct {
	ComplaintID     int64
	CommandKey      string
	ExpectedVersion uint64
	Status          string
	HandleNote      string
}

type ComplaintResult struct {
	Complaint Complaint
	Replayed  bool
}

type AftersaleQuery struct {
	CustomerSubject string
	Status          string
	Type            string
	Page            int
	PageSize        int
}

type AftersaleItem struct {
	ID               int64
	SKUID            int64
	Title            string
	Quantity         int64
	RefundAmount     int64
	ReceivedQuantity int64
}

type Aftersale struct {
	ID              int64
	CustomerSubject string
	OrderID         int64
	PaymentNo       string
	Type            string
	RequestedAmount int64
	Amount          int64
	Reason          string
	Status          string
	ReturnStatus    string
	HandleNote      string
	Items           []AftersaleItem
	Version         uint64
	CreatedAt       string
	UpdatedAt       string
	ReviewedAt      *string
	ReceivedAt      *string
}

type AftersalePage struct {
	Items    []Aftersale
	Page     int
	PageSize int
	Total    int64
}

type ReviewAftersale struct {
	AftersaleID     int64
	CommandKey      string
	ExpectedVersion uint64
	Status          string
	Amount          int64
	HandleNote      string
}

type ReceiveAftersale struct {
	AftersaleID     int64
	CommandKey      string
	ExpectedVersion uint64
}

type AftersaleResult struct {
	Aftersale Aftersale
	Replayed  bool
}

type ShipmentQuery struct {
	OrderID  int64
	Status   string
	Page     int
	PageSize int
}

type ShipmentTrace struct {
	OccurredAt string
	Node       string
}

type Shipment struct {
	ID         int64
	OrderID    int64
	Carrier    string
	TrackingNo string
	Status     string
	Traces     []ShipmentTrace
	Version    uint64
	CreatedAt  string
	UpdatedAt  string
}

type ShipmentPage struct {
	Items    []Shipment
	Page     int
	PageSize int
	Total    int64
}

type CreateShipment struct {
	CommandKey string
	OrderID    int64
	Carrier    string
	TrackingNo string
}

type CreateShipmentTrace struct {
	ShipmentID      int64
	CommandKey      string
	ExpectedVersion uint64
	Node            string
}

type CloseShipment struct {
	ShipmentID      int64
	CommandKey      string
	ExpectedVersion uint64
}

type ShipmentResult struct {
	Shipment Shipment
	Replayed bool
}

type ShippingShop struct {
	ShopID     int64
	MerchantID int64
	Name       string
	Code       string
	Status     string
}

type ShippingQuery struct {
	ShopID   int64
	Status   string
	Page     int
	PageSize int
}

type ShippingRule struct {
	ID                   int64
	MerchantID           int64
	ShopID               int64
	Name                 string
	Regions              string
	FeeFen               int64
	FreeOverFen          int64
	MinDays              int
	MaxDays              int
	SortOrder            int
	Status               string
	Version              uint64
	CreatedAt            string
	UpdatedAt            string
	PlatformStatus       string
	PlatformReasonPublic string
	Editable             bool
}

type ShippingRulePage struct {
	Items                []ShippingRule
	Page                 int
	PageSize             int
	Total                int64
	PlatformStatus       string
	PlatformReasonPublic string
}

type SaveShippingRule struct {
	ID              int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
	Name            string
	Regions         string
	FeeFen          int64
	FreeOverFen     int64
	MinDays         int
	MaxDays         int
	SortOrder       int
	Status          string
}

type RetireShipping struct {
	ID              int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
}

type ShippingRuleResult struct {
	Rule     ShippingRule
	Replayed bool
}

type ShippingRegion struct {
	RegionCode      string
	RegionName      string
	CountryCode     string
	CountryName     string
	SubdivisionCode string
	SubdivisionName string
}

type ShippingRate struct {
	ID          int64
	Name        string
	IsFree      bool
	PriceFen    int64
	TransitType string
	MinDays     int
	MaxDays     int
	SortOrder   int
	Status      string
}

type ShippingZone struct {
	ID        int64
	Name      string
	SortOrder int
	Regions   []ShippingRegion
	Rates     []ShippingRate
}

type ShippingPreset struct {
	ID                    int64
	MerchantID            int64
	ShopID                int64
	Name                  string
	IsDefault             bool
	ProductScope          string
	ProductIDs            []int64
	OriginName            string
	OriginRegionCode      string
	OriginRegionName      string
	OriginCountryCode     string
	OriginCountryName     string
	OriginSubdivisionCode string
	OriginSubdivisionName string
	Status                string
	Zones                 []ShippingZone
	Version               uint64
	CreatedAt             string
	UpdatedAt             string
	PlatformStatus        string
	PlatformReasonPublic  string
	Editable              bool
}

type ShippingPresetPage struct {
	Items                []ShippingPreset
	Page                 int
	PageSize             int
	Total                int64
	PlatformStatus       string
	PlatformReasonPublic string
}

type SaveShippingPreset struct {
	ID                    int64
	ShopID                int64
	CommandKey            string
	ExpectedVersion       uint64
	Name                  string
	IsDefault             bool
	ProductScope          string
	ProductIDs            []int64
	OriginName            string
	OriginRegionCode      string
	OriginRegionName      string
	OriginCountryCode     string
	OriginCountryName     string
	OriginSubdivisionCode string
	OriginSubdivisionName string
	Status                string
	Zones                 []ShippingZone
}

type SetShippingPresetEnabled struct {
	PresetID        int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
	Enabled         bool
}

type ShippingPresetResult struct {
	Preset   ShippingPreset
	Replayed bool
}

type CustomerAccountShop struct {
	ShopID     int64  `json:"shopId"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
}

type CustomerAccountQuery struct {
	ShopID   int64
	Platform string
	Account  string
	Status   string
	Page     int
	PageSize int
}

type CustomerAccount struct {
	ID         int64  `json:"id"`
	MerchantID int64  `json:"merchantId"`
	ShopID     int64  `json:"shopId"`
	Platform   string `json:"platform"`
	Account    string `json:"account"`
	Nickname   string `json:"nickname"`
	Status     string `json:"status"`
	Config     string `json:"config"`
	Remark     string `json:"remark"`
	Version    uint64 `json:"version"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type CustomerAccountPage struct {
	Items    []CustomerAccount `json:"items"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
	Total    int64             `json:"total"`
}

type SaveCustomerAccount struct {
	CommandKey      string
	ExpectedVersion uint64
	ShopID          int64
	ID              int64
	Platform        string
	Account         string
	Nickname        string
	Status          string
	Config          string
	Remark          string
}

type CustomerAccountResult struct {
	Account  CustomerAccount
	Replayed bool
}

type DeleteCustomerAccount struct {
	AccountID       int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
}

type CustomerAccountDeleteResult struct {
	ID       int64
	Deleted  bool
	Version  uint64
	Replayed bool
}

type LanguageItem struct {
	Locale            string `json:"locale"`
	Label             string `json:"label"`
	Published         bool   `json:"published"`
	IsDefault         bool   `json:"isDefault"`
	SortOrder         int    `json:"sortOrder"`
	CompletionPercent int    `json:"completionPercent"`
	PlatformStatus    string `json:"platformStatus"`
}

type Languages struct {
	DefaultLocale string         `json:"defaultLocale"`
	Version       uint64         `json:"version"`
	Items         []LanguageItem `json:"items"`
}

type UpdateLanguages struct {
	CommandKey       string
	ExpectedVersion  uint64
	DefaultLocale    string
	PublishedLocales []string
}

type LanguagesMutation struct {
	Languages Languages
	Replayed  bool
}
