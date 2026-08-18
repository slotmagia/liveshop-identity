package domains

import "github.com/gogf/gf/v2/frame/g"

type Shop struct {
	ShopID     int64  `json:"shopId"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
}

type Domain struct {
	ID                   int64  `json:"id"`
	MerchantID           int64  `json:"merchantId"`
	ShopID               int64  `json:"shopId"`
	Host                 string `json:"host"`
	Scene                string `json:"scene"`
	Status               string `json:"status"`
	IsPrimary            bool   `json:"isPrimary"`
	TxtName              string `json:"txtName"`
	TxtValue             string `json:"txtValue"`
	CnameTarget          string `json:"cnameTarget,omitempty"`
	Version              uint64 `json:"version"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
	PlatformStatus       string `json:"platformStatus"`
	PlatformReasonPublic string `json:"platformReasonPublic,omitempty"`
	Editable             bool   `json:"editable"`
}

type ListShopsReq struct {
	g.Meta `path:"/domains/shops" method:"get" tags:"Identity-merch"`
}
type ListShopsRes []Shop

type ListReq struct {
	g.Meta   `path:"/domains" method:"get" tags:"Identity-merch"`
	ShopID   int64  `json:"shopId" in:"query"`
	Scene    string `json:"scene" in:"query"`
	Status   string `json:"status" in:"query"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"pageSize" in:"query" d:"20"`
}
type ListRes struct {
	Items                []Domain `json:"items"`
	Page                 int      `json:"page"`
	PageSize             int      `json:"pageSize"`
	Total                int64    `json:"total"`
	CnameTarget          string   `json:"cnameTarget,omitempty"`
	PlatformStatus       string   `json:"platformStatus"`
	PlatformReasonPublic string   `json:"platformReasonPublic,omitempty"`
}

type CreateReq struct {
	g.Meta     `path:"/domains" method:"post" tags:"Identity-merch"`
	CommandKey string `json:"commandKey"`
	ShopID     int64  `json:"shopId"`
	Host       string `json:"host"`
	Scene      string `json:"scene"`
}
type CreateRes struct {
	Domain   Domain `json:"domain"`
	Replayed bool   `json:"replayed"`
}

type TestReq struct {
	g.Meta          `path:"/domains/{domainId}/test" method:"post" tags:"Identity-merch"`
	DomainId        int64  `json:"domainId" in:"path"`
	ShopID          int64  `json:"shopId"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	Scene           string `json:"scene"`
}
type TestRes = CreateRes

type ActivateReq struct {
	g.Meta          `path:"/domains/{domainId}/activate" method:"post" tags:"Identity-merch"`
	DomainId        int64  `json:"domainId" in:"path"`
	ShopID          int64  `json:"shopId"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	Scene           string `json:"scene"`
}
type ActivateRes = CreateRes

type DeleteReq struct {
	g.Meta          `path:"/domains/{domainId}" method:"delete" tags:"Identity-merch"`
	DomainId        int64  `json:"domainId" in:"path"`
	ShopID          int64  `json:"shopId"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	Scene           string `json:"scene"`
}
type DeleteRes = CreateRes
