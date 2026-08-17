package policy

import "github.com/gogf/gf/v2/frame/g"

type Shop struct {
	ShopID     int64  `json:"shopId"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
}

type Policy struct {
	ID                   int64   `json:"id"`
	MerchantID           int64   `json:"merchantId"`
	ShopID               int64   `json:"shopId"`
	PolicyType           string  `json:"policyType"`
	Title                string  `json:"title"`
	Content              string  `json:"content"`
	VersionNo            int     `json:"versionNo"`
	Status               string  `json:"status"`
	Version              uint64  `json:"version"`
	PublishedAt          *string `json:"publishedAt,omitempty"`
	CreatedAt            string  `json:"createdAt"`
	UpdatedAt            string  `json:"updatedAt"`
	PlatformStatus       string  `json:"platformStatus"`
	PlatformReasonPublic string  `json:"platformReasonPublic,omitempty"`
	Editable             bool    `json:"editable"`
}

type ListShopsReq struct {
	g.Meta `path:"/policies/shops" method:"get" tags:"Identity-merch"`
}
type ListShopsRes []Shop

type ListReq struct {
	g.Meta     `path:"/policies" method:"get" tags:"Identity-merch"`
	ShopID     int64  `json:"shopId" in:"query"`
	PolicyType string `json:"policyType" in:"query"`
	Status     string `json:"status" in:"query"`
	Page       int    `json:"page" in:"query" d:"1"`
	PageSize   int    `json:"pageSize" in:"query" d:"20"`
}
type ListRes struct {
	Items                []Policy `json:"items"`
	Page                 int      `json:"page"`
	PageSize             int      `json:"pageSize"`
	Total                int64    `json:"total"`
	PlatformStatus       string   `json:"platformStatus"`
	PlatformReasonPublic string   `json:"platformReasonPublic,omitempty"`
}

type CreateReq struct {
	g.Meta     `path:"/policies" method:"post" tags:"Identity-merch"`
	CommandKey string `json:"commandKey"`
	ShopID     int64  `json:"shopId"`
	PolicyType string `json:"policyType"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Publish    bool   `json:"publish"`
}
type CreateRes struct {
	Policy   Policy `json:"policy"`
	Replayed bool   `json:"replayed"`
}

type PublishReq struct {
	g.Meta          `path:"/policies/{policyId}/publish" method:"post" tags:"Identity-merch"`
	PolicyId        int64  `json:"policyId" in:"path"`
	ShopID          int64  `json:"shopId"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type PublishRes = CreateRes
