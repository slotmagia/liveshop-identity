package health

import "github.com/gogf/gf/v2/frame/g"

type ReadReq struct {
	g.Meta `path:"/health" method:"get" tags:"Identity-shop" summary:"Read module health"`
}

type ReadRes struct {
	Status string `json:"status"`
}
