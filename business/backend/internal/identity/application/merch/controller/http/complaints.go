package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/complaints"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type ComplaintQueryController struct{ service service.Merch }

func NewComplaintQuery(s service.Merch) *ComplaintQueryController {
	return &ComplaintQueryController{service: s}
}

func (c *ComplaintQueryController) List(ctx context.Context, request *api.ListReq) (*api.ListRes, error) {
	value, err := c.service.Complaints(ctx, appmodel.ComplaintQuery{
		CustomerSubject: request.CustomerSubject, Status: request.Status, TargetType: request.TargetType,
		Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	out := api.ListRes{Page: value.Page, PageSize: value.PageSize, Total: value.Total, Items: []api.Complaint{}}
	for _, item := range value.Items {
		out.Items = append(out.Items, wireComplaint(item))
	}
	return &out, nil
}

func (c *ComplaintQueryController) Get(ctx context.Context, request *api.GetReq) (*api.GetRes, error) {
	value, err := c.service.Complaint(ctx, request.ComplaintId)
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.GetRes{Complaint: wireComplaint(value)}, nil
}

type ComplaintWriteController struct{ service service.Merch }

func NewComplaintWrite(s service.Merch) *ComplaintWriteController {
	return &ComplaintWriteController{service: s}
}

func (c *ComplaintWriteController) Review(ctx context.Context, request *api.ReviewReq) (*api.ReviewRes, error) {
	value, err := c.service.ReviewComplaint(ctx, appmodel.ReviewComplaint{
		ComplaintID: request.ComplaintId, CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
		Status: request.Status, HandleNote: request.HandleNote,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &api.ReviewRes{Complaint: wireComplaint(value.Complaint), Replayed: value.Replayed}, nil
}

func wireComplaint(value appmodel.Complaint) api.Complaint {
	return api.Complaint{
		ID: value.ID, CustomerSubject: value.CustomerSubject, TargetType: value.TargetType, TargetID: value.TargetID,
		ReasonCode: value.ReasonCode, Content: value.Content, Status: value.Status, HandleNote: value.HandleNote,
		Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, HandledAt: value.HandledAt,
	}
}
