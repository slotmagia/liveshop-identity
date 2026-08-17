package mysql

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	subscriptionv1 "github.com/lvtuopen-ai/liveshop-identity/protocol/gen/go/subscription/v1"
)

func (r *AuthorizationRepository) MerchantDomainIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT domain_id FROM identity_authorization_domain WHERE domain_type='MERCHANT' ORDER BY domain_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *AuthorizationRepository) ReplaceEntitlementSnapshot(ctx context.Context, snapshot *subscriptionv1.GetMerchantPermissionEntitlementSnapshotResponse) error {
	if snapshot == nil || snapshot.GetMerchantId() <= 0 || snapshot.GetRevision() <= 0 {
		return model.ErrEntitlementUnavailable
	}
	codes := append([]string{}, snapshot.GetPermissionCodes()...)
	sort.Strings(codes)
	for index, code := range codes {
		if strings.TrimSpace(code) != code || code == "" || (index > 0 && codes[index-1] == code) {
			return model.ErrEntitlementUnavailable
		}
	}
	calculated := sha256.Sum256([]byte(strings.Join(codes, "\n")))
	declared, err := hex.DecodeString(snapshot.GetSnapshotDigest())
	if err != nil || len(declared) != len(calculated) || subtle.ConstantTimeCompare(declared, calculated[:]) != 1 {
		return model.ErrEntitlementUnavailable
	}
	sourceUpdatedAt := time.UnixMilli(snapshot.GetUpdatedAtUnixMs()).UTC()
	if snapshot.GetUpdatedAtUnixMs() <= 0 || sourceUpdatedAt.After(time.Now().UTC().Add(time.Minute)) {
		return model.ErrEntitlementUnavailable
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current uint64
	var currentDigest []byte
	err = tx.QueryRowContext(ctx, `SELECT entitlement_revision,snapshot_digest FROM identity_entitlement_projection_state WHERE merchant_id=? FOR UPDATE`, snapshot.GetMerchantId()).Scan(&current, &currentDigest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	incoming := uint64(snapshot.GetRevision())
	decision, err := decideEntitlementProjection(current, currentDigest, incoming, calculated[:])
	if err != nil {
		return err
	}
	if decision == entitlementRefresh {
		if _, err = tx.ExecContext(ctx, `UPDATE identity_entitlement_projection_state SET projected_at=CURRENT_TIMESTAMP(3) WHERE merchant_id=?`, snapshot.GetMerchantId()); err != nil {
			return err
		}
		return tx.Commit()
	}
	var domainCount int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_authorization_domain WHERE domain_type='MERCHANT' AND domain_id=? FOR UPDATE`, snapshot.GetMerchantId()).Scan(&domainCount); err != nil {
		return err
	}
	if domainCount != 1 {
		return model.ErrAuthorizationNotFound
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO identity_entitlement_projection_state
  (merchant_id,entitlement_revision,snapshot_digest,source_updated_at,projected_at)
VALUES(?,?,?,?,CURRENT_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE entitlement_revision=VALUES(entitlement_revision),snapshot_digest=VALUES(snapshot_digest),source_updated_at=VALUES(source_updated_at),projected_at=VALUES(projected_at)`, snapshot.GetMerchantId(), incoming, calculated[:], sourceUpdatedAt); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM identity_entitlement_projection WHERE merchant_id=?`, snapshot.GetMerchantId()); err != nil {
		return err
	}
	for _, code := range codes {
		if _, err = tx.ExecContext(ctx, `INSERT INTO identity_entitlement_projection(merchant_id,permission_code,status,entitlement_revision) VALUES(?,?,'ACTIVE',?)`, snapshot.GetMerchantId(), code, incoming); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_authorization_domain
SET entitlement_revision=?,revision=revision+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE domain_type='MERCHANT' AND domain_id=?`, incoming, snapshot.GetMerchantId())
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.ErrAuthorizationNotFound
	}
	return tx.Commit()
}

type entitlementProjectionDecision uint8

const (
	entitlementReplace entitlementProjectionDecision = iota + 1
	entitlementRefresh
)

func decideEntitlementProjection(current uint64, currentDigest []byte, incoming uint64, incomingDigest []byte) (entitlementProjectionDecision, error) {
	if incoming == 0 || current > incoming {
		return 0, model.ErrAuthorizationConflict
	}
	if current == incoming {
		if len(currentDigest) != len(incomingDigest) || subtle.ConstantTimeCompare(currentDigest, incomingDigest) != 1 {
			return 0, model.ErrAuthorizationConflict
		}
		return entitlementRefresh, nil
	}
	return entitlementReplace, nil
}

func (r *AuthorizationRepository) EntitlementsReady(ctx context.Context) error {
	if r.entitlementMaxStaleness <= 0 {
		return model.ErrEntitlementUnavailable
	}
	var missing int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)
FROM identity_authorization_domain d
LEFT JOIN identity_entitlement_projection_state s ON s.merchant_id=d.domain_id
WHERE d.domain_type='MERCHANT' AND (
  d.entitlement_revision=0 OR s.entitlement_revision IS NULL OR
  s.entitlement_revision<>d.entitlement_revision OR
  s.projected_at < (CURRENT_TIMESTAMP(3) - INTERVAL ? MICROSECOND)
)`, r.entitlementMaxStaleness.Microseconds()).Scan(&missing)
	if err != nil {
		return err
	}
	if missing != 0 {
		return model.ErrEntitlementUnavailable
	}
	return nil
}

func requireFreshEntitlement(ctx context.Context, tx *sql.Tx, merchantID int64, revision uint64, maxStaleness time.Duration) error {
	if revision == 0 || maxStaleness <= 0 {
		return model.ErrEntitlementUnavailable
	}
	var projectedAt time.Time
	var projectedRevision uint64
	if err := tx.QueryRowContext(ctx, `SELECT entitlement_revision,projected_at FROM identity_entitlement_projection_state WHERE merchant_id=?`, merchantID).Scan(&projectedRevision, &projectedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrEntitlementUnavailable
		}
		return err
	}
	if projectedRevision != revision || time.Since(projectedAt) > maxStaleness {
		return model.ErrEntitlementUnavailable
	}
	return nil
}
