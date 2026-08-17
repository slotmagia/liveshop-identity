package mysql

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
)

func TestMerchantLifecycleIsIdempotentVersionedAndTerminal(t *testing.T) {
	database := integrationDatabase(t)
	repository := NewRepository(database)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	account := "owner" + suffix[len(suffix)-10:]
	keyPrefix := "merchant-admin-" + suffix
	var created model.CreateResult
	t.Cleanup(func() {
		if created.Merchant.ID == 0 {
			return
		}
		subject := model.SubjectForMerchant(created.Merchant.ID)
		_, _ = database.Exec(`DELETE FROM identity_member_shop WHERE shop_id=?`, created.ShopID)
		_, _ = database.Exec(`DELETE FROM identity_organization_membership WHERE member_id IN (SELECT member_id FROM identity_workforce_member WHERE merchant_id=?)`, created.Merchant.ID)
		_, _ = database.Exec(`DELETE FROM identity_credential WHERE subject=?`, subject)
		_, _ = database.Exec(`DELETE FROM identity_workforce_member WHERE merchant_id=?`, created.Merchant.ID)
		_, _ = database.Exec(`DELETE FROM identity_organization_unit WHERE organization_id IN (SELECT organization_id FROM identity_organization WHERE merchant_id=?)`, created.Merchant.ID)
		_, _ = database.Exec(`DELETE FROM identity_organization WHERE merchant_id=?`, created.Merchant.ID)
		_, _ = database.Exec(`DELETE FROM identity_authorization_domain WHERE domain_type='MERCHANT' AND domain_id=?`, created.Merchant.ID)
		_, _ = database.Exec(`DELETE FROM identity_subject WHERE subject=?`, subject)
		_, _ = database.Exec(`DELETE FROM identity_shop WHERE merchant_id=?`, created.Merchant.ID)
		_, _ = database.Exec(`DELETE FROM identity_merchant WHERE merchant_id=?`, created.Merchant.ID)
		_, _ = database.Exec(`DELETE FROM identity_merchant_command WHERE command_key LIKE ?`, keyPrefix+"%")
	})

	create, err := (model.CreateCommand{CommandKey: keyPrefix + "-create", Account: account, Password: "password1", Name: "开户商户"}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	created, replayed, err := repository.CreateMerchant(context.Background(), create)
	if err != nil || replayed || created.Merchant.ID <= 0 || created.ShopID <= 0 || created.Account != account {
		t.Fatalf("created=%+v replayed=%v err=%v", created, replayed, err)
	}
	repeated, replayed, err := repository.CreateMerchant(context.Background(), create)
	if err != nil || !replayed || repeated.Merchant.ID != created.Merchant.ID || repeated.ShopID != created.ShopID {
		t.Fatalf("repeated=%+v replayed=%v err=%v", repeated, replayed, err)
	}
	changed := create
	changed.Name = "不同输入"
	if _, _, err := repository.CreateMerchant(context.Background(), changed); !errors.Is(err, model.ErrIdempotency) {
		t.Fatalf("changed create error=%v", err)
	}

	page, err := repository.ListMerchantPage(context.Background(), model.Query{Keyword: account, Page: 1, PageSize: 20})
	if err != nil || page.Total < 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}

	update, err := (model.UpdateCommand{MerchantID: created.Merchant.ID, CommandKey: keyPrefix + "-update", ExpectedVersion: 99, Name: "改名", Status: model.StatusDisabled}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.UpdateMerchant(context.Background(), update); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("version conflict error=%v", err)
	}
	update.ExpectedVersion = created.Merchant.Version
	updated, replayed, err := repository.UpdateMerchant(context.Background(), update)
	if err != nil || replayed || updated.Status != model.StatusDisabled || updated.Version != created.Merchant.Version+1 {
		t.Fatalf("updated=%+v replayed=%v err=%v", updated, replayed, err)
	}

	closeCommand, err := (model.CloseCommand{MerchantID: created.Merchant.ID, CommandKey: keyPrefix + "-close", ExpectedVersion: updated.Version}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	closed, replayed, err := repository.CloseMerchant(context.Background(), closeCommand)
	if err != nil || replayed || closed.Status != model.StatusClosed {
		t.Fatalf("closed=%+v replayed=%v err=%v", closed, replayed, err)
	}
	repeatedClose, replayed, err := repository.CloseMerchant(context.Background(), closeCommand)
	if err != nil || !replayed || repeatedClose.Status != model.StatusClosed {
		t.Fatalf("repeated close=%+v replayed=%v err=%v", repeatedClose, replayed, err)
	}
	if _, _, err := repository.UpdateMerchant(context.Background(), update); !errors.Is(err, model.ErrClosed) {
		t.Fatalf("update after close error=%v", err)
	}
	page, err = repository.ListMerchantPage(context.Background(), model.Query{Keyword: account, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range page.Items {
		if item.ID == created.Merchant.ID {
			t.Fatalf("closed merchant leaked into page: %+v", item)
		}
	}
}
