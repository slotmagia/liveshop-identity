package http

import (
	"context"
	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/api/http/v1/directory"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type DirectoryController struct{ service service.Admin }

func NewDirectory(s service.Admin) *DirectoryController { return &DirectoryController{service: s} }
func (c *DirectoryController) Read(ctx context.Context, r *api.ReadReq) (*api.ReadRes, error) {
	v, e := c.service.Directory(ctx, appmodel.DirectoryQuery{OrganizationID: r.OrganizationID, MerchantID: r.MerchantID})
	if e != nil {
		return nil, web.Failure(e)
	}
	return &api.ReadRes{Organization: v.Organization, Units: v.Units, Members: v.Members, Shops: v.Shops}, nil
}
