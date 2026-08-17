package apps

import "github.com/gogf/gf/v2/frame/g"

type Shop struct {
	ShopID     int64  `json:"shopId"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
}

type Scope struct {
	Code  string `json:"code"`
	Group string `json:"group"`
	Label string `json:"label"`
}

type App struct {
	ID                   int64  `json:"id"`
	MerchantID           int64  `json:"merchantId"`
	ShopID               int64  `json:"shopId"`
	Name                 string `json:"name"`
	ClientID             string `json:"clientId"`
	SecretHint           string `json:"secretHint"`
	Scopes               string `json:"scopes"`
	Status               string `json:"status"`
	Version              uint64 `json:"version"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
	PlatformStatus       string `json:"platformStatus"`
	PlatformReasonPublic string `json:"platformReasonPublic,omitempty"`
	Editable             bool   `json:"editable"`
}

type ListShopsReq struct {
	g.Meta `path:"/apps/shops" method:"get" tags:"Identity-merch"`
}
type ListShopsRes []Shop

type ListScopesReq struct {
	g.Meta `path:"/apps/scopes" method:"get" tags:"Identity-merch"`
}
type ListScopesRes []Scope

type ListReq struct {
	g.Meta   `path:"/apps" method:"get" tags:"Identity-merch"`
	ShopID   int64  `json:"shopId" in:"query"`
	Status   string `json:"status" in:"query"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"pageSize" in:"query" d:"20"`
}
type ListRes struct {
	Items                []App  `json:"items"`
	Page                 int    `json:"page"`
	PageSize             int    `json:"pageSize"`
	Total                int64  `json:"total"`
	PlatformStatus       string `json:"platformStatus"`
	PlatformReasonPublic string `json:"platformReasonPublic,omitempty"`
}

type CreateReq struct {
	g.Meta     `path:"/apps" method:"post" tags:"Identity-merch"`
	CommandKey string `json:"commandKey"`
	ShopID     int64  `json:"shopId"`
	Name       string `json:"name"`
	Scopes     string `json:"scopes"`
}
type CreateRes struct {
	App          App    `json:"app"`
	ClientSecret string `json:"clientSecret,omitempty"`
	Replayed     bool   `json:"replayed"`
}

type ResetReq struct {
	g.Meta          `path:"/apps/{appId}/reset" method:"post" tags:"Identity-merch"`
	AppId           int64  `json:"appId" in:"path"`
	ShopID          int64  `json:"shopId"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type ResetRes = CreateRes

type EnableReq struct {
	g.Meta          `path:"/apps/{appId}/enable" method:"post" tags:"Identity-merch"`
	AppId           int64  `json:"appId" in:"path"`
	ShopID          int64  `json:"shopId"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type EnableRes struct {
	App      App  `json:"app"`
	Replayed bool `json:"replayed"`
}

type DisableReq struct {
	g.Meta          `path:"/apps/{appId}/disable" method:"post" tags:"Identity-merch"`
	AppId           int64  `json:"appId" in:"path"`
	ShopID          int64  `json:"shopId"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type DisableRes = EnableRes
