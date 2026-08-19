package http

import (
	"context"

	addressapi "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/api/http/v1/address"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type AddressReader struct{ service service.Shop }
type AddressWriter struct{ service service.Shop }
type AddressDefaultWriter struct{ service service.Shop }

func NewAddressReader(application service.Shop) *AddressReader { return &AddressReader{application} }
func NewAddressWriter(application service.Shop) *AddressWriter { return &AddressWriter{application} }
func NewAddressDefaultWriter(application service.Shop) *AddressDefaultWriter {
	return &AddressDefaultWriter{application}
}

func (c *AddressReader) List(ctx context.Context, _ *addressapi.ListReq) (*addressapi.ListRes, error) {
	items, err := c.service.Addresses(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	result := &addressapi.ListRes{Items: make([]addressapi.Item, 0, len(items))}
	for _, item := range items {
		result.Items = append(result.Items, addressItem(item))
	}
	return result, nil
}

func (c *AddressWriter) Create(ctx context.Context, request *addressapi.CreateReq) (*addressapi.CreateRes, error) {
	item, err := c.service.SaveAddress(ctx, appmodel.SaveAddress{
		Recipient: request.Recipient, Phone: request.Phone, Country: request.Country, Province: request.Province,
		City: request.City, District: request.District, Detail: request.Detail, PostalCode: request.PostalCode,
		IsDefault: request.IsDefault, CommandKey: request.CommandKey,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	result := addressapi.CreateRes(addressItem(item))
	return &result, nil
}

func (c *AddressWriter) Update(ctx context.Context, request *addressapi.UpdateReq) (*addressapi.UpdateRes, error) {
	item, err := c.service.SaveAddress(ctx, appmodel.SaveAddress{
		ID: request.AddressID, Recipient: request.Recipient, Phone: request.Phone, Country: request.Country,
		Province: request.Province, City: request.City, District: request.District, Detail: request.Detail,
		PostalCode: request.PostalCode, IsDefault: request.IsDefault, CommandKey: request.CommandKey,
		ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	result := addressapi.UpdateRes(addressItem(item))
	return &result, nil
}

func (c *AddressWriter) Delete(ctx context.Context, request *addressapi.DeleteReq) (*addressapi.DeleteRes, error) {
	if err := c.service.DeleteAddress(ctx, appmodel.DeleteAddress{
		ID: request.AddressID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
	}); err != nil {
		return nil, web.Failure(err)
	}
	return &addressapi.DeleteRes{OK: true}, nil
}

func (c *AddressDefaultWriter) Replace(ctx context.Context, request *addressapi.ReplaceDefaultReq) (*addressapi.ReplaceDefaultRes, error) {
	item, err := c.service.ReplaceDefaultAddress(ctx, appmodel.ReplaceDefault{
		ID: request.AddressID, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	result := addressapi.ReplaceDefaultRes(addressItem(item))
	return &result, nil
}

func addressItem(item appmodel.Address) addressapi.Item {
	return addressapi.Item{
		ID: item.ID, Recipient: item.Recipient, Phone: item.Phone, Country: item.Country,
		Province: item.Province, City: item.City, District: item.District, Detail: item.Detail,
		PostalCode: item.PostalCode, IsDefault: item.IsDefault, Version: item.Version,
	}
}
