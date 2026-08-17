package customer_service

import (
	"context"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service/model"
)

type accountRepositoryStub struct {
	query model.Query
	save  model.SaveCommand
	page  model.Page
}

func (s *accountRepositoryStub) List(_ context.Context, query model.Query) (model.Page, error) {
	s.query = query
	return s.page, nil
}
func (s *accountRepositoryStub) Save(_ context.Context, command model.SaveCommand) (model.Account, bool, error) {
	s.save = command
	return command.Account, false, nil
}
func (s *accountRepositoryStub) Delete(_ context.Context, command model.DeleteCommand) (model.DeleteResult, bool, error) {
	return model.DeleteResult{ID: command.AccountID, Deleted: true, Version: command.ExpectedVersion}, false, nil
}

func TestListAllowsEmptyMerchantAndShopFilters(t *testing.T) {
	now := time.Now()
	repository := &accountRepositoryStub{page: model.Page{Page: 1, PageSize: 20, Total: 1, Items: []model.Account{{
		ID: 1, MerchantID: 10, ShopID: 20, Platform: "telegram", Account: "support",
		Status: model.StatusActive, Version: 1, CreatedAt: now, UpdatedAt: now,
	}}}}
	page, err := NewAccounts(repository).List(context.Background(), model.Query{Page: 1, PageSize: 20})
	if err != nil || page.Total != 1 || repository.query.MerchantID != 0 || repository.query.ShopID != 0 {
		t.Fatalf("page=%+v query=%+v err=%v", page, repository.query, err)
	}
}

func TestListPreservesExplicitMerchantAndShopScope(t *testing.T) {
	now := time.Now()
	repository := &accountRepositoryStub{page: model.Page{Page: 1, PageSize: 20, Total: 1, Items: []model.Account{{
		ID: 1, MerchantID: 10, ShopID: 20, Platform: "telegram", Account: "support",
		Status: model.StatusActive, Version: 1, CreatedAt: now, UpdatedAt: now,
	}}}}
	page, err := NewAccounts(repository).List(context.Background(), model.Query{MerchantID: 10, ShopID: 20, Page: 1, PageSize: 20})
	if err != nil || page.Total != 1 || repository.query.MerchantID != 10 || repository.query.ShopID != 20 {
		t.Fatalf("page=%+v query=%+v err=%v", page, repository.query, err)
	}
}

func TestSaveNormalizesBeforeRepository(t *testing.T) {
	repository := &accountRepositoryStub{}
	_, _, err := NewAccounts(repository).Save(context.Background(), model.SaveCommand{CommandKey: "customer-service-create", Account: model.Account{
		MerchantID: 10, ShopID: 20, Platform: " WhatsApp ", Account: " support ", Status: model.StatusActive,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if repository.save.Account.Platform != "whatsapp" || repository.save.Account.Account != "support" {
		t.Fatalf("command=%+v", repository.save)
	}
}
