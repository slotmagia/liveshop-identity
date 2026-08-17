package directory

import "github.com/gogf/gf/v2/frame/g"

type ReadReq struct {
	g.Meta         `path:"/directory" method:"get" tags:"Identity-admin"`
	OrganizationID int64 `json:"organizationId" in:"query"`
	MerchantID     int64 `json:"merchantId" in:"query"`
}
type ReadRes struct {
	Organization any `json:"organization"`
	Units        any `json:"units"`
	Members      any `json:"members"`
	Shops        any `json:"shops"`
}
