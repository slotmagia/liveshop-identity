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

func TestShopCategoryCommandsAreIdempotentVersionedAndRetainReferences(t *testing.T) {
	database := integrationDatabase(t)
	repository := NewCategoryRepository(database)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	code := "test_" + suffix
	commandPrefix := "shop-category-test-" + suffix
	var categoryID int64
	base := time.Now().UnixNano() / 1000
	merchantID, shopID := base, base+1
	t.Cleanup(func() {
		_, _ = database.Exec(`DELETE FROM identity_shop WHERE shop_id=?`, shopID)
		_, _ = database.Exec(`DELETE FROM identity_merchant WHERE merchant_id=?`, merchantID)
		_, _ = database.Exec(`DELETE FROM identity_shop_category_command WHERE command_key LIKE ?`, commandPrefix+"%")
		_, _ = database.Exec(`DELETE FROM identity_shop_category WHERE category_id=?`, categoryID)
	})

	create := model.SaveCategoryCommand{CommandKey: commandPrefix + "-create", Category: model.Category{
		Code: code, Name: "集成测试品类", Icon: "🧪", Sort: 200, Status: model.CategoryActive,
	}}
	created, replayed, err := repository.SaveCategory(context.Background(), create)
	if err != nil || replayed || created.Version != 1 {
		t.Fatalf("create=%+v replayed=%v err=%v", created, replayed, err)
	}
	categoryID = created.ID
	if _, err := database.Exec(`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(?,?,'ACTIVE',1)`, merchantID, "Category Reference Merchant"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO identity_shop(shop_id,merchant_id,code,name,default_locale,currency,category_code,status,version)
VALUES(?,?,?,?,?,?,?,'ACTIVE',1)`, shopID, merchantID, "catref"+suffix, "Category Reference Shop", "zh-CN", "CNY", code); err != nil {
		t.Fatal(err)
	}
	replayedCategory, replayed, err := repository.SaveCategory(context.Background(), create)
	if err != nil || !replayed || replayedCategory.ID != created.ID {
		t.Fatalf("replay=%+v replayed=%v err=%v", replayedCategory, replayed, err)
	}
	var createCommands int
	if err := database.QueryRow(`SELECT COUNT(*) FROM identity_shop_category_command WHERE command_key=?`, create.CommandKey).Scan(&createCommands); err != nil || createCommands != 1 {
		t.Fatalf("create command rows=%d err=%v", createCommands, err)
	}
	changed := create
	changed.Category.Name = "changed"
	if _, _, err := repository.SaveCategory(context.Background(), changed); !errors.Is(err, model.ErrCategoryIdempotency) {
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
			candidate := created
			candidate.Name = fmt.Sprintf("并发品类 %d", index)
			_, _, err := repository.SaveCategory(context.Background(), model.SaveCategoryCommand{
				CommandKey: fmt.Sprintf("%s-update-%d", commandPrefix, index), ExpectedVersion: 1, Category: candidate,
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
		case errors.Is(err, model.ErrCategoryConflict):
			conflicts++
		default:
			t.Fatalf("concurrent update error=%v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}

	var currentVersion uint64
	if err := database.QueryRow(`SELECT version FROM identity_shop_category WHERE category_id=?`, categoryID).Scan(&currentVersion); err != nil {
		t.Fatal(err)
	}
	if currentVersion != 2 {
		t.Fatalf("concurrent updates advanced version to %d, want 2", currentVersion)
	}
	retire := model.RetireCategoryCommand{
		CategoryID: categoryID, CommandKey: commandPrefix + "-retire", ExpectedVersion: currentVersion,
	}
	retired, replayed, err := repository.RetireCategory(context.Background(), retire)
	if err != nil || replayed || retired.Status != model.CategoryRetired || retired.Version != currentVersion+1 {
		t.Fatalf("retired=%+v replayed=%v err=%v", retired, replayed, err)
	}
	if retired.UsedShopCount != 1 {
		t.Fatalf("retired category lost shop reference count: %+v", retired)
	}
	replayedRetire, replayed, err := repository.RetireCategory(context.Background(), retire)
	if err != nil || !replayed || replayedRetire.Version != retired.Version {
		t.Fatalf("retire replay=%+v replayed=%v err=%v", replayedRetire, replayed, err)
	}
	var retainedCode string
	if err := database.QueryRow(`SELECT category_code FROM identity_shop WHERE shop_id=?`, shopID).Scan(&retainedCode); err != nil || retainedCode != code {
		t.Fatalf("shop reference=%q err=%v", retainedCode, err)
	}
	var retained int
	if err := database.QueryRow(`SELECT COUNT(*) FROM identity_shop_category WHERE category_id=? AND status='RETIRED'`, categoryID).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("retained=%d err=%v", retained, err)
	}
	values, err := repository.ListCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range values {
		if value.ID == categoryID {
			t.Fatal("retired category remained in the active management list")
		}
	}
}

func TestShopCategoryDuplicateCodeRollsBackCommand(t *testing.T) {
	database := integrationDatabase(t)
	repository := NewCategoryRepository(database)
	key := fmt.Sprintf("shop-category-duplicate-%d", time.Now().UnixNano())
	_, _, err := repository.SaveCategory(context.Background(), model.SaveCategoryCommand{CommandKey: key, Category: model.Category{
		Code: "apparel", Name: "重复代码", Status: model.CategoryActive,
	}})
	if !errors.Is(err, model.ErrCategoryConflict) {
		t.Fatalf("duplicate error=%v", err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM identity_shop_category_command WHERE command_key=?`, key).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed command leaked ledger rows=%d", count)
	}
}
