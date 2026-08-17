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

func TestMerchantDirectoryIncludesDisabledAndExcludesClosed(t *testing.T) {
	database := integrationDatabase(t)
	base := time.Now().UnixNano() / 1000
	activeID, disabledID, closedID := base, base+1, base+2
	for _, row := range []struct {
		id     int64
		status string
	}{{activeID, "ACTIVE"}, {disabledID, "DISABLED"}, {closedID, "CLOSED"}} {
		if _, err := database.Exec(`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(?,?,?,1)`, row.id, fmt.Sprintf("Merchant %d", row.id), row.status); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = database.Exec(`DELETE FROM identity_merchant WHERE merchant_id IN (?,?,?)`, activeID, disabledID, closedID)
	})

	values, err := NewRepository(database).ListMerchants(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := map[int64]int{}
	for index, value := range values {
		if value.ID == activeID || value.ID == disabledID || value.ID == closedID {
			found[value.ID] = index
		}
	}
	if _, exists := found[closedID]; exists {
		t.Fatalf("closed merchant %d leaked into directory", closedID)
	}
	if _, active := found[activeID]; !active {
		t.Fatalf("active merchant %d missing", activeID)
	}
	if _, disabled := found[disabledID]; !disabled {
		t.Fatalf("disabled merchant %d missing", disabledID)
	}
	if found[disabledID] >= found[activeID] {
		t.Fatalf("directory is not merchant_id descending: disabled index=%d active index=%d", found[disabledID], found[activeID])
	}
}
