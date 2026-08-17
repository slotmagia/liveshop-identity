package mysql

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

func TestShopPrivacyCommandsAreIdempotentVersionedAndDefaulted(t *testing.T) {
	database := integrationDatabase(t)
	repository := NewPrivacyRepository(database)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	commandPrefix := "shop-privacy-test-" + suffix
	base := time.Now().UnixNano() / 1000
	merchantID, shopID := base, base+1
	t.Cleanup(func() {
		_, _ = database.Exec(`DELETE FROM identity_outbox WHERE aggregate_type='shop_privacy' AND aggregate_id IN (SELECT CAST(privacy_id AS CHAR) FROM identity_shop_privacy WHERE shop_id=?)`, shopID)
		_, _ = database.Exec(`DELETE FROM identity_shop_privacy WHERE shop_id=?`, shopID)
		_, _ = database.Exec(`DELETE FROM identity_shop_privacy_command WHERE command_key LIKE ?`, commandPrefix+"%")
		_, _ = database.Exec(`DELETE FROM identity_shop WHERE shop_id=?`, shopID)
		_, _ = database.Exec(`DELETE FROM identity_merchant WHERE merchant_id=?`, merchantID)
	})
	if _, err := database.Exec(`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(?,?,'ACTIVE',1)`, merchantID, "Privacy Merchant"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO identity_shop(shop_id,merchant_id,code,name,default_locale,currency,status,version)
VALUES(?,?,?,?,?,?, 'ACTIVE',1)`, shopID, merchantID, "prv"+suffix, "Privacy Shop", "zh-CN", "CNY"); err != nil {
		t.Fatal(err)
	}

	defaults, err := repository.GetPrivacy(context.Background(), merchantID, shopID)
	if err != nil || defaults.Version != 0 || !defaults.CollectConsent || !defaults.CookieBanner || defaults.MarketingConsent || defaults.DataRetentionDays != 365 {
		t.Fatalf("defaults=%+v err=%v", defaults, err)
	}

	create := model.SavePrivacyCommand{CommandKey: commandPrefix + "-create", Privacy: model.Privacy{
		MerchantID: merchantID, ShopID: shopID, CollectConsent: true, MarketingConsent: true,
		CookieBanner: false, DataRetentionDays: 90, ContactEmail: "dpo@example.com",
	}}
	created, replayed, err := repository.SavePrivacy(context.Background(), create)
	if err != nil || replayed || created.Version != 1 || !created.MarketingConsent || created.CookieBanner || created.DataRetentionDays != 90 {
		t.Fatalf("create=%+v replayed=%v err=%v", created, replayed, err)
	}
	replayedPrivacy, replayed, err := repository.SavePrivacy(context.Background(), create)
	if err != nil || !replayed || replayedPrivacy.ID != created.ID || replayedPrivacy.Version != 1 {
		t.Fatalf("replay=%+v replayed=%v err=%v", replayedPrivacy, replayed, err)
	}
	changed := create
	changed.Privacy.DataRetentionDays = 180
	if _, _, err := repository.SavePrivacy(context.Background(), changed); !errors.Is(err, model.ErrPrivacyIdempotency) {
		t.Fatalf("changed replay error=%v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for index := range 2 {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			_, _, err := repository.SavePrivacy(context.Background(), model.SavePrivacyCommand{
				CommandKey: fmt.Sprintf("%s-update-%d", commandPrefix, index), ExpectedVersion: 1,
				Privacy: model.Privacy{MerchantID: merchantID, ShopID: shopID, CollectConsent: true, CookieBanner: true, DataRetentionDays: 30 + index, ContactEmail: "dpo@example.com"},
			})
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
		case errors.Is(err, model.ErrPrivacyConflict):
			conflicts++
		default:
			t.Fatalf("concurrent error=%v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}

	closedShop := shopID + 10
	if _, err := database.Exec(`INSERT INTO identity_shop(shop_id,merchant_id,code,name,default_locale,currency,status,version)
VALUES(?,?,?,?,?,?,'CLOSED',1)`, closedShop, merchantID, "cl"+suffix, "Closed Shop", "zh-CN", "CNY"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = database.Exec(`DELETE FROM identity_shop WHERE shop_id=?`, closedShop) })
	if _, err := repository.GetPrivacy(context.Background(), merchantID, closedShop); !errors.Is(err, model.ErrPrivacyNotFound) {
		t.Fatalf("closed shop error=%v", err)
	}
}
