package mysql

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

func TestShopAppCommandsAreIdempotentVersionedAndRespectOverlay(t *testing.T) {
	database := integrationDatabase(t)
	repository := NewAppRepository(database)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	commandPrefix := "shop-app-test-" + suffix
	base := time.Now().UnixNano() / 1000
	merchantID, shopID := base, base+1
	var appID int64
	t.Cleanup(func() {
		if appID != 0 {
			_, _ = database.Exec(`DELETE FROM identity_outbox WHERE aggregate_type='shop_app' AND aggregate_id=?`, fmt.Sprintf("%d", appID))
		}
		_, _ = database.Exec(`DELETE FROM identity_shop_app WHERE shop_id=?`, shopID)
		_, _ = database.Exec(`DELETE FROM identity_shop_app_command WHERE command_key LIKE ?`, commandPrefix+"%")
		_, _ = database.Exec(`DELETE FROM identity_merchant_capability WHERE merchant_id=? AND shop_id=?`, merchantID, shopID)
		_, _ = database.Exec(`DELETE FROM identity_shop WHERE shop_id=?`, shopID)
		_, _ = database.Exec(`DELETE FROM identity_merchant WHERE merchant_id=?`, merchantID)
	})
	if _, err := database.Exec(`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(?,?,'ACTIVE',1)`, merchantID, "App Merchant"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO identity_shop(shop_id,merchant_id,code,name,default_locale,currency,status,version)
VALUES(?,?,?,?,?,?,'ACTIVE',1)`, shopID, merchantID, "app"+suffix, "App Shop", "zh-CN", "CNY"); err != nil {
		t.Fatal(err)
	}

	create := model.CreateAppCommand{CommandKey: commandPrefix + "-create", MerchantID: merchantID, ShopID: shopID,
		Name: "订单同步", Scopes: "orders:read,products:read"}
	created, replayed, err := repository.CreateApp(context.Background(), create)
	if err != nil || replayed || created.App.Version != 1 || created.App.Status != model.AppActive || created.ClientSecret == "" || created.App.SecretHint != created.ClientSecret[len(created.ClientSecret)-6:] {
		t.Fatalf("create=%+v replayed=%v err=%v", created, replayed, err)
	}
	appID = created.App.ID
	replayedApp, replayed, err := repository.CreateApp(context.Background(), create)
	if err != nil || !replayed || replayedApp.App.ID != created.App.ID || replayedApp.ClientSecret != created.ClientSecret {
		t.Fatalf("replay=%+v replayed=%v err=%v", replayedApp, replayed, err)
	}
	changed := create
	changed.Name = "changed"
	if _, _, err := repository.CreateApp(context.Background(), changed); !errors.Is(err, model.ErrAppIdempotency) {
		t.Fatalf("changed replay error=%v", err)
	}

	reset, replayed, err := repository.ResetAppSecret(context.Background(), model.ResetAppSecretCommand{
		AppID: appID, MerchantID: merchantID, ShopID: shopID, CommandKey: commandPrefix + "-reset", ExpectedVersion: created.App.Version,
	})
	if err != nil || replayed || reset.App.Version != 2 || reset.ClientSecret == "" || reset.ClientSecret == created.ClientSecret {
		t.Fatalf("reset=%+v replayed=%v err=%v", reset, replayed, err)
	}
	if _, _, err := repository.ResetAppSecret(context.Background(), model.ResetAppSecretCommand{
		AppID: appID, MerchantID: merchantID, ShopID: shopID, CommandKey: commandPrefix + "-reset-stale", ExpectedVersion: 1,
	}); !errors.Is(err, model.ErrAppConflict) {
		t.Fatalf("stale reset error=%v", err)
	}

	disabled, replayed, err := repository.SetAppEnabled(context.Background(), model.SetAppEnabledCommand{
		AppID: appID, MerchantID: merchantID, ShopID: shopID, CommandKey: commandPrefix + "-disable", ExpectedVersion: reset.App.Version, Enabled: false,
	})
	if err != nil || replayed || disabled.Status != model.AppDisabled || disabled.Version != 3 {
		t.Fatalf("disable=%+v replayed=%v err=%v", disabled, replayed, err)
	}
	enabled, replayed, err := repository.SetAppEnabled(context.Background(), model.SetAppEnabledCommand{
		AppID: appID, MerchantID: merchantID, ShopID: shopID, CommandKey: commandPrefix + "-enable", ExpectedVersion: disabled.Version, Enabled: true,
	})
	if err != nil || replayed || enabled.Status != model.AppActive || enabled.Version != 4 {
		t.Fatalf("enable=%+v replayed=%v err=%v", enabled, replayed, err)
	}

	if _, err := database.Exec(`INSERT INTO identity_merchant_capability(merchant_id,shop_id,module,name,merchant_status,platform_status,platform_reason_public,version,updated_by)
VALUES(?,?,?,?,?,?,?,1,?)`, merchantID, shopID, "apps", "应用", "unset", "restricted", "平台限制应用凭据", "tester"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.CreateApp(context.Background(), model.CreateAppCommand{
		CommandKey: commandPrefix + "-restricted-create", MerchantID: merchantID, ShopID: shopID, Name: "被拒应用", Scopes: "live:read",
	}); !errors.Is(err, model.ErrAppRestricted) {
		t.Fatalf("restricted create error=%v", err)
	}
	if _, _, err := repository.ResetAppSecret(context.Background(), model.ResetAppSecretCommand{
		AppID: appID, MerchantID: merchantID, ShopID: shopID, CommandKey: commandPrefix + "-restricted-reset", ExpectedVersion: enabled.Version,
	}); !errors.Is(err, model.ErrAppRestricted) {
		t.Fatalf("restricted reset error=%v", err)
	}
	restrictedDisabled, _, err := repository.SetAppEnabled(context.Background(), model.SetAppEnabledCommand{
		AppID: appID, MerchantID: merchantID, ShopID: shopID, CommandKey: commandPrefix + "-restricted-disable", ExpectedVersion: enabled.Version, Enabled: false,
	})
	if err != nil || restrictedDisabled.Status != model.AppDisabled {
		t.Fatalf("restricted disable=%+v err=%v", restrictedDisabled, err)
	}
	if _, _, err := repository.SetAppEnabled(context.Background(), model.SetAppEnabledCommand{
		AppID: appID, MerchantID: merchantID, ShopID: shopID, CommandKey: commandPrefix + "-restricted-enable", ExpectedVersion: restrictedDisabled.Version, Enabled: true,
	}); !errors.Is(err, model.ErrAppRestricted) {
		t.Fatalf("restricted enable error=%v", err)
	}

	page, err := repository.ListApps(context.Background(), model.AppQuery{MerchantID: merchantID, ShopID: shopID, Page: 1, PageSize: 20})
	if err != nil || page.Total != 1 || page.Items[0].Status != model.AppDisabled || page.Items[0].SecretHint == "" {
		t.Fatalf("list=%+v err=%v", page, err)
	}
}
