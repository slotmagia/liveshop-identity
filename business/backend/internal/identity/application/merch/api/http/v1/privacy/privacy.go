package privacy

import "github.com/gogf/gf/v2/frame/g"

type Setting struct {
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

type GetReq struct {
	g.Meta `path:"/privacy" method:"get" tags:"Identity-merch"`
}
type GetRes Setting

type SaveReq struct {
	g.Meta            `path:"/privacy" method:"put" tags:"Identity-merch"`
	CommandKey        string `json:"commandKey"`
	ExpectedVersion   uint64 `json:"expectedVersion"`
	CollectConsent    bool   `json:"collectConsent"`
	MarketingConsent  bool   `json:"marketingConsent"`
	CookieBanner      bool   `json:"cookieBanner"`
	DataRetentionDays int    `json:"dataRetentionDays"`
	ContactEmail      string `json:"contactEmail"`
}
type SaveRes struct {
	Setting  Setting `json:"setting"`
	Replayed bool    `json:"replayed"`
}
