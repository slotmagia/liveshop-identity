package grpcdirectory

import (
	"errors"
	"testing"

	"github.com/lvtuopen-ai/kernel-go/principal"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSubjectMessagePreservesLegacyUID(t *testing.T) {
	message := subjectMessage(model.Subject{
		ID:            "customer-subject",
		Realm:         principal.RealmCustomer,
		PrincipalType: principal.TypeCustomer,
		DisplayName:   "Customer",
		LegacyUID:     726,
		Status:        model.StatusActive,
		Version:       3,
	})
	if message.GetLegacyUid() != 726 {
		t.Fatalf("legacy uid = %d, want 726", message.GetLegacyUid())
	}
}

func TestShopResolutionResponseKeepsCurrencyOutsideIdentityTuple(t *testing.T) {
	resolved := model.ShopResolution{
		Context:  model.ShopContext{MerchantID: 7, ShopID: 101},
		Currency: "CNY",
		Status:   model.StatusActive,
		Version:  4,
	}
	message := shopResolutionResponse(resolved)
	if message.GetCurrency() != "CNY" || message.GetContext().GetShopId() != 101 {
		t.Fatalf("shop response lost authoritative facts: %+v", message)
	}
	if fields := message.GetContext().ProtoReflect().Descriptor().Fields(); fields.ByName("currency") != nil {
		t.Fatal("currency must not be part of the ShopContext identity tuple")
	}
}

func TestInvalidShopCurrencyFailsClosedAtGRPCBoundary(t *testing.T) {
	err := grpcError(errors.Join(model.ErrInvalidShopCurrency, errors.New("lowercase currency")))
	if got := status.Code(err); got != codes.Unavailable {
		t.Fatalf("gRPC code = %s, want %s", got, codes.Unavailable)
	}
}
