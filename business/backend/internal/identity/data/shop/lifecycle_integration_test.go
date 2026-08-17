package mysql

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

func TestShopLifecycleIsMerchantScopedIdempotentAndKeepsLastShop(t *testing.T) {
	database := integrationDatabase(t)
	repository := NewRepository(database)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	base := time.Now().UnixNano() / 1000
	merchantID, otherMerchantID, firstShop := base, base+1, base+10
	subject := fmt.Sprintf("sub_shop_%d", merchantID)
	commandPrefix := "shop-lifecycle-" + suffix
	var createdID int64

	if _, err := database.Exec(`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(?,?,'ACTIVE',1),(?,?,'ACTIVE',1)`,
		merchantID, "Lifecycle Merchant", otherMerchantID, "Other Merchant"); err != nil {
		t.Fatal(err)
	}
	org, err := database.Exec(`INSERT INTO identity_organization(organization_type,merchant_id,name,status,version) VALUES('MERCHANT',?,?,'ACTIVE',1)`, merchantID, "Lifecycle Merchant")
	if err != nil {
		t.Fatal(err)
	}
	organizationID, err := org.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO identity_subject(subject,realm,principal_type,display_name,status,version)
VALUES(?,'MERCHANT','MERCHANT_OWNER','店主','ACTIVE',1)`, subject); err != nil {
		t.Fatal(err)
	}
	member, err := database.Exec(`INSERT INTO identity_workforce_member(organization_id,merchant_id,subject,member_type,status,access_version)
VALUES(?,?,?,'OWNER','ACTIVE',1)`, organizationID, merchantID, subject)
	if err != nil {
		t.Fatal(err)
	}
	memberID, err := member.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO identity_shop(shop_id,merchant_id,code,subdomain,name,default_locale,currency,status,version)
VALUES(?,?,?,?,?,'zh-CN','CNY','ACTIVE',1)`, firstShop, merchantID, fmt.Sprintf("s%d", firstShop), fmt.Sprintf("sub-%d", firstShop), "首店"); err != nil {
		t.Fatal(err)
	}
	categoryCode := "lc_" + suffix[len(suffix)-12:]
	if _, err := database.Exec(`INSERT INTO identity_shop_category(code,name,icon,sort_order,status,version) VALUES(?,?,'🧪',1,'ACTIVE',1)`, categoryCode, "生命周期品类"); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = database.Exec(`DELETE FROM identity_member_shop WHERE member_id=? OR shop_id IN (?,?)`, memberID, firstShop, createdID)
		_, _ = database.Exec(`DELETE FROM identity_shop_command WHERE command_key LIKE ?`, commandPrefix+"%")
		_, _ = database.Exec(`DELETE FROM identity_shop WHERE merchant_id IN (?,?)`, merchantID, otherMerchantID)
		_, _ = database.Exec(`DELETE FROM identity_workforce_member WHERE member_id=?`, memberID)
		_, _ = database.Exec(`DELETE FROM identity_organization WHERE organization_id=?`, organizationID)
		_, _ = database.Exec(`DELETE FROM identity_subject WHERE subject=?`, subject)
		_, _ = database.Exec(`DELETE FROM identity_merchant WHERE merchant_id IN (?,?)`, merchantID, otherMerchantID)
		_, _ = database.Exec(`DELETE FROM identity_shop_category WHERE code=?`, categoryCode)
	})

	create, err := (model.CreateCommand{
		CommandKey: commandPrefix + "-create", MerchantID: merchantID, Name: "二店", Subdomain: "second-" + suffix[len(suffix)-10:], Currency: "USD", CategoryCode: categoryCode,
	}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	created, replayed, err := repository.CreateShop(context.Background(), create)
	if err != nil || replayed || created.MerchantID != merchantID || created.Code != model.ShopCodeForID(created.ID) || created.Currency != "USD" || created.CategoryCode != categoryCode || created.DefaultLocale != model.DefaultLocale {
		t.Fatalf("created=%+v replayed=%v err=%v", created, replayed, err)
	}
	createdID = created.ID
	var ownerAssignments int
	if err := database.QueryRow(`SELECT COUNT(*) FROM identity_member_shop WHERE member_id=? AND shop_id=? AND assignment_kind='OPERATE' AND status='ACTIVE'`, memberID, created.ID).Scan(&ownerAssignments); err != nil || ownerAssignments != 1 {
		t.Fatalf("owner assignment=%d err=%v", ownerAssignments, err)
	}
	repeated, replayed, err := repository.CreateShop(context.Background(), create)
	if err != nil || !replayed || repeated.ID != created.ID {
		t.Fatalf("replay=%+v replayed=%v err=%v", repeated, replayed, err)
	}
	changed := create
	changed.Name = "不同输入"
	if _, _, err := repository.CreateShop(context.Background(), changed); !errors.Is(err, model.ErrIdempotency) {
		t.Fatalf("changed create error=%v", err)
	}

	page, err := repository.ListManagedShops(context.Background(), model.Query{MerchantID: merchantID, Keyword: "二店", Page: 1, PageSize: 20})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != created.ID {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	current, err := repository.GetManagedShop(context.Background(), merchantID, created.ID)
	if err != nil || current.ID != created.ID || current.MerchantID != merchantID {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	if _, err := repository.GetManagedShop(context.Background(), otherMerchantID, created.ID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("foreign get error=%v", err)
	}

	update, err := (model.UpdateCommand{ShopID: created.ID, MerchantID: merchantID, CommandKey: commandPrefix + "-update", ExpectedVersion: 99, Name: "二店改名", Subdomain: created.Subdomain}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.UpdateShop(context.Background(), update); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("version conflict error=%v", err)
	}
	update.ExpectedVersion = created.Version
	updated, replayed, err := repository.UpdateShop(context.Background(), update)
	if err != nil || replayed || updated.Name != "二店改名" || updated.CategoryCode != categoryCode || updated.Currency != "USD" || updated.Version != created.Version+1 {
		t.Fatalf("updated=%+v replayed=%v err=%v", updated, replayed, err)
	}
	if _, _, err := repository.UpdateShop(context.Background(), model.UpdateCommand{
		ShopID: created.ID, MerchantID: otherMerchantID, CommandKey: commandPrefix + "-foreign", ExpectedVersion: updated.Version, Name: "越权", Subdomain: updated.Subdomain,
	}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("foreign update error=%v", err)
	}

	disabled, replayed, err := repository.SetShopEnabled(context.Background(), model.SetEnabledCommand{
		ShopID: created.ID, MerchantID: merchantID, CommandKey: commandPrefix + "-disable", ExpectedVersion: updated.Version, Enabled: false,
	})
	if err != nil || replayed || disabled.Status != model.StatusDisabled {
		t.Fatalf("disabled=%+v replayed=%v err=%v", disabled, replayed, err)
	}
	same, replayed, err := repository.SetShopEnabled(context.Background(), model.SetEnabledCommand{
		ShopID: created.ID, MerchantID: merchantID, CommandKey: commandPrefix + "-disable-again", ExpectedVersion: disabled.Version, Enabled: false,
	})
	if err != nil || replayed || same.Version != disabled.Version || same.Status != model.StatusDisabled {
		t.Fatalf("already disabled=%+v replayed=%v err=%v", same, replayed, err)
	}

	closed, replayed, err := repository.CloseShop(context.Background(), model.CloseCommand{
		ShopID: created.ID, MerchantID: merchantID, CommandKey: commandPrefix + "-close", ExpectedVersion: disabled.Version,
	})
	if err != nil || replayed || closed.Status != model.StatusClosed {
		t.Fatalf("closed=%+v replayed=%v err=%v", closed, replayed, err)
	}
	if _, _, err := repository.CloseShop(context.Background(), model.CloseCommand{
		ShopID: firstShop, MerchantID: merchantID, CommandKey: commandPrefix + "-last", ExpectedVersion: 1,
	}); !errors.Is(err, model.ErrLastShop) {
		t.Fatalf("last shop error=%v", err)
	}

	if _, err := database.Exec(`UPDATE identity_merchant SET status='CLOSED' WHERE merchant_id=?`, merchantID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.CreateShop(context.Background(), model.CreateCommand{
		CommandKey: commandPrefix + "-closed-merchant", MerchantID: merchantID, Name: "关户后", Subdomain: "closed-" + suffix[len(suffix)-10:],
	}); !errors.Is(err, model.ErrMerchantClosed) {
		t.Fatalf("closed merchant create error=%v", err)
	}
}
