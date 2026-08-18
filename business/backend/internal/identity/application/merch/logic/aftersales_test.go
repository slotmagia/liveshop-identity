package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment"
	fulfillmentmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type stubAftersaleRepo struct {
	query    fulfillmentmodel.AftersaleQuery
	page     fulfillmentmodel.AftersalePage
	item     fulfillmentmodel.Aftersale
	reviewed fulfillmentmodel.ReviewAftersaleCommand
	received fulfillmentmodel.ReceiveAftersaleCommand
	err      error
}

func (s *stubAftersaleRepo) ListAftersales(_ context.Context, query fulfillmentmodel.AftersaleQuery) (fulfillmentmodel.AftersalePage, error) {
	s.query = query
	if s.err != nil {
		return fulfillmentmodel.AftersalePage{}, s.err
	}
	if s.page.Page == 0 {
		return fulfillmentmodel.AftersalePage{Items: []fulfillmentmodel.Aftersale{}, Page: query.Page, PageSize: query.PageSize}, nil
	}
	return s.page, nil
}

func (s *stubAftersaleRepo) GetAftersale(_ context.Context, _, _, _ int64) (fulfillmentmodel.Aftersale, error) {
	if s.err != nil {
		return fulfillmentmodel.Aftersale{}, s.err
	}
	return s.item, nil
}

func (s *stubAftersaleRepo) ReviewAftersale(_ context.Context, command fulfillmentmodel.ReviewAftersaleCommand) (fulfillmentmodel.Aftersale, bool, error) {
	s.reviewed = command
	if s.err != nil {
		return fulfillmentmodel.Aftersale{}, false, s.err
	}
	return s.item, false, nil
}

func (s *stubAftersaleRepo) ReceiveAftersale(_ context.Context, command fulfillmentmodel.ReceiveAftersaleCommand) (fulfillmentmodel.Aftersale, bool, error) {
	s.received = command
	if s.err != nil {
		return fulfillmentmodel.Aftersale{}, false, s.err
	}
	return s.item, false, nil
}

func merchAftersaleLogic(repo *stubAftersaleRepo) *Logic {
	return New(nil, nil, nil, nil, nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil, nil, nil, nil, fulfillment.NewAftersales(repo), nil, nil)
}

func merchAftersaleContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{MerchantID: 2001, ShopID: 3001})
}

func sampleMerchAftersale(status fulfillmentmodel.AftersaleStatus, note string, reviewed *time.Time, version uint64) fulfillmentmodel.Aftersale {
	return fulfillmentmodel.Aftersale{
		ID: 21, MerchantID: 2001, ShopID: 3001, CustomerSubject: "cust-2001", OrderID: 8802,
		PaymentNo: "pay-21", Type: fulfillmentmodel.AftersaleReturnRefund, RequestedAmount: 9900, Amount: 9900,
		Reason: "尺码不合适", Status: status, ReturnStatus: fulfillmentmodel.ReturnPending, HandleNote: note,
		Version: version, CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(2, 0).UTC(), ReviewedAt: reviewed,
		Items: []fulfillmentmodel.AftersaleItem{{ID: 1, SKUID: 501, Title: "外套", Quantity: 1, RefundAmount: 9900}},
	}
}

func TestAftersalesRequireShopContext(t *testing.T) {
	if _, err := merchAftersaleLogic(&stubAftersaleRepo{}).Aftersales(context.Background(), appmodel.AftersaleQuery{}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestAftersalesUseSessionShopAndProjectRows(t *testing.T) {
	repo := &stubAftersaleRepo{page: fulfillmentmodel.AftersalePage{Page: 1, PageSize: 20, Total: 1, Items: []fulfillmentmodel.Aftersale{
		sampleMerchAftersale(fulfillmentmodel.AftersalePending, "", nil, 1),
	}}}
	page, err := merchAftersaleLogic(repo).Aftersales(merchAftersaleContext(), appmodel.AftersaleQuery{CustomerSubject: "cust-2001", Status: "PENDING", Type: "RETURN_REFUND", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if repo.query.MerchantID != 2001 || repo.query.ShopID != 3001 || repo.query.CustomerSubject != "cust-2001" || repo.query.Status != fulfillmentmodel.AftersalePending {
		t.Fatalf("query=%+v", repo.query)
	}
	if page.Total != 1 || page.Items[0].CustomerSubject != "cust-2001" || page.Items[0].Status != "PENDING" || page.Items[0].CreatedAt != "1970-01-01T00:00:01Z" {
		t.Fatalf("page=%+v", page)
	}
}

func TestAftersalesRejectClosedShop(t *testing.T) {
	_, err := merchAftersaleLogic(&stubAftersaleRepo{err: fulfillmentmodel.ErrAftersaleNotFound}).Aftersales(merchAftersaleContext(), appmodel.AftersaleQuery{})
	if !errors.Is(err, fulfillmentmodel.ErrAftersaleNotFound) {
		t.Fatalf("error=%v", err)
	}
}

func TestReviewAftersaleUsesSessionShop(t *testing.T) {
	reviewed := time.Unix(2, 0).UTC()
	repo := &stubAftersaleRepo{item: sampleMerchAftersale(fulfillmentmodel.AftersaleApproved, "同意退货退款", &reviewed, 2)}
	value, err := merchAftersaleLogic(repo).ReviewAftersale(merchAftersaleContext(), appmodel.ReviewAftersale{
		AftersaleID: 21, CommandKey: "review-0021", ExpectedVersion: 1, Status: "APPROVED", Amount: 9900, HandleNote: "同意退货退款",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.reviewed.MerchantID != 2001 || repo.reviewed.ShopID != 3001 || repo.reviewed.AftersaleID != 21 || repo.reviewed.Status != fulfillmentmodel.AftersaleApproved {
		t.Fatalf("command=%+v", repo.reviewed)
	}
	if value.Aftersale.Status != "APPROVED" || value.Replayed || value.Aftersale.HandleNote != "同意退货退款" {
		t.Fatalf("value=%+v", value)
	}
}

func TestReceiveAftersaleUsesSessionShop(t *testing.T) {
	reviewed := time.Unix(2, 0).UTC()
	item := sampleMerchAftersale(fulfillmentmodel.AftersaleApproved, "同意退货退款", &reviewed, 2)
	received := time.Unix(3, 0).UTC()
	item.ReturnStatus = fulfillmentmodel.ReturnReceived
	item.ReceivedAt = &received
	item.Version = 3
	item.Items[0].ReceivedQuantity = 1
	repo := &stubAftersaleRepo{item: item}
	value, err := merchAftersaleLogic(repo).ReceiveAftersale(merchAftersaleContext(), appmodel.ReceiveAftersale{
		AftersaleID: 21, CommandKey: "return-0021", ExpectedVersion: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.received.MerchantID != 2001 || repo.received.ShopID != 3001 || repo.received.AftersaleID != 21 {
		t.Fatalf("command=%+v", repo.received)
	}
	if value.Aftersale.ReturnStatus != "RECEIVED" || value.Replayed {
		t.Fatalf("value=%+v", value)
	}
}
