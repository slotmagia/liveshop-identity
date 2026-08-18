// Command authorizationimport performs the one-time, fail-closed import of
// the retired Platform browser-user authorization facts. It has no runtime
// fallback: the imported Identity tables become the only write/read path.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	bundleSchemaVersion       = 1
	identityAuthSchemaVersion = 4
	importSource              = "liveshop-platform-authorization"
)

type envelope struct {
	SchemaVersion int             `json:"schemaVersion"`
	SHA256        string          `json:"sha256"`
	RowCount      int64           `json:"rowCount"`
	Payload       json.RawMessage `json:"payload"`
}
type bundle struct {
	SchemaVersion int           `json:"schemaVersion"`
	Tables        []tableBundle `json:"tables"`
}
type tableBundle struct {
	Name    string           `json:"name"`
	Present bool             `json:"present"`
	Rows    []map[string]any `json:"rows"`
}
type receipt struct {
	SchemaVersion               int    `json:"schemaVersion"`
	Source                      string `json:"source"`
	ImportID                    string `json:"importId"`
	SHA256                      string `json:"sha256"`
	RowCount                    int64  `json:"rowCount"`
	Imported                    bool   `json:"imported"`
	ImportedAt                  string `json:"importedAt"`
	TargetIdentityInstance      string `json:"targetIdentityInstance"`
	TargetIdentitySchemaVersion int    `json:"targetIdentitySchemaVersion"`
	KeyID                       string `json:"keyId"`
	Signature                   string `json:"signature"`
}
type receiptFields struct {
	SchemaVersion               int    `json:"schemaVersion"`
	Source                      string `json:"source"`
	ImportID                    string `json:"importId"`
	SHA256                      string `json:"sha256"`
	RowCount                    int64  `json:"rowCount"`
	Imported                    bool   `json:"imported"`
	ImportedAt                  string `json:"importedAt"`
	TargetIdentityInstance      string `json:"targetIdentityInstance"`
	TargetIdentitySchemaVersion int    `json:"targetIdentitySchemaVersion"`
	KeyID                       string `json:"keyId"`
}

func main() {
	dsn := flag.String("dsn", "", "Identity MySQL DSN after migrations and initial Registry projection")
	bootstrapLocal := flag.Bool("bootstrap-local", false, "install the explicit fresh-local IAM manifest after Registry projection")
	bootstrapID := flag.String("bootstrap-id", "liveshop-local-v1", "stable fresh-local bootstrap id")
	input := flag.String("input", "", "Platform authorization export envelope")
	importID := flag.String("import-id", "", "stable operator-selected import id")
	instance := flag.String("identity-instance", "", "stable target Identity instance identifier")
	receiptOutput := flag.String("receipt-output", "", "signed receipt file consumed by Platform finalize")
	keyID := flag.String("receipt-key-id", "", "Identity migration receipt signing key id")
	privateKey := flag.String("receipt-private-key", "", "base64url Ed25519 seed or private key")
	flag.Parse()
	if strings.TrimSpace(*dsn) == "" {
		fatal(errors.New("authorizationimport: -dsn is required"))
	}
	if !*bootstrapLocal && (strings.TrimSpace(*input) == "" || strings.TrimSpace(*importID) == "" || strings.TrimSpace(*instance) == "" || strings.TrimSpace(*receiptOutput) == "" || strings.TrimSpace(*keyID) == "" || strings.TrimSpace(*privateKey) == "") {
		fatal(errors.New("authorizationimport: -dsn, -input, -import-id, -identity-instance, -receipt-output, -receipt-key-id and -receipt-private-key are required"))
	}
	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if *bootstrapLocal {
		digest, err := bootstrapFreshLocal(ctx, db, *bootstrapID)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Identity local authorization bootstrap id=%s sha256=%s status=converged\n", *bootstrapID, digest)
		return
	}
	document, err := os.ReadFile(*input)
	if err != nil {
		fatal(fmt.Errorf("read export: %w", err))
	}
	export, tables, err := decodeExport(document)
	if err != nil {
		fatal(err)
	}
	key, err := decodePrivateKey(*privateKey)
	if err != nil {
		fatal(err)
	}
	unsigned, err := importBundle(ctx, db, export, tables, *importID, *instance, *keyID)
	if err != nil {
		fatal(err)
	}
	signed := signReceipt(unsigned, key)
	if err := writeReceipt(*receiptOutput, signed); err != nil {
		fatal(err)
	}
	fmt.Printf("Identity authorization import id=%s sha256=%s rows=%d receipt=%s\n", signed.ImportID, signed.SHA256, signed.RowCount, *receiptOutput)
}

func decodeExport(document []byte) (envelope, map[string][]map[string]any, error) {
	var e envelope
	if err := strictDecode(document, &e); err != nil {
		return e, nil, fmt.Errorf("authorizationimport: decode envelope: %w", err)
	}
	if e.SchemaVersion != bundleSchemaVersion || e.RowCount < 0 || len(e.Payload) == 0 {
		return e, nil, errors.New("authorizationimport: unsupported or incomplete export envelope")
	}
	digest := sha256.Sum256(e.Payload)
	if !strings.EqualFold(e.SHA256, hex.EncodeToString(digest[:])) {
		return e, nil, errors.New("authorizationimport: payload SHA256 does not match envelope")
	}
	var b bundle
	if err := strictDecode(e.Payload, &b); err != nil {
		return e, nil, fmt.Errorf("authorizationimport: decode payload: %w", err)
	}
	if b.SchemaVersion != bundleSchemaVersion {
		return e, nil, fmt.Errorf("authorizationimport: unsupported payload schema version %d", b.SchemaVersion)
	}
	known := []string{"platform_authorization_domain", "platform_authorization_role", "platform_authorization_role_permission", "platform_authorization_role_scope", "platform_subject_grant", "platform_authorization_operation", "platform_business_delegation", "platform_entitlement_projection", "platform_inbox", "platform_outbox", "platform_role", "platform_role_permission", "platform_role_data_scope", "platform_role_scope_department", "platform_subject_role", "platform_subject_department"}
	tables := make(map[string][]map[string]any, len(b.Tables))
	var rows int64
	var presentCount int
	for index, table := range b.Tables {
		if index >= len(known) || table.Name != known[index] {
			return e, nil, fmt.Errorf("authorizationimport: unknown or out-of-order table %q", table.Name)
		}
		if _, duplicate := tables[table.Name]; duplicate {
			return e, nil, fmt.Errorf("authorizationimport: duplicate table %s", table.Name)
		}
		if !table.Present && len(table.Rows) != 0 {
			return e, nil, fmt.Errorf("authorizationimport: absent table %s contains rows", table.Name)
		}
		if table.Present {
			presentCount++
		}
		tables[table.Name] = table.Rows
		rows += int64(len(table.Rows))
	}
	if len(b.Tables) != len(known) || presentCount == 0 {
		return e, nil, errors.New("authorizationimport: export must describe all known tables and at least one must be present")
	}
	if rows != e.RowCount {
		return e, nil, fmt.Errorf("authorizationimport: row count %d does not match envelope %d", rows, e.RowCount)
	}
	for _, name := range []string{"platform_authorization_domain", "platform_authorization_role", "platform_authorization_role_permission", "platform_authorization_role_scope", "platform_subject_grant", "platform_entitlement_projection"} {
		for _, table := range b.Tables {
			if table.Name == name && !table.Present {
				return e, nil, fmt.Errorf("authorizationimport: required v2 table %s is absent", name)
			}
		}
	}
	// These old models do not have a lossless mapping into the final domain.
	// A non-empty occurrence means the export was taken before the required
	// Platform v2 migration and must be remediated instead of silently ignored.
	for _, name := range []string{"platform_role", "platform_role_permission", "platform_role_data_scope", "platform_role_scope_department", "platform_subject_role", "platform_subject_department", "platform_business_delegation"} {
		if len(tables[name]) != 0 {
			return e, nil, fmt.Errorf("authorizationimport: unsupported non-empty retired table %s", name)
		}
	}
	return e, tables, nil
}

func strictDecode(document []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(document)))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON data")
	}
	return nil
}

type localBootstrapManifest struct {
	PlatformDomainID    int64    `json:"platformDomainId"`
	PlatformSubject     string   `json:"platformSubject"`
	PlatformRoleID      int64    `json:"platformRoleId"`
	PlatformRoleCode    string   `json:"platformRoleCode"`
	PlatformPermissions []string `json:"platformPermissions"`
	MerchantID          int64    `json:"merchantId"`
	CustomerSubject     string   `json:"customerSubject"`
	CustomerRoleID      int64    `json:"customerRoleId"`
	CustomerRoleCode    string   `json:"customerRoleCode"`
	CustomerPermissions []string `json:"customerPermissions"`
}

func freshLocalManifest() localBootstrapManifest {
	return localBootstrapManifest{
		PlatformDomainID: 1, PlatformSubject: "platform-admin", PlatformRoleID: 9000001, PlatformRoleCode: "LOCAL_PLATFORM_ADMIN",
		PlatformPermissions: []string{
			"identity.authorization.manage", "identity.customer-account.manage", "identity.directory.read", "identity.merchant.manage", "identity.merchant-governance.manage", "identity.session.manage", "identity.shop.read", "identity.shop-category.manage", "identity.subscription.manage", "identity.user.manage",
			"identity.placeholder.read", "catalog.placeholder.read", "trade.placeholder.read", "live.placeholder.read",
			"live.room.read",
			"platform.audit.read", "platform.registry.manage", "platform.settings.read", "platform.settings.write",
			"platform.live-provider.read", "platform.live-provider.manage",
			"platform.sms.read", "platform.sms.manage",
			"platform.email.read", "platform.email.manage",
			"platform.storage.read", "platform.storage.manage",
			"platform.notify-event.read", "platform.notify-event.manage",
			"platform.notify-template.read", "platform.notify-template.manage",
			"platform.notify-channel.read", "platform.notify-channel.manage",
			"platform.placeholder.read",
			"trade.overview.read", "trade.checkout.read", "trade.order.read", "trade.payment.read",
			"trade.payment-config.read", "trade.payment-config.manage",
			"trade.wallet.read", "trade.wallet.review", "trade.wallet.reconcile",
			"trade.finance.read", "trade.finance.policy.manage", "trade.finance.reconcile", "trade.settlement.read",
		},
		MerchantID: 2001, CustomerSubject: "customer-local", CustomerRoleID: 9000002, CustomerRoleCode: "LOCAL_CUSTOMER",
		CustomerPermissions: []string{
			"identity.placeholder.read",
			"catalog.category.read", "catalog.product.read", "catalog.cart.write", "catalog.placeholder.read",
			"trade.checkout.create", "trade.placeholder.read", "live.placeholder.read",
		},
	}
}

func bootstrapFreshLocal(ctx context.Context, db *sql.DB, bootstrapID string) (string, error) {
	if strings.TrimSpace(bootstrapID) == "" {
		return "", errors.New("authorizationimport: bootstrap id is required")
	}
	manifest := freshLocalManifest()
	document, _ := json.Marshal(manifest)
	digestBytes := sha256.Sum256(document)
	digest := hex.EncodeToString(digestBytes[:])
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT manifest_sha256 FROM identity_authorization_bootstrap_ledger WHERE bootstrap_id=? FOR UPDATE`, bootstrapID).Scan(&existing)
	if err == nil {
		if existing != digest {
			return "", errors.New("authorizationimport: bootstrap id is already bound to a different explicit permission manifest")
		}
		if err := tx.Commit(); err != nil {
			return "", err
		}
		return digest, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	permissions := append(append([]string{}, manifest.PlatformPermissions...), manifest.CustomerPermissions...)
	if err := validateExplicitPermissions(ctx, tx, permissions); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_domain(domain_type,domain_id,revision,entitlement_revision,platform_boundary_revision)
		VALUES('PLATFORM_ORG',?,1,0,1)
		ON DUPLICATE KEY UPDATE revision=revision+1,platform_boundary_revision=platform_boundary_revision+1,updated_at=CURRENT_TIMESTAMP(3)`, manifest.PlatformDomainID); err != nil {
		return "", fmt.Errorf("authorizationimport: bootstrap platform domain: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_role(domain_type,domain_id,role_id,code,name,status,system_role,version)
		VALUES('PLATFORM_ORG',?,?,?,'Local Platform Administrator','ACTIVE',1,1)
		ON DUPLICATE KEY UPDATE code=VALUES(code),name=VALUES(name),status='ACTIVE',system_role=1,version=version+1,updated_at=CURRENT_TIMESTAMP(3)`, manifest.PlatformDomainID, manifest.PlatformRoleID, manifest.PlatformRoleCode); err != nil {
		return "", fmt.Errorf("authorizationimport: bootstrap platform role: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM identity_authorization_role_permission WHERE domain_type='PLATFORM_ORG' AND domain_id=? AND role_id=?`, manifest.PlatformDomainID, manifest.PlatformRoleID); err != nil {
		return "", fmt.Errorf("authorizationimport: replace platform role policy: %w", err)
	}
	for _, code := range manifest.PlatformPermissions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_role_permission(domain_type,domain_id,role_id,permission_code) VALUES('PLATFORM_ORG',?,?,?)`, manifest.PlatformDomainID, manifest.PlatformRoleID, code); err != nil {
			return "", fmt.Errorf("authorizationimport: bootstrap platform permission %s: %w", code, err)
		}
	}
	if err := convergeLocalGrant(ctx, tx, "local-platform-admin-grant", "PLATFORM_ORG", manifest.PlatformDomainID, manifest.PlatformSubject, manifest.PlatformRoleID, 1, "local-platform-admin-grant-v2"); err != nil {
		return "", fmt.Errorf("authorizationimport: bootstrap platform grant: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_domain(domain_type,domain_id,revision,entitlement_revision,platform_boundary_revision)
		VALUES('MERCHANT',?,1,0,0) ON DUPLICATE KEY UPDATE revision=revision+1,updated_at=CURRENT_TIMESTAMP(3)`, manifest.MerchantID); err != nil {
		return "", fmt.Errorf("authorizationimport: bootstrap merchant domain: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_role(domain_type,domain_id,role_id,code,name,status,system_role,version)
		VALUES('MERCHANT',?,?,?,'Local Customer','ACTIVE',1,1)
		ON DUPLICATE KEY UPDATE code=VALUES(code),name=VALUES(name),status='ACTIVE',system_role=1,version=version+1,updated_at=CURRENT_TIMESTAMP(3)`, manifest.MerchantID, manifest.CustomerRoleID, manifest.CustomerRoleCode); err != nil {
		return "", fmt.Errorf("authorizationimport: bootstrap customer role: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM identity_authorization_role_permission WHERE domain_type='MERCHANT' AND domain_id=? AND role_id=?`, manifest.MerchantID, manifest.CustomerRoleID); err != nil {
		return "", fmt.Errorf("authorizationimport: replace customer role policy: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM identity_authorization_role_scope WHERE domain_type='MERCHANT' AND domain_id=? AND role_id=?`, manifest.MerchantID, manifest.CustomerRoleID); err != nil {
		return "", fmt.Errorf("authorizationimport: replace customer role scopes: %w", err)
	}
	for _, code := range manifest.CustomerPermissions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_role_permission(domain_type,domain_id,role_id,permission_code) VALUES('MERCHANT',?,?,?)`, manifest.MerchantID, manifest.CustomerRoleID, code); err != nil {
			return "", fmt.Errorf("authorizationimport: bootstrap customer permission %s: %w", code, err)
		}
		var resource string
		if err := tx.QueryRowContext(ctx, `SELECT resource_code FROM identity_permission_projection WHERE permission_code=? AND active=1`, code).Scan(&resource); err != nil {
			return "", fmt.Errorf("authorizationimport: resolve customer permission resource %s: %w", code, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_role_scope(domain_type,domain_id,role_id,resource_code,scope_type,reference_json)
			VALUES('MERCHANT',?,?,?,'CURRENT_SHOP',JSON_OBJECT())
			ON DUPLICATE KEY UPDATE scope_type='CURRENT_SHOP',reference_json=JSON_OBJECT()`, manifest.MerchantID, manifest.CustomerRoleID, resource); err != nil {
			return "", fmt.Errorf("authorizationimport: bootstrap customer scope %s: %w", resource, err)
		}
	}
	if err := convergeLocalGrant(ctx, tx, "local-customer-grant", "MERCHANT", manifest.MerchantID, manifest.CustomerSubject, manifest.CustomerRoleID, 0, "local-customer-grant-v1"); err != nil {
		return "", fmt.Errorf("authorizationimport: bootstrap customer grant: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_bootstrap_ledger(bootstrap_id,manifest_sha256,manifest_json) VALUES(?,?,?)`, bootstrapID, digest, document); err != nil {
		return "", fmt.Errorf("authorizationimport: record bootstrap ledger: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return digest, nil
}

func convergeLocalGrant(ctx context.Context, tx *sql.Tx, grantID, domainType string, domainID int64, subject string, roleID int64, accessVersion uint64, operationID string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE identity_subject_grant SET status='REVOKED',revoked_at=CURRENT_TIMESTAMP(3)
		WHERE domain_type=? AND domain_id=? AND subject=? AND role_id=? AND status='ACTIVE' AND grant_id<>?`, domainType, domainID, subject, roleID, grantID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_subject_grant(grant_id,domain_type,domain_id,subject,role_id,status,access_version,operation_id)
		VALUES(?,?,?,?,?,'ACTIVE',?,?)
		ON DUPLICATE KEY UPDATE domain_type=VALUES(domain_type),domain_id=VALUES(domain_id),subject=VALUES(subject),role_id=VALUES(role_id),status='ACTIVE',access_version=VALUES(access_version),operation_id=VALUES(operation_id),revoked_at=NULL`, grantID, domainType, domainID, subject, roleID, accessVersion, operationID)
	return err
}

func validateExplicitPermissions(ctx context.Context, tx *sql.Tx, permissions []string) error {
	var revision uint64
	if err := tx.QueryRowContext(ctx, `SELECT registry_revision FROM identity_registry_projection_state WHERE singleton_id=1 FOR SHARE`).Scan(&revision); err != nil || revision == 0 {
		return errors.New("authorizationimport: current Registry projection is required before local bootstrap")
	}
	seen := map[string]bool{}
	for _, code := range permissions {
		if seen[code] {
			continue
		}
		seen[code] = true
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_permission_projection WHERE permission_code=? AND active=1 AND registry_revision=?`, code, revision).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("authorizationimport: explicit local permission %q is absent from Registry revision %d", code, revision)
		}
	}
	return nil
}

func importBundle(ctx context.Context, db *sql.DB, e envelope, tables map[string][]map[string]any, importID, instance, keyID string) (receiptFields, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return receiptFields{}, err
	}
	defer tx.Rollback()
	if existing, found, err := existingReceipt(ctx, tx, importID, e.SHA256); err != nil {
		return receiptFields{}, err
	} else if found {
		if existing.TargetIdentityInstance != instance || existing.KeyID != keyID {
			return receiptFields{}, errors.New("authorizationimport: retry target instance or receipt key differs from the durable import")
		}
		if err := tx.Commit(); err != nil {
			return receiptFields{}, err
		}
		return existing, nil
	}
	if err := validatePermissionCatalog(ctx, tx, tables); err != nil {
		return receiptFields{}, err
	}
	if err := insertDomains(ctx, tx, tables["platform_authorization_domain"]); err != nil {
		return receiptFields{}, err
	}
	if err := insertRoles(ctx, tx, tables["platform_authorization_role"]); err != nil {
		return receiptFields{}, err
	}
	if err := insertRolePermissions(ctx, tx, tables["platform_authorization_role_permission"]); err != nil {
		return receiptFields{}, err
	}
	if err := insertRoleScopes(ctx, tx, tables["platform_authorization_role_scope"]); err != nil {
		return receiptFields{}, err
	}
	if err := insertGrants(ctx, tx, tables["platform_subject_grant"]); err != nil {
		return receiptFields{}, err
	}
	// platform_entitlement_projection belongs to the Subscription half of the
	// same one-time handoff. Identity deliberately does not consume or mirror it
	// here; runtime projection starts only from Subscription's versioned API.
	unsigned := receiptFields{SchemaVersion: bundleSchemaVersion, Source: importSource, ImportID: importID, SHA256: strings.ToLower(e.SHA256), RowCount: e.RowCount, Imported: true, ImportedAt: time.Now().UTC().Format(time.RFC3339Nano), TargetIdentityInstance: instance, TargetIdentitySchemaVersion: identityAuthSchemaVersion, KeyID: keyID}
	payload, _ := json.Marshal(unsigned)
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_import_ledger(import_id,source,schema_version,payload_sha256,row_count,target_identity_instance,target_identity_schema_version,receipt_key_id,receipt_json,imported_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, unsigned.ImportID, unsigned.Source, unsigned.SchemaVersion, unsigned.SHA256, unsigned.RowCount, unsigned.TargetIdentityInstance, unsigned.TargetIdentitySchemaVersion, unsigned.KeyID, payload, unsigned.ImportedAt); err != nil {
		return receiptFields{}, fmt.Errorf("authorizationimport: record durable import ledger: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return receiptFields{}, err
	}
	return unsigned, nil
}

func existingReceipt(ctx context.Context, tx *sql.Tx, importID, digest string) (receiptFields, bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT import_id,payload_sha256,receipt_json FROM identity_authorization_import_ledger WHERE import_id=? OR payload_sha256=? FOR UPDATE`, importID, strings.ToLower(digest))
	if err != nil {
		return receiptFields{}, false, err
	}
	defer rows.Close()
	var found *receiptFields
	for rows.Next() {
		var existingID, existingDigest string
		var raw []byte
		if err := rows.Scan(&existingID, &existingDigest, &raw); err != nil {
			return receiptFields{}, false, err
		}
		if existingID == importID && !strings.EqualFold(existingDigest, digest) {
			return receiptFields{}, false, errors.New("authorizationimport: import id is already bound to a different digest")
		}
		var current receiptFields
		if err := strictDecode(raw, &current); err != nil || !strings.EqualFold(current.SHA256, digest) {
			return receiptFields{}, false, errors.New("authorizationimport: persisted import ledger receipt is invalid")
		}
		found = &current
	}
	if err := rows.Err(); err != nil {
		return receiptFields{}, false, err
	}
	if found != nil {
		return *found, true, nil
	}
	return receiptFields{}, false, nil
}

func validatePermissionCatalog(ctx context.Context, tx *sql.Tx, tables map[string][]map[string]any) error {
	var revision uint64
	if err := tx.QueryRowContext(ctx, `SELECT registry_revision FROM identity_registry_projection_state WHERE singleton_id=1 FOR SHARE`).Scan(&revision); err != nil || revision == 0 {
		return errors.New("authorizationimport: current Registry projection is required before import")
	}
	codes := map[string]bool{}
	for _, name := range []string{"platform_authorization_role_permission", "platform_entitlement_projection"} {
		for _, row := range tables[name] {
			code, err := stringField(row, "permission_code")
			if err != nil {
				return tableError(name, err)
			}
			codes[code] = true
		}
	}
	ordered := make([]string, 0, len(codes))
	for code := range codes {
		ordered = append(ordered, code)
	}
	sort.Strings(ordered)
	for _, code := range ordered {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_permission_projection WHERE permission_code=? AND active=1 AND registry_revision=?`, code, revision).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("authorizationimport: permission %q is absent from the current active Registry projection", code)
		}
	}
	return nil
}

func insertDomains(ctx context.Context, tx *sql.Tx, rows []map[string]any) error {
	for _, row := range rows {
		typeValue, err := stringField(row, "domain_type")
		if err != nil || (typeValue != "PLATFORM_ORG" && typeValue != "MERCHANT") {
			return tableError("platform_authorization_domain", errors.New("invalid domain_type"))
		}
		id, err := intField(row, "domain_id")
		if err != nil || id <= 0 {
			return tableError("platform_authorization_domain", errors.New("invalid domain_id"))
		}
		revision, err := intField(row, "revision")
		if err != nil || revision <= 0 {
			return tableError("platform_authorization_domain", errors.New("invalid revision"))
		}
		entitlement := int64(0)
		boundary := int64(0)
		if typeValue == "PLATFORM_ORG" {
			identityRevision, _ := optionalIntField(row, "identity_revision")
			boundary = maxInt64(1, revision, identityRevision)
			entitlement = 0
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_domain(domain_type,domain_id,revision,entitlement_revision,platform_boundary_revision) VALUES(?,?,?,?,?)`, typeValue, id, revision, entitlement, boundary); err != nil {
			return fmt.Errorf("authorizationimport: insert domain %s:%d: %w", typeValue, id, err)
		}
	}
	return nil
}

func insertRoles(ctx context.Context, tx *sql.Tx, rows []map[string]any) error {
	for _, row := range rows {
		domainType, domainID, roleID, err := roleKey(row)
		if err != nil {
			return tableError("platform_authorization_role", err)
		}
		code, err := stringField(row, "code")
		if err != nil {
			return tableError("platform_authorization_role", err)
		}
		name, err := stringField(row, "name")
		if err != nil {
			return tableError("platform_authorization_role", err)
		}
		status, err := stringField(row, "status")
		if err != nil {
			return tableError("platform_authorization_role", err)
		}
		systemRole, err := boolField(row, "system_role")
		if err != nil {
			return tableError("platform_authorization_role", err)
		}
		version, err := intField(row, "version")
		if err != nil || roleID <= 0 || version <= 0 {
			return tableError("platform_authorization_role", errors.New("invalid role id/version"))
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_role(domain_type,domain_id,role_id,code,name,status,system_role,version) VALUES(?,?,?,?,?,?,?,?)`, domainType, domainID, roleID, code, name, status, systemRole, version); err != nil {
			return fmt.Errorf("authorizationimport: insert role %s:%d:%d: %w", domainType, domainID, roleID, err)
		}
	}
	return nil
}

func insertRolePermissions(ctx context.Context, tx *sql.Tx, rows []map[string]any) error {
	for _, row := range rows {
		domainType, domainID, roleID, err := roleKey(row)
		if err != nil {
			return tableError("platform_authorization_role_permission", err)
		}
		code, err := stringField(row, "permission_code")
		if err != nil {
			return tableError("platform_authorization_role_permission", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_role_permission(domain_type,domain_id,role_id,permission_code) VALUES(?,?,?,?)`, domainType, domainID, roleID, code); err != nil {
			return fmt.Errorf("authorizationimport: insert role permission: %w", err)
		}
	}
	return nil
}

func insertRoleScopes(ctx context.Context, tx *sql.Tx, rows []map[string]any) error {
	for _, row := range rows {
		domainType, domainID, roleID, err := roleKey(row)
		if err != nil {
			return tableError("platform_authorization_role_scope", err)
		}
		resource, err := stringField(row, "resource_code")
		if err != nil {
			return tableError("platform_authorization_role_scope", err)
		}
		scope, err := stringField(row, "scope_type")
		if err != nil {
			return tableError("platform_authorization_role_scope", err)
		}
		references, err := jsonField(row, "reference_json")
		if err != nil {
			return tableError("platform_authorization_role_scope", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_role_scope(domain_type,domain_id,role_id,resource_code,scope_type,reference_json) VALUES(?,?,?,?,?,?)`, domainType, domainID, roleID, resource, scope, references); err != nil {
			return fmt.Errorf("authorizationimport: insert role scope: %w", err)
		}
	}
	return nil
}

func insertGrants(ctx context.Context, tx *sql.Tx, rows []map[string]any) error {
	for _, row := range rows {
		grantID, err := stringField(row, "grant_id")
		if err != nil {
			return tableError("platform_subject_grant", err)
		}
		domainType, err := stringField(row, "domain_type")
		if err != nil {
			return tableError("platform_subject_grant", err)
		}
		domainID, _ := intField(row, "domain_id")
		subject, err := stringField(row, "subject")
		if err != nil {
			return tableError("platform_subject_grant", err)
		}
		roleID, _ := intField(row, "role_id")
		status, err := stringField(row, "status")
		if err != nil {
			return tableError("platform_subject_grant", err)
		}
		accessVersion, _ := intField(row, "access_version")
		operationID, err := stringField(row, "operation_id")
		if err != nil {
			return tableError("platform_subject_grant", err)
		}
		createdAt, _ := optionalTimeField(row, "created_at")
		revokedAt, _ := optionalTimeField(row, "revoked_at")
		if domainID <= 0 || roleID <= 0 || accessVersion <= 0 {
			return tableError("platform_subject_grant", errors.New("invalid domain/role/access version"))
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_subject_grant(grant_id,domain_type,domain_id,subject,role_id,status,access_version,operation_id,created_at,revoked_at) VALUES(?,?,?,?,?,?,?,?,COALESCE(?,CURRENT_TIMESTAMP(3)),?)`, grantID, domainType, domainID, subject, roleID, status, accessVersion, operationID, createdAt, revokedAt); err != nil {
			return fmt.Errorf("authorizationimport: insert subject grant: %w", err)
		}
	}
	return nil
}

func roleKey(row map[string]any) (string, int64, int64, error) {
	domainType, err := stringField(row, "domain_type")
	if err != nil {
		return "", 0, 0, err
	}
	domainID, err := intField(row, "domain_id")
	if err != nil {
		return "", 0, 0, err
	}
	roleID, err := intField(row, "role_id")
	return domainType, domainID, roleID, err
}

func stringField(row map[string]any, name string) (string, error) {
	value, ok := row[name]
	if !ok {
		return "", fmt.Errorf("missing %s", name)
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("invalid %s", name)
	}
	return text, nil
}
func intField(row map[string]any, name string) (int64, error) {
	value, ok := row[name]
	if !ok {
		return 0, fmt.Errorf("missing %s", name)
	}
	switch current := value.(type) {
	case json.Number:
		return current.Int64()
	case float64:
		return int64(current), nil
	case string:
		return strconv.ParseInt(current, 10, 64)
	default:
		return 0, fmt.Errorf("invalid %s", name)
	}
}
func optionalIntField(row map[string]any, name string) (int64, error) {
	if row[name] == nil {
		return 0, nil
	}
	return intField(row, name)
}
func boolField(row map[string]any, name string) (bool, error) {
	if value, ok := row[name].(bool); ok {
		return value, nil
	}
	value, err := intField(row, name)
	return value != 0, err
}
func jsonField(row map[string]any, name string) ([]byte, error) {
	value, ok := row[name]
	if !ok || value == nil {
		return nil, fmt.Errorf("missing %s", name)
	}
	if text, ok := value.(string); ok {
		var decoded any
		if json.Unmarshal([]byte(text), &decoded) != nil {
			return nil, fmt.Errorf("invalid %s", name)
		}
		return []byte(text), nil
	}
	return json.Marshal(value)
}
func optionalTimeField(row map[string]any, name string) (any, error) {
	value, ok := row[name]
	if !ok || value == nil {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("invalid %s", name)
	}
	parsed, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return nil, fmt.Errorf("invalid %s", name)
	}
	return parsed.UTC(), nil
}
func tableError(table string, err error) error {
	return fmt.Errorf("authorizationimport: invalid %s row: %w", table, err)
}
func maxInt64(values ...int64) int64 {
	var result int64
	for _, value := range values {
		if value > result {
			result = value
		}
	}
	return result
}

func decodePrivateKey(encoded string) (ed25519.PrivateKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("authorizationimport: receipt private key is not base64url")
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	default:
		return nil, errors.New("authorizationimport: receipt private key must be an Ed25519 seed or private key")
	}
}
func signReceipt(fields receiptFields, key ed25519.PrivateKey) receipt {
	payload, _ := json.Marshal(fields)
	return receipt{SchemaVersion: fields.SchemaVersion, Source: fields.Source, ImportID: fields.ImportID, SHA256: fields.SHA256, RowCount: fields.RowCount, Imported: fields.Imported, ImportedAt: fields.ImportedAt, TargetIdentityInstance: fields.TargetIdentityInstance, TargetIdentitySchemaVersion: fields.TargetIdentitySchemaVersion, KeyID: fields.KeyID, Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, payload))}
}
func writeReceipt(path string, value receipt) error {
	document, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".identity-authorization-receipt-*.tmp")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(document, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
