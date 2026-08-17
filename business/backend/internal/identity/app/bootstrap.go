package app

import (
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/app/authendpoint"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/app/capabilityendpoint"

	adminlogic "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/logic"
	adminrouter "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/router"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/internalgrant"
	merchlogic "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/logic"
	merchrouter "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/router"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/server"
)

// NewServer mounts every surface this module contributes. Surfaces are listed
// here explicitly: none of them registers itself, so the set a process serves
// is readable in one place and a test can mount a subset.
func NewServer(dependencies *Dependencies) *server.Server {
	surfaces := []server.Surface{
		{
			Name: adminrouter.Surface,
			Register: func(root *ghttp.RouterGroup) {
				adminrouter.RegisterHTTP(root, adminrouter.Deps{
					Application:          adminlogic.New(dependencies.Health, dependencies.Directory, dependencies.Authorization, dependencies.Users, dependencies.Plans, dependencies.Merchants, dependencies.Shops, dependencies.ShopCategories, dependencies.CustomerService, dependencies.MerchantGovernance, dependencies.Assignments, dependencies.PermissionPlans, dependencies.SubscriptionQuotas, dependencies.Grants),
					ModuleSessions:       dependencies.ModuleSessions,
					CurrentAuthorization: dependencies.Users,
				})
			},
		},
		{
			Name: merchrouter.Surface,
			Register: func(root *ghttp.RouterGroup) {
				merchrouter.RegisterHTTP(root, merchrouter.Deps{
					Application: merchlogic.New(dependencies.Health, dependencies.Directory, dependencies.Authorization, dependencies.Users, dependencies.Shops, dependencies.Privacy, dependencies.Policies, dependencies.Apps, dependencies.MerchantGovernance, merchlogic.Subscription{
						Plans: dependencies.Plans, Assignments: dependencies.Assignments,
						Permissions: dependencies.PermissionPlans, Quotas: dependencies.SubscriptionQuotas, Orders: dependencies.Orders,
					}, dependencies.Merchants, dependencies.ShopCategories, dependencies.RiskEvents),
					ModuleSessions:       dependencies.ModuleSessions,
					CurrentAuthorization: dependencies.Users,
				})
			},
		},
	}
	if dependencies.AuthEndpoint != nil {
		surfaces = append([]server.Surface{{Name: "auth", Register: func(root *ghttp.RouterGroup) { authendpoint.Register(root, dependencies.AuthEndpoint) }}}, surfaces...)
	}
	if dependencies.CapabilityEndpoint != nil {
		surfaces = append([]server.Surface{{Name: "runtime", Register: func(root *ghttp.RouterGroup) { capabilityendpoint.Register(root, dependencies.CapabilityEndpoint) }}}, surfaces...)
	}
	if dependencies.Config.Compose.InternalToken != "" {
		surfaces = append(surfaces, server.Surface{
			Name: "internal-directory",
			Register: func(root *ghttp.RouterGroup) {
				internalgrant.Register(root, dependencies.Config.Compose.InternalToken, dependencies.Merchants, dependencies.Shops)
			},
		})
	}
	return server.New(dependencies.Config.Server.HTTP, surfaces)
}
