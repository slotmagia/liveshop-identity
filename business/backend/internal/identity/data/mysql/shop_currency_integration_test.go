//go:build integration

package mysql

import (
	"context"
	"testing"
)

func TestResolveShopReadsAuthoritativeCurrency(t *testing.T) {
	database := integrationDatabase(t)
	if _, err := database.Exec(`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(7,'Merchant Seven','ACTIVE',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO identity_shop(shop_id,merchant_id,code,name,currency,status,version) VALUES(101,7,'shop-101','Shop 101','CNY','ACTIVE',4)`); err != nil {
		t.Fatal(err)
	}
	repository, err := NewDirectoryRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := repository.ResolveShopByID(context.Background(), 101)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Currency != "CNY" || resolved.Context.ShopID != 101 || resolved.Version != 4 {
		t.Fatalf("unexpected shop resolution: %+v", resolved)
	}

	if _, err := database.Exec(`UPDATE identity_shop SET currency='cny' WHERE shop_id=101`); err == nil {
		t.Fatal("database accepted a non-canonical shop currency")
	}
}
