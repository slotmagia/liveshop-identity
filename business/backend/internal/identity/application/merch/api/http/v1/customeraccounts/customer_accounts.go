package customeraccounts

import "github.com/gogf/gf/v2/frame/g"

type Shop struct {
	ShopID     int64  `json:"shopId"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
}

type Account struct {
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

type ListShopsReq struct {
	g.Meta `path:"/customer-accounts/shops" method:"get" tags:"Identity-merch"`
}
type ListShopsRes []Shop

type ListReq struct {
	g.Meta   `path:"/customer-accounts" method:"get" tags:"Identity-merch"`
	ShopID   int64  `json:"shopId" in:"query"`
	Platform string `json:"platform" in:"query"`
	Account  string `json:"account" in:"query"`
	Status   string `json:"status" in:"query"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"pageSize" in:"query" d:"20"`
}
type ListRes struct {
	Items    []Account `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
	Total    int64     `json:"total"`
}

type SaveFields struct {
	CommandKey string `json:"commandKey"`
	ShopID     int64  `json:"shopId"`
	Platform   string `json:"platform"`
	Account    string `json:"account"`
	Nickname   string `json:"nickname"`
	Status     string `json:"status"`
	Config     string `json:"config"`
	Remark     string `json:"remark"`
}

type CreateReq struct {
	g.Meta `path:"/customer-accounts" method:"post" tags:"Identity-merch"`
	SaveFields
}
type CreateRes struct {
	Account  Account `json:"account"`
	Replayed bool    `json:"replayed"`
}

type UpdateReq struct {
	g.Meta          `path:"/customer-accounts/{accountId}" method:"put" tags:"Identity-merch"`
	AccountID       int64  `json:"accountId" in:"path"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	SaveFields
}
type UpdateRes = CreateRes

type DeleteReq struct {
	g.Meta          `path:"/customer-accounts/{accountId}" method:"delete" tags:"Identity-merch"`
	AccountID       int64  `json:"accountId" in:"path"`
	ShopID          int64  `json:"shopId"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type DeleteRes struct {
	ID       int64  `json:"id"`
	Deleted  bool   `json:"deleted"`
	Version  uint64 `json:"version"`
	Replayed bool   `json:"replayed"`
}
