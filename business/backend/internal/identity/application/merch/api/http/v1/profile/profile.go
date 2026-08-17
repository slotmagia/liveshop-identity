package profile

import "github.com/gogf/gf/v2/frame/g"

type Setting struct {
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

type GetReq struct {
	g.Meta `path:"/profile" method:"get" tags:"Identity-merch"`
}
type GetRes Setting

type SaveReq struct {
	g.Meta              `path:"/profile" method:"put" tags:"Identity-merch"`
	CommandKey          string `json:"commandKey"`
	ExpectedVersion     uint64 `json:"expectedVersion"`
	ExternalID          string `json:"externalId"`
	ContactName         string `json:"contactName"`
	ContactPhone        string `json:"contactPhone"`
	MarketingEmailOptIn bool   `json:"marketingEmailOptIn"`
	MarketingSMSOptIn   bool   `json:"marketingSmsOptIn"`
}
type SaveRes struct {
	Profile  Setting `json:"profile"`
	Replayed bool    `json:"replayed"`
}
