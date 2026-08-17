package account

import "github.com/gogf/gf/v2/frame/g"

type Shop struct {
	ID         int64  `json:"id"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
	Version    uint64 `json:"version"`
}

type Merchant struct {
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Status     string `json:"status"`
}

type Organization struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	UnitCount   int    `json:"unitCount"`
	MemberCount int    `json:"memberCount"`
	ShopCount   int    `json:"shopCount"`
}

type Subscription struct {
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

type GetReq struct {
	g.Meta `path:"/account" method:"get" tags:"Identity-merch"`
}

type GetRes struct {
	Subject         string       `json:"subject"`
	DisplayName     string       `json:"displayName"`
	Account         string       `json:"account"`
	PrincipalType   string       `json:"principalType"`
	Owner           bool         `json:"owner"`
	Status          string       `json:"status"`
	Merchant        Merchant     `json:"merchant"`
	CurrentShopID   int64        `json:"currentShopId"`
	Shops           []Shop       `json:"shops"`
	Subscription    Subscription `json:"subscription"`
	PermissionNames []string     `json:"permissionNames"`
	Organization    Organization `json:"organization"`
}

type Session struct {
	ID              string `json:"id"`
	DeviceName      string `json:"deviceName"`
	IPAddress       string `json:"ipAddress"`
	UserAgent       string `json:"userAgent"`
	Status          string `json:"status"`
	CreatedAt       string `json:"createdAt"`
	LastRefreshedAt string `json:"lastRefreshedAt"`
	ExpiresAt       string `json:"expiresAt"`
	Current         bool   `json:"current"`
}

type SessionsReq struct {
	g.Meta   `path:"/account/sessions" method:"get" tags:"Identity-merch"`
	Status   string `json:"status" in:"query"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"pageSize" in:"query" d:"20"`
}

type SessionsRes struct {
	Items    []Session `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
	Total    int64     `json:"total"`
}

type RevokeSessionReq struct {
	g.Meta         `path:"/account/sessions/{sessionId}/revoke" method:"post" tags:"Identity-merch"`
	SessionID      string `json:"sessionId" in:"path"`
	IdempotencyKey string `json:"idempotencyKey"`
	OperationID    string `json:"operationId"`
}

type RevokeSessionRes struct {
	CurrentRevoked bool `json:"currentRevoked"`
}

type Credential struct {
	ID         uint64 `json:"id"`
	Version    uint64 `json:"version"`
	Kind       string `json:"kind"`
	Identifier string `json:"identifier"`
	Status     string `json:"status"`
}

type SecurityGetReq struct {
	g.Meta `path:"/account/security" method:"get" tags:"Identity-merch"`
}

type SecurityGetRes struct {
	Subject        string     `json:"subject"`
	DisplayName    string     `json:"displayName"`
	Account        string     `json:"account"`
	PrincipalType  string     `json:"principalType"`
	Owner          bool       `json:"owner"`
	Status         string     `json:"status"`
	Credential     Credential `json:"credential"`
	ActiveSessions int        `json:"activeSessions"`
}

type CredentialsUpdateReq struct {
	g.Meta          `path:"/account/credentials" method:"put" tags:"Identity-merch"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	OldPassword     string `json:"oldPassword"`
	Password        string `json:"password"`
}

type CredentialsUpdateRes struct {
	Credential      Credential `json:"credential"`
	RevokedSessions int64      `json:"revokedSessions"`
	CurrentRetained bool       `json:"currentRetained"`
	Replayed        bool       `json:"replayed"`
}
