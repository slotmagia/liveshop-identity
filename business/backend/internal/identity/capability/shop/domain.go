package shop

import (
	"context"
	"net"
	"strings"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type TXTLookup func(ctx context.Context, name string) ([]string, error)

func LookupTXT(ctx context.Context, name string) ([]string, error) {
	var resolver net.Resolver
	return resolver.LookupTXT(ctx, name)
}

type DomainRepository interface {
	ListDomains(context.Context, model.DomainQuery) (model.DomainPage, error)
	GetDomain(context.Context, int64, int64, int64) (model.Domain, error)
	GetDomainByHost(context.Context, string) (model.Domain, error)
	CreateDomain(context.Context, model.CreateDomainCommand) (model.Domain, bool, error)
	TestDomain(context.Context, model.DomainWriteCommand, bool) (model.Domain, bool, error)
	ActivateDomain(context.Context, model.DomainWriteCommand) (model.Domain, bool, error)
	DeleteDomain(context.Context, model.DomainWriteCommand) (model.Domain, bool, error)
}

type CustomDomains struct {
	repository  DomainRepository
	lookup      TXTLookup
	cnameTarget string
}

func NewCustomDomains(repository DomainRepository, lookup TXTLookup, cnameTarget string) *CustomDomains {
	return &CustomDomains{repository: repository, lookup: lookup, cnameTarget: strings.TrimSpace(cnameTarget)}
}

func (d *CustomDomains) CNAMETarget() string {
	if d == nil {
		return ""
	}
	return d.cnameTarget
}

func (d *CustomDomains) GetByHost(ctx context.Context, host string) (model.Domain, error) {
	if d == nil || d.repository == nil {
		return model.Domain{}, model.ErrUnavailable
	}
	normalized, err := model.NormalizeHost(host)
	if err != nil {
		return model.Domain{}, err
	}
	value, err := d.repository.GetDomainByHost(ctx, normalized)
	if err != nil {
		return model.Domain{}, err
	}
	if err := value.ValidatePersisted(); err != nil || value.Status == model.DomainDeleted {
		return model.Domain{}, model.ErrDomainNotFound
	}
	return value, nil
}

func (d *CustomDomains) List(ctx context.Context, query model.DomainQuery) (model.DomainPage, error) {
	if d == nil || d.repository == nil {
		return model.DomainPage{}, model.ErrUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.DomainPage{}, err
	}
	page, err := d.repository.ListDomains(ctx, normalized)
	if err != nil {
		return model.DomainPage{}, err
	}
	if page.Page != normalized.Page || page.PageSize != normalized.PageSize || page.Total < 0 {
		return model.DomainPage{}, model.ErrDomainInvalid
	}
	for _, item := range page.Items {
		if err := item.ValidatePersisted(); err != nil {
			return model.DomainPage{}, model.ErrDomainInvalid
		}
		if item.MerchantID != normalized.MerchantID || item.ShopID != normalized.ShopID || item.Scene != normalized.Scene {
			return model.DomainPage{}, model.ErrDomainInvalid
		}
		if item.Status == model.DomainDeleted {
			return model.DomainPage{}, model.ErrDomainInvalid
		}
	}
	return page, nil
}

func (d *CustomDomains) Create(ctx context.Context, command model.CreateDomainCommand) (model.Domain, bool, error) {
	if d == nil || d.repository == nil {
		return model.Domain{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Domain{}, false, err
	}
	return d.repository.CreateDomain(ctx, normalized)
}

func (d *CustomDomains) Test(ctx context.Context, command model.DomainWriteCommand) (model.Domain, bool, error) {
	if d == nil || d.repository == nil {
		return model.Domain{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Domain{}, false, err
	}
	if d.lookup == nil {
		return model.Domain{}, false, model.ErrDomainUnavailable
	}
	current, err := d.repository.GetDomain(ctx, normalized.DomainID, normalized.MerchantID, normalized.ShopID)
	if err != nil {
		return model.Domain{}, false, err
	}
	if current.Status == model.DomainDeleted {
		return model.Domain{}, false, model.ErrDomainNotFound
	}
	if normalized.Scene != "" && current.Scene != normalized.Scene {
		return model.Domain{}, false, model.ErrDomainNotFound
	}
	records, err := d.lookup(ctx, current.TxtName)
	if err != nil {
		return model.Domain{}, false, model.ErrDomainUnavailable
	}
	matched := false
	for _, record := range records {
		if strings.Trim(record, `"`) == current.TxtValue {
			matched = true
			break
		}
	}
	return d.repository.TestDomain(ctx, normalized, matched)
}

func (d *CustomDomains) Activate(ctx context.Context, command model.DomainWriteCommand) (model.Domain, bool, error) {
	if d == nil || d.repository == nil {
		return model.Domain{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Domain{}, false, err
	}
	return d.repository.ActivateDomain(ctx, normalized)
}

func (d *CustomDomains) Delete(ctx context.Context, command model.DomainWriteCommand) (model.Domain, bool, error) {
	if d == nil || d.repository == nil {
		return model.Domain{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Domain{}, false, err
	}
	return d.repository.DeleteDomain(ctx, normalized)
}
