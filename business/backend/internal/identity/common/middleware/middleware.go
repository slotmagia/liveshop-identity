// Package middleware is the only place in identity that turns a credential
// into a verified request context.
package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/lvtuopen-ai/kernel-go/logctx"
	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/kernel-go/requestmeta"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type CurrentAuthorizationValidator interface {
	ValidateCurrentAuthorization(context.Context, modulesession.Claims) error
}

func RequireCurrentAuthorization(validator CurrentAuthorizationValidator) ghttp.HandlerFunc {
	return func(request *ghttp.Request) {
		if request.Method == http.MethodGet || request.Method == http.MethodHead {
			request.Middleware.Next()
			return
		}
		if validator == nil || validator.ValidateCurrentAuthorization(request.GetCtx(), authctx.Caller(request.GetCtx())) != nil {
			deny(request, http.StatusForbidden, "current authorization is required", "authorization-revision")
			return
		}
		request.Middleware.Next()
	}
}

// ModuleID must match module.json. A session minted for another module is
// rejected even when its signature is valid.
const ModuleID = "identity"

// Audience is what the Gateway puts in a session issued for this module.
const Audience = "liveshop-module:" + ModuleID

// RequestMetadata gives every request a stable id so a failure in the logs can
// be traced back to the caller.
func RequestMetadata(request *ghttp.Request) {
	requestID := requestmeta.Ensure(request.Request)
	request.Response.Header().Set(requestmeta.HeaderRequestID, requestID)
	request.SetCtx(requestmeta.Context(request.GetCtx(), requestID))
	request.Middleware.Next()
}

// ModuleSession admits only an Identity-issued capability minted for this module,
// this surface and this route. A nil verifier denies everything, so a
// misassembled process fails closed rather than open.
func ModuleSession(verifier *modulesession.Verifier, surface string) ghttp.HandlerFunc {
	return func(request *ghttp.Request) {
		token, ok := modulesession.Bearer(request.Header.Get("Authorization"))
		if !ok || verifier == nil {
			deny(request, http.StatusUnauthorized, "module session is required", surface)
			return
		}
		// Verify already binds issuer, audience and lifetime. What remains is
		// what only this module knows: which module, which surface, and whether
		// the contribution was granted this very route.
		claims, err := verifier.Verify(token)
		if err != nil ||
			claims.ModuleID != ModuleID ||
			claims.Surface != surface ||
			request.Header.Get("X-Liveshop-Surface") != surface ||
			!modulesession.AllowsRequest(claims, request.Method, request.URL.Path) {
			deny(request, http.StatusForbidden, "invalid or out-of-scope module session", surface)
			return
		}
		request.SetCtx(authctx.With(request.GetCtx(), claims))
		request.Middleware.Next()
	}
}

// RequirePermission gates one capability. Install it per capability group so a
// grant for one capability can never reach another.
func RequirePermission(permission string) ghttp.HandlerFunc {
	return func(request *ghttp.Request) {
		if !authctx.Has(request.GetCtx(), permission) {
			deny(request, http.StatusForbidden, "required permission is not granted", permission)
			return
		}
		request.Middleware.Next()
	}
}

func RequireSurface(surface string) ghttp.HandlerFunc {
	return func(request *ghttp.Request) {
		if request.Header.Get("X-Liveshop-Surface") != surface {
			deny(request, http.StatusForbidden, "surface header is required", surface)
			return
		}
		request.Middleware.Next()
	}
}

// RequireShopperSession admits a guest or signed-in customer in the shop realm.
func RequireShopperSession(request *ghttp.Request) {
	claims := authctx.Caller(request.GetCtx())
	if (claims.PrincipalType != principal.TypeGuest && claims.PrincipalType != principal.TypeCustomer) ||
		claims.Realm != principal.RealmCustomer || claims.MerchantID <= 0 || claims.ShopID <= 0 {
		deny(request, http.StatusForbidden, "customer or guest shop session is required", "shopper")
		return
	}
	request.Middleware.Next()
}

// RequireCustomerSession admits only a signed-in customer in the shop realm.
func RequireCustomerSession(request *ghttp.Request) {
	claims := authctx.Caller(request.GetCtx())
	if claims.PrincipalType != principal.TypeCustomer ||
		claims.Realm != principal.RealmCustomer || claims.MerchantID <= 0 || claims.ShopID <= 0 {
		deny(request, http.StatusForbidden, "customer shop session is required", "customer")
		return
	}
	request.Middleware.Next()
}

// deny logs the decision before answering. An access denial the operator cannot
// see is indistinguishable from a broken client.
func deny(request *ghttp.Request, status int, message, subject string) {
	logctx.FromContext(request.GetCtx()).Warn("identity authorization denied",
		"status", status, "reason", message, "subject", subject,
		"method", request.Method, "path", request.URL.Path)
	web.WriteFailure(request, status, errors.New(message))
	request.ExitAll()
}
