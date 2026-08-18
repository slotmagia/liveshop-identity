package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

func merchOwnerContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{
		Subject: "owner-1", PrincipalType: principal.TypeMerchantOwner, OrganizationID: 9, MerchantID: 7,
	})
}

func merchStaffContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{
		Subject: "staff-1", PrincipalType: principal.TypeMerchantStaff, OrganizationID: 9, MerchantID: 7,
	})
}

func TestMemberReadsAndWritesAreOwnerOnly(t *testing.T) {
	logic := New(nil, nil, nil, nil, nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if _, err := logic.Members(merchStaffContext(), appmodel.MemberQuery{}); !errors.Is(err, model.ErrProtectedOwner) {
		t.Fatalf("list error=%v", err)
	}
	if _, err := logic.MemberOptions(merchStaffContext()); !errors.Is(err, model.ErrProtectedOwner) {
		t.Fatalf("options error=%v", err)
	}
	if _, err := logic.Member(merchStaffContext(), "staff-1"); !errors.Is(err, model.ErrProtectedOwner) {
		t.Fatalf("get error=%v", err)
	}
	if _, err := logic.UpdateMember(merchStaffContext(), appmodel.UpdateMember{Subject: "staff-1", DisplayName: "Ada", MemberType: "STAFF", ShopIDs: []int64{101}, RoleIDs: []int64{1}}); !errors.Is(err, model.ErrProtectedOwner) {
		t.Fatalf("update error=%v", err)
	}
	if _, err := logic.CreateMember(merchStaffContext(), appmodel.CreateMember{DisplayName: "Ada", MemberType: "STAFF", Username: "ada", Password: "password1", ShopIDs: []int64{101}, RoleIDs: []int64{1}}); !errors.Is(err, model.ErrProtectedOwner) {
		t.Fatalf("create error=%v", err)
	}
}

func TestCreateMemberRejectsShortPasswordAndStaffWithoutShops(t *testing.T) {
	logic := New(nil, nil, nil, nil, nil, nil, nil, nil, nil, Subscription{}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if _, err := logic.CreateMember(merchOwnerContext(), appmodel.CreateMember{DisplayName: "Ada", MemberType: "STAFF", Username: "ada", Password: "short", ShopIDs: []int64{101}, RoleIDs: []int64{1}}); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("short password error=%v", err)
	}
	if _, err := logic.CreateMember(merchOwnerContext(), appmodel.CreateMember{DisplayName: "Ada", MemberType: "STAFF", Username: "ada", Password: "password1", RoleIDs: []int64{1}}); !errors.Is(err, model.ErrInvalidAssignment) {
		t.Fatalf("staff without shops error=%v", err)
	}
	if _, err := logic.CreateMember(merchOwnerContext(), appmodel.CreateMember{DisplayName: "Ada", MemberType: "STAFF", Username: "ada", Password: "password1", ShopIDs: []int64{101}}); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("missing roles error=%v", err)
	}
	if _, err := logic.CreateMember(merchOwnerContext(), appmodel.CreateMember{DisplayName: "Ada", MemberType: "ANCHOR", Username: "ada", Password: "password1", ShopIDs: []int64{101, 102}, RoleIDs: []int64{1}}); !errors.Is(err, model.ErrInvalidAssignment) {
		t.Fatalf("anchor with two shops error=%v", err)
	}
}
