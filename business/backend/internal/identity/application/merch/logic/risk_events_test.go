package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/risk"
	riskmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/risk/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type stubRiskRepo struct {
	query riskmodel.Query
	page  riskmodel.Page
	err   error
}

func (s *stubRiskRepo) List(_ context.Context, query riskmodel.Query) (riskmodel.Page, error) {
	s.query = query
	if s.err != nil {
		return riskmodel.Page{}, s.err
	}
	if s.page.Page == 0 {
		return riskmodel.Page{Items: []riskmodel.Event{}, Page: query.Page, PageSize: query.PageSize}, nil
	}
	return s.page, nil
}

func merchRiskLogic(repo *stubRiskRepo) *Logic {
	return New(nil, nil, nil, nil, nil, nil, nil, nil, nil, Subscription{}, nil, nil, risk.NewEvents(repo))
}

func merchRiskContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{MerchantID: 2001, ShopID: 3001})
}

func TestRiskEventsRequireShopContext(t *testing.T) {
	if _, err := merchRiskLogic(&stubRiskRepo{}).RiskEvents(context.Background(), appmodel.RiskEventQuery{}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestRiskEventsUseSessionShopAndProjectRows(t *testing.T) {
	repo := &stubRiskRepo{page: riskmodel.Page{Page: 1, PageSize: 20, Total: 1, Items: []riskmodel.Event{{
		ID: 3, MerchantID: 2001, ShopID: 3001, VisitorID: "v-1001", Nickname: "Ada", RoomID: 9002, Reason: "flood",
		ScoreBefore: 48, ScoreAfterDecay: 44, ScoreDelta: 44, ScoreAfter: 88, CurrentScore: 88,
		CurrentLevel: riskmodel.LevelHigh, VisitorStatus: riskmodel.StatusRestricted,
		CreatedAt: time.Unix(1, 0).UTC(),
	}}}}
	page, err := merchRiskLogic(repo).RiskEvents(merchRiskContext(), appmodel.RiskEventQuery{VisitorID: "v-1001", RoomID: 9002, Reason: "flood", VisitorStatus: "RESTRICTED", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if repo.query.MerchantID != 2001 || repo.query.ShopID != 3001 || repo.query.VisitorID != "v-1001" || repo.query.RoomID != 9002 {
		t.Fatalf("query=%+v", repo.query)
	}
	if page.Total != 1 || page.Items[0].VisitorID != "v-1001" || page.Items[0].CurrentLevel != "HIGH" || page.Items[0].CreatedAt != "1970-01-01T00:00:01Z" {
		t.Fatalf("page=%+v", page)
	}
}

func TestRiskEventsRejectClosedShop(t *testing.T) {
	_, err := merchRiskLogic(&stubRiskRepo{err: riskmodel.ErrNotFound}).RiskEvents(merchRiskContext(), appmodel.RiskEventQuery{})
	if !errors.Is(err, riskmodel.ErrNotFound) {
		t.Fatalf("error=%v", err)
	}
}
