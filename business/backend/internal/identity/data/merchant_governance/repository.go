package mysql

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
)

type Repository struct{ database *sql.DB }

var _ merchant_governance.Repository = (*Repository)(nil)

func NewRepository(database *sql.DB) *Repository { return &Repository{database: database} }

func (r *Repository) List(ctx context.Context, query model.Query) (model.Page, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.Page{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureShopScope(ctx, tx, query.MerchantID, query.ShopID, false); err != nil {
		return model.Page{}, err
	}
	where := "merchant_id=? AND shop_id=?"
	args := []any{query.MerchantID, query.ShopID}
	if query.Module != "" {
		where += " AND module=?"
		args = append(args, query.Module)
	}
	rows, err := tx.QueryContext(ctx, `SELECT capability_id,merchant_id,shop_id,module,name,merchant_status,platform_status,platform_reason_public,version,updated_by,updated_at
FROM identity_merchant_capability WHERE `+where+` ORDER BY module`, args...)
	if err != nil {
		return model.Page{}, fmt.Errorf("merchant governance list: %w", err)
	}
	defer rows.Close()
	items := []model.Capability{}
	for rows.Next() {
		value, err := scanCapability(rows)
		if err != nil {
			return model.Page{}, fmt.Errorf("merchant governance list scan: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.Page{}, fmt.Errorf("merchant governance list iterate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Page{}, fmt.Errorf("merchant governance list commit: %w", err)
	}
	return model.Page{Items: items}, nil
}

func (r *Repository) Audit(ctx context.Context, query model.AuditQuery) ([]model.AuditItem, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureShopScope(ctx, tx, query.MerchantID, query.ShopID, false); err != nil {
		return nil, err
	}
	where := "merchant_id=? AND shop_id=?"
	args := []any{query.MerchantID, query.ShopID}
	if query.Module != "" {
		where += " AND module=?"
		args = append(args, query.Module)
	}
	args = append(args, query.Limit)
	rows, err := tx.QueryContext(ctx, `SELECT audit_id,merchant_id,shop_id,module,capability_id,action,operator,reason_internal,reason_public,created_at
FROM identity_merchant_capability_audit WHERE `+where+` ORDER BY audit_id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("merchant governance audit: %w", err)
	}
	defer rows.Close()
	items := []model.AuditItem{}
	for rows.Next() {
		var item model.AuditItem
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.ShopID, &item.Module,
			&item.CapabilityID, &item.Action, &item.Operator, &item.ReasonInternal, &item.ReasonPublic, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("merchant governance audit scan: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("merchant governance audit iterate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("merchant governance audit commit: %w", err)
	}
	return items, nil
}

func (r *Repository) Intervene(ctx context.Context, command model.InterveneCommand) (model.Capability, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Capability{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	document, found, err := insertOrReplay(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Capability{}, false, err
	}
	if found {
		var replay model.Capability
		if json.Unmarshal(document, &replay) != nil || replay.ValidatePersisted() != nil {
			return model.Capability{}, false, fmt.Errorf("merchant governance command response is incomplete")
		}
		if err := tx.Commit(); err != nil {
			return model.Capability{}, false, fmt.Errorf("merchant governance replay commit: %w", err)
		}
		return replay, true, nil
	}
	if err := ensureShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Capability{}, false, err
	}
	current, err := readCapabilityForUpdate(ctx, tx, command.MerchantID, command.ShopID, command.Module)
	exists := err == nil
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return model.Capability{}, false, err
	}
	if exists && current.Version != command.ExpectedVersion {
		return model.Capability{}, false, model.ErrConflict
	}
	if !exists && command.ExpectedVersion != 0 {
		return model.Capability{}, false, model.ErrConflict
	}
	before, _ := json.Marshal(map[string]any{
		"platformStatus": current.PlatformStatus, "platformReasonPublic": current.PlatformReasonPublic, "version": current.Version,
	})
	after := map[string]any{"platformStatus": command.PlatformStatus, "platformReasonPublic": command.ReasonPublic}
	name := model.ModuleLabel(command.Module)
	var value model.Capability
	if !exists {
		result, err := tx.ExecContext(ctx, `INSERT INTO identity_merchant_capability
(merchant_id,shop_id,module,name,merchant_status,platform_status,platform_reason_public,version,updated_by)
VALUES(?,?,?,?,?,?,?,1,?)`, command.MerchantID, command.ShopID, command.Module, name,
			model.MerchantUnset, command.PlatformStatus, command.ReasonPublic, command.Operator)
		if duplicate(err) {
			return model.Capability{}, false, model.ErrConflict
		}
		if err != nil {
			return model.Capability{}, false, fmt.Errorf("merchant governance insert: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return model.Capability{}, false, fmt.Errorf("merchant governance inserted id: %w", err)
		}
		value, err = readCapability(ctx, tx, id, command.MerchantID, command.ShopID)
		if err != nil {
			return model.Capability{}, false, err
		}
	} else {
		result, err := tx.ExecContext(ctx, `UPDATE identity_merchant_capability
SET platform_status=?,platform_reason_public=?,version=version+1,updated_by=?,updated_at=CURRENT_TIMESTAMP(3)
WHERE capability_id=? AND merchant_id=? AND shop_id=? AND version=?`, command.PlatformStatus, command.ReasonPublic, command.Operator,
			current.ID, command.MerchantID, command.ShopID, command.ExpectedVersion)
		if err != nil {
			return model.Capability{}, false, fmt.Errorf("merchant governance update: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return model.Capability{}, false, model.ErrConflict
		}
		value, err = readCapability(ctx, tx, current.ID, command.MerchantID, command.ShopID)
		if err != nil {
			return model.Capability{}, false, err
		}
	}
	after["version"] = value.Version
	afterJSON, _ := json.Marshal(after)
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_merchant_capability_audit
(merchant_id,shop_id,module,capability_id,action,operator,reason_internal,reason_public,before_json,after_json)
VALUES(?,?,?,?,?,?,?,?,?,?)`, value.MerchantID, value.ShopID, value.Module, value.ID,
		"set_platform_status", command.Operator, command.ReasonInternal, command.ReasonPublic, before, afterJSON); err != nil {
		return model.Capability{}, false, fmt.Errorf("merchant governance audit: %w", err)
	}
	if err := appendOutbox(ctx, tx, "merchant_capability", fmt.Sprintf("%d", value.ID), value.Version, "identity.merchant_capability.changed", map[string]any{
		"capabilityId": value.ID, "merchantId": value.MerchantID, "shopId": value.ShopID,
		"module": value.Module, "platformStatus": value.PlatformStatus,
		"platformReasonPublic": value.PlatformReasonPublic, "operator": command.Operator,
	}); err != nil {
		return model.Capability{}, false, err
	}
	if err := completeCommand(ctx, tx, command.CommandKey, value.Version, value); err != nil {
		return model.Capability{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Capability{}, false, fmt.Errorf("merchant governance commit: %w", err)
	}
	return value, false, nil
}

func (r *Repository) begin(ctx context.Context, readOnly bool) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: readOnly})
	if err != nil {
		return nil, fmt.Errorf("merchant governance begin: %w", err)
	}
	return tx, nil
}

func ensureShopScope(ctx context.Context, tx *sql.Tx, merchantID, shopID int64, lock bool) error {
	query := `SELECT merchant_id,shop_id FROM identity_shop WHERE merchant_id=? AND shop_id=? AND status<>'CLOSED'`
	if lock {
		query += " FOR UPDATE"
	}
	var foundMerchantID, foundShopID int64
	err := tx.QueryRowContext(ctx, query, merchantID, shopID).Scan(&foundMerchantID, &foundShopID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("merchant governance shop scope: %w", err)
	}
	return nil
}

type scanner interface{ Scan(...any) error }

func scanCapability(row scanner) (model.Capability, error) {
	var value model.Capability
	err := row.Scan(&value.ID, &value.MerchantID, &value.ShopID, &value.Module,
		&value.Name, &value.MerchantStatus, &value.PlatformStatus, &value.PlatformReasonPublic, &value.Version, &value.UpdatedBy, &value.UpdatedAt)
	value.ModuleLabel = model.ModuleLabel(value.Module)
	return value, err
}

func readCapability(ctx context.Context, tx *sql.Tx, id, merchantID, shopID int64) (model.Capability, error) {
	value, err := scanCapability(tx.QueryRowContext(ctx, `SELECT capability_id,merchant_id,shop_id,module,name,merchant_status,platform_status,platform_reason_public,version,updated_by,updated_at
FROM identity_merchant_capability WHERE capability_id=? AND merchant_id=? AND shop_id=?`, id, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Capability{}, model.ErrNotFound
	}
	if err != nil {
		return model.Capability{}, fmt.Errorf("merchant governance read: %w", err)
	}
	return value, nil
}

func readCapabilityForUpdate(ctx context.Context, tx *sql.Tx, merchantID, shopID int64, module string) (model.Capability, error) {
	value, err := scanCapability(tx.QueryRowContext(ctx, `SELECT capability_id,merchant_id,shop_id,module,name,merchant_status,platform_status,platform_reason_public,version,updated_by,updated_at
FROM identity_merchant_capability WHERE merchant_id=? AND shop_id=? AND module=? FOR UPDATE`, merchantID, shopID, module))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Capability{}, model.ErrNotFound
	}
	if err != nil {
		return model.Capability{}, fmt.Errorf("merchant governance lock: %w", err)
	}
	return value, nil
}

func insertOrReplay(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) ([]byte, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_merchant_capability_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !duplicate(err) {
		if err != nil {
			return nil, false, fmt.Errorf("merchant governance insert command: %w", err)
		}
		return nil, false, nil
	}
	var stored, document []byte
	var version uint64
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_merchant_capability_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &version, &document); err != nil {
		return nil, false, fmt.Errorf("merchant governance read command: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return nil, false, model.ErrIdempotency
	}
	if version == 0 || len(document) == 0 {
		return nil, false, fmt.Errorf("merchant governance command response is incomplete")
	}
	return document, true, nil
}

func completeCommand(ctx context.Context, tx *sql.Tx, key string, version uint64, response any) error {
	document, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("merchant governance encode response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_merchant_capability_command
SET response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, version, document, key)
	if err != nil {
		return fmt.Errorf("merchant governance complete command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("merchant governance command completion affected no row")
	}
	return nil
}

func appendOutbox(ctx context.Context, tx *sql.Tx, aggregateType, aggregateID string, version uint64, eventType string, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	id := make([]byte, 18)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO identity_outbox (event_id, aggregate_type, aggregate_id, aggregate_version, event_type, payload_json) VALUES (?, ?, ?, ?, ?, ?)`,
		hex.EncodeToString(id), aggregateType, aggregateID, version, eventType, encoded)
	return err
}

func duplicate(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
