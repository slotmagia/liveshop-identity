package shops

import "github.com/gogf/gf/v2/frame/g"

type Shop struct {
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

type Category struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type ListCategoriesReq struct {
	g.Meta `path:"/shops/categories" method:"get" tags:"Identity-merch"`
}
type ListCategoriesRes []Category

type ListReq struct {
	g.Meta   `path:"/shops" method:"get" tags:"Identity-merch"`
	Keyword  string `json:"keyword" in:"query"`
	Status   string `json:"status" in:"query"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"pageSize" in:"query" d:"20"`
}
type ListRes struct {
	Items    []Shop `json:"items"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Total    int64  `json:"total"`
	Owner    bool   `json:"owner"`
}

type CurrentReq struct {
	g.Meta `path:"/shops/current" method:"get" tags:"Identity-merch"`
}
type CurrentRes struct {
	Shop  Shop `json:"shop"`
	Owner bool `json:"owner"`
}

type CreateReq struct {
	g.Meta       `path:"/shops" method:"post" tags:"Identity-merch"`
	CommandKey   string `json:"commandKey"`
	Name         string `json:"name"`
	Subdomain    string `json:"subdomain"`
	Currency     string `json:"currency"`
	CategoryCode string `json:"categoryCode"`
	Status       string `json:"status"`
}
type CreateRes struct {
	Shop     Shop `json:"shop"`
	Replayed bool `json:"replayed"`
}

type UpdateReq struct {
	g.Meta          `path:"/shops/{shopId}" method:"put" tags:"Identity-merch"`
	ShopId          int64  `json:"shopId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	Name            string `json:"name"`
	Subdomain       string `json:"subdomain"`
}
type UpdateRes = CreateRes

type EnableReq struct {
	g.Meta          `path:"/shops/{shopId}/enable" method:"post" tags:"Identity-merch"`
	ShopId          int64  `json:"shopId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type EnableRes = CreateRes

type DisableReq struct {
	g.Meta          `path:"/shops/{shopId}/disable" method:"post" tags:"Identity-merch"`
	ShopId          int64  `json:"shopId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type DisableRes = CreateRes

type CloseReq struct {
	g.Meta          `path:"/shops/{shopId}/close" method:"post" tags:"Identity-merch"`
	ShopId          int64  `json:"shopId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type CloseRes = CreateRes
