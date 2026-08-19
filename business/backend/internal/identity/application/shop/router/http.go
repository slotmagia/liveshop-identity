package router

import (
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/lvtuopen-ai/kernel-go/modulesession"

	surfacehttp "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/controller/http"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/middleware"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

const (
	Surface = "shop"
	Prefix  = "/shop/identity"
)

type Deps struct {
	Application    service.Shop
	ModuleSessions *modulesession.Verifier
}

func RegisterHTTP(root *ghttp.RouterGroup, deps Deps) {
	root.Group(Prefix, func(group *ghttp.RouterGroup) {
		group.Middleware(web.ResponseHandler)
		group.Middleware(middleware.RequestMetadata)
		group.Middleware(middleware.RequireSurface(Surface))
		group.Bind(surfacehttp.NewHealth(deps.Application), surfacehttp.NewLogin(deps.Application))
	})
	bindShopper := func(target any) {
		root.Group(Prefix, func(group *ghttp.RouterGroup) {
			group.Middleware(web.ResponseHandler)
			group.Middleware(middleware.RequestMetadata)
			group.Middleware(middleware.ModuleSession(deps.ModuleSessions, Surface))
			group.Middleware(middleware.RequireShopperSession)
			group.Bind(target)
		})
	}
	bindCustomer := func(target any) {
		root.Group(Prefix, func(group *ghttp.RouterGroup) {
			group.Middleware(web.ResponseHandler)
			group.Middleware(middleware.RequestMetadata)
			group.Middleware(middleware.ModuleSession(deps.ModuleSessions, Surface))
			group.Middleware(middleware.RequireCustomerSession)
			group.Bind(target)
		})
	}
	bindShopper(surfacehttp.NewProfile(deps.Application))
	bindCustomer(surfacehttp.NewAddressDefaultWriter(deps.Application))
	bindCustomer(surfacehttp.NewAddressReader(deps.Application))
	bindCustomer(surfacehttp.NewAddressWriter(deps.Application))
	bindCustomer(surfacehttp.NewWishlistReader(deps.Application))
	bindCustomer(surfacehttp.NewWishlistWriter(deps.Application))
	bindCustomer(surfacehttp.NewAftersales(deps.Application))
}
