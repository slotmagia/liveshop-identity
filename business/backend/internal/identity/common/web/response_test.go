package web

import (
	"errors"
	"net/http"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

func TestAuthorizationDomainErrorsHaveExplicitHTTPStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{model.ErrAuthorizationDenied, http.StatusForbidden},
		{model.ErrAuthorizationInvalid, http.StatusBadRequest},
		{model.ErrAuthorizationNotFound, http.StatusNotFound},
		{model.ErrAuthorizationConflict, http.StatusConflict},
		{model.ErrSystemRoleProtected, http.StatusForbidden},
		{model.ErrRegistryProjectionStale, http.StatusServiceUnavailable},
		{model.ErrEntitlementUnavailable, http.StatusServiceUnavailable},
		{model.ErrInvalidCredential, http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.err.Error(), func(t *testing.T) {
			var got *HTTPError
			if !errors.As(Failure(test.err), &got) || got.Status != test.status {
				t.Fatalf("Failure(%v) status = %#v, want %d", test.err, got, test.status)
			}
		})
	}
}
