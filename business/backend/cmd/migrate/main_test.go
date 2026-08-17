package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type memoryStore struct {
	applied map[string]string
	calls   []string
	fail    string
}

func (s *memoryStore) Applied(context.Context) (map[string]string, error) {
	copy := map[string]string{}
	for k, v := range s.applied {
		copy[k] = v
	}
	return copy, nil
}
func (s *memoryStore) Apply(_ context.Context, file migration) error {
	s.calls = append(s.calls, file.name)
	if s.fail == file.name {
		return errors.New("failed")
	}
	s.applied[file.name] = file.checksum
	return nil
}

func TestFirstStartAndRestart(t *testing.T) {
	store := &memoryStore{applied: map[string]string{}}
	files := []migration{{name: "001.sql", checksum: "a"}, {name: "002.sql", checksum: "b"}}
	if err := migrate(context.Background(), store, files); err != nil {
		t.Fatal(err)
	}
	if len(store.calls) != 2 {
		t.Fatalf("first start applied %d files", len(store.calls))
	}
	store.calls = nil
	if err := migrate(context.Background(), store, files); err != nil {
		t.Fatal(err)
	}
	if len(store.calls) != 0 {
		t.Fatalf("restart reapplied %v", store.calls)
	}
}

func TestAppliedChecksumChangeFails(t *testing.T) {
	store := &memoryStore{applied: map[string]string{"001.sql": "old"}}
	err := migrate(context.Background(), store, []migration{{name: "001.sql", checksum: "changed"}})
	if err == nil || !strings.Contains(err.Error(), "checksum changed") {
		t.Fatalf("unexpected error %v", err)
	}
	if len(store.calls) != 0 {
		t.Fatalf("changed migration was executed")
	}
}

func TestFailedMigrationIsNotRecorded(t *testing.T) {
	store := &memoryStore{applied: map[string]string{}, fail: "002.sql"}
	err := migrate(context.Background(), store, []migration{{name: "001.sql", checksum: "a"}, {name: "002.sql", checksum: "b"}})
	if err == nil {
		t.Fatal("expected failure")
	}
	if _, ok := store.applied["002.sql"]; ok {
		t.Fatal("failed migration recorded")
	}
}
