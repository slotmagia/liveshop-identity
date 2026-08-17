// Package authctx carries the verified module session across one request. It is
// framework-free so logic can read the caller without importing a transport.
package authctx

import (
	"context"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
)

type key struct{}

func With(ctx context.Context, claims modulesession.Claims) context.Context {
	return context.WithValue(ctx, key{}, claims)
}

// Caller returns the verified session. The zero value grants nothing, so code
// reached without the middleware fails closed instead of running unauthorized.
func Caller(ctx context.Context) modulesession.Claims {
	claims, _ := ctx.Value(key{}).(modulesession.Claims)
	return claims
}

// Has reports whether the caller was granted every listed permission. An empty
// list is a programming error and is denied rather than silently allowed.
func Has(ctx context.Context, permissions ...string) bool {
	if len(permissions) == 0 {
		return false
	}
	return modulesession.HasPermissions(Caller(ctx), permissions...)
}

// Scope returns the data boundary IAM resolved for a resource. Queries must
// narrow by it; a browser cannot widen it.
func Scope(ctx context.Context, resource string) modulesession.DataScope {
	return modulesession.ScopeFor(Caller(ctx), resource)
}
