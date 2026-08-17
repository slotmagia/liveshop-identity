package merchant

import "github.com/gogf/gf/v2/frame/g"

type Merchant struct {
	MerchantID   int64  `json:"merchantId"`
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

type ListReq struct {
	g.Meta   `path:"/merchants" method:"get" tags:"Identity-Merchant"`
	Keyword  string `json:"keyword" in:"query"`
	Status   string `json:"status" in:"query"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"pageSize" in:"query" d:"20"`
}
type ListRes struct {
	Items    []Merchant `json:"items"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
	Total    int64      `json:"total"`
}

type CreateReq struct {
	g.Meta       `path:"/merchants" method:"post" tags:"Identity-Merchant"`
	CommandKey   string `json:"commandKey"`
	Account      string `json:"account"`
	Password     string `json:"password"`
	Name         string `json:"name"`
	ContactName  string `json:"contactName"`
	ContactPhone string `json:"contactPhone"`
}
type CreateRes struct {
	Merchant Merchant `json:"merchant"`
	ShopID   int64    `json:"shopId"`
	ShopCode string   `json:"shopCode"`
	Account  string   `json:"account"`
	Replayed bool     `json:"replayed"`
}

type UpdateReq struct {
	g.Meta          `path:"/merchants/{merchantId}" method:"put" tags:"Identity-Merchant"`
	MerchantID      int64  `json:"merchantId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	ContactName     string `json:"contactName"`
	ContactPhone    string `json:"contactPhone"`
}
type UpdateRes struct {
	Merchant Merchant `json:"merchant"`
	Replayed bool     `json:"replayed"`
}

type ResetPasswordReq struct {
	g.Meta     `path:"/merchants/{merchantId}/credentials/reset" method:"post" tags:"Identity-Merchant"`
	MerchantID int64  `json:"merchantId" in:"path"`
	CommandKey string `json:"commandKey"`
	Password   string `json:"password"`
}
type ResetPasswordRes struct {
	Replayed bool `json:"replayed"`
}

type CloseReq struct {
	g.Meta          `path:"/merchants/{merchantId}/close" method:"post" tags:"Identity-Merchant"`
	MerchantID      int64  `json:"merchantId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type CloseRes UpdateRes

type ListShopsReq struct {
	g.Meta     `path:"/merchants/{merchantId}/shops" method:"get" tags:"Identity-Merchant"`
	MerchantID int64 `json:"merchantId" in:"path"`
}
type Shop struct {
	ShopID     int64  `json:"shopId"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
}
type ListShopsRes []Shop

type GetSubscriptionReq struct {
	g.Meta     `path:"/merchants/{merchantId}/subscription" method:"get" tags:"Identity-Merchant"`
	MerchantID int64 `json:"merchantId" in:"path"`
}
type Subscription struct {
	MerchantID   int64  `json:"merchantId"`
	PlanID       int64  `json:"planId"`
	PlanCode     string `json:"planCode"`
	PlanName     string `json:"planName"`
	ExpiresAt    string `json:"expiresAt"`
	Version      uint64 `json:"version"`
	DurationDays int    `json:"durationDays"`
}
type GetSubscriptionRes Subscription

type PutSubscriptionReq struct {
	g.Meta          `path:"/merchants/{merchantId}/subscription" method:"put" tags:"Identity-Merchant"`
	MerchantID      int64  `json:"merchantId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	PlanID          int64  `json:"planId"`
}
type PutSubscriptionRes struct {
	Assignment Subscription `json:"assignment"`
	Replayed   bool         `json:"replayed"`
}

type ListPlansReq struct {
	g.Meta `path:"/merchants/subscription-plans" method:"get" tags:"Identity-Merchant"`
}
type Plan struct {
	ID           int64  `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	DurationDays int    `json:"durationDays"`
	Status       string `json:"status"`
	Default      bool   `json:"default"`
}
type ListPlansRes []Plan

type GetPaymentChannelsReq struct {
	g.Meta     `path:"/merchants/{merchantId}/payment-channels" method:"get" tags:"Identity-Merchant"`
	MerchantID int64 `json:"merchantId" in:"path"`
	ShopID     int64 `json:"shopId" in:"query"`
}
type PaymentGrant struct {
	ChannelCode string `json:"channelCode"`
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`
}
type PaymentChannels struct {
	MerchantID int64          `json:"merchantId"`
	ShopID     int64          `json:"shopId"`
	Channels   []PaymentGrant `json:"channels"`
	Version    int64          `json:"version"`
}
type GetPaymentChannelsRes PaymentChannels
type PutPaymentChannelsReq struct {
	g.Meta          `path:"/merchants/{merchantId}/payment-channels" method:"put" tags:"Identity-Merchant"`
	MerchantID      int64          `json:"merchantId" in:"path"`
	ShopID          int64          `json:"shopId"`
	CommandKey      string         `json:"commandKey"`
	ExpectedVersion int64          `json:"expectedVersion"`
	Channels        []PaymentGrant `json:"channels"`
}
type PutPaymentChannelsRes PaymentChannels

type GetSMSRegionsReq struct {
	g.Meta     `path:"/merchants/{merchantId}/sms-regions" method:"get" tags:"Identity-Merchant"`
	MerchantID int64 `json:"merchantId" in:"path"`
	ShopID     int64 `json:"shopId" in:"query"`
}
type SMSRegionOption struct {
	DialCode string `json:"dialCode"`
	Name     string `json:"name"`
	ISO2     string `json:"iso2"`
	Emoji    string `json:"emoji"`
	Enabled  bool   `json:"enabled"`
}
type SMSRegions struct {
	MerchantID   int64              `json:"merchantId"`
	ShopID       int64              `json:"shopId"`
	DialCodes    []string           `json:"dialCodes"`
	Unrestricted bool               `json:"unrestricted"`
	Regions      []SMSRegionOption  `json:"regions"`
	Version      int64              `json:"version"`
}
type GetSMSRegionsRes SMSRegions
type PutSMSRegionsReq struct {
	g.Meta          `path:"/merchants/{merchantId}/sms-regions" method:"put" tags:"Identity-Merchant"`
	MerchantID      int64    `json:"merchantId" in:"path"`
	ShopID          int64    `json:"shopId"`
	CommandKey      string   `json:"commandKey"`
	ExpectedVersion int64    `json:"expectedVersion"`
	DialCodes       []string `json:"dialCodes"`
}
type PutSMSRegionsRes SMSRegions

type GetLiveProvidersReq struct {
	g.Meta     `path:"/merchants/{merchantId}/live-providers" method:"get" tags:"Identity-Merchant"`
	MerchantID int64 `json:"merchantId" in:"path"`
}
type LiveAssignment struct {
	ProviderCode string `json:"providerCode"`
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	Default      bool   `json:"default"`
}
type LiveProviders struct {
	MerchantID int64            `json:"merchantId"`
	Providers  []LiveAssignment `json:"providers"`
	Version    int64            `json:"version"`
}
type GetLiveProvidersRes LiveProviders
type PutLiveProvidersReq struct {
	g.Meta          `path:"/merchants/{merchantId}/live-providers" method:"put" tags:"Identity-Merchant"`
	MerchantID      int64            `json:"merchantId" in:"path"`
	CommandKey      string           `json:"commandKey"`
	ExpectedVersion int64            `json:"expectedVersion"`
	Providers       []LiveAssignment `json:"providers"`
}
type PutLiveProvidersRes LiveProviders
