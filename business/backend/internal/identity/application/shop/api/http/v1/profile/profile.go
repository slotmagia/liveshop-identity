package profile

import "github.com/gogf/gf/v2/frame/g"

type GetReq struct {
	g.Meta `path:"/profile" method:"get" tags:"Identity-shop" summary:"Read the current shopper profile"`
}

type GetRes struct {
	Subject       string `json:"subject"`
	PrincipalType string `json:"principalType"`
	SignedIn      bool   `json:"signedIn"`
	DisplayName   string `json:"displayName"`
}
