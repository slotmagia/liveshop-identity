package subscription

import "github.com/gogf/gf/v2/frame/g"

type Plan struct {
	ID              int64    `json:"id"`
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	Level           int      `json:"level"`
	PriceMinor      int64    `json:"priceMinor"`
	DurationDays    int      `json:"durationDays"`
	Description     string   `json:"description"`
	Default         bool     `json:"default"`
	Current         bool     `json:"current"`
	Buyable         bool     `json:"buyable"`
	ProductLimit    *int64   `json:"productLimit"`
	PermissionNames []string `json:"permissionNames"`
}

type PayMethod struct {
	ChannelCode string `json:"channelCode"`
	DisplayName string `json:"displayName"`
	TypeCode    string `json:"typeCode"`
	DriverKey   string `json:"driverKey"`
}

type Current struct {
	MerchantID      int64    `json:"merchantId"`
	PlanID          int64    `json:"planId"`
	PlanCode        string   `json:"planCode"`
	PlanName        string   `json:"planName"`
	ExpiresAt       string   `json:"expiresAt"`
	Version         uint64   `json:"version"`
	ProductLimit    *int64   `json:"productLimit"`
	QuotaConfigured bool     `json:"quotaConfigured"`
	PermissionNames []string `json:"permissionNames"`
}

type Order struct {
	OrderNo      string            `json:"orderNo"`
	PlanID       int64             `json:"planId"`
	PlanCode     string            `json:"planCode"`
	PlanName     string            `json:"planName"`
	PriceMinor   int64             `json:"priceMinor"`
	DurationDays int               `json:"durationDays"`
	Status       string            `json:"status"`
	PayNo        string            `json:"payNo"`
	ChannelCode  string            `json:"channelCode"`
	DriverKey    string            `json:"driverKey"`
	PayStatus    string            `json:"payStatus"`
	Payload      map[string]string `json:"payload"`
	Activated    bool              `json:"activated"`
	ExpiresAt    string            `json:"expiresAt"`
	CreatedAt    string            `json:"createdAt"`
	PaidAt       string            `json:"paidAt"`
	Replayed     bool              `json:"replayed"`
}

type ListPlansReq struct {
	g.Meta `path:"/subscription/plans" method:"get" tags:"Identity-merch"`
}
type ListPlansRes struct {
	Items         []Plan `json:"items"`
	CurrentPlanID int64  `json:"currentPlanId"`
}

type GetReq struct {
	g.Meta `path:"/subscription" method:"get" tags:"Identity-merch"`
}
type GetRes Current

type ListPayMethodsReq struct {
	g.Meta `path:"/subscription/pay-methods" method:"get" tags:"Identity-merch"`
	PlanID int64 `json:"planId" in:"query"`
}
type ListPayMethodsRes []PayMethod

type CreateOrderReq struct {
	g.Meta      `path:"/subscription/orders" method:"post" tags:"Identity-merch"`
	CommandKey  string `json:"commandKey"`
	PlanID      int64  `json:"planId"`
	ChannelCode string `json:"channelCode"`
}
type CreateOrderRes Order

type GetOrderReq struct {
	g.Meta  `path:"/subscription/orders/{orderNo}" method:"get" tags:"Identity-merch"`
	OrderNo string `json:"orderNo" in:"path"`
}
type GetOrderRes Order

type ConfirmOrderReq struct {
	g.Meta     `path:"/subscription/orders/{orderNo}/confirm" method:"post" tags:"Identity-merch"`
	OrderNo    string `json:"orderNo" in:"path"`
	CommandKey string `json:"commandKey"`
}
type ConfirmOrderRes Order

type ListOrdersReq struct {
	g.Meta   `path:"/subscription/orders" method:"get" tags:"Identity-merch"`
	Status   string `json:"status" in:"query"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"pageSize" in:"query" d:"20"`
}
type ListOrdersRes struct {
	Items    []Order `json:"items"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
	Total    int64   `json:"total"`
	Owner    bool    `json:"owner"`
}

type CloseOrderReq struct {
	g.Meta     `path:"/subscription/orders/{orderNo}/close" method:"post" tags:"Identity-merch"`
	OrderNo    string `json:"orderNo" in:"path"`
	CommandKey string `json:"commandKey"`
}
type CloseOrderRes Order
