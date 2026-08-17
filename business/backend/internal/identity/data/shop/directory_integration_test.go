package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
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

func TestShopDirectoryIsMerchantScopedAndKeepsDisabledRows(t *testing.T) {
	database := integrationDatabase(t)
	base := time.Now().UnixNano() / 1000
	merchantID, otherMerchantID := base, base+1
	if _, err := database.Exec(`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(?,?,'ACTIVE',1),(?,?,'ACTIVE',1)`, merchantID, "Scoped Merchant", otherMerchantID, "Other Merchant"); err != nil {
		t.Fatal(err)
	}
	activeShop, disabledShop, closedShop, otherShop := base+10, base+11, base+12, base+13
	insert := `INSERT INTO identity_shop(shop_id,merchant_id,code,subdomain,name,default_locale,currency,status,version) VALUES(?,?,?,?,?,?,?,?,1)`
	for _, row := range []struct {
		shopID, ownerID int64
		status          string
	}{{activeShop, merchantID, "ACTIVE"}, {disabledShop, merchantID, "DISABLED"}, {closedShop, merchantID, "CLOSED"}, {otherShop, otherMerchantID, "ACTIVE"}} {
		code := fmt.Sprintf("s%d", row.shopID)
		if _, err := database.Exec(insert, row.shopID, row.ownerID, code, code, "Shop "+code, "zh-CN", "CNY", row.status); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = database.Exec(`DELETE FROM identity_shop WHERE shop_id IN (?,?,?,?)`, activeShop, disabledShop, closedShop, otherShop)
		_, _ = database.Exec(`DELETE FROM identity_merchant WHERE merchant_id IN (?,?)`, merchantID, otherMerchantID)
	})

	values, err := NewRepository(database).ListShopsByMerchant(context.Background(), merchantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 {
		t.Fatalf("shop count=%d values=%+v", len(values), values)
	}
	if values[0].ID != disabledShop || values[0].MerchantID != merchantID || values[1].ID != activeShop || values[1].MerchantID != merchantID {
		t.Fatalf("unexpected scoped order: %+v", values)
	}
}
