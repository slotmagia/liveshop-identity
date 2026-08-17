package logic

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
	subscriptionmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

func (l *Logic) merchantID(ctx context.Context) (int64, error) {
	claims := authctx.Caller(ctx)
	if claims.MerchantID <= 0 {
		return 0, model.ErrInvalidContext
	}
	return claims.MerchantID, nil
}

func (l *Logic) requireOwner(ctx context.Context) error {
	if authctx.Caller(ctx).PrincipalType != principal.TypeMerchantOwner {
		return model.ErrProtectedOwner
	}
	return nil
}

func (l *Logic) currentAssignment(ctx context.Context, merchantID int64) (subscriptionmodel.Assignment, error) {
	if l.assignments == nil {
		return subscriptionmodel.Assignment{}, subscriptionmodel.ErrAssignmentInvalid
	}
	value, err := l.assignments.Get(ctx, merchantID)
	if errors.Is(err, subscriptionmodel.ErrAssignmentNotFound) {
		return subscriptionmodel.Assignment{MerchantID: merchantID}, nil
	}
	return value, err
}

func (l *Logic) SubscriptionPlans(ctx context.Context) (appmodel.SubscriptionPlans, error) {
	merchantID, err := l.merchantID(ctx)
	if err != nil {
		return appmodel.SubscriptionPlans{}, err
	}
	if l.plans == nil {
		return appmodel.SubscriptionPlans{}, model.ErrUnavailable
	}
	plans, err := l.plans.List(ctx)
	if err != nil {
		return appmodel.SubscriptionPlans{}, err
	}
	current, err := l.currentAssignment(ctx, merchantID)
	if err != nil {
		return appmodel.SubscriptionPlans{}, err
	}
	names, err := l.permissionNameIndex(ctx)
	if err != nil {
		return appmodel.SubscriptionPlans{}, err
	}
	out := appmodel.SubscriptionPlans{Items: []appmodel.SubscriptionPlan{}, CurrentPlanID: current.PlanID}
	for _, plan := range plans {
		if plan.Status != subscriptionmodel.PlanActive {
			continue
		}
		policy, policyErr := l.plans.Policy(ctx, plan.ID)
		if policyErr != nil && !errors.Is(policyErr, subscriptionmodel.ErrPlanNotFound) {
			return appmodel.SubscriptionPlans{}, policyErr
		}
		out.Items = append(out.Items, appmodel.SubscriptionPlan{
			ID: plan.ID, Code: plan.Code, Name: plan.Name, Level: plan.Level, PriceMinor: plan.PriceMinor,
			DurationDays: plan.DurationDays, Description: plan.Description, Default: plan.Default,
			Current: plan.ID == current.PlanID, Buyable: subscriptionmodel.Buyable(plan, current) == nil,
			ProductLimit: policy.ProductLimit, PermissionNames: permissionLabels(policy.PermissionCodes, names),
		})
	}
	sort.SliceStable(out.Items, func(i, j int) bool {
		if out.Items[i].Level != out.Items[j].Level {
			return out.Items[i].Level < out.Items[j].Level
		}
		return out.Items[i].ID < out.Items[j].ID
	})
	return out, nil
}

func (l *Logic) Subscription(ctx context.Context) (appmodel.SubscriptionCurrent, error) {
	merchantID, err := l.merchantID(ctx)
	if err != nil {
		return appmodel.SubscriptionCurrent{}, err
	}
	if l.assignments == nil {
		return appmodel.SubscriptionCurrent{}, subscriptionmodel.ErrAssignmentInvalid
	}
	current, err := l.assignments.Get(ctx, merchantID)
	if err != nil {
		return appmodel.SubscriptionCurrent{}, err
	}
	names, err := l.permissionNameIndex(ctx)
	if err != nil {
		return appmodel.SubscriptionCurrent{}, err
	}
	out := appmodel.SubscriptionCurrent{
		MerchantID: current.MerchantID, PlanID: current.PlanID, PlanCode: current.PlanCode, PlanName: current.PlanName,
		ExpiresAt: current.ExpiresAt, Version: current.Version, PermissionNames: []string{},
	}
	if l.permissionPlans != nil {
		entitlement, entitlementErr := l.permissionPlans.Get(ctx, merchantID)
		if entitlementErr != nil && !errors.Is(entitlementErr, subscription.ErrPermissionEntitlementNotConfigured) {
			return appmodel.SubscriptionCurrent{}, entitlementErr
		}
		if entitlementErr == nil {
			out.PermissionNames = permissionLabels(entitlement.PermissionCodes, names)
		}
	}
	if l.quotas == nil {
		return out, nil
	}
	quota, quotaErr := l.quotas.Get(ctx, merchantID, subscription.CatalogProductsQuota, time.Now().UTC())
	if errors.Is(quotaErr, subscription.ErrNotConfigured) {
		return out, nil
	}
	if quotaErr != nil {
		return appmodel.SubscriptionCurrent{}, quotaErr
	}
	out.QuotaConfigured = true
	out.ProductLimit = quota.Limit
	return out, nil
}

func (l *Logic) SubscriptionPayMethods(ctx context.Context, planID int64) ([]appmodel.PayMethod, error) {
	if _, err := l.merchantID(ctx); err != nil {
		return nil, err
	}
	if l.payments == nil {
		return []appmodel.PayMethod{}, nil
	}
	amount := int64(0)
	if planID > 0 {
		plan, err := l.activePlan(ctx, planID)
		if err != nil {
			return nil, err
		}
		amount = plan.PriceMinor
	}
	methods, err := l.payments.Methods(ctx, amount)
	if err != nil {
		return nil, err
	}
	if methods == nil {
		return []appmodel.PayMethod{}, nil
	}
	return methods, nil
}

func (l *Logic) CreateSubscriptionOrder(ctx context.Context, input appmodel.CreateSubscriptionOrder) (appmodel.SubscriptionOrder, error) {
	if err := l.requireOwner(ctx); err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	merchantID, err := l.merchantID(ctx)
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	if l.payments == nil {
		return appmodel.SubscriptionOrder{}, subscriptionmodel.ErrPaymentUnavailable
	}
	if l.orders == nil {
		return appmodel.SubscriptionOrder{}, subscriptionmodel.ErrOrderInvalid
	}
	plan, err := l.activePlan(ctx, input.PlanID)
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	current, err := l.currentAssignment(ctx, merchantID)
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	if err := subscriptionmodel.Buyable(plan, current); err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	methods, err := l.payments.Methods(ctx, plan.PriceMinor)
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	selected, ok := payMethod(methods, input.ChannelCode)
	if !ok {
		return appmodel.SubscriptionOrder{}, subscriptionmodel.ErrPaymentUnavailable
	}
	order, replayed, err := l.orders.Create(ctx, subscriptionmodel.CreateOrderCommand{
		MerchantID: merchantID, CommandKey: input.CommandKey, PlanID: plan.ID, PlanCode: plan.Code, PlanName: plan.Name,
		PriceMinor: plan.PriceMinor, DurationDays: plan.DurationDays, ChannelCode: selected.ChannelCode,
	})
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	projected := projectSubscriptionOrder(order, selected.DriverKey, "", nil, replayed)
	if order.Paid() {
		return l.finishPaidOrder(ctx, merchantID, order, currentAssignmentExpires(current), selected.DriverKey, "", nil, true)
	}
	charge, err := l.payments.Charge(ctx, appmodel.ChargePayment{
		OrderNo: order.OrderNo, AmountMinor: order.PriceMinor, ChannelCode: selected.ChannelCode,
	})
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	if strings.TrimSpace(charge.PayNo) == "" {
		return projected, subscriptionmodel.ErrPaymentUnavailable
	}
	attached, err := l.orders.AttachPayment(ctx, subscriptionmodel.AttachPaymentCommand{
		MerchantID: merchantID, OrderNo: order.OrderNo, PayNo: charge.PayNo,
	})
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	driver := firstNonEmpty(charge.DriverKey, selected.DriverKey)
	if charge.Paid {
		return l.activatePaidOrder(ctx, merchantID, "activate-"+attached.OrderNo, attached, driver, charge.PayStatus, charge.Payload)
	}
	return projectSubscriptionOrder(attached, driver, charge.PayStatus, charge.Payload, replayed), nil
}

func (l *Logic) SubscriptionOrder(ctx context.Context, orderNo string) (appmodel.SubscriptionOrder, error) {
	merchantID, err := l.merchantID(ctx)
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	return l.settleOrder(ctx, merchantID, strings.TrimSpace(orderNo), "activate-"+strings.TrimSpace(orderNo), false)
}

func (l *Logic) ConfirmSubscriptionOrder(ctx context.Context, input appmodel.ConfirmSubscriptionOrder) (appmodel.SubscriptionOrder, error) {
	if err := l.requireOwner(ctx); err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	merchantID, err := l.merchantID(ctx)
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	return l.settleOrder(ctx, merchantID, input.OrderNo, input.CommandKey, true)
}

func (l *Logic) SubscriptionOrders(ctx context.Context, query appmodel.SubscriptionOrderQuery) (appmodel.SubscriptionOrderPage, error) {
	merchantID, err := l.merchantID(ctx)
	if err != nil {
		return appmodel.SubscriptionOrderPage{}, err
	}
	if l.orders == nil {
		return appmodel.SubscriptionOrderPage{}, subscriptionmodel.ErrOrderInvalid
	}
	page, err := l.orders.List(ctx, subscriptionmodel.OrderQuery{
		MerchantID: merchantID, Status: subscriptionmodel.OrderStatus(strings.TrimSpace(query.Status)),
		Page: query.Page, PageSize: query.PageSize,
	})
	if err != nil {
		return appmodel.SubscriptionOrderPage{}, err
	}
	out := appmodel.SubscriptionOrderPage{
		Items: make([]appmodel.SubscriptionOrder, 0, len(page.Items)),
		Page:  page.Page, PageSize: page.PageSize, Total: page.Total,
		Owner: authctx.Caller(ctx).PrincipalType == principal.TypeMerchantOwner,
	}
	for _, item := range page.Items {
		out.Items = append(out.Items, projectSubscriptionOrder(item, "", "", nil, false))
	}
	return out, nil
}

func (l *Logic) CloseSubscriptionOrder(ctx context.Context, input appmodel.CloseSubscriptionOrder) (appmodel.SubscriptionOrder, error) {
	if err := l.requireOwner(ctx); err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	merchantID, err := l.merchantID(ctx)
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	if l.orders == nil {
		return appmodel.SubscriptionOrder{}, subscriptionmodel.ErrOrderInvalid
	}
	closed, replayed, err := l.orders.Close(ctx, subscriptionmodel.CloseOrderCommand{
		MerchantID: merchantID, CommandKey: input.CommandKey, OrderNo: input.OrderNo,
	})
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	return projectSubscriptionOrder(closed, "", "", nil, replayed), nil
}

func (l *Logic) settleOrder(ctx context.Context, merchantID int64, orderNo, commandKey string, confirmWallet bool) (appmodel.SubscriptionOrder, error) {
	if l.orders == nil {
		return appmodel.SubscriptionOrder{}, subscriptionmodel.ErrOrderInvalid
	}
	order, err := l.orders.Get(ctx, merchantID, orderNo)
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	if order.Paid() {
		current, _ := l.currentAssignment(ctx, merchantID)
		return withExpiry(projectSubscriptionOrder(order, "", "", nil, true), current.ExpiresAt), nil
	}
	if l.payments == nil {
		return projectSubscriptionOrder(order, "", "", nil, false), nil
	}
	if strings.TrimSpace(order.PayNo) == "" {
		return projectSubscriptionOrder(order, "", "", nil, false), nil
	}
	var status appmodel.PaymentStatus
	if confirmWallet {
		status, err = l.payments.ConfirmWallet(ctx, order.PayNo)
	} else {
		status, err = l.payments.Status(ctx, order.PayNo)
	}
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	if !status.Paid {
		return projectSubscriptionOrder(order, status.DriverKey, status.Status, status.Payload, false), nil
	}
	return l.activatePaidOrder(ctx, merchantID, commandKey, order, status.DriverKey, status.Status, status.Payload)
}

func (l *Logic) activatePaidOrder(ctx context.Context, merchantID int64, commandKey string, order subscriptionmodel.Order, driverKey, payStatus string, payload map[string]string) (appmodel.SubscriptionOrder, error) {
	activated, assignment, replayed, err := l.orders.Activate(ctx, subscriptionmodel.ActivateOrderCommand{
		MerchantID: merchantID, CommandKey: commandKey, OrderNo: order.OrderNo, Now: time.Now().UTC(),
	})
	if err != nil {
		return appmodel.SubscriptionOrder{}, err
	}
	if !replayed {
		if err := l.applyPlanEntitlements(ctx, merchantID, commandKey, activated.PlanID); err != nil {
			return appmodel.SubscriptionOrder{}, err
		}
	}
	return l.finishPaidOrder(ctx, merchantID, activated, assignment.ExpiresAt, driverKey, payStatus, payload, replayed)
}

func (l *Logic) finishPaidOrder(ctx context.Context, merchantID int64, order subscriptionmodel.Order, expiresAt, driverKey, payStatus string, payload map[string]string, replayed bool) (appmodel.SubscriptionOrder, error) {
	_ = ctx
	_ = merchantID
	out := projectSubscriptionOrder(order, driverKey, payStatus, payload, replayed)
	out.Activated = true
	out.ExpiresAt = expiresAt
	return out, nil
}

func (l *Logic) applyPlanEntitlements(ctx context.Context, merchantID int64, commandKey string, planID int64) error {
	if l.plans == nil {
		return nil
	}
	policy, err := l.plans.Policy(ctx, planID)
	if err != nil {
		return err
	}
	if l.permissionPlans != nil {
		expected := uint64(0)
		current, currentErr := l.permissionPlans.Get(ctx, merchantID)
		if currentErr != nil && !errors.Is(currentErr, subscription.ErrPermissionEntitlementNotConfigured) {
			return currentErr
		}
		if currentErr == nil {
			expected = current.Revision
		}
		if _, _, err := l.permissionPlans.Apply(ctx, subscription.ApplyPermissionEntitlementCommand{
			MerchantID: merchantID, CommandKey: commandKey + ":permissions", ExpectedRevision: expected, PermissionCodes: policy.PermissionCodes,
		}); err != nil {
			return err
		}
	}
	if l.quotas != nil {
		var expected int64
		current, currentErr := l.quotas.Get(ctx, merchantID, subscription.CatalogProductsQuota, time.Now().UTC())
		if currentErr != nil && !errors.Is(currentErr, subscription.ErrNotConfigured) {
			return currentErr
		}
		if currentErr == nil {
			expected = current.Revision
		}
		if _, _, err := l.quotas.Apply(ctx, subscription.ApplyQuotaCommand{
			MerchantID: merchantID, CommandKey: commandKey + ":quota", Code: subscription.CatalogProductsQuota,
			ExpectedRevision: expected, Limit: policy.ProductLimit, EffectiveFrom: time.Now().UTC(),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (l *Logic) activePlan(ctx context.Context, planID int64) (subscriptionmodel.Plan, error) {
	if l.plans == nil || planID <= 0 {
		return subscriptionmodel.Plan{}, subscriptionmodel.ErrPlanNotFound
	}
	plans, err := l.plans.List(ctx)
	if err != nil {
		return subscriptionmodel.Plan{}, err
	}
	for _, plan := range plans {
		if plan.ID == planID {
			if plan.Status != subscriptionmodel.PlanActive {
				return subscriptionmodel.Plan{}, subscriptionmodel.ErrOrderNotBuyable
			}
			return plan, nil
		}
	}
	return subscriptionmodel.Plan{}, subscriptionmodel.ErrPlanNotFound
}

func (l *Logic) permissionNameIndex(ctx context.Context) (map[string]string, error) {
	if l.authorization == nil {
		return map[string]string{}, nil
	}
	permissions, err := l.authorization.Permissions(ctx, l.domain(ctx))
	if err != nil {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(permissions))
	for _, permission := range permissions {
		if strings.TrimSpace(permission.Name) != "" {
			out[permission.Code] = permission.Name
		}
	}
	return out, nil
}

func permissionLabels(codes []string, names map[string]string) []string {
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if name := strings.TrimSpace(names[code]); name != "" {
			out = append(out, name)
			continue
		}
		out = append(out, code)
	}
	return out
}

func payMethod(methods []appmodel.PayMethod, channelCode string) (appmodel.PayMethod, bool) {
	wanted := strings.TrimSpace(channelCode)
	for _, method := range methods {
		if strings.TrimSpace(method.ChannelCode) == wanted && wanted != "" {
			return method, true
		}
	}
	return appmodel.PayMethod{}, false
}

func projectSubscriptionOrder(order subscriptionmodel.Order, driverKey, payStatus string, payload map[string]string, replayed bool) appmodel.SubscriptionOrder {
	if payload == nil {
		payload = map[string]string{}
	}
	return appmodel.SubscriptionOrder{
		OrderNo: order.OrderNo, PlanID: order.PlanID, PlanCode: order.PlanCode, PlanName: order.PlanName,
		PriceMinor: order.PriceMinor, DurationDays: order.DurationDays, Status: string(order.Status),
		PayNo: order.PayNo, ChannelCode: order.ChannelCode, DriverKey: driverKey, PayStatus: payStatus,
		Payload: payload, Activated: order.Paid(), ExpiresAt: "", CreatedAt: order.CreatedAt, PaidAt: order.PaidAt, Replayed: replayed,
	}
}

func withExpiry(value appmodel.SubscriptionOrder, expiresAt string) appmodel.SubscriptionOrder {
	value.ExpiresAt = expiresAt
	value.Activated = value.Status == string(subscriptionmodel.OrderPaid)
	return value
}

func currentAssignmentExpires(value subscriptionmodel.Assignment) string {
	return value.ExpiresAt
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
