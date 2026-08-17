package subscription

import (
	"context"
	"strings"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
)

type OrderRepository interface {
	CreateOrder(context.Context, model.CreateOrderCommand) (model.Order, bool, error)
	GetOrder(context.Context, int64, string) (model.Order, error)
	ListOrders(context.Context, model.OrderQuery) (model.OrderPage, error)
	AttachPayment(context.Context, model.AttachPaymentCommand) (model.Order, error)
	Activate(context.Context, model.ActivateOrderCommand) (model.Order, model.Assignment, bool, error)
	Close(context.Context, model.CloseOrderCommand) (model.Order, bool, error)
}

type Orders struct{ repository OrderRepository }

func NewOrders(repository OrderRepository) *Orders { return &Orders{repository: repository} }

func (o *Orders) Create(ctx context.Context, command model.CreateOrderCommand) (model.Order, bool, error) {
	if o == nil || o.repository == nil {
		return model.Order{}, false, model.ErrOrderInvalid
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Order{}, false, err
	}
	return o.repository.CreateOrder(ctx, normalized)
}

func (o *Orders) Get(ctx context.Context, merchantID int64, orderNo string) (model.Order, error) {
	if o == nil || o.repository == nil {
		return model.Order{}, model.ErrOrderInvalid
	}
	orderNo = strings.TrimSpace(orderNo)
	if merchantID <= 0 || orderNo == "" {
		return model.Order{}, model.ErrOrderInvalid
	}
	return o.repository.GetOrder(ctx, merchantID, orderNo)
}

func (o *Orders) AttachPayment(ctx context.Context, command model.AttachPaymentCommand) (model.Order, error) {
	if o == nil || o.repository == nil {
		return model.Order{}, model.ErrOrderInvalid
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Order{}, err
	}
	return o.repository.AttachPayment(ctx, normalized)
}

func (o *Orders) Activate(ctx context.Context, command model.ActivateOrderCommand) (model.Order, model.Assignment, bool, error) {
	if o == nil || o.repository == nil {
		return model.Order{}, model.Assignment{}, false, model.ErrOrderInvalid
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Order{}, model.Assignment{}, false, err
	}
	return o.repository.Activate(ctx, normalized)
}

func (o *Orders) List(ctx context.Context, query model.OrderQuery) (model.OrderPage, error) {
	if o == nil || o.repository == nil {
		return model.OrderPage{}, model.ErrOrderInvalid
	}
	normalized, err := query.Normalize()
	if err != nil {
		return model.OrderPage{}, err
	}
	return o.repository.ListOrders(ctx, normalized)
}

func (o *Orders) Close(ctx context.Context, command model.CloseOrderCommand) (model.Order, bool, error) {
	if o == nil || o.repository == nil {
		return model.Order{}, false, model.ErrOrderInvalid
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Order{}, false, err
	}
	return o.repository.Close(ctx, normalized)
}
