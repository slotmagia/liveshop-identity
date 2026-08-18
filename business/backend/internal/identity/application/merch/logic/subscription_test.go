package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
	subscriptionmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type stubPlanRepo struct {
	plans  []subscriptionmodel.Plan
	policy subscriptionmodel.PlanPolicy
}

func (s stubPlanRepo) ListPlans(context.Context) ([]subscriptionmodel.Plan, error) {
	return s.plans, nil
}
func (stubPlanRepo) SavePlan(context.Context, subscriptionmodel.SavePlanCommand) (subscriptionmodel.Plan, bool, error) {
	return subscriptionmodel.Plan{}, false, nil
}
func (stubPlanRepo) RetirePlan(context.Context, subscriptionmodel.RetirePlanCommand) (subscriptionmodel.Plan, bool, error) {
	return subscriptionmodel.Plan{}, false, nil
}
func (s stubPlanRepo) GetPlanPolicy(_ context.Context, planID int64) (subscriptionmodel.PlanPolicy, error) {
	s.policy.PlanID = planID
	return s.policy, nil
}
func (stubPlanRepo) SavePlanPolicy(context.Context, subscriptionmodel.SavePlanPolicyCommand) (subscriptionmodel.PlanPolicy, bool, error) {
	return subscriptionmodel.PlanPolicy{}, false, nil
}

type stubAssignmentRepo struct{ current subscriptionmodel.Assignment }

func (s stubAssignmentRepo) GetAssignment(context.Context, int64) (subscriptionmodel.Assignment, error) {
	if s.current.PlanID == 0 {
		return subscriptionmodel.Assignment{}, subscriptionmodel.ErrAssignmentNotFound
	}
	return s.current, nil
}
func (stubAssignmentRepo) Assign(context.Context, subscriptionmodel.AssignCommand) (subscriptionmodel.Assignment, bool, error) {
	return subscriptionmodel.Assignment{}, false, nil
}

type stubOrderRepo struct {
	items    []subscriptionmodel.Order
	closed   subscriptionmodel.Order
	replayed bool
	closeErr error
}

func (stubOrderRepo) CreateOrder(context.Context, subscriptionmodel.CreateOrderCommand) (subscriptionmodel.Order, bool, error) {
	return subscriptionmodel.Order{}, false, nil
}
func (stubOrderRepo) GetOrder(context.Context, int64, string) (subscriptionmodel.Order, error) {
	return subscriptionmodel.Order{}, subscriptionmodel.ErrOrderNotFound
}
func (s stubOrderRepo) ListOrders(_ context.Context, query subscriptionmodel.OrderQuery) (subscriptionmodel.OrderPage, error) {
	items := make([]subscriptionmodel.Order, 0, len(s.items))
	for _, item := range s.items {
		if query.Status == "" || item.Status == query.Status {
			items = append(items, item)
		}
	}
	return subscriptionmodel.OrderPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: int64(len(items))}, nil
}
func (stubOrderRepo) AttachPayment(context.Context, subscriptionmodel.AttachPaymentCommand) (subscriptionmodel.Order, error) {
	return subscriptionmodel.Order{}, subscriptionmodel.ErrOrderNotFound
}
func (stubOrderRepo) Activate(context.Context, subscriptionmodel.ActivateOrderCommand) (subscriptionmodel.Order, subscriptionmodel.Assignment, bool, error) {
	return subscriptionmodel.Order{}, subscriptionmodel.Assignment{}, false, subscriptionmodel.ErrOrderNotFound
}
func (s stubOrderRepo) Close(context.Context, subscriptionmodel.CloseOrderCommand) (subscriptionmodel.Order, bool, error) {
	if s.closeErr != nil {
		return subscriptionmodel.Order{}, false, s.closeErr
	}
	if s.closed.OrderNo == "" {
		return subscriptionmodel.Order{}, false, subscriptionmodel.ErrOrderNotFound
	}
	return s.closed, s.replayed, nil
}

func merchSubscriptionLogic(plans []subscriptionmodel.Plan, current subscriptionmodel.Assignment) *Logic {
	return merchSubscriptionOrderLogic(plans, current, stubOrderRepo{})
}

func merchSubscriptionOrderLogic(plans []subscriptionmodel.Plan, current subscriptionmodel.Assignment, orders stubOrderRepo) *Logic {
	return New(nil, nil, nil, nil, nil, nil, nil, nil, nil, Subscription{
		Plans:       subscription.NewPlans(stubPlanRepo{plans: plans, policy: subscriptionmodel.PlanPolicy{Revision: 1}}),
		Assignments: subscription.NewAssignments(stubAssignmentRepo{current: current}),
		Orders:      subscription.NewOrders(orders),
	}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

func TestSubscriptionPlansHideInactiveAndMarkBuyable(t *testing.T) {
	logic := merchSubscriptionLogic([]subscriptionmodel.Plan{
		{ID: 1, Code: "free", Name: "免费版", Status: subscriptionmodel.PlanActive, Default: true, PriceMinor: 0},
		{ID: 2, Code: "pro", Name: "专业版", Status: subscriptionmodel.PlanActive, PriceMinor: 9900, DurationDays: 30},
		{ID: 3, Code: "old", Name: "已下架", Status: subscriptionmodel.PlanDisabled, PriceMinor: 100},
	}, subscriptionmodel.Assignment{MerchantID: 7, PlanID: 1, PlanCode: "free", PlanName: "免费版"})
	value, err := logic.SubscriptionPlans(merchOwnerContext())
	if err != nil {
		t.Fatal(err)
	}
	if value.CurrentPlanID != 1 || len(value.Items) != 2 {
		t.Fatalf("plans=%+v", value)
	}
	if value.Items[0].ID != 1 || value.Items[0].Buyable || !value.Items[0].Current {
		t.Fatalf("free=%+v", value.Items[0])
	}
	if value.Items[1].ID != 2 || !value.Items[1].Buyable || value.Items[1].Current {
		t.Fatalf("pro=%+v", value.Items[1])
	}
}

func TestCreateSubscriptionOrderIsOwnerOnlyAndFailClosedWithoutCollector(t *testing.T) {
	logic := merchSubscriptionLogic([]subscriptionmodel.Plan{
		{ID: 2, Code: "pro", Name: "专业版", Status: subscriptionmodel.PlanActive, PriceMinor: 9900, DurationDays: 30},
	}, subscriptionmodel.Assignment{MerchantID: 7, PlanID: 1})
	_, err := logic.CreateSubscriptionOrder(merchStaffContext(), appmodel.CreateSubscriptionOrder{
		CommandKey: "order-key-0001", PlanID: 2, ChannelCode: "wallet",
	})
	if !errors.Is(err, model.ErrProtectedOwner) {
		t.Fatalf("staff error=%v", err)
	}
	_, err = logic.CreateSubscriptionOrder(merchOwnerContext(), appmodel.CreateSubscriptionOrder{
		CommandKey: "order-key-0001", PlanID: 2, ChannelCode: "wallet",
	})
	if !errors.Is(err, subscriptionmodel.ErrPaymentUnavailable) {
		t.Fatalf("missing collector error=%v", err)
	}
}

func TestSubscriptionCurrentRequiresMerchantContext(t *testing.T) {
	logic := merchSubscriptionLogic(nil, subscriptionmodel.Assignment{})
	if _, err := logic.Subscription(context.Background()); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
	_, err := logic.Subscription(authctx.With(context.Background(), modulesession.Claims{
		Subject: "owner-1", PrincipalType: principal.TypeMerchantOwner, MerchantID: 7,
	}))
	if !errors.Is(err, subscriptionmodel.ErrAssignmentNotFound) {
		t.Fatalf("missing assignment error=%v", err)
	}
}

func TestSubscriptionOrdersRequireMerchantContextAndExposeOwner(t *testing.T) {
	logic := merchSubscriptionOrderLogic(nil, subscriptionmodel.Assignment{}, stubOrderRepo{
		items: []subscriptionmodel.Order{{OrderNo: "SUB-1", Status: subscriptionmodel.OrderPending, PlanName: "专业版", CreatedAt: "2026-08-18T00:00:00Z"}},
	})
	if _, err := logic.SubscriptionOrders(context.Background(), appmodel.SubscriptionOrderQuery{}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("missing merchant error=%v", err)
	}
	staff, err := logic.SubscriptionOrders(merchStaffContext(), appmodel.SubscriptionOrderQuery{})
	if err != nil || staff.Owner || len(staff.Items) != 1 || staff.Items[0].OrderNo != "SUB-1" {
		t.Fatalf("staff page=%+v err=%v", staff, err)
	}
	owner, err := logic.SubscriptionOrders(merchOwnerContext(), appmodel.SubscriptionOrderQuery{})
	if err != nil || !owner.Owner || owner.Items[0].CreatedAt == "" {
		t.Fatalf("owner page=%+v err=%v", owner, err)
	}
}

func TestCloseSubscriptionOrderIsOwnerOnly(t *testing.T) {
	logic := merchSubscriptionOrderLogic(nil, subscriptionmodel.Assignment{}, stubOrderRepo{
		closed: subscriptionmodel.Order{OrderNo: "SUB-1", Status: subscriptionmodel.OrderCancelled},
	})
	_, err := logic.CloseSubscriptionOrder(merchStaffContext(), appmodel.CloseSubscriptionOrder{
		CommandKey: "close-key-0001", OrderNo: "SUB-1",
	})
	if !errors.Is(err, model.ErrProtectedOwner) {
		t.Fatalf("staff error=%v", err)
	}
	closed, err := logic.CloseSubscriptionOrder(merchOwnerContext(), appmodel.CloseSubscriptionOrder{
		CommandKey: "close-key-0001", OrderNo: "SUB-1",
	})
	if err != nil || closed.OrderNo != "SUB-1" || closed.Status != string(subscriptionmodel.OrderCancelled) {
		t.Fatalf("close=%+v err=%v", closed, err)
	}
}
