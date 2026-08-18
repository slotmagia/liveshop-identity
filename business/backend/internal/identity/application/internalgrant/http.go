package internalgrant

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance"
	governancemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

func Register(root *ghttp.RouterGroup, token string, merchants *merchant.Directory, shops *shop.Directory, domains *shop.CustomDomains, governance *merchant_governance.Capabilities) {
	if token == "" {
		return
	}
	root.Group("/internal/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(web.ResponseHandler)
		group.Middleware(requireToken(token))
		group.Bind(&Controller{merchants: merchants, shops: shops, domains: domains, governance: governance})
	})
}

type Controller struct {
	merchants  *merchant.Directory
	shops      *shop.Directory
	domains    *shop.CustomDomains
	governance *merchant_governance.Capabilities
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

type HostBinding struct {
	Kind          string `json:"kind"`
	Host          string `json:"host"`
	MerchantID    int64  `json:"merchantId"`
	ShopID        int64  `json:"shopId"`
	ShopStatus    string `json:"shopStatus"`
	Scene         string `json:"scene"`
	DomainStatus  string `json:"domainStatus"`
	OverlayStatus string `json:"overlayStatus"`
}

type listMerchantsReq struct {
	g.Meta `path:"/directory/merchants" method:"get"`
}

type listShopsReq struct {
	g.Meta     `path:"/directory/shops" method:"get"`
	MerchantID int64 `json:"merchantId" in:"query"`
}

type resolveHostReq struct {
	g.Meta     `path:"/domains/resolve" method:"get"`
	Host       string `json:"host" in:"query"`
	RootDomain string `json:"rootDomain" in:"query"`
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

func (c *Controller) ResolveHost(ctx context.Context, request *resolveHostReq) (*HostBinding, error) {
	host, err := shopmodel.NormalizeHost(request.Host)
	if err != nil {
		return nil, web.Failure(err)
	}
	if c.domains != nil {
		if domain, err := c.domains.GetByHost(ctx, host); err == nil {
			shop, err := c.shops.GetManaged(ctx, domain.MerchantID, domain.ShopID)
			if err != nil {
				return nil, web.Failure(shopmodel.ErrDomainNotFound)
			}
			overlay, err := c.overlay(ctx, domain.MerchantID, domain.ShopID)
			if err != nil {
				return nil, web.Failure(err)
			}
			return &HostBinding{
				Kind: "custom", Host: domain.Host, MerchantID: domain.MerchantID, ShopID: domain.ShopID,
				ShopStatus: string(shop.Status), Scene: string(domain.Scene), DomainStatus: string(domain.Status), OverlayStatus: overlay,
			}, nil
		} else if err != nil && !errors.Is(err, shopmodel.ErrDomainNotFound) && !errors.Is(err, shopmodel.ErrDomainInvalid) {
			return nil, web.Failure(err)
		}
	}
	root, _ := shopmodel.NormalizeHost(request.RootDomain)
	slug := strings.TrimSuffix(host, "."+root)
	if root == "" || host == root || slug == host || strings.Contains(slug, ".") {
		return nil, web.Failure(shopmodel.ErrDomainNotFound)
	}
	shop, err := c.shops.GetBySlug(ctx, slug)
	if err != nil {
		return nil, web.Failure(shopmodel.ErrDomainNotFound)
	}
	return &HostBinding{
		Kind: "slug", Host: host, MerchantID: shop.MerchantID, ShopID: shop.ID,
		ShopStatus: string(shop.Status), OverlayStatus: "active",
	}, nil
}

func (c *Controller) overlay(ctx context.Context, merchantID, shopID int64) (string, error) {
	if c.governance == nil {
		return string(governancemodel.PlatformActive), nil
	}
	page, err := c.governance.List(ctx, governancemodel.Query{MerchantID: merchantID, ShopID: shopID, Module: "domains"})
	if err != nil {
		return "", err
	}
	if len(page.Items) == 0 {
		return string(governancemodel.PlatformActive), nil
	}
	status := string(page.Items[0].PlatformStatus)
	if status == "" {
		return string(governancemodel.PlatformActive), nil
	}
	return status, nil
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
