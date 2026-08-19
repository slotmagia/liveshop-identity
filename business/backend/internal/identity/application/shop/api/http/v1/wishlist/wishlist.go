package wishlist

import "github.com/gogf/gf/v2/frame/g"

type Item struct {
	ProductID int64 `json:"productId"`
	CreatedAt int64 `json:"createdAt"`
}

type ListReq struct {
	g.Meta `path:"/wishlist" method:"get" tags:"Identity-shop" summary:"List wishlist items"`
	Cursor int64 `json:"cursor" in:"query"`
	Limit  int   `json:"limit" in:"query"`
}

type ListRes struct {
	Items []Item `json:"items"`
}

type CreateReq struct {
	g.Meta     `path:"/wishlist/items" method:"post" tags:"Identity-shop" summary:"Add a wishlist item"`
	ProductID  int64  `json:"productId"`
	CommandKey string `json:"commandKey"`
}

type CreateRes Item

type DeleteReq struct {
	g.Meta    `path:"/wishlist/items/{productId}" method:"delete" tags:"Identity-shop" summary:"Remove a wishlist item"`
	ProductID int64 `json:"productId" in:"path"`
}

type DeleteRes struct {
	OK bool `json:"ok"`
}
