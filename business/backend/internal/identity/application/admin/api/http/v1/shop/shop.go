package shop

import "github.com/gogf/gf/v2/frame/g"

type Merchant struct {
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	ExternalID string `json:"externalId"`
	Status     string `json:"status"`
	Version    uint64 `json:"version"`
}

type Shop struct {
	ShopID        int64  `json:"shopId"`
	MerchantID    int64  `json:"merchantId"`
	Code          string `json:"code"`
	Subdomain     string `json:"subdomain"`
	Name          string `json:"name"`
	DefaultLocale string `json:"defaultLocale"`
	Currency      string `json:"currency"`
	Status        string `json:"status"`
	Version       uint64 `json:"version"`
}

type ListMerchantsReq struct {
	g.Meta `path:"/shops/merchants" method:"get" tags:"Identity-Shop"`
}
type ListMerchantsRes []Merchant

type ListShopsReq struct {
	g.Meta     `path:"/shops" method:"get" tags:"Identity-Shop"`
	MerchantID int64 `json:"merchantId" in:"query"`
}
type ListShopsRes []Shop
