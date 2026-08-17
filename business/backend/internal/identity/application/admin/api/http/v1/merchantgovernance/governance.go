package merchantgovernance

import "github.com/gogf/gf/v2/frame/g"

type Module struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type Merchant struct {
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Status     string `json:"status"`
}

type Shop struct {
	ShopID       int64  `json:"shopId"`
	MerchantID   int64  `json:"merchantId"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	Status       string `json:"status"`
}

type Capability struct {
	ID                   int64  `json:"id"`
	MerchantID           int64  `json:"merchantId"`
	ShopID               int64  `json:"shopId"`
	Module               string `json:"module"`
	ModuleLabel          string `json:"moduleLabel"`
	Name                 string `json:"name"`
	MerchantStatus       string `json:"merchantStatus"`
	PlatformStatus       string `json:"platformStatus"`
	PlatformReasonPublic string `json:"platformReasonPublic"`
	Version              uint64 `json:"version"`
	UpdatedBy            string `json:"updatedBy"`
	UpdatedAt            string `json:"updatedAt"`
}

type AuditItem struct {
	ID             int64  `json:"id"`
	MerchantID     int64  `json:"merchantId"`
	ShopID         int64  `json:"shopId"`
	Module         string `json:"module"`
	CapabilityID   int64  `json:"capabilityId"`
	Action         string `json:"action"`
	Operator       string `json:"operator"`
	ReasonInternal string `json:"reasonInternal"`
	ReasonPublic   string `json:"reasonPublic"`
	CreatedAt      string `json:"createdAt"`
}

type CatalogReq struct {
	g.Meta `path:"/merchant-governance/catalog" method:"get" tags:"Identity-Merchant-Governance"`
}
type CatalogRes []Module

type ListMerchantsReq struct {
	g.Meta `path:"/merchant-governance/merchants" method:"get" tags:"Identity-Merchant-Governance"`
}
type ListMerchantsRes []Merchant

type ListShopsReq struct {
	g.Meta     `path:"/merchant-governance/shops" method:"get" tags:"Identity-Merchant-Governance"`
	MerchantID int64 `json:"merchantId" in:"query"`
}
type ListShopsRes []Shop

type ListReq struct {
	g.Meta     `path:"/merchant-governance" method:"get" tags:"Identity-Merchant-Governance"`
	MerchantID int64  `json:"merchantId" in:"query"`
	ShopID     int64  `json:"shopId" in:"query"`
	Module     string `json:"module" in:"query"`
}
type ListRes []Capability

type AuditReq struct {
	g.Meta     `path:"/merchant-governance/audit" method:"get" tags:"Identity-Merchant-Governance"`
	MerchantID int64  `json:"merchantId" in:"query"`
	ShopID     int64  `json:"shopId" in:"query"`
	Module     string `json:"module" in:"query"`
}
type AuditRes []AuditItem

type InterveneReq struct {
	g.Meta          `path:"/merchant-governance/intervene" method:"post" tags:"Identity-Merchant-Governance"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	MerchantID      int64  `json:"merchantId"`
	ShopID          int64  `json:"shopId"`
	Module          string `json:"module"`
	PlatformStatus  string `json:"platformStatus"`
	ReasonInternal  string `json:"reasonInternal"`
	ReasonPublic    string `json:"reasonPublic"`
}
type InterveneRes struct {
	Capability Capability `json:"capability"`
	Replayed   bool       `json:"replayed"`
}
