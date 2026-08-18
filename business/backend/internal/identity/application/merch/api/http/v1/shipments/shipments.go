package shipments

import "github.com/gogf/gf/v2/frame/g"

type Trace struct {
	OccurredAt string `json:"occurredAt"`
	Node       string `json:"node"`
}

type Shipment struct {
	ID         int64   `json:"id"`
	OrderID    int64   `json:"orderId"`
	Carrier    string  `json:"carrier"`
	TrackingNo string  `json:"trackingNo"`
	Status     string  `json:"status"`
	Traces     []Trace `json:"traces"`
	Version    uint64  `json:"version"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
}

type ListReq struct {
	g.Meta   `path:"/shipments" method:"get" tags:"Identity-merch"`
	OrderId  int64  `json:"orderId" in:"query"`
	Status   string `json:"status" in:"query"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"pageSize" in:"query" d:"20"`
}

type ListRes struct {
	Items    []Shipment `json:"items"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
	Total    int64      `json:"total"`
}

type GetReq struct {
	g.Meta     `path:"/shipments/{shipmentId}" method:"get" tags:"Identity-merch"`
	ShipmentId int64 `json:"shipmentId" in:"path"`
}

type GetRes struct {
	Shipment Shipment `json:"shipment"`
}

type CreateReq struct {
	g.Meta     `path:"/shipments" method:"post" tags:"Identity-merch"`
	CommandKey string `json:"commandKey"`
	OrderId    int64  `json:"orderId"`
	Carrier    string `json:"carrier"`
	TrackingNo string `json:"trackingNo"`
}

type CreateRes struct {
	Shipment Shipment `json:"shipment"`
	Replayed bool     `json:"replayed"`
}

type CreateTraceReq struct {
	g.Meta          `path:"/shipments/{shipmentId}/traces" method:"post" tags:"Identity-merch"`
	ShipmentId      int64  `json:"shipmentId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	Node            string `json:"node"`
}

type CreateTraceRes struct {
	Shipment Shipment `json:"shipment"`
	Replayed bool     `json:"replayed"`
}

type CloseReq struct {
	g.Meta          `path:"/shipments/{shipmentId}/close" method:"post" tags:"Identity-merch"`
	ShipmentId      int64  `json:"shipmentId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

type CloseRes struct {
	Shipment Shipment `json:"shipment"`
	Replayed bool     `json:"replayed"`
}
