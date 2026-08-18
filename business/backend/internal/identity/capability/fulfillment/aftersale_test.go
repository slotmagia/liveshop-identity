package fulfillment

import (
	"context"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

type aftersaleRepositoryStub struct {
	query      model.AftersaleQuery
	page       model.AftersalePage
	item       model.Aftersale
	reviewed   model.ReviewAftersaleCommand
	received   model.ReceiveAftersaleCommand
	replayed   bool
	listErr    error
	getErr     error
	reviewErr  error
	receiveErr error
}

func (s *aftersaleRepositoryStub) ListAftersales(_ context.Context, query model.AftersaleQuery) (model.AftersalePage, error) {
	s.query = query
	if s.listErr != nil {
		return model.AftersalePage{}, s.listErr
	}
	if s.page.Page == 0 {
		return model.AftersalePage{Items: []model.Aftersale{}, Page: query.Page, PageSize: query.PageSize}, nil
	}
	return s.page, nil
}

func (s *aftersaleRepositoryStub) GetAftersale(_ context.Context, _, _, _ int64) (model.Aftersale, error) {
	if s.getErr != nil {
		return model.Aftersale{}, s.getErr
	}
	return s.item, nil
}

func (s *aftersaleRepositoryStub) ReviewAftersale(_ context.Context, command model.ReviewAftersaleCommand) (model.Aftersale, bool, error) {
	s.reviewed = command
	if s.reviewErr != nil {
		return model.Aftersale{}, false, s.reviewErr
	}
	return s.item, s.replayed, nil
}

func (s *aftersaleRepositoryStub) ReceiveAftersale(_ context.Context, command model.ReceiveAftersaleCommand) (model.Aftersale, bool, error) {
	s.received = command
	if s.receiveErr != nil {
		return model.Aftersale{}, false, s.receiveErr
	}
	return s.item, s.replayed, nil
}

func sampleAftersale(status model.AftersaleStatus) model.Aftersale {
	reviewed := time.Unix(2, 0).UTC()
	item := model.Aftersale{
		ID: 21, MerchantID: 2001, ShopID: 3001, CustomerSubject: "cust-2001", OrderID: 8802,
		PaymentNo: "pay-21", Type: model.AftersaleReturnRefund, RequestedAmount: 9900, Amount: 9900,
		Reason: "尺码不合适", Status: status, ReturnStatus: model.ReturnPending, Version: 1,
		CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(1, 0).UTC(),
		Items: []model.AftersaleItem{{ID: 1, SKUID: 501, Title: "外套", Quantity: 1, RefundAmount: 9900}},
	}
	if status != model.AftersalePending {
		item.HandleNote = "同意退货退款"
		item.ReviewedAt = &reviewed
		item.Version = 2
		item.UpdatedAt = reviewed
	}
	return item
}

func TestAftersaleListRequiresShopScope(t *testing.T) {
	if _, err := NewAftersales(&aftersaleRepositoryStub{}).List(context.Background(), model.AftersaleQuery{Page: 1, PageSize: 20}); err != model.ErrAftersaleInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestAftersaleListPreservesShopScope(t *testing.T) {
	item := sampleAftersale(model.AftersalePending)
	repository := &aftersaleRepositoryStub{page: model.AftersalePage{Page: 1, PageSize: 20, Total: 1, Items: []model.Aftersale{item}}}
	page, err := NewAftersales(repository).List(context.Background(), model.AftersaleQuery{MerchantID: 2001, ShopID: 3001})
	if err != nil || page.Total != 1 || repository.query.MerchantID != 2001 || repository.query.ShopID != 3001 {
		t.Fatalf("page=%+v query=%+v err=%v", page, repository.query, err)
	}
}

func TestAftersaleReviewRejectsOpenStatus(t *testing.T) {
	_, _, err := NewAftersales(&aftersaleRepositoryStub{item: sampleAftersale(model.AftersaleApproved)}).Review(context.Background(), model.ReviewAftersaleCommand{
		AftersaleID: 21, MerchantID: 2001, ShopID: 3001, CommandKey: "review-21", ExpectedVersion: 1,
		Status: model.AftersalePending, HandleNote: "说明",
	})
	if err != model.ErrAftersaleInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestAftersaleReviewReturnsPersistedResult(t *testing.T) {
	item := sampleAftersale(model.AftersaleApproved)
	repository := &aftersaleRepositoryStub{item: item}
	value, replayed, err := NewAftersales(repository).Review(context.Background(), model.ReviewAftersaleCommand{
		AftersaleID: 21, MerchantID: 2001, ShopID: 3001, CommandKey: "review-21", ExpectedVersion: 1,
		Status: model.AftersaleApproved, HandleNote: "同意退货退款",
	})
	if err != nil || replayed || value.ID != 21 || repository.reviewed.Status != model.AftersaleApproved {
		t.Fatalf("value=%+v replayed=%v err=%v command=%+v", value, replayed, err, repository.reviewed)
	}
}
