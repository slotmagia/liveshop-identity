package logic

import (
	"context"
	"fmt"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/compose"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	governancemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

func (l *Logic) DomainShops(ctx context.Context) ([]appmodel.DomainShop, error) {
	values, err := l.PolicyShops(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appmodel.DomainShop, 0, len(values))
	for _, value := range values {
		out = append(out, appmodel.DomainShop{
			ShopID: value.ShopID, MerchantID: value.MerchantID, Name: value.Name, Code: value.Code, Status: value.Status,
		})
	}
	return out, nil
}

func (l *Logic) Domains(ctx context.Context, input appmodel.DomainQuery) (appmodel.DomainPage, error) {
	merchantID, shopID, err := l.appShopID(ctx, input.ShopID)
	if err != nil {
		return appmodel.DomainPage{}, err
	}
	if l.domains == nil {
		return appmodel.DomainPage{}, model.ErrUnavailable
	}
	scene, err := shopmodel.DefaultDomainScene(input.Scene)
	if err != nil {
		return appmodel.DomainPage{}, err
	}
	page, err := l.domains.List(ctx, shopmodel.DomainQuery{
		MerchantID: merchantID, ShopID: shopID, Scene: scene,
		Status: shopmodel.DomainStatus(input.Status), Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return appmodel.DomainPage{}, err
	}
	overlay, err := l.domainOverlay(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.DomainPage{}, err
	}
	out := appmodel.DomainPage{
		Page: page.Page, PageSize: page.PageSize, Total: page.Total, Items: []appmodel.Domain{},
		CnameTarget: l.publishedCNAME(ctx), PlatformStatus: string(overlay.PlatformStatus), PlatformReason: overlay.PlatformReasonPublic,
	}
	if out.PlatformStatus == "" {
		out.PlatformStatus = string(governancemodel.PlatformActive)
	}
	for _, item := range page.Items {
		out.Items = append(out.Items, l.projectDomain(ctx, item, overlay))
	}
	return out, nil
}

func (l *Logic) CreateDomain(ctx context.Context, input appmodel.CreateDomain) (appmodel.DomainResult, error) {
	merchantID, shopID, overlay, err := l.prepareDomainWrite(ctx, input.ShopID, true)
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	scene, err := shopmodel.DefaultDomainScene(input.Scene)
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	normalized, err := shopmodel.NormalizeHost(input.Host)
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	if l.hostReserved(ctx, normalized) {
		return appmodel.DomainResult{}, shopmodel.ErrDomainInvalid
	}
	value, replayed, err := l.domains.Create(ctx, shopmodel.CreateDomainCommand{
		CommandKey: input.CommandKey, MerchantID: merchantID, ShopID: shopID, Host: input.Host, Scene: scene,
	})
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	return appmodel.DomainResult{Domain: l.projectDomain(ctx, value, overlay), Replayed: replayed}, nil
}

func (l *Logic) TestDomain(ctx context.Context, input appmodel.DomainWrite) (appmodel.DomainResult, error) {
	merchantID, shopID, overlay, err := l.prepareDomainWrite(ctx, input.ShopID, true)
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	command, err := merchDomainWrite(input, merchantID, shopID)
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	value, replayed, err := l.domains.Test(ctx, command)
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	return appmodel.DomainResult{Domain: l.projectDomain(ctx, value, overlay), Replayed: replayed}, nil
}

func (l *Logic) ActivateDomain(ctx context.Context, input appmodel.DomainWrite) (appmodel.DomainResult, error) {
	merchantID, shopID, overlay, err := l.prepareDomainWrite(ctx, input.ShopID, true)
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	command, err := merchDomainWrite(input, merchantID, shopID)
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	value, replayed, err := l.domains.Activate(ctx, command)
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	return appmodel.DomainResult{Domain: l.projectDomain(ctx, value, overlay), Replayed: replayed}, nil
}

func (l *Logic) DeleteDomain(ctx context.Context, input appmodel.DomainWrite) (appmodel.DomainResult, error) {
	merchantID, shopID, overlay, err := l.prepareDomainWrite(ctx, input.ShopID, false)
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	command, err := merchDomainWrite(input, merchantID, shopID)
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	value, replayed, err := l.domains.Delete(ctx, command)
	if err != nil {
		return appmodel.DomainResult{}, err
	}
	return appmodel.DomainResult{Domain: l.projectDomain(ctx, value, overlay), Replayed: replayed}, nil
}

func merchDomainWrite(input appmodel.DomainWrite, merchantID, shopID int64) (shopmodel.DomainWriteCommand, error) {
	scene, err := shopmodel.OptionalDomainScene(input.Scene)
	if err != nil {
		return shopmodel.DomainWriteCommand{}, err
	}
	return shopmodel.DomainWriteCommand{
		DomainID: input.DomainID, MerchantID: merchantID, ShopID: shopID,
		CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion, Scene: scene,
	}, nil
}

func (l *Logic) prepareDomainWrite(ctx context.Context, requestedShopID int64, requireWritable bool) (int64, int64, governancemodel.Capability, error) {
	merchantID, shopID, err := l.appShopID(ctx, requestedShopID)
	if err != nil {
		return 0, 0, governancemodel.Capability{}, err
	}
	if l.domains == nil {
		return 0, 0, governancemodel.Capability{}, model.ErrUnavailable
	}
	overlay, err := l.domainOverlay(ctx, merchantID, shopID)
	if err != nil {
		return 0, 0, governancemodel.Capability{}, err
	}
	if requireWritable {
		if err := l.requireDomainWritable(overlay); err != nil {
			return 0, 0, governancemodel.Capability{}, err
		}
	}
	return merchantID, shopID, overlay, nil
}

func (l *Logic) projectDomain(ctx context.Context, value shopmodel.Domain, overlay governancemodel.Capability) appmodel.Domain {
	status := string(overlay.PlatformStatus)
	if status == "" {
		status = string(governancemodel.PlatformActive)
	}
	editable := overlay.PlatformStatus == "" || overlay.PlatformStatus == governancemodel.PlatformActive
	return appmodel.Domain{
		ID: value.ID, MerchantID: value.MerchantID, ShopID: value.ShopID, Host: value.Host, Scene: string(value.Scene),
		Status: string(value.Status), IsPrimary: value.IsPrimary, TxtName: value.TxtName, TxtValue: value.TxtValue,
		CnameTarget: l.publishedCNAME(ctx), Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		PlatformStatus: status, PlatformReason: overlay.PlatformReasonPublic, Editable: editable,
	}
}

func (l *Logic) publishedCNAME(ctx context.Context) string {
	if snap, ok := l.edgeSnapshot(ctx); ok && snap.CNAMETarget != "" {
		return snap.CNAMETarget
	}
	if l.domains != nil {
		return l.domains.CNAMETarget()
	}
	return ""
}

func (l *Logic) edgeSnapshot(ctx context.Context) (compose.EdgeSnapshot, bool) {
	if l == nil || l.grants == nil {
		return compose.EdgeSnapshot{}, false
	}
	snap, err := l.grants.EdgeSnapshot(ctx)
	if err != nil {
		return compose.EdgeSnapshot{}, false
	}
	return snap, true
}

func (l *Logic) hostReserved(ctx context.Context, host string) bool {
	snap, ok := l.edgeSnapshot(ctx)
	if !ok {
		return false
	}
	for _, item := range snap.ReservedHosts {
		if item == host {
			return true
		}
	}
	return false
}

func (l *Logic) domainOverlay(ctx context.Context, merchantID, shopID int64) (governancemodel.Capability, error) {
	if l.merchantGovernance == nil {
		return governancemodel.Capability{PlatformStatus: governancemodel.PlatformActive}, nil
	}
	page, err := l.merchantGovernance.List(ctx, governancemodel.Query{MerchantID: merchantID, ShopID: shopID, Module: "domains"})
	if err != nil {
		return governancemodel.Capability{}, err
	}
	if len(page.Items) == 0 {
		return governancemodel.Capability{MerchantID: merchantID, ShopID: shopID, Module: "domains", PlatformStatus: governancemodel.PlatformActive}, nil
	}
	return page.Items[0], nil
}

func (l *Logic) requireDomainWritable(overlay governancemodel.Capability) error {
	if overlay.PlatformStatus != "" && overlay.PlatformStatus != governancemodel.PlatformActive {
		return fmt.Errorf("%w: %s", shopmodel.ErrDomainRestricted, overlay.PlatformReasonPublic)
	}
	return nil
}
