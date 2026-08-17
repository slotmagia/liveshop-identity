package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	biz "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
)

var integrationSequence atomic.Int64

func integrationRepository(t *testing.T) (*QuotaRepository, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("SUBSCRIPTION_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("SUBSCRIPTION_MYSQL_TEST_DSN is not set")
	}
	database, err := Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.Ping(); err != nil {
		t.Fatal(err)
	}
	return NewQuotaRepository(database), database
}

func testMerchantID() int64 {
	return time.Now().UnixMilli()*100 + integrationSequence.Add(1)
}

func TestQuotaApplyIsIdempotentUnderConcurrency(t *testing.T) {
	repository, database := integrationRepository(t)
	merchantID := testMerchantID()
	limit := int64(50)
	command := biz.ApplyQuotaCommand{
		MerchantID: merchantID, CommandKey: "catalog-products-create-0001", Code: biz.CatalogProductsQuota,
		Limit: &limit, EffectiveFrom: time.Now().UTC().Add(-time.Minute),
	}

	type outcome struct {
		quota  biz.QuotaLimit
		replay bool
		err    error
	}
	start := make(chan struct{})
	results := make(chan outcome, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			quota, replay, err := repository.Apply(context.Background(), command)
			results <- outcome{quota: quota, replay: replay, err: err}
		}()
	}
	close(start)
	group.Wait()
	close(results)

	replays := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent apply: %v", result.err)
		}
		if result.quota.Revision != 1 {
			t.Fatalf("revision = %d, want 1", result.quota.Revision)
		}
		if result.replay {
			replays++
		}
	}
	if replays != 1 {
		t.Fatalf("replays = %d, want 1", replays)
	}
	assertRowCount(t, database, "subscription_quota_entitlement", merchantID, 1)
	assertRowCount(t, database, "subscription_quota_command", merchantID, 1)
}

func TestQuotaConcurrentDifferentCommandsCannotLoseAnUpdate(t *testing.T) {
	repository, _ := integrationRepository(t)
	merchantID := testMerchantID()
	start := make(chan struct{})
	results := make(chan error, 2)
	for index, value := range []int64{25, 75} {
		go func(index int, value int64) {
			<-start
			_, _, err := repository.Apply(context.Background(), biz.ApplyQuotaCommand{
				MerchantID: merchantID, CommandKey: fmt.Sprintf("catalog-products-race-%04d", index),
				Code: biz.CatalogProductsQuota, Limit: &value, EffectiveFrom: time.Now().UTC().Add(-time.Minute),
			})
			results <- err
		}(index, value)
	}
	close(start)
	successes, conflicts := 0, 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, biz.ErrVersionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestQuotaCommandPayloadConflictDoesNotMutateFact(t *testing.T) {
	repository, _ := integrationRepository(t)
	merchantID := testMerchantID()
	from := time.Now().UTC().Add(-time.Minute)
	first := int64(10)
	command := biz.ApplyQuotaCommand{MerchantID: merchantID, CommandKey: "catalog-products-conflict", Code: biz.CatalogProductsQuota, Limit: &first, EffectiveFrom: from}
	if _, _, err := repository.Apply(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	second := int64(99)
	command.Limit = &second
	if _, _, err := repository.Apply(context.Background(), command); !errors.Is(err, biz.ErrIdempotencyConflict) {
		t.Fatalf("error = %v, want idempotency conflict", err)
	}
	quota, err := repository.GetEffective(context.Background(), merchantID, biz.CatalogProductsQuota, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if quota.Limit == nil || *quota.Limit != first || quota.Revision != 1 {
		t.Fatalf("fact changed after conflict: %+v", quota)
	}
}

func TestQuotaDatabaseFailureRollsBackCommandAndFact(t *testing.T) {
	repository, database := integrationRepository(t)
	merchantID := testMerchantID()
	trigger := fmt.Sprintf("subscription_quota_fail_%d", integrationSequence.Add(1))
	adminDSN := os.Getenv("SUBSCRIPTION_MYSQL_ADMIN_TEST_DSN")
	if adminDSN == "" {
		t.Fatal("SUBSCRIPTION_MYSQL_ADMIN_TEST_DSN is required for failure injection")
	}
	admin, err := Open(adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close() }()
	_, err = admin.Exec(`CREATE TRIGGER ` + trigger + ` BEFORE INSERT ON subscription_quota_entitlement
FOR EACH ROW BEGIN IF NEW.limit_value = 13 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected quota write failure'; END IF; END`)
	if err != nil {
		t.Fatalf("create failure-injection trigger: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP TRIGGER IF EXISTS ` + trigger) })
	limit := int64(13)
	_, _, err = repository.Apply(context.Background(), biz.ApplyQuotaCommand{
		MerchantID: merchantID, CommandKey: "catalog-products-db-failure", Code: biz.CatalogProductsQuota,
		Limit: &limit, EffectiveFrom: time.Now().UTC().Add(-time.Minute),
	})
	if err == nil {
		t.Fatal("injected database failure unexpectedly succeeded")
	}
	assertRowCount(t, database, "subscription_quota_entitlement", merchantID, 0)
	assertRowCount(t, database, "subscription_quota_command", merchantID, 0)
}

func TestQuotaLookupRequiresExplicitEffectiveEntitlement(t *testing.T) {
	repository, _ := integrationRepository(t)
	merchantID := testMerchantID()
	if _, err := repository.GetEffective(context.Background(), merchantID, biz.CatalogProductsQuota, time.Now()); !errors.Is(err, biz.ErrNotConfigured) {
		t.Fatalf("missing entitlement error = %v", err)
	}
	until := time.Now().UTC().Add(-time.Minute)
	_, _, err := repository.Apply(context.Background(), biz.ApplyQuotaCommand{
		MerchantID: merchantID, CommandKey: "catalog-products-expired", Code: biz.CatalogProductsQuota,
		Limit: nil, EffectiveFrom: until.Add(-time.Hour), EffectiveUntil: &until,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetEffective(context.Background(), merchantID, biz.CatalogProductsQuota, time.Now()); !errors.Is(err, biz.ErrNotConfigured) {
		t.Fatalf("expired entitlement error = %v", err)
	}
}

func assertRowCount(t *testing.T, database *sql.DB, table string, merchantID, want int64) {
	t.Helper()
	var count int64
	if err := database.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE merchant_id = ?`, merchantID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("%s rows = %d, want %d", table, count, want)
	}
}
