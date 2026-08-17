package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service/model"
)

func integrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("IDENTITY_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("IDENTITY_MYSQL_TEST_DSN is not set")
	}
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.Ping(); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestCustomerServiceCommandsAreScopedVersionedAndIdempotent(t *testing.T) {
	database := integrationDatabase(t)
	repository := NewRepository(database)
	base := time.Now().UnixNano() / 1000
	merchantID, otherMerchantID := base, base+1
	shopID, otherShopID := base+10, base+11
	keyPrefix := fmt.Sprintf("customer-service-%d", base)
	var accountID int64
	t.Cleanup(func() {
		_, _ = database.Exec(`DELETE FROM identity_customer_service_account WHERE shop_id IN (?,?)`, shopID, otherShopID)
		_, _ = database.Exec(`DELETE FROM identity_customer_service_command WHERE command_key LIKE ?`, keyPrefix+"%")
		_, _ = database.Exec(`DELETE FROM identity_shop WHERE shop_id IN (?,?)`, shopID, otherShopID)
		_, _ = database.Exec(`DELETE FROM identity_merchant WHERE merchant_id IN (?,?)`, merchantID, otherMerchantID)
	})
	if _, err := database.Exec(`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(?,?,'ACTIVE',1),(?,?,'ACTIVE',1)`,
		merchantID, "Customer Service Merchant", otherMerchantID, "Other Merchant"); err != nil {
		t.Fatal(err)
	}
	insertShop := `INSERT INTO identity_shop(shop_id,merchant_id,code,name,default_locale,currency,status,version) VALUES(?,?,?,?,?,?,'CNY','ACTIVE',1)`
	if _, err := database.Exec(insertShop, shopID, merchantID, fmt.Sprintf("cs%d", shopID), "Customer Service Shop", "zh-CN"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(insertShop, otherShopID, otherMerchantID, fmt.Sprintf("cs%d", otherShopID), "Other Shop", "zh-CN"); err != nil {
		t.Fatal(err)
	}

	create, err := (model.SaveCommand{CommandKey: keyPrefix + "-create", Account: model.Account{
		MerchantID: merchantID, ShopID: shopID, Platform: " WhatsApp ", Account: " support ", Nickname: "客服",
		Status: model.StatusActive, Config: `{"country_code":"US"}`, Remark: "主账号",
	}}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	created, replayed, err := repository.Save(context.Background(), create)
	if err != nil || replayed || created.Version != 1 || created.MerchantID != merchantID || created.ShopID != shopID {
		t.Fatalf("created=%+v replayed=%v err=%v", created, replayed, err)
	}
	accountID = created.ID
	repeated, replayed, err := repository.Save(context.Background(), create)
	if err != nil || !replayed || repeated.ID != accountID {
		t.Fatalf("repeated=%+v replayed=%v err=%v", repeated, replayed, err)
	}
	changedKey := create
	changedKey.Account.Remark = "different input"
	if _, _, err := repository.Save(context.Background(), changedKey); !errors.Is(err, model.ErrIdempotency) {
		t.Fatalf("changed key error=%v", err)
	}

	page, err := repository.List(context.Background(), model.Query{MerchantID: merchantID, ShopID: shopID, Platform: "whatsapp", Account: "upp", Page: 1, PageSize: 20})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != accountID {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if _, _, err := repository.Save(context.Background(), model.SaveCommand{CommandKey: keyPrefix + "-cross-scope", ExpectedVersion: 1,
		Account: model.Account{ID: accountID, MerchantID: otherMerchantID, ShopID: otherShopID, Platform: "whatsapp", Account: "support", Status: model.StatusActive}}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("cross-scope update error=%v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for index := range 2 {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			candidate := created
			candidate.Nickname = fmt.Sprintf("并发客服 %d", index)
			_, _, err := repository.Save(context.Background(), model.SaveCommand{CommandKey: fmt.Sprintf("%s-update-%d", keyPrefix, index), ExpectedVersion: 1, Account: candidate})
			results <- err
		}(index)
	}
	close(start)
	group.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, model.ErrConflict):
			conflicts++
		default:
			t.Fatalf("concurrent update error=%v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}

	deleteCommand, err := (model.DeleteCommand{AccountID: accountID, MerchantID: merchantID, ShopID: shopID,
		CommandKey: keyPrefix + "-delete", ExpectedVersion: 2}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	deleted, replayed, err := repository.Delete(context.Background(), deleteCommand)
	if err != nil || replayed || !deleted.Deleted {
		t.Fatalf("deleted=%+v replayed=%v err=%v", deleted, replayed, err)
	}
	repeatedDelete, replayed, err := repository.Delete(context.Background(), deleteCommand)
	if err != nil || !replayed || repeatedDelete.ID != accountID {
		t.Fatalf("delete replay=%+v replayed=%v err=%v", repeatedDelete, replayed, err)
	}
	var remaining int
	if err := database.QueryRow(`SELECT COUNT(*) FROM identity_customer_service_account WHERE account_id=?`, accountID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("remaining=%d err=%v", remaining, err)
	}
}
