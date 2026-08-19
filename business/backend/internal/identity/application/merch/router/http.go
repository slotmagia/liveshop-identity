// Package router registers the merch surface transports. Adding a
// controller changes this file; adding a surface changes app.
package router

import (
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/lvtuopen-ai/kernel-go/modulesession"

	surfacehttp "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/controller/http"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/middleware"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

const (
	Surface = "merch"
	Prefix  = "/merch/identity"
)

type Deps struct {
	Application          service.Merch
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
	bind("identity.organization.read", surfacehttp.NewDirectory(deps.Application))
	bind("identity.organization.read", surfacehttp.NewAccount(deps.Application))
	bind("identity.profile.manage", surfacehttp.NewProfile(deps.Application))
	bind("identity.organization.manage", surfacehttp.NewUnit(deps.Application))
	bind("identity.staff.manage", surfacehttp.NewMember(deps.Application))
	bind("identity.authorization.manage", surfacehttp.NewAuthorization(deps.Application))
	bind("identity.staff.manage", surfacehttp.NewUsers(deps.Application))
	bind("identity.session.manage", surfacehttp.NewSessions(deps.Application))
	bind("identity.privacy.manage", surfacehttp.NewPrivacy(deps.Application))
	bind("identity.policy.read", surfacehttp.NewPolicyQuery(deps.Application))
	bind("identity.policy.manage", surfacehttp.NewPolicyWrite(deps.Application))
	bind("identity.app.read", surfacehttp.NewAppQuery(deps.Application))
	bind("identity.app.manage", surfacehttp.NewAppWrite(deps.Application))
	bind("identity.domain.read", surfacehttp.NewDomainQuery(deps.Application))
	bind("identity.domain.manage", surfacehttp.NewDomainWrite(deps.Application))
	bind("identity.subscription.read", surfacehttp.NewSubscriptionQuery(deps.Application))
	bind("identity.subscription.purchase", surfacehttp.NewSubscriptionWrite(deps.Application))
	bind("identity.shop.read", surfacehttp.NewShopQuery(deps.Application))
	bind("identity.shop.manage", surfacehttp.NewShopWrite(deps.Application))
	bind("identity.risk.read", surfacehttp.NewRiskEventQuery(deps.Application))
	bind("identity.complaint.read", surfacehttp.NewComplaintQuery(deps.Application))
	bind("identity.complaint.manage", surfacehttp.NewComplaintWrite(deps.Application))
	bind("identity.aftersale.read", surfacehttp.NewAftersaleQuery(deps.Application))
	bind("identity.aftersale.manage", surfacehttp.NewAftersaleWrite(deps.Application))
	bind("identity.shipment.read", surfacehttp.NewShipmentQuery(deps.Application))
	bind("identity.shipment.manage", surfacehttp.NewShipmentWrite(deps.Application))
	bind("identity.shipping.read", surfacehttp.NewShippingQuery(deps.Application))
	bind("identity.shipping.manage", surfacehttp.NewShippingWrite(deps.Application))
	bind("identity.language.read", surfacehttp.NewLanguagesQuery(deps.Application))
	bind("identity.language.manage", surfacehttp.NewLanguagesWrite(deps.Application))
	bind("identity.customer-account.manage", surfacehttp.NewCustomerAccount(deps.Application))
}
