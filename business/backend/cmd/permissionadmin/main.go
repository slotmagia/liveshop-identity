package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	biz "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
	mysqlrepo "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/data/subscription"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("IDENTITY_DATABASE_DSN"), "Identity MySQL DSN")
	merchantID := flag.Int64("merchant-id", 0, "merchant identity")
	permissions := flag.String("permissions", "", "comma-separated complete permission entitlement; empty is explicit none")
	expectedRevision := flag.Uint64("expected-revision", 0, "current revision, zero for initial create")
	commandKey := flag.String("command-key", "", "stable idempotency key")
	flag.Parse()
	codes := []string{}
	if strings.TrimSpace(*permissions) != "" {
		codes = strings.Split(*permissions, ",")
	}
	database, err := mysqlrepo.Open(*dsn)
	if err != nil {
		fail(err)
	}
	defer func() { _ = database.Close() }()
	service := biz.NewPermissionEntitlements(mysqlrepo.NewPermissionEntitlementRepository(database))
	snapshot, replay, err := service.Apply(context.Background(), biz.ApplyPermissionEntitlementCommand{
		MerchantID: *merchantID, CommandKey: *commandKey, PermissionCodes: codes, ExpectedRevision: *expectedRevision,
	})
	if err != nil {
		fail(err)
	}
	output := struct {
		MerchantID      int64    `json:"merchantId"`
		PermissionCodes []string `json:"permissionCodes"`
		Revision        uint64   `json:"revision"`
		SnapshotDigest  string   `json:"snapshotDigest"`
		Replay          bool     `json:"replay"`
	}{snapshot.MerchantID, snapshot.PermissionCodes, snapshot.Revision, snapshot.SnapshotDigest, replay}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
