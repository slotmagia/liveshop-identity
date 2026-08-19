package aftersales

import "github.com/gogf/gf/v2/frame/g"

type AftersaleItem struct {
	ID               int64  `json:"id"`
	SKUID            int64  `json:"skuId"`
	Title            string `json:"title"`
	Quantity         int64  `json:"quantity"`
	RefundAmount     int64  `json:"refundAmount"`
	ReceivedQuantity int64  `json:"receivedQuantity"`
}

type Aftersale struct {
	ID              int64           `json:"id"`
	OrderID         int64           `json:"orderId"`
	PaymentNo       string          `json:"paymentNo"`
	Type            string          `json:"type"`
	RequestedAmount int64           `json:"requestedAmount"`
	Amount          int64           `json:"amount"`
	Reason          string          `json:"reason"`
	Status          string          `json:"status"`
	ReturnStatus    string          `json:"returnStatus"`
	HandleNote      string          `json:"handleNote"`
	Items           []AftersaleItem `json:"items"`
	Version         uint64          `json:"version"`
	CreatedAt       string          `json:"createdAt"`
	UpdatedAt       string          `json:"updatedAt"`
	ReviewedAt      *string         `json:"reviewedAt,omitempty"`
	ReceivedAt      *string         `json:"receivedAt,omitempty"`
}

type ListReq struct {
	g.Meta   `path:"/aftersales" method:"get" tags:"Identity-shop" summary:"List customer aftersales"`
	Status   string `json:"status" in:"query"`
	Type     string `json:"type" in:"query"`
	Page     int    `json:"page" in:"query"`
	PageSize int    `json:"pageSize" in:"query"`
}

type ListRes struct {
	Items    []Aftersale `json:"items"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	Total    int64       `json:"total"`
}

type GetReq struct {
	g.Meta      `path:"/aftersales/{aftersaleId}" method:"get" tags:"Identity-shop" summary:"Get a customer aftersale"`
	AftersaleId int64 `json:"aftersaleId" in:"path"`
}

type GetRes struct {
	Aftersale Aftersale `json:"aftersale"`
}
