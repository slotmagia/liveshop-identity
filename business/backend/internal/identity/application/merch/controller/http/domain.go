package http

import (
	"context"
	"time"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/domains"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type DomainQueryController struct{ service service.Merch }

func NewDomainQuery(s service.Merch) *DomainQueryController {
	return &DomainQueryController{service: s}
}

func (c *DomainQueryController) ListShops(ctx context.Context, _ *api.ListShopsReq) (*api.ListShopsRes, error) {
	values, err := c.service.DomainShops(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	out := make(api.ListShopsRes, 0, len(values))
	for _, value := range values {
		out = append(out, api.Shop{
			ShopID: value.ShopID, MerchantID: value.MerchantID, Name: value.Name, Code: value.Code, Status: value.Status,
		})
	}
	return &out, nil
}

func (c *DomainQueryController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	value, err := c.service.Domains(ctx, appmodel.DomainQuery{
		ShopID: request.ShopID, Scene: request.Scene, Status: request.Status, Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRes{
		Page: value.Page, PageSize: value.PageSize, Total: value.Total, Items: []api.Domain{},
		CnameTarget: value.CnameTarget, PlatformStatus: value.PlatformStatus, PlatformReasonPublic: value.PlatformReason,
	}
	for _, item := range value.Items {
		out.Items = append(out.Items, wireDomain(item))
	}
	return &out, nil
}

type DomainWriteController struct{ service service.Merch }

func NewDomainWrite(s service.Merch) *DomainWriteController {
	return &DomainWriteController{service: s}
}

func (c *DomainWriteController) Create(ctx context.Context, request *api.CreateReq) (*api.CreateRes, error) {
	value, err := c.service.CreateDomain(ctx, appmodel.CreateDomain{
		CommandKey: request.CommandKey, ShopID: request.ShopID, Host: request.Host, Scene: request.Scene,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.CreateRes{Domain: wireDomain(value.Domain), Replayed: value.Replayed}, nil
}

func (c *DomainWriteController) Test(ctx context.Context, request *api.TestReq) (*api.TestRes, error) {
	value, err := c.service.TestDomain(ctx, appmodel.DomainWrite{
		DomainID: request.DomainId, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Scene: request.Scene,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.TestRes{Domain: wireDomain(value.Domain), Replayed: value.Replayed}, nil
}

func (c *DomainWriteController) Activate(ctx context.Context, request *api.ActivateReq) (*api.ActivateRes, error) {
	value, err := c.service.ActivateDomain(ctx, appmodel.DomainWrite{
		DomainID: request.DomainId, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Scene: request.Scene,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.ActivateRes{Domain: wireDomain(value.Domain), Replayed: value.Replayed}, nil
}

func (c *DomainWriteController) Delete(ctx context.Context, request *api.DeleteReq) (*api.DeleteRes, error) {
	value, err := c.service.DeleteDomain(ctx, appmodel.DomainWrite{
		DomainID: request.DomainId, ShopID: request.ShopID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion, Scene: request.Scene,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.DeleteRes{Domain: wireDomain(value.Domain), Replayed: value.Replayed}, nil
}

func wireDomain(value appmodel.Domain) api.Domain {
	return api.Domain{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID, Host: value.Host, Scene: value.Scene,
		Status: value.Status, IsPrimary: value.IsPrimary, TxtName: value.TxtName, TxtValue: value.TxtValue,
		CnameTarget: value.CnameTarget, Version: value.Version,
		CreatedAt: formatDomainTime(value.CreatedAt), UpdatedAt: formatDomainTime(value.UpdatedAt),
		PlatformStatus: value.PlatformStatus, PlatformReasonPublic: value.PlatformReason, Editable: value.Editable,
	}
}

func formatDomainTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
