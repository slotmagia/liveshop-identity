package shopcategory

import "github.com/gogf/gf/v2/frame/g"

type Category struct {
	ID            int64  `json:"id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	Icon          string `json:"icon"`
	Sort          int    `json:"sort"`
	Status        string `json:"status"`
	Version       uint64 `json:"version"`
	UsedShopCount int64  `json:"usedShopCount"`
}

type ListReq struct {
	g.Meta `path:"/shop-categories" method:"get" tags:"Identity-ShopCategory"`
}
type ListRes []Category

type CreateReq struct {
	g.Meta          `path:"/shop-categories" method:"post" tags:"Identity-ShopCategory"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Icon            string `json:"icon"`
	Sort            int    `json:"sort"`
	Status          string `json:"status"`
}

type UpdateReq struct {
	g.Meta          `path:"/shop-categories/{categoryId}" method:"put" tags:"Identity-ShopCategory"`
	CategoryID      int64  `json:"categoryId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Icon            string `json:"icon"`
	Sort            int    `json:"sort"`
	Status          string `json:"status"`
}

type SaveRes struct {
	Category Category `json:"category"`
	Replayed bool     `json:"replayed"`
}

type EnableReq struct {
	g.Meta          `path:"/shop-categories/{categoryId}/enable" method:"post" tags:"Identity-ShopCategory"`
	CategoryID      int64  `json:"categoryId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type EnableRes SaveRes
type DisableReq struct {
	g.Meta          `path:"/shop-categories/{categoryId}/disable" method:"post" tags:"Identity-ShopCategory"`
	CategoryID      int64  `json:"categoryId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type DisableRes SaveRes

type RetireReq struct {
	g.Meta          `path:"/shop-categories/{categoryId}/retire" method:"post" tags:"Identity-ShopCategory"`
	CategoryID      int64  `json:"categoryId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type RetireRes SaveRes
