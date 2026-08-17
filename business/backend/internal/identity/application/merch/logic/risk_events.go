package logic

import (
	"context"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	riskmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/risk/model"
)

func (l *Logic) RiskEvents(ctx context.Context, query appmodel.RiskEventQuery) (appmodel.RiskEventPage, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.RiskEventPage{}, err
	}
	if l.riskEvents == nil {
		return appmodel.RiskEventPage{}, model.ErrUnavailable
	}
	page, err := l.riskEvents.List(ctx, riskmodel.Query{
		MerchantID: merchantID, ShopID: shopID, VisitorID: query.VisitorID, RoomID: query.RoomID,
		Reason: query.Reason, VisitorStatus: riskmodel.Status(query.VisitorStatus),
		Page: query.Page, PageSize: query.PageSize,
	})
	if err != nil {
		return appmodel.RiskEventPage{}, err
	}
	out := appmodel.RiskEventPage{
		Items: make([]appmodel.RiskEvent, 0, len(page.Items)), Page: page.Page, PageSize: page.PageSize, Total: page.Total,
	}
	for _, item := range page.Items {
		out.Items = append(out.Items, appmodel.RiskEvent{
			ID: item.ID, VisitorID: item.VisitorID, Nickname: item.Nickname, RoomID: item.RoomID, Reason: item.Reason,
			ScoreBefore: item.ScoreBefore, ScoreAfterDecay: item.ScoreAfterDecay, ScoreDelta: item.ScoreDelta,
			ScoreAfter: item.ScoreAfter, CurrentScore: item.CurrentScore, CurrentLevel: string(item.CurrentLevel),
			VisitorStatus: string(item.VisitorStatus), CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}
