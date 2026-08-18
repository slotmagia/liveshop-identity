package logic

import (
	"context"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	fulfillmentmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

func (l *Logic) Complaints(ctx context.Context, query appmodel.ComplaintQuery) (appmodel.ComplaintPage, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.ComplaintPage{}, err
	}
	if l.complaints == nil {
		return appmodel.ComplaintPage{}, model.ErrUnavailable
	}
	page, err := l.complaints.List(ctx, fulfillmentmodel.Query{
		MerchantID: merchantID, ShopID: shopID, CustomerSubject: query.CustomerSubject,
		Status: fulfillmentmodel.Status(query.Status), TargetType: fulfillmentmodel.TargetType(query.TargetType),
		Page: query.Page, PageSize: query.PageSize,
	})
	if err != nil {
		return appmodel.ComplaintPage{}, err
	}
	out := appmodel.ComplaintPage{
		Items: make([]appmodel.Complaint, 0, len(page.Items)), Page: page.Page, PageSize: page.PageSize, Total: page.Total,
	}
	for _, item := range page.Items {
		out.Items = append(out.Items, projectComplaint(item))
	}
	return out, nil
}

func (l *Logic) Complaint(ctx context.Context, complaintID int64) (appmodel.Complaint, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.Complaint{}, err
	}
	if l.complaints == nil {
		return appmodel.Complaint{}, model.ErrUnavailable
	}
	value, err := l.complaints.Get(ctx, merchantID, shopID, complaintID)
	if err != nil {
		return appmodel.Complaint{}, err
	}
	return projectComplaint(value), nil
}

func (l *Logic) ReviewComplaint(ctx context.Context, input appmodel.ReviewComplaint) (appmodel.ComplaintResult, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.ComplaintResult{}, err
	}
	if l.complaints == nil {
		return appmodel.ComplaintResult{}, model.ErrUnavailable
	}
	value, replayed, err := l.complaints.Review(ctx, fulfillmentmodel.ReviewCommand{
		ComplaintID: input.ComplaintID, MerchantID: merchantID, ShopID: shopID, CommandKey: input.CommandKey,
		ExpectedVersion: input.ExpectedVersion, Status: fulfillmentmodel.Status(input.Status), HandleNote: input.HandleNote,
	})
	if err != nil {
		return appmodel.ComplaintResult{}, err
	}
	return appmodel.ComplaintResult{Complaint: projectComplaint(value), Replayed: replayed}, nil
}

func projectComplaint(item fulfillmentmodel.Complaint) appmodel.Complaint {
	out := appmodel.Complaint{
		ID: item.ID, CustomerSubject: item.CustomerSubject, TargetType: string(item.TargetType), TargetID: item.TargetID,
		ReasonCode: item.ReasonCode, Content: item.Content, Status: string(item.Status), HandleNote: item.HandleNote,
		Version: item.Version, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if item.HandledAt != nil && !item.HandledAt.IsZero() {
		formatted := item.HandledAt.UTC().Format(time.RFC3339)
		out.HandledAt = &formatted
	}
	return out
}
