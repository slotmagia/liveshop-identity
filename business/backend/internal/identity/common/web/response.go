// Package web provides the identity HTTP response envelope and the single
// mapping from application errors to HTTP status codes.
package web

import (
	"errors"
	"net/http"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/lvtuopen-ai/kernel-go/logctx"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	authmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/auth/model"
	customerservicemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service/model"
	fulfillmentmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
	merchantmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
	governancemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
	riskmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/risk/model"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
	subscriptionmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
)

type HTTPError struct {
	Status int
	Cause  error
	// Internal carries the disclosed-to-logs-only origin of the failure.
	Internal error
}

func (e *HTTPError) Error() string { return e.Cause.Error() }
func (e *HTTPError) Unwrap() error { return e.Cause }

// domainStatus is the single HTTP projection of the domain outcomes. Add one
// row per domain error; never derive a status from another transport.
var domainStatus = []struct {
	err    error
	status int
}{
	{model.ErrUnavailable, http.StatusServiceUnavailable},
	{model.ErrNotFound, http.StatusNotFound},
	{model.ErrConflict, http.StatusConflict},
	{model.ErrIdempotencyConflict, http.StatusConflict},
	{model.ErrInvalidCredential, http.StatusForbidden},
	{model.ErrInvalidContext, http.StatusForbidden},
	{model.ErrInactive, http.StatusForbidden},
	{model.ErrProtectedOwner, http.StatusForbidden},
	{model.ErrLastActiveOperator, http.StatusForbidden},
	{model.ErrInvalidAssignment, http.StatusBadRequest},
	{model.ErrAuthorizationDenied, http.StatusForbidden},
	{model.ErrAuthorizationInvalid, http.StatusBadRequest},
	{model.ErrAuthorizationNotFound, http.StatusNotFound},
	{model.ErrAuthorizationConflict, http.StatusConflict},
	{model.ErrSystemRoleProtected, http.StatusForbidden},
	{model.ErrRegistryProjectionStale, http.StatusServiceUnavailable},
	{model.ErrEntitlementUnavailable, http.StatusServiceUnavailable},
	{subscriptionmodel.ErrPlanNotFound, http.StatusNotFound},
	{subscriptionmodel.ErrPlanConflict, http.StatusConflict},
	{subscriptionmodel.ErrPlanIdempotency, http.StatusConflict},
	{subscriptionmodel.ErrPlanInvalid, http.StatusBadRequest},
	{subscriptionmodel.ErrPlanDefaultRequired, http.StatusConflict},
	{subscriptionmodel.ErrPlanPermissionInactive, http.StatusBadRequest},
	{merchantmodel.ErrUnavailable, http.StatusServiceUnavailable},
	{merchantmodel.ErrInvalidMerchant, http.StatusServiceUnavailable},
	{merchantmodel.ErrNotFound, http.StatusNotFound},
	{merchantmodel.ErrConflict, http.StatusConflict},
	{merchantmodel.ErrIdempotency, http.StatusConflict},
	{merchantmodel.ErrInvalid, http.StatusBadRequest},
	{merchantmodel.ErrClosed, http.StatusConflict},
	{subscriptionmodel.ErrAssignmentNotFound, http.StatusNotFound},
	{subscriptionmodel.ErrAssignmentConflict, http.StatusConflict},
	{subscriptionmodel.ErrAssignmentIdempotency, http.StatusConflict},
	{subscriptionmodel.ErrAssignmentInvalid, http.StatusBadRequest},
	{subscriptionmodel.ErrOrderNotFound, http.StatusNotFound},
	{subscriptionmodel.ErrOrderConflict, http.StatusConflict},
	{subscriptionmodel.ErrOrderIdempotency, http.StatusConflict},
	{subscriptionmodel.ErrOrderInvalid, http.StatusBadRequest},
	{subscriptionmodel.ErrOrderNotBuyable, http.StatusConflict},
	{subscriptionmodel.ErrPaymentUnavailable, http.StatusServiceUnavailable},
	{shopmodel.ErrUnavailable, http.StatusServiceUnavailable},
	{shopmodel.ErrInvalidMerchantID, http.StatusBadRequest},
	{shopmodel.ErrInvalidShop, http.StatusServiceUnavailable},
	{shopmodel.ErrNotFound, http.StatusNotFound},
	{shopmodel.ErrConflict, http.StatusConflict},
	{shopmodel.ErrIdempotency, http.StatusConflict},
	{shopmodel.ErrInvalid, http.StatusBadRequest},
	{shopmodel.ErrLastShop, http.StatusConflict},
	{shopmodel.ErrMerchantClosed, http.StatusConflict},
	{shopmodel.ErrCategoryInactive, http.StatusBadRequest},
	{shopmodel.ErrCategoryNotFound, http.StatusNotFound},
	{shopmodel.ErrCategoryConflict, http.StatusConflict},
	{shopmodel.ErrCategoryIdempotency, http.StatusConflict},
	{shopmodel.ErrCategoryInvalid, http.StatusBadRequest},
	{shopmodel.ErrPrivacyNotFound, http.StatusNotFound},
	{shopmodel.ErrPrivacyConflict, http.StatusConflict},
	{shopmodel.ErrPrivacyIdempotency, http.StatusConflict},
	{shopmodel.ErrPrivacyInvalid, http.StatusBadRequest},
	{shopmodel.ErrPrivacyRestricted, http.StatusForbidden},
	{shopmodel.ErrPolicyNotFound, http.StatusNotFound},
	{shopmodel.ErrPolicyConflict, http.StatusConflict},
	{shopmodel.ErrPolicyIdempotency, http.StatusConflict},
	{shopmodel.ErrPolicyInvalid, http.StatusBadRequest},
	{shopmodel.ErrPolicyRestricted, http.StatusForbidden},
	{shopmodel.ErrAppNotFound, http.StatusNotFound},
	{shopmodel.ErrAppConflict, http.StatusConflict},
	{shopmodel.ErrAppIdempotency, http.StatusConflict},
	{shopmodel.ErrAppInvalid, http.StatusBadRequest},
	{shopmodel.ErrAppRestricted, http.StatusForbidden},
	{shopmodel.ErrDomainNotFound, http.StatusNotFound},
	{shopmodel.ErrDomainConflict, http.StatusConflict},
	{shopmodel.ErrDomainIdempotency, http.StatusConflict},
	{shopmodel.ErrDomainInvalid, http.StatusBadRequest},
	{shopmodel.ErrDomainRestricted, http.StatusForbidden},
	{shopmodel.ErrDomainUnavailable, http.StatusServiceUnavailable},
	{customerservicemodel.ErrUnavailable, http.StatusServiceUnavailable},
	{customerservicemodel.ErrNotFound, http.StatusNotFound},
	{customerservicemodel.ErrConflict, http.StatusConflict},
	{customerservicemodel.ErrIdempotency, http.StatusConflict},
	{customerservicemodel.ErrInvalid, http.StatusBadRequest},
	{riskmodel.ErrUnavailable, http.StatusServiceUnavailable},
	{riskmodel.ErrNotFound, http.StatusNotFound},
	{riskmodel.ErrInvalid, http.StatusBadRequest},
	{fulfillmentmodel.ErrUnavailable, http.StatusServiceUnavailable},
	{fulfillmentmodel.ErrNotFound, http.StatusNotFound},
	{fulfillmentmodel.ErrInvalid, http.StatusBadRequest},
	{fulfillmentmodel.ErrConflict, http.StatusConflict},
	{fulfillmentmodel.ErrIdempotency, http.StatusConflict},
	{fulfillmentmodel.ErrAftersaleUnavailable, http.StatusServiceUnavailable},
	{fulfillmentmodel.ErrAftersaleNotFound, http.StatusNotFound},
	{fulfillmentmodel.ErrAftersaleInvalid, http.StatusBadRequest},
	{fulfillmentmodel.ErrAftersaleConflict, http.StatusConflict},
	{fulfillmentmodel.ErrAftersaleIdempotency, http.StatusConflict},
	{fulfillmentmodel.ErrShippingUnavailable, http.StatusServiceUnavailable},
	{fulfillmentmodel.ErrShippingNotFound, http.StatusNotFound},
	{fulfillmentmodel.ErrShippingInvalid, http.StatusBadRequest},
	{fulfillmentmodel.ErrShippingConflict, http.StatusConflict},
	{fulfillmentmodel.ErrShippingIdempotency, http.StatusConflict},
	{fulfillmentmodel.ErrShippingRestricted, http.StatusForbidden},
	{governancemodel.ErrUnavailable, http.StatusServiceUnavailable},
	{governancemodel.ErrNotFound, http.StatusNotFound},
	{governancemodel.ErrConflict, http.StatusConflict},
	{governancemodel.ErrIdempotency, http.StatusConflict},
	{governancemodel.ErrInvalid, http.StatusBadRequest},
	{authmodel.ErrUnavailable, http.StatusServiceUnavailable},
	{authmodel.ErrInvalid, http.StatusBadRequest},
	{authmodel.ErrNotFound, http.StatusNotFound},
	{authmodel.ErrExpired, http.StatusConflict},
	{authmodel.ErrDeliveryFailed, http.StatusServiceUnavailable},
}

// Failure maps an application error to its transport status. Anything that is
// not a known domain outcome becomes an unavailable dependency so storage and
// driver details never reach the client.
func Failure(err error) error {
	if err == nil {
		return nil
	}
	var httpError *HTTPError
	if errors.As(err, &httpError) {
		return err
	}
	for _, entry := range domainStatus {
		if errors.Is(err, entry.err) {
			return &HTTPError{Status: entry.status, Cause: err}
		}
	}
	return &HTTPError{Status: http.StatusServiceUnavailable, Cause: model.ErrUnavailable, Internal: err}
}

func ResponseHandler(request *ghttp.Request) {
	request.Middleware.Next()
	if request.Response.BufferLength() > 0 {
		return
	}
	if err := request.GetError(); err != nil {
		status := http.StatusInternalServerError
		var httpError *HTTPError
		if errors.As(err, &httpError) {
			status = httpError.Status
			// The client is told only the domain outcome. Without this line the
			// real cause would be lost entirely, leaving nothing to debug.
			if httpError.Internal != nil {
				logctx.FromContext(request.GetCtx()).Error("identity request failed",
					"status", status, "method", request.Method,
					"path", request.URL.Path, "error", httpError.Internal)
			}
		}
		WriteFailure(request, status, err)
		return
	}
	request.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
	request.Response.WriteJson(map[string]any{"code": 0, "data": request.GetHandlerResponse()})
}

func WriteFailure(request *ghttp.Request, status int, err error) {
	request.Response.ClearBuffer()
	request.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
	request.Response.WriteHeader(status)
	request.Response.WriteJson(map[string]any{"code": status * 100, "message": err.Error()})
}
