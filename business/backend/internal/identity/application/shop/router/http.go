package router

import (
	"github.com/gogf/gf/v2/net/ghttp"

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
	Application service.Shop
}

func RegisterHTTP(root *ghttp.RouterGroup, deps Deps) {
	root.Group(Prefix, func(group *ghttp.RouterGroup) {
		group.Middleware(web.ResponseHandler)
		group.Middleware(middleware.RequestMetadata)
		group.Middleware(middleware.RequireSurface(Surface))
		group.Bind(surfacehttp.NewHealth(deps.Application), surfacehttp.NewLogin(deps.Application))
	})
}
