package fulfillment

import (
	"context"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

type complaintRepositoryStub struct {
	query     model.Query
	page      model.Page
	item      model.Complaint
	reviewed  model.ReviewCommand
	replayed  bool
	listErr   error
	getErr    error
	reviewErr error
}

func (s *complaintRepositoryStub) List(_ context.Context, query model.Query) (model.Page, error) {
	s.query = query
	if s.listErr != nil {
		return model.Page{}, s.listErr
	}
	if s.page.Page == 0 {
		return model.Page{Items: []model.Complaint{}, Page: query.Page, PageSize: query.PageSize}, nil
	}
	return s.page, nil
}

func (s *complaintRepositoryStub) Get(_ context.Context, _, _, _ int64) (model.Complaint, error) {
	if s.getErr != nil {
		return model.Complaint{}, s.getErr
	}
	return s.item, nil
}

func (s *complaintRepositoryStub) Review(_ context.Context, command model.ReviewCommand) (model.Complaint, bool, error) {
	s.reviewed = command
	if s.reviewErr != nil {
		return model.Complaint{}, false, s.reviewErr
	}
	return s.item, s.replayed, nil
}

func sampleComplaint() model.Complaint {
	handled := time.Unix(2, 0).UTC()
	return model.Complaint{
		ID: 11, MerchantID: 2001, ShopID: 3001, CustomerSubject: "cust-1001",
		TargetType: model.TargetOrder, TargetID: 8801, ReasonCode: "quality", Content: "商品与描述不符",
		Status: model.StatusAccepted, HandleNote: "已核对订单并同意处理", Version: 2,
		CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(2, 0).UTC(), HandledAt: &handled,
	}
}

func openComplaint() model.Complaint {
	return model.Complaint{
		ID: 11, MerchantID: 2001, ShopID: 3001, CustomerSubject: "cust-1001",
		TargetType: model.TargetOrder, TargetID: 8801, ReasonCode: "quality", Content: "商品与描述不符",
		Status: model.StatusOpen, Version: 1,
		CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(1, 0).UTC(),
	}
}

func TestListRequiresShopScope(t *testing.T) {
	if _, err := NewComplaints(&complaintRepositoryStub{}).List(context.Background(), model.Query{Page: 1, PageSize: 20}); err != model.ErrInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestListPreservesShopScopeAndDefaultPage(t *testing.T) {
	item := openComplaint()
	repository := &complaintRepositoryStub{page: model.Page{Page: 1, PageSize: 20, Total: 1, Items: []model.Complaint{item}}}
	page, err := NewComplaints(repository).List(context.Background(), model.Query{MerchantID: 2001, ShopID: 3001})
	if err != nil || page.Total != 1 || repository.query.MerchantID != 2001 || repository.query.ShopID != 3001 || repository.query.Page != 1 || repository.query.PageSize != 20 {
		t.Fatalf("page=%+v query=%+v err=%v", page, repository.query, err)
	}
}

func TestListRejectsForeignShopRows(t *testing.T) {
	item := openComplaint()
	item.ShopID = 4001
	repository := &complaintRepositoryStub{page: model.Page{Page: 1, PageSize: 20, Total: 1, Items: []model.Complaint{item}}}
	if _, err := NewComplaints(repository).List(context.Background(), model.Query{MerchantID: 2001, ShopID: 3001, Page: 1, PageSize: 20}); err != model.ErrInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestGetUnavailableWithoutRepository(t *testing.T) {
	if _, err := NewComplaints(nil).Get(context.Background(), 2001, 3001, 11); err != model.ErrUnavailable {
		t.Fatalf("error=%v", err)
	}
}

func TestReviewNormalizesAndRejectsNonReviewStatus(t *testing.T) {
	_, _, err := NewComplaints(&complaintRepositoryStub{item: sampleComplaint()}).Review(context.Background(), model.ReviewCommand{
		ComplaintID: 11, MerchantID: 2001, ShopID: 3001, CommandKey: "review-01", ExpectedVersion: 1, Status: model.StatusOpen, HandleNote: "说明",
	})
	if err != model.ErrInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestReviewReturnsPersistedResult(t *testing.T) {
	item := sampleComplaint()
	repository := &complaintRepositoryStub{item: item}
	value, replayed, err := NewComplaints(repository).Review(context.Background(), model.ReviewCommand{
		ComplaintID: 11, MerchantID: 2001, ShopID: 3001, CommandKey: "review-01", ExpectedVersion: 1,
		Status: model.StatusAccepted, HandleNote: "已核对订单并同意处理",
	})
	if err != nil || replayed || value.ID != 11 || repository.reviewed.Status != model.StatusAccepted {
		t.Fatalf("value=%+v replayed=%v err=%v command=%+v", value, replayed, err, repository.reviewed)
	}
}
