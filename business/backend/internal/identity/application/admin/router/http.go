// Package router registers the admin surface transports. Adding a
// controller changes this file; adding a surface changes app.
package router

import (
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/lvtuopen-ai/kernel-go/modulesession"

	surfacehttp "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/controller/http"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/middleware"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

const (
	Surface = "admin"
	Prefix  = "/admin/identity"
)

type Deps struct {
	Application          service.Admin
	ModuleSessions       *modulesession.Verifier
	CurrentAuthorization middleware.CurrentAuthorizationValidator
}

// RegisterHTTP mounts every capability behind the module session and its own
// permission, so a grant for one capability can never reach another. The
// permission below must stay identical to the one module.json declares for the
// same operation: the manifest is the contract, this is the enforcement.
func RegisterHTTP(root *ghttp.RouterGroup, deps Deps) {
	bind := func(permission string, target any) {
		root.Group(Prefix, func(group *ghttp.RouterGroup) {
			group.Middleware(web.ResponseHandler)
			group.Middleware(middleware.RequestMetadata)
			group.Middleware(middleware.ModuleSession(deps.ModuleSessions, Surface))
			group.Middleware(middleware.RequirePermission(permission))
			group.Middleware(middleware.RequireCurrentAuthorization(deps.CurrentAuthorization))
			group.Bind(target)
		})
	}
	bind("identity.directory.read", surfacehttp.NewHealth(deps.Application))
	bind("identity.directory.read", surfacehttp.NewDirectory(deps.Application))
	bind("identity.authorization.manage", surfacehttp.NewAuthorization(deps.Application))
	bind("identity.user.manage", surfacehttp.NewUsers(deps.Application))
	bind("identity.session.manage", surfacehttp.NewSessions(deps.Application))
	bind("identity.subscription.manage", surfacehttp.NewSubscription(deps.Application))
	bind("identity.shop.read", surfacehttp.NewShops(deps.Application))
	bind("identity.shop-category.manage", surfacehttp.NewShopCategories(deps.Application))
	bind("identity.customer-account.manage", surfacehttp.NewCustomerService(deps.Application))
	bind("identity.merchant-governance.manage", surfacehttp.NewMerchantGovernance(deps.Application))
	bind("identity.merchant.manage", surfacehttp.NewMerchants(deps.Application))
}
