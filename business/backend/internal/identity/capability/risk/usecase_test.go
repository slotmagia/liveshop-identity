package risk

import (
	"context"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/risk/model"
)

type eventRepositoryStub struct {
	query model.Query
	page  model.Page
}

func (s *eventRepositoryStub) List(_ context.Context, query model.Query) (model.Page, error) {
	s.query = query
	return s.page, nil
}

func sampleEvent() model.Event {
	return model.Event{
		ID: 1, MerchantID: 2001, ShopID: 3001, VisitorID: "v-1001", Nickname: "Ada",
		RoomID: 9001, Reason: "spam", ScoreBefore: 10, ScoreAfterDecay: 10, ScoreDelta: 20, ScoreAfter: 30,
		CurrentScore: 30, CurrentLevel: model.LevelLow, VisitorStatus: model.StatusWatch,
		CreatedAt: time.Unix(1, 0).UTC(),
	}
}

func TestListRequiresShopScope(t *testing.T) {
	if _, err := NewEvents(&eventRepositoryStub{}).List(context.Background(), model.Query{Page: 1, PageSize: 20}); err != model.ErrInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestListPreservesShopScopeAndDefaultPage(t *testing.T) {
	repository := &eventRepositoryStub{page: model.Page{Page: 1, PageSize: 20, Total: 1, Items: []model.Event{sampleEvent()}}}
	page, err := NewEvents(repository).List(context.Background(), model.Query{MerchantID: 2001, ShopID: 3001})
	if err != nil || page.Total != 1 || repository.query.MerchantID != 2001 || repository.query.ShopID != 3001 || repository.query.Page != 1 || repository.query.PageSize != 20 {
		t.Fatalf("page=%+v query=%+v err=%v", page, repository.query, err)
	}
}

func TestListRejectsForeignShopRows(t *testing.T) {
	item := sampleEvent()
	item.ShopID = 4001
	repository := &eventRepositoryStub{page: model.Page{Page: 1, PageSize: 20, Total: 1, Items: []model.Event{item}}}
	if _, err := NewEvents(repository).List(context.Background(), model.Query{MerchantID: 2001, ShopID: 3001, Page: 1, PageSize: 20}); err != model.ErrInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestListUnavailableWithoutRepository(t *testing.T) {
	if _, err := NewEvents(nil).List(context.Background(), model.Query{MerchantID: 2001, ShopID: 3001}); err != model.ErrUnavailable {
		t.Fatalf("error=%v", err)
	}
}
