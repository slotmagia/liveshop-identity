package mysql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	biz "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
)

type PermissionEntitlementRepository struct{ database *sql.DB }

func NewPermissionEntitlementRepository(database *sql.DB) *PermissionEntitlementRepository {
	return &PermissionEntitlementRepository{database: database}
}

func (r *PermissionEntitlementRepository) GetPermissionEntitlement(ctx context.Context, merchantID int64) (biz.PermissionEntitlement, error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return biz.PermissionEntitlement{}, fmt.Errorf("subscription begin permission snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	snapshot, err := readPermissionEntitlement(ctx, tx, merchantID)
	if err != nil {
		return biz.PermissionEntitlement{}, err
	}
	if err := tx.Commit(); err != nil {
		return biz.PermissionEntitlement{}, fmt.Errorf("subscription commit permission snapshot: %w", err)
	}
	return snapshot, nil
}

func (r *PermissionEntitlementRepository) ApplyPermissionEntitlement(ctx context.Context, command biz.ApplyPermissionEntitlementCommand) (biz.PermissionEntitlement, bool, error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription begin permission command: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	requestDigest := command.RequestDigest()
	_, err = tx.ExecContext(ctx, `INSERT INTO subscription_permission_entitlement_command
  (merchant_id,command_key,request_hash) VALUES(?,?,?)`, command.MerchantID, command.CommandKey, requestDigest[:])
	if duplicateKey(err) {
		var storedDigest []byte
		var revision uint64
		var responseJSON []byte
		if scanErr := tx.QueryRowContext(ctx, `SELECT request_hash,response_revision,response_json
FROM subscription_permission_entitlement_command
WHERE merchant_id=? AND command_key=? FOR UPDATE`, command.MerchantID, command.CommandKey).Scan(&storedDigest, &revision, &responseJSON); scanErr != nil {
			return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription read permission replay: %w", scanErr)
		}
		if len(storedDigest) != len(requestDigest) || subtle.ConstantTimeCompare(storedDigest, requestDigest[:]) != 1 {
			return biz.PermissionEntitlement{}, false, biz.ErrIdempotencyConflict
		}
		if revision == 0 || len(responseJSON) == 0 {
			return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription permission command is incomplete")
		}
		var snapshot biz.PermissionEntitlement
		if unmarshalErr := json.Unmarshal(responseJSON, &snapshot); unmarshalErr != nil || snapshot.MerchantID != command.MerchantID || snapshot.Revision != revision {
			return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription permission command response is invalid")
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription commit permission replay: %w", commitErr)
		}
		return snapshot, true, nil
	}
	if err != nil {
		return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription insert permission command: %w", err)
	}

	var currentRevision uint64
	err = tx.QueryRowContext(ctx, `SELECT revision FROM subscription_permission_entitlement_state WHERE merchant_id=? FOR UPDATE`, command.MerchantID).Scan(&currentRevision)
	switch {
	case errors.Is(err, sql.ErrNoRows) && command.ExpectedRevision == 0:
		currentRevision = 0
	case errors.Is(err, sql.ErrNoRows):
		return biz.PermissionEntitlement{}, false, biz.ErrVersionConflict
	case err != nil:
		return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription lock permission entitlement: %w", err)
	case currentRevision != command.ExpectedRevision:
		return biz.PermissionEntitlement{}, false, biz.ErrVersionConflict
	}

	nextRevision := currentRevision + 1
	snapshotDigest := biz.PermissionSnapshotDigest(command.PermissionCodes)
	_, err = tx.ExecContext(ctx, `INSERT INTO subscription_permission_entitlement_state
  (merchant_id,revision,snapshot_digest,updated_at) VALUES(?,?,?,CURRENT_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE revision=VALUES(revision),snapshot_digest=VALUES(snapshot_digest),updated_at=VALUES(updated_at)`, command.MerchantID, nextRevision, snapshotDigest[:])
	if err != nil {
		return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription write permission state: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM subscription_permission_entitlement_item WHERE merchant_id=?`, command.MerchantID); err != nil {
		return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription clear permission items: %w", err)
	}
	for _, code := range command.PermissionCodes {
		if _, err = tx.ExecContext(ctx, `INSERT INTO subscription_permission_entitlement_item
  (merchant_id,permission_code,revision) VALUES(?,?,?)`, command.MerchantID, code, nextRevision); err != nil {
			return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription write permission item %s: %w", code, err)
		}
	}
	snapshot, err := readPermissionEntitlementRevision(ctx, tx, command.MerchantID, nextRevision)
	if err != nil {
		return biz.PermissionEntitlement{}, false, err
	}
	responseJSON, err := json.Marshal(snapshot)
	if err != nil {
		return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription encode permission command response: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE subscription_permission_entitlement_command
SET response_revision=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=? AND command_key=?`, nextRevision, responseJSON, command.MerchantID, command.CommandKey); err != nil {
		return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription complete permission command: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return biz.PermissionEntitlement{}, false, fmt.Errorf("subscription commit permission command: %w", err)
	}
	return snapshot, false, nil
}

type permissionQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readPermissionEntitlement(ctx context.Context, query permissionQueryer, merchantID int64) (biz.PermissionEntitlement, error) {
	return readPermissionEntitlementRevision(ctx, query, merchantID, 0)
}

func readPermissionEntitlementRevision(ctx context.Context, query permissionQueryer, merchantID int64, expected uint64) (biz.PermissionEntitlement, error) {
	var snapshot biz.PermissionEntitlement
	var digest []byte
	err := query.QueryRowContext(ctx, `SELECT merchant_id,revision,snapshot_digest,updated_at
FROM subscription_permission_entitlement_state WHERE merchant_id=?`, merchantID).
		Scan(&snapshot.MerchantID, &snapshot.Revision, &digest, &snapshot.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return biz.PermissionEntitlement{}, biz.ErrPermissionEntitlementNotConfigured
	}
	if err != nil {
		return biz.PermissionEntitlement{}, fmt.Errorf("subscription read permission state: %w", err)
	}
	if expected != 0 && snapshot.Revision != expected {
		return biz.PermissionEntitlement{}, fmt.Errorf("subscription permission replay revision changed")
	}
	rows, err := query.QueryContext(ctx, `SELECT permission_code FROM subscription_permission_entitlement_item
WHERE merchant_id=? AND revision=? ORDER BY permission_code`, merchantID, snapshot.Revision)
	if err != nil {
		return biz.PermissionEntitlement{}, fmt.Errorf("subscription read permission items: %w", err)
	}
	defer rows.Close()
	snapshot.PermissionCodes = []string{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return biz.PermissionEntitlement{}, fmt.Errorf("subscription scan permission item: %w", err)
		}
		snapshot.PermissionCodes = append(snapshot.PermissionCodes, code)
	}
	if err := rows.Err(); err != nil {
		return biz.PermissionEntitlement{}, fmt.Errorf("subscription iterate permission items: %w", err)
	}
	calculated := biz.PermissionSnapshotDigest(snapshot.PermissionCodes)
	if len(digest) != len(calculated) || subtle.ConstantTimeCompare(digest, calculated[:]) != 1 {
		return biz.PermissionEntitlement{}, fmt.Errorf("subscription permission snapshot digest mismatch")
	}
	snapshot.SnapshotDigest = biz.HexDigest(calculated)
	snapshot.UpdatedAt = snapshot.UpdatedAt.UTC().Truncate(time.Millisecond)
	return snapshot, nil
}
