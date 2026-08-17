package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/riskevents"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type RiskEventQueryController struct{ service service.Merch }

func NewRiskEventQuery(s service.Merch) *RiskEventQueryController {
	return &RiskEventQueryController{service: s}
}

func (c *RiskEventQueryController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	value, err := c.service.RiskEvents(ctx, appmodel.RiskEventQuery{
		VisitorID: request.VisitorID, RoomID: request.RoomID, Reason: request.Reason,
		VisitorStatus: request.VisitorStatus, Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRes{Page: value.Page, PageSize: value.PageSize, Total: value.Total, Items: []api.Event{}}
	for _, item := range value.Items {
		out.Items = append(out.Items, api.Event{
			ID: item.ID, VisitorID: item.VisitorID, Nickname: item.Nickname, RoomID: item.RoomID, Reason: item.Reason,
			ScoreBefore: item.ScoreBefore, ScoreAfterDecay: item.ScoreAfterDecay, ScoreDelta: item.ScoreDelta,
			ScoreAfter: item.ScoreAfter, CurrentScore: item.CurrentScore, CurrentLevel: item.CurrentLevel,
			VisitorStatus: item.VisitorStatus, CreatedAt: item.CreatedAt,
		})
	}
	return &out, nil
}
