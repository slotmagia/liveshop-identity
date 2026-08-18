package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service"
	customerservicemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type stubCustomerServiceRepo struct {
	query   customerservicemodel.Query
	save    customerservicemodel.SaveCommand
	delete  customerservicemodel.DeleteCommand
	page    customerservicemodel.Page
	account customerservicemodel.Account
	result  customerservicemodel.DeleteResult
}

func (s *stubCustomerServiceRepo) List(_ context.Context, query customerservicemodel.Query) (customerservicemodel.Page, error) {
	s.query = query
	if s.page.Page == 0 {
		return customerservicemodel.Page{Items: []customerservicemodel.Account{}, Page: query.Page, PageSize: query.PageSize}, nil
	}
	return s.page, nil
}

func (s *stubCustomerServiceRepo) Save(_ context.Context, command customerservicemodel.SaveCommand) (customerservicemodel.Account, bool, error) {
	s.save = command
	return s.account, false, nil
}

func (s *stubCustomerServiceRepo) Delete(_ context.Context, command customerservicemodel.DeleteCommand) (customerservicemodel.DeleteResult, bool, error) {
	s.delete = command
	return s.result, false, nil
}

func merchCustomerLogic(repo *stubCustomerServiceRepo) *Logic {
	return New(nil, nil, nil, nil, shop.NewDirectory(stubShopRepo{}), nil, nil, nil, nil, Subscription{}, nil, nil, nil, customer_service.NewAccounts(repo), nil, nil, nil, nil, nil)
}

func merchCustomerOwnerContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{
		PrincipalType: principal.TypeMerchantOwner, MerchantID: 2001, ShopID: 3001,
	})
}

func merchCustomerStaffContext() context.Context {
	return authctx.With(context.Background(), modulesession.Claims{
		PrincipalType: principal.TypeMerchantStaff, MerchantID: 2001, ShopID: 3001,
	})
}

func TestCustomerAccountShopsRequireMerchantContext(t *testing.T) {
	if _, err := merchCustomerLogic(&stubCustomerServiceRepo{}).CustomerAccountShops(context.Background()); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestCustomerAccountShopsUseSessionMerchant(t *testing.T) {
	values, err := merchCustomerLogic(&stubCustomerServiceRepo{}).CustomerAccountShops(merchCustomerOwnerContext())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].ShopID != 3001 || values[0].MerchantID != 2001 {
		t.Fatalf("shops=%+v", values)
	}
}

func TestCustomerAccountsForceSessionMerchant(t *testing.T) {
	repo := &stubCustomerServiceRepo{page: customerservicemodel.Page{Page: 1, PageSize: 20, Total: 1, Items: []customerservicemodel.Account{{
		ID: 9, MerchantID: 2001, ShopID: 3001, Platform: "whatsapp", Account: "support", Nickname: "客服",
		Status: customerservicemodel.StatusActive, Version: 1, CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(2, 0).UTC(),
	}}}}
	page, err := merchCustomerLogic(repo).CustomerAccounts(merchCustomerOwnerContext(), appmodel.CustomerAccountQuery{ShopID: 3001, Platform: "whatsapp", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if repo.query.MerchantID != 2001 || repo.query.ShopID != 3001 || repo.query.Platform != "whatsapp" {
		t.Fatalf("query=%+v", repo.query)
	}
	if page.Total != 1 || page.Items[0].Account != "support" || page.Items[0].CreatedAt != "1970-01-01T00:00:01Z" {
		t.Fatalf("page=%+v", page)
	}
}

func TestCustomerAccountsRejectForeignShop(t *testing.T) {
	if _, err := merchCustomerLogic(&stubCustomerServiceRepo{}).CustomerAccounts(merchCustomerOwnerContext(), appmodel.CustomerAccountQuery{ShopID: 4001}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestCustomerAccountsStaffCannotLeaveSessionShop(t *testing.T) {
	if _, err := merchCustomerLogic(&stubCustomerServiceRepo{}).CustomerAccounts(merchCustomerStaffContext(), appmodel.CustomerAccountQuery{ShopID: 4001}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestSaveCustomerAccountUsesSessionMerchant(t *testing.T) {
	repo := &stubCustomerServiceRepo{account: customerservicemodel.Account{
		ID: 9, MerchantID: 2001, ShopID: 3001, Platform: "whatsapp", Account: "support", Status: customerservicemodel.StatusActive, Version: 1,
	}}
	_, err := merchCustomerLogic(repo).SaveCustomerAccount(merchCustomerOwnerContext(), appmodel.SaveCustomerAccount{
		CommandKey: "customer-create-01", ShopID: 3001, Platform: "WhatsApp", Account: "support", Status: "ACTIVE",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.save.Account.MerchantID != 2001 || repo.save.Account.ShopID != 3001 || repo.save.Account.Platform != "whatsapp" {
		t.Fatalf("save=%+v", repo.save)
	}
}

func TestCustomerAccountsOwnerListsMerchantWhenShopOmitted(t *testing.T) {
	repo := &stubCustomerServiceRepo{}
	if _, err := merchCustomerLogic(repo).CustomerAccounts(merchCustomerOwnerContext(), appmodel.CustomerAccountQuery{Page: 1, PageSize: 20}); err != nil {
		t.Fatal(err)
	}
	if repo.query.MerchantID != 2001 || repo.query.ShopID != 0 {
		t.Fatalf("query=%+v", repo.query)
	}
}

func TestDeleteCustomerAccountRejectsForeignShop(t *testing.T) {
	if _, err := merchCustomerLogic(&stubCustomerServiceRepo{}).DeleteCustomerAccount(merchCustomerOwnerContext(), appmodel.DeleteCustomerAccount{
		AccountID: 9, ShopID: 4001, CommandKey: "customer-delete-01", ExpectedVersion: 1,
	}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}
