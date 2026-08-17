// Package health is the private HTTP wire contract of the admin surface.
// API DTOs are surface-private and must not reference biz or data types.
package health

import "github.com/gogf/gf/v2/frame/g"

type ReadReq struct {
	g.Meta `path:"/health" method:"get" tags:"Identity-admin" summary:"Read module health"`
}

type ReadRes struct {
	Status string `json:"status"`
}
