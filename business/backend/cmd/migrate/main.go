package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type migration struct{ name, checksum, sql string }

type store interface {
	Applied(context.Context) (map[string]string, error)
	Apply(context.Context, migration) error
}

type mysqlStore struct{ database *sql.DB }

func main() {
	dsn := flag.String("dsn", "", "MySQL DSN")
	directory := flag.String("dir", "/app/migrations", "migration directory")
	flag.Parse()
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "identity migrate: -dsn is required")
		os.Exit(2)
	}
	files, err := load(*directory)
	if err != nil {
		fail(err)
	}
	// A migration file is intentionally submitted as one multi-statement call.
	// MySQL implicitly commits many DDL statements, so the ledger is written
	// only after the full call succeeds; a mid-DDL crash remains visible and is
	// never falsely recorded as applied.
	connection := *dsn
	if strings.Contains(connection, "?") {
		connection += "&multiStatements=true"
	} else {
		connection += "?multiStatements=true"
	}
	database, err := sql.Open("mysql", connection)
	if err != nil {
		fail(err)
	}
	defer database.Close()
	if err := migrate(context.Background(), &mysqlStore{database: database}, files); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, "identity migrate:", err); os.Exit(1) }

func load(directory string) ([]migration, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	var result []migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(content)
		result = append(result, migration{name: entry.Name(), checksum: hex.EncodeToString(digest[:]), sql: string(content)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	return result, nil
}

func migrate(ctx context.Context, destination store, files []migration) error {
	applied, err := destination.Applied(ctx)
	if err != nil {
		return err
	}
	for _, file := range files {
		if checksum, ok := applied[file.name]; ok {
			if checksum != file.checksum {
				return fmt.Errorf("applied migration %s checksum changed", file.name)
			}
			continue
		}
		if err := destination.Apply(ctx, file); err != nil {
			return fmt.Errorf("apply %s: %w", file.name, err)
		}
	}
	return nil
}

func (s *mysqlStore) Applied(ctx context.Context) (map[string]string, error) {
	if _, err := s.database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS identity_schema_migration (
filename VARCHAR(255) COLLATE utf8mb4_0900_as_cs NOT NULL,
checksum CHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
PRIMARY KEY (filename)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`); err != nil {
		return nil, err
	}
	rows, err := s.database.QueryContext(ctx, `SELECT filename, checksum FROM identity_schema_migration ORDER BY filename`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var name, checksum string
		if err := rows.Scan(&name, &checksum); err != nil {
			return nil, err
		}
		result[name] = checksum
	}
	return result, rows.Err()
}

func (s *mysqlStore) Apply(ctx context.Context, file migration) error {
	connection, err := s.database.Conn(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	if _, err := connection.ExecContext(ctx, file.sql); err != nil {
		return err
	}
	result, err := connection.ExecContext(ctx, `INSERT INTO identity_schema_migration (filename, checksum) VALUES (?, ?)`, file.name, file.checksum)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return errors.New("migration ledger insert did not affect one row")
	}
	return nil
}
