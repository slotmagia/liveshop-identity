package logic

import (
	"context"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	fulfillmentmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

func (l *Logic) Aftersales(ctx context.Context, query appmodel.AftersaleQuery) (appmodel.AftersalePage, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.AftersalePage{}, err
	}
	if l.aftersales == nil {
		return appmodel.AftersalePage{}, model.ErrUnavailable
	}
	page, err := l.aftersales.List(ctx, fulfillmentmodel.AftersaleQuery{
		MerchantID: merchantID, ShopID: shopID, CustomerSubject: query.CustomerSubject,
		Status: fulfillmentmodel.AftersaleStatus(query.Status), Type: fulfillmentmodel.AftersaleType(query.Type),
		Page: query.Page, PageSize: query.PageSize,
	})
	if err != nil {
		return appmodel.AftersalePage{}, err
	}
	out := appmodel.AftersalePage{
		Items: make([]appmodel.Aftersale, 0, len(page.Items)), Page: page.Page, PageSize: page.PageSize, Total: page.Total,
	}
	for _, item := range page.Items {
		out.Items = append(out.Items, projectAftersale(item))
	}
	return out, nil
}

func (l *Logic) Aftersale(ctx context.Context, aftersaleID int64) (appmodel.Aftersale, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.Aftersale{}, err
	}
	if l.aftersales == nil {
		return appmodel.Aftersale{}, model.ErrUnavailable
	}
	value, err := l.aftersales.Get(ctx, merchantID, shopID, aftersaleID)
	if err != nil {
		return appmodel.Aftersale{}, err
	}
	return projectAftersale(value), nil
}

func (l *Logic) ReviewAftersale(ctx context.Context, input appmodel.ReviewAftersale) (appmodel.AftersaleResult, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.AftersaleResult{}, err
	}
	if l.aftersales == nil {
		return appmodel.AftersaleResult{}, model.ErrUnavailable
	}
	value, replayed, err := l.aftersales.Review(ctx, fulfillmentmodel.ReviewAftersaleCommand{
		AftersaleID: input.AftersaleID, MerchantID: merchantID, ShopID: shopID, CommandKey: input.CommandKey,
		ExpectedVersion: input.ExpectedVersion, Status: fulfillmentmodel.AftersaleStatus(input.Status),
		Amount: input.Amount, HandleNote: input.HandleNote,
	})
	if err != nil {
		return appmodel.AftersaleResult{}, err
	}
	return appmodel.AftersaleResult{Aftersale: projectAftersale(value), Replayed: replayed}, nil
}

func (l *Logic) ReceiveAftersale(ctx context.Context, input appmodel.ReceiveAftersale) (appmodel.AftersaleResult, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.AftersaleResult{}, err
	}
	if l.aftersales == nil {
		return appmodel.AftersaleResult{}, model.ErrUnavailable
	}
	value, replayed, err := l.aftersales.Receive(ctx, fulfillmentmodel.ReceiveAftersaleCommand{
		AftersaleID: input.AftersaleID, MerchantID: merchantID, ShopID: shopID, CommandKey: input.CommandKey,
		ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return appmodel.AftersaleResult{}, err
	}
	return appmodel.AftersaleResult{Aftersale: projectAftersale(value), Replayed: replayed}, nil
}

func projectAftersale(item fulfillmentmodel.Aftersale) appmodel.Aftersale {
	out := appmodel.Aftersale{
		ID: item.ID, CustomerSubject: item.CustomerSubject, OrderID: item.OrderID, PaymentNo: item.PaymentNo,
		Type: string(item.Type), RequestedAmount: item.RequestedAmount, Amount: item.Amount, Reason: item.Reason,
		Status: string(item.Status), ReturnStatus: string(item.ReturnStatus), HandleNote: item.HandleNote,
		Items: make([]appmodel.AftersaleItem, 0, len(item.Items)), Version: item.Version,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	for _, line := range item.Items {
		out.Items = append(out.Items, appmodel.AftersaleItem{
			ID: line.ID, SKUID: line.SKUID, Title: line.Title, Quantity: line.Quantity,
			RefundAmount: line.RefundAmount, ReceivedQuantity: line.ReceivedQuantity,
		})
	}
	if item.ReviewedAt != nil && !item.ReviewedAt.IsZero() {
		formatted := item.ReviewedAt.UTC().Format(time.RFC3339)
		out.ReviewedAt = &formatted
	}
	if item.ReceivedAt != nil && !item.ReceivedAt.IsZero() {
		formatted := item.ReceivedAt.UTC().Format(time.RFC3339)
		out.ReceivedAt = &formatted
	}
	return out
}
