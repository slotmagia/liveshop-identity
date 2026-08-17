package mysql

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	biz "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
)

func TestPermissionEntitlementReplaySurvivesLaterRevision(t *testing.T) {
	_, database := integrationRepository(t)
	repository := NewPermissionEntitlementRepository(database)
	merchantID := testMerchantID()
	first := biz.ApplyPermissionEntitlementCommand{MerchantID: merchantID, CommandKey: "permission-plan-rev-0001", PermissionCodes: []string{"catalog.product.read"}}
	revisionOne, replay, err := repository.ApplyPermissionEntitlement(context.Background(), mustNormalizePermissionCommand(t, first))
	if err != nil || replay || revisionOne.Revision != 1 {
		t.Fatalf("first apply = %+v replay=%t err=%v", revisionOne, replay, err)
	}
	second := biz.ApplyPermissionEntitlementCommand{MerchantID: merchantID, CommandKey: "permission-plan-rev-0002", ExpectedRevision: 1, PermissionCodes: []string{"trade.order.read"}}
	if current, _, err := repository.ApplyPermissionEntitlement(context.Background(), mustNormalizePermissionCommand(t, second)); err != nil || current.Revision != 2 {
		t.Fatalf("second apply = %+v err=%v", current, err)
	}
	replayed, replay, err := repository.ApplyPermissionEntitlement(context.Background(), mustNormalizePermissionCommand(t, first))
	if err != nil || !replay || replayed.Revision != 1 || len(replayed.PermissionCodes) != 1 || replayed.PermissionCodes[0] != "catalog.product.read" {
		t.Fatalf("historical replay = %+v replay=%t err=%v", replayed, replay, err)
	}
	current, err := repository.GetPermissionEntitlement(context.Background(), merchantID)
	if err != nil || current.Revision != 2 || len(current.PermissionCodes) != 1 || current.PermissionCodes[0] != "trade.order.read" {
		t.Fatalf("replay mutated current fact: %+v err=%v", current, err)
	}
}

func TestPermissionEntitlementConflictAndDatabaseRollback(t *testing.T) {
	_, database := integrationRepository(t)
	repository := NewPermissionEntitlementRepository(database)
	merchantID := testMerchantID()
	command := biz.ApplyPermissionEntitlementCommand{MerchantID: merchantID, CommandKey: "permission-plan-conflict", PermissionCodes: []string{"catalog.product.read"}}
	if _, _, err := repository.ApplyPermissionEntitlement(context.Background(), mustNormalizePermissionCommand(t, command)); err != nil {
		t.Fatal(err)
	}
	conflict := command
	conflict.PermissionCodes = []string{"trade.order.read"}
	if _, _, err := repository.ApplyPermissionEntitlement(context.Background(), mustNormalizePermissionCommand(t, conflict)); !errors.Is(err, biz.ErrIdempotencyConflict) {
		t.Fatalf("payload conflict = %v", err)
	}
	wrongRevision := biz.ApplyPermissionEntitlementCommand{MerchantID: merchantID, CommandKey: "permission-plan-wrong-revision", ExpectedRevision: 7, PermissionCodes: []string{}}
	if _, _, err := repository.ApplyPermissionEntitlement(context.Background(), mustNormalizePermissionCommand(t, wrongRevision)); !errors.Is(err, biz.ErrVersionConflict) {
		t.Fatalf("version conflict = %v", err)
	}

	adminDSN := os.Getenv("SUBSCRIPTION_MYSQL_ADMIN_TEST_DSN")
	if adminDSN == "" {
		t.Fatal("SUBSCRIPTION_MYSQL_ADMIN_TEST_DSN is required for failure injection")
	}
	admin, err := Open(adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	trigger := fmt.Sprintf("subscription_permission_fail_%d", integrationSequence.Add(1))
	_, err = admin.Exec(`CREATE TRIGGER ` + trigger + ` BEFORE INSERT ON subscription_permission_entitlement_item
FOR EACH ROW BEGIN IF NEW.permission_code = 'failure.inject' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected permission write failure'; END IF; END`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP TRIGGER IF EXISTS ` + trigger) })
	failing := biz.ApplyPermissionEntitlementCommand{MerchantID: merchantID, CommandKey: "permission-plan-db-failure", ExpectedRevision: 1, PermissionCodes: []string{"failure.inject"}}
	if _, _, err := repository.ApplyPermissionEntitlement(context.Background(), mustNormalizePermissionCommand(t, failing)); err == nil {
		t.Fatal("injected failure unexpectedly succeeded")
	}
	current, err := repository.GetPermissionEntitlement(context.Background(), merchantID)
	if err != nil || current.Revision != 1 || current.PermissionCodes[0] != "catalog.product.read" {
		t.Fatalf("database failure changed fact: %+v err=%v", current, err)
	}
	var commands int
	if err := database.QueryRow(`SELECT COUNT(*) FROM subscription_permission_entitlement_command WHERE merchant_id=? AND command_key=?`, merchantID, failing.CommandKey).Scan(&commands); err != nil || commands != 0 {
		t.Fatalf("failed command ledger rows=%d err=%v", commands, err)
	}
}

func mustNormalizePermissionCommand(t *testing.T, command biz.ApplyPermissionEntitlementCommand) biz.ApplyPermissionEntitlementCommand {
	t.Helper()
	normalized, err := command.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	return normalized
}
