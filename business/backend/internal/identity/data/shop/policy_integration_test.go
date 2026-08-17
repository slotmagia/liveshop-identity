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

func TestShopPolicyCommandsAreIdempotentVersionedAndArchiveOnPublish(t *testing.T) {
	database := integrationDatabase(t)
	repository := NewPolicyRepository(database)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	commandPrefix := "shop-policy-test-" + suffix
	base := time.Now().UnixNano() / 1000
	merchantID, shopID := base, base+1
	var firstID, secondID int64
	t.Cleanup(func() {
		_, _ = database.Exec(`DELETE FROM identity_shop_policy WHERE shop_id=?`, shopID)
		_, _ = database.Exec(`DELETE FROM identity_shop_policy_command WHERE command_key LIKE ?`, commandPrefix+"%")
		_, _ = database.Exec(`DELETE FROM identity_merchant_capability WHERE merchant_id=? AND shop_id=?`, merchantID, shopID)
		_, _ = database.Exec(`DELETE FROM identity_shop WHERE shop_id=?`, shopID)
		_, _ = database.Exec(`DELETE FROM identity_merchant WHERE merchant_id=?`, merchantID)
	})
	if _, err := database.Exec(`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(?,?,'ACTIVE',1)`, merchantID, "Policy Merchant"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO identity_shop(shop_id,merchant_id,code,name,default_locale,currency,status,version)
VALUES(?,?,?,?,?,?,'ACTIVE',1)`, shopID, merchantID, "pol"+suffix, "Policy Shop", "zh-CN", "CNY"); err != nil {
		t.Fatal(err)
	}

	create := model.SavePolicyCommand{CommandKey: commandPrefix + "-create", MerchantID: merchantID, ShopID: shopID,
		PolicyType: model.PolicyPrivacy, Title: "隐私政策", Content: "这是一份足够长的店铺隐私政策正文。"}
	created, replayed, err := repository.SavePolicy(context.Background(), create)
	if err != nil || replayed || created.Version != 1 || created.Status != model.PolicyDraft || created.VersionNo != 1 {
		t.Fatalf("create=%+v replayed=%v err=%v", created, replayed, err)
	}
	firstID = created.ID
	replayedPolicy, replayed, err := repository.SavePolicy(context.Background(), create)
	if err != nil || !replayed || replayedPolicy.ID != created.ID {
		t.Fatalf("replay=%+v replayed=%v err=%v", replayedPolicy, replayed, err)
	}
	changed := create
	changed.Title = "changed"
	if _, _, err := repository.SavePolicy(context.Background(), changed); !errors.Is(err, model.ErrPolicyIdempotency) {
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
			_, _, err := repository.PublishPolicy(context.Background(), model.PublishPolicyCommand{
				PolicyID: firstID, MerchantID: merchantID, ShopID: shopID,
				CommandKey: fmt.Sprintf("%s-publish-%d", commandPrefix, index), ExpectedVersion: 1,
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
		case errors.Is(err, model.ErrPolicyConflict):
			conflicts++
		default:
			t.Fatalf("unexpected publish error=%v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("publish race successes=%d conflicts=%d", successes, conflicts)
	}

	published, replayed, err := repository.SavePolicy(context.Background(), model.SavePolicyCommand{
		CommandKey: commandPrefix + "-publish-save", MerchantID: merchantID, ShopID: shopID,
		PolicyType: model.PolicyPrivacy, Title: "新隐私政策", Content: "这是一份足够长的新版店铺隐私政策正文。", Publish: true,
	})
	if err != nil || replayed || published.Status != model.PolicyPublished || published.VersionNo != 2 {
		t.Fatalf("publish-on-save=%+v replayed=%v err=%v", published, replayed, err)
	}
	secondID = published.ID
	page, err := repository.ListPolicies(context.Background(), model.PolicyQuery{MerchantID: merchantID, ShopID: shopID, Page: 1, PageSize: 20})
	if err != nil || page.Total != 2 {
		t.Fatalf("list=%+v err=%v", page, err)
	}
	var archived, current string
	if err := database.QueryRow(`SELECT status FROM identity_shop_policy WHERE policy_id=?`, firstID).Scan(&archived); err != nil || archived != string(model.PolicyArchived) {
		t.Fatalf("archived status=%s err=%v", archived, err)
	}
	if err := database.QueryRow(`SELECT status FROM identity_shop_policy WHERE policy_id=?`, secondID).Scan(&current); err != nil || current != string(model.PolicyPublished) {
		t.Fatalf("published status=%s err=%v", current, err)
	}

	if _, err := database.Exec(`INSERT INTO identity_merchant_capability(merchant_id,shop_id,module,name,merchant_status,platform_status,platform_reason_public,version,updated_by)
VALUES(?,?,?,?,?,?,?,1,?)`, merchantID, shopID, "policies", "政策", "unset", "restricted", "平台限制政策发布", "tester"); err != nil {
		t.Fatal(err)
	}
	draft, _, err := repository.SavePolicy(context.Background(), model.SavePolicyCommand{
		CommandKey: commandPrefix + "-restricted-draft", MerchantID: merchantID, ShopID: shopID,
		PolicyType: model.PolicyTerms, Title: "服务条款", Content: "这是一份足够长的店铺服务条款正文。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.PublishPolicy(context.Background(), model.PublishPolicyCommand{
		PolicyID: draft.ID, MerchantID: merchantID, ShopID: shopID, CommandKey: commandPrefix + "-restricted-publish", ExpectedVersion: draft.Version,
	}); !errors.Is(err, model.ErrPolicyRestricted) {
		t.Fatalf("restricted publish error=%v", err)
	}
}
