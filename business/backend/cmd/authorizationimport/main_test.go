package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestDecodeExportRejectsDigestAndLegacyFacts(t *testing.T) {
	payload, _ := json.Marshal(bundle{SchemaVersion: 1, Tables: requiredTables()})
	sum := sha256.Sum256(payload)
	document, _ := json.Marshal(envelope{SchemaVersion: 1, SHA256: hex.EncodeToString(sum[:]), RowCount: 1, Payload: payload})
	if _, _, err := decodeExport(document); err == nil {
		t.Fatal("row count mismatch was accepted")
	}
	legacy := requiredTables()
	for index := range legacy {
		if legacy[index].Name == "platform_role" {
			legacy[index].Present = true
			legacy[index].Rows = []map[string]any{{"role_id": 1}}
		}
	}
	payload, _ = json.Marshal(bundle{SchemaVersion: 1, Tables: legacy})
	sum = sha256.Sum256(payload)
	document, _ = json.Marshal(envelope{SchemaVersion: 1, SHA256: hex.EncodeToString(sum[:]), RowCount: 1, Payload: payload})
	if _, _, err := decodeExport(document); err == nil {
		t.Fatal("non-empty legacy authorization table was silently ignored")
	}
}

func TestReceiptMatchesPlatformCanonicalSignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fields := receiptFields{SchemaVersion: 1, Source: importSource, ImportID: "import-1", SHA256: "abc", RowCount: 3, Imported: true, ImportedAt: "2026-08-14T08:00:00.123Z", TargetIdentityInstance: "identity-local", TargetIdentitySchemaVersion: identityAuthSchemaVersion, KeyID: "migration-1"}
	value := signReceipt(fields, privateKey)
	signature, _ := base64.RawURLEncoding.DecodeString(value.Signature)
	canonical, _ := json.Marshal(fields)
	if !ed25519.Verify(publicKey, canonical, signature) {
		t.Fatal("receipt signature does not cover the canonical Platform field order")
	}
}

func TestFreshLocalPlatformAdminCanManageLiveProviders(t *testing.T) {
	permissions := map[string]bool{}
	for _, permission := range freshLocalManifest().PlatformPermissions {
		permissions[permission] = true
	}
	for _, required := range []string{"platform.live-provider.read", "platform.live-provider.manage", "platform.sms.read", "platform.sms.manage", "platform.email.read", "platform.email.manage", "platform.storage.read", "platform.storage.manage", "trade.payment-config.read", "trade.payment-config.manage"} {
		if !permissions[required] {
			t.Fatalf("fresh-local administrator is missing %s", required)
		}
	}
}

func requiredTables() []tableBundle {
	names := []string{"platform_authorization_domain", "platform_authorization_role", "platform_authorization_role_permission", "platform_authorization_role_scope", "platform_subject_grant", "platform_authorization_operation", "platform_business_delegation", "platform_entitlement_projection", "platform_inbox", "platform_outbox", "platform_role", "platform_role_permission", "platform_role_data_scope", "platform_role_scope_department", "platform_subject_role", "platform_subject_department"}
	tables := make([]tableBundle, 0, len(names))
	for _, name := range names {
		present := name == "platform_authorization_domain" || name == "platform_authorization_role" || name == "platform_authorization_role_permission" || name == "platform_authorization_role_scope" || name == "platform_subject_grant" || name == "platform_entitlement_projection"
		tables = append(tables, tableBundle{Name: name, Present: present, Rows: []map[string]any{}})
	}
	return tables
}
