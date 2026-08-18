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

type stubComplaintRepo struct {
	query    fulfillmentmodel.Query
	page     fulfillmentmodel.Page
	item     fulfillmentmodel.Complaint
	reviewed fulfillmentmodel.ReviewCommand
	err      error
}

func (s *stubComplaintRepo) List(_ context.Context, query fulfillmentmodel.Query) (fulfillmentmodel.Page, error) {
	s.query = query
	if s.err != nil {
		return fulfillmentmodel.Page{}, s.err
	}
	if s.page.Page == 0 {
		return fulfillmentmodel.Page{Items: []fulfillmentmodel.Complaint{}, Page: query.Page, PageSize: query.PageSize}, nil
	}
	return s.page, nil
}

func (s *stubComplaintRepo) Get(_ context.Context, _, _, _ int64) (fulfillmentmodel.Complaint, error) {
	if s.err != nil {
		return fulfillmentmodel.Complaint{}, s.err
	}
	return s.item, nil
}

func (s *stubComplaintRepo) Review(_ context.Context, command fulfillmentmodel.ReviewCommand) (fulfillmentmodel.Complaint, bool, error) {
	s.reviewed = command
	if s.err != nil {
		return fulfillmentmodel.Complaint{}, false, s.err
	}
	return s.item, false, nil
}

func merchComplaintLogic(repo *stubComplaintRepo) *Logic {
	return New(nil, nil, nil, nil, nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil, nil, fulfillment.NewComplaints(repo), nil, nil, nil, nil)
}

func merchComplaintContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{MerchantID: 2001, ShopID: 3001})
}

func sampleMerchComplaint(status fulfillmentmodel.Status, note string, handled *time.Time, version uint64) fulfillmentmodel.Complaint {
	return fulfillmentmodel.Complaint{
		ID: 11, MerchantID: 2001, ShopID: 3001, CustomerSubject: "cust-1001",
		TargetType: fulfillmentmodel.TargetOrder, TargetID: 8801, ReasonCode: "quality", Content: "商品与描述不符",
		Status: status, HandleNote: note, Version: version,
		CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(2, 0).UTC(), HandledAt: handled,
	}
}

func TestComplaintsRequireShopContext(t *testing.T) {
	if _, err := merchComplaintLogic(&stubComplaintRepo{}).Complaints(context.Background(), appmodel.ComplaintQuery{}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestComplaintsUseSessionShopAndProjectRows(t *testing.T) {
	repo := &stubComplaintRepo{page: fulfillmentmodel.Page{Page: 1, PageSize: 20, Total: 1, Items: []fulfillmentmodel.Complaint{
		sampleMerchComplaint(fulfillmentmodel.StatusOpen, "", nil, 1),
	}}}
	page, err := merchComplaintLogic(repo).Complaints(merchComplaintContext(), appmodel.ComplaintQuery{CustomerSubject: "cust-1001", Status: "OPEN", TargetType: "ORDER", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if repo.query.MerchantID != 2001 || repo.query.ShopID != 3001 || repo.query.CustomerSubject != "cust-1001" || repo.query.Status != fulfillmentmodel.StatusOpen {
		t.Fatalf("query=%+v", repo.query)
	}
	if page.Total != 1 || page.Items[0].CustomerSubject != "cust-1001" || page.Items[0].Status != "OPEN" || page.Items[0].CreatedAt != "1970-01-01T00:00:01Z" {
		t.Fatalf("page=%+v", page)
	}
}

func TestComplaintsRejectClosedShop(t *testing.T) {
	_, err := merchComplaintLogic(&stubComplaintRepo{err: fulfillmentmodel.ErrNotFound}).Complaints(merchComplaintContext(), appmodel.ComplaintQuery{})
	if !errors.Is(err, fulfillmentmodel.ErrNotFound) {
		t.Fatalf("error=%v", err)
	}
}

func TestReviewComplaintUsesSessionShop(t *testing.T) {
	handled := time.Unix(2, 0).UTC()
	repo := &stubComplaintRepo{item: sampleMerchComplaint(fulfillmentmodel.StatusAccepted, "已核对订单并同意处理", &handled, 2)}
	value, err := merchComplaintLogic(repo).ReviewComplaint(merchComplaintContext(), appmodel.ReviewComplaint{
		ComplaintID: 11, CommandKey: "review-0001", ExpectedVersion: 1, Status: "ACCEPTED", HandleNote: "已核对订单并同意处理",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.reviewed.MerchantID != 2001 || repo.reviewed.ShopID != 3001 || repo.reviewed.ComplaintID != 11 || repo.reviewed.Status != fulfillmentmodel.StatusAccepted {
		t.Fatalf("command=%+v", repo.reviewed)
	}
	if value.Complaint.Status != "ACCEPTED" || value.Replayed || value.Complaint.HandleNote != "已核对订单并同意处理" {
		t.Fatalf("value=%+v", value)
	}
}
