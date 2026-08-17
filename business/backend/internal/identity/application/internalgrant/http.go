package internalgrant

import (
	"context"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

func Register(root *ghttp.RouterGroup, token string, merchants *merchant.Directory, shops *shop.Directory) {
	if token == "" {
		return
	}
	root.Group("/internal/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(web.ResponseHandler)
		group.Middleware(requireToken(token))
		group.Bind(&Controller{merchants: merchants, shops: shops})
	})
}

type Controller struct {
	merchants *merchant.Directory
	shops     *shop.Directory
}

type Merchant struct {
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Status     string `json:"status"`
}

type Shop struct {
	ShopID     int64  `json:"shopId"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
}

type listMerchantsReq struct {
	g.Meta `path:"/directory/merchants" method:"get"`
}

type listShopsReq struct {
	g.Meta     `path:"/directory/shops" method:"get"`
	MerchantID int64 `json:"merchantId" in:"query"`
}

func (c *Controller) ListMerchants(ctx context.Context, _ *listMerchantsReq) (*[]Merchant, error) {
	values, err := c.merchants.List(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make([]Merchant, 0, len(values))
	for _, value := range values {
		out = append(out, Merchant{MerchantID: value.ID, Name: value.Name, Status: string(value.Status)})
	}
	return &out, nil
}

func (c *Controller) ListShops(ctx context.Context, request *listShopsReq) (*[]Shop, error) {
	values, err := c.shops.List(ctx, request.MerchantID)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make([]Shop, 0, len(values))
	for _, value := range values {
		out = append(out, Shop{ShopID: value.ID, MerchantID: value.MerchantID, Name: value.Name, Code: value.Code, Status: string(value.Status)})
	}
	return &out, nil
}

func requireToken(token string) func(*ghttp.Request) {
	return func(request *ghttp.Request) {
		if token != "" && request.Header.Get("X-Liveshop-Internal-Grant") == token {
			request.Middleware.Next()
			return
		}
		request.Response.WriteStatus(http.StatusUnauthorized)
		request.ExitAll()
	}
}
