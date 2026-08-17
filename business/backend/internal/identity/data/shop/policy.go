package mysql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type PolicyRepository struct{ database *sql.DB }

var _ shop.PolicyRepository = (*PolicyRepository)(nil)

func NewPolicyRepository(database *sql.DB) *PolicyRepository {
	return &PolicyRepository{database: database}
}

func (r *PolicyRepository) ListPolicies(ctx context.Context, query model.PolicyQuery) (model.PolicyPage, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.PolicyPage{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensurePolicyShopScope(ctx, tx, query.MerchantID, query.ShopID, false); err != nil {
		return model.PolicyPage{}, err
	}
	where := "merchant_id=? AND shop_id=?"
	args := []any{query.MerchantID, query.ShopID}
	if query.PolicyType != "" {
		where += " AND policy_type=?"
		args = append(args, query.PolicyType)
	}
	if query.Status != "" {
		where += " AND status=?"
		args = append(args, query.Status)
	}
	var total int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM identity_shop_policy WHERE "+where, args...).Scan(&total); err != nil {
		return model.PolicyPage{}, fmt.Errorf("shop policy count: %w", err)
	}
	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := tx.QueryContext(ctx, `SELECT policy_id,merchant_id,shop_id,policy_type,title,content,version_no,status,version,published_at,created_at,updated_at
FROM identity_shop_policy WHERE `+where+` ORDER BY policy_type ASC, version_no DESC, policy_id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return model.PolicyPage{}, fmt.Errorf("shop policy list: %w", err)
	}
	defer rows.Close()
	items := []model.Policy{}
	for rows.Next() {
		value, err := scanPolicy(rows)
		if err != nil {
			return model.PolicyPage{}, fmt.Errorf("shop policy list scan: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.PolicyPage{}, fmt.Errorf("shop policy list iterate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.PolicyPage{}, fmt.Errorf("shop policy list commit: %w", err)
	}
	return model.PolicyPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *PolicyRepository) SavePolicy(ctx context.Context, command model.SavePolicyCommand) (model.Policy, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Policy{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayPolicyCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Policy{}, false, err
	}
	if found {
		return commitPolicyReplay(tx, replay, "save")
	}
	if err := ensurePolicyShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Policy{}, false, err
	}
	if command.Publish {
		if err := requirePolicyOverlayActive(ctx, tx, command.MerchantID, command.ShopID); err != nil {
			return model.Policy{}, false, err
		}
		if err := archivePublishedPolicies(ctx, tx, command.MerchantID, command.ShopID, command.PolicyType); err != nil {
			return model.Policy{}, false, err
		}
	}
	var versionNo int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_no),0) FROM identity_shop_policy WHERE merchant_id=? AND shop_id=? AND policy_type=?`,
		command.MerchantID, command.ShopID, command.PolicyType).Scan(&versionNo); err != nil {
		return model.Policy{}, false, fmt.Errorf("shop policy next version: %w", err)
	}
	status := model.PolicyDraft
	var publishedAt any
	if command.Publish {
		status = model.PolicyPublished
		publishedAt = time.Now().UTC()
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_policy
(merchant_id,shop_id,policy_type,title,content,version_no,status,version,published_at)
VALUES(?,?,?,?,?,?,?,1,?)`, command.MerchantID, command.ShopID, command.PolicyType, command.Title, command.Content, versionNo+1, status, publishedAt)
	if policyDuplicateKey(err) {
		return model.Policy{}, false, model.ErrPolicyConflict
	}
	if err != nil {
		return model.Policy{}, false, fmt.Errorf("shop policy insert: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return model.Policy{}, false, fmt.Errorf("shop policy inserted id: %w", err)
	}
	saved, err := readPolicy(ctx, tx, id, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Policy{}, false, err
	}
	return completeAndCommitPolicy(ctx, tx, command.CommandKey, saved, "save")
}

func (r *PolicyRepository) PublishPolicy(ctx context.Context, command model.PublishPolicyCommand) (model.Policy, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Policy{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayPolicyCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Policy{}, false, err
	}
	if found {
		return commitPolicyReplay(tx, replay, "publish")
	}
	if err := ensurePolicyShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Policy{}, false, err
	}
	if err := requirePolicyOverlayActive(ctx, tx, command.MerchantID, command.ShopID); err != nil {
		return model.Policy{}, false, err
	}
	current, err := readPolicyForUpdate(ctx, tx, command.PolicyID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Policy{}, false, err
	}
	if current.Version != command.ExpectedVersion || current.Status != model.PolicyDraft {
		return model.Policy{}, false, model.ErrPolicyConflict
	}
	if err := archivePublishedPolicies(ctx, tx, current.MerchantID, current.ShopID, current.PolicyType); err != nil {
		return model.Policy{}, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_policy
SET status=?,published_at=CURRENT_TIMESTAMP(3),version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE policy_id=? AND merchant_id=? AND shop_id=? AND version=? AND status=?`,
		model.PolicyPublished, current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion, model.PolicyDraft)
	if err != nil {
		return model.Policy{}, false, fmt.Errorf("shop policy publish: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Policy{}, false, model.ErrPolicyConflict
	}
	saved, err := readPolicy(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.Policy{}, false, err
	}
	return completeAndCommitPolicy(ctx, tx, command.CommandKey, saved, "publish")
}

func (r *PolicyRepository) begin(ctx context.Context, readOnly bool) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: readOnly})
	if err != nil {
		return nil, fmt.Errorf("shop policy begin: %w", err)
	}
	return tx, nil
}

func ensurePolicyShopScope(ctx context.Context, tx *sql.Tx, merchantID, shopID int64, lock bool) error {
	query := `SELECT merchant_id,shop_id FROM identity_shop WHERE merchant_id=? AND shop_id=? AND status<>'CLOSED'`
	if lock {
		query += " FOR UPDATE"
	}
	var storedMerchant, storedShop int64
	err := tx.QueryRowContext(ctx, query, merchantID, shopID).Scan(&storedMerchant, &storedShop)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrPolicyInvalid
	}
	if err != nil {
		return fmt.Errorf("shop policy scope: %w", err)
	}
	if storedMerchant != merchantID || storedShop != shopID {
		return model.ErrPolicyInvalid
	}
	return nil
}

func requirePolicyOverlayActive(ctx context.Context, tx *sql.Tx, merchantID, shopID int64) error {
	var status string
	err := tx.QueryRowContext(ctx, `SELECT platform_status FROM identity_merchant_capability
WHERE merchant_id=? AND shop_id=? AND module='policies' FOR UPDATE`, merchantID, shopID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("shop policy overlay: %w", err)
	}
	if status != "active" {
		return model.ErrPolicyRestricted
	}
	return nil
}

func archivePublishedPolicies(ctx context.Context, tx *sql.Tx, merchantID, shopID int64, policyType model.PolicyType) error {
	if _, err := tx.ExecContext(ctx, `UPDATE identity_shop_policy SET status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=? AND shop_id=? AND policy_type=? AND status=?`, model.PolicyArchived, merchantID, shopID, policyType, model.PolicyPublished); err != nil {
		return fmt.Errorf("shop policy archive: %w", err)
	}
	return nil
}

type policyScanner interface{ Scan(...any) error }

func scanPolicy(row policyScanner) (model.Policy, error) {
	var value model.Policy
	var publishedAt sql.NullTime
	err := row.Scan(&value.ID, &value.MerchantID, &value.ShopID, &value.PolicyType, &value.Title, &value.Content,
		&value.VersionNo, &value.Status, &value.Version, &publishedAt, &value.CreatedAt, &value.UpdatedAt)
	if publishedAt.Valid {
		moment := publishedAt.Time
		value.PublishedAt = &moment
	}
	return value, err
}

func readPolicy(ctx context.Context, tx *sql.Tx, policyID, merchantID, shopID int64) (model.Policy, error) {
	value, err := scanPolicy(tx.QueryRowContext(ctx, `SELECT policy_id,merchant_id,shop_id,policy_type,title,content,version_no,status,version,published_at,created_at,updated_at
FROM identity_shop_policy WHERE policy_id=? AND merchant_id=? AND shop_id=?`, policyID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Policy{}, model.ErrPolicyNotFound
	}
	if err != nil {
		return model.Policy{}, fmt.Errorf("shop policy read: %w", err)
	}
	return value, nil
}

func readPolicyForUpdate(ctx context.Context, tx *sql.Tx, policyID, merchantID, shopID int64) (model.Policy, error) {
	value, err := scanPolicy(tx.QueryRowContext(ctx, `SELECT policy_id,merchant_id,shop_id,policy_type,title,content,version_no,status,version,published_at,created_at,updated_at
FROM identity_shop_policy WHERE policy_id=? AND merchant_id=? AND shop_id=? FOR UPDATE`, policyID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Policy{}, model.ErrPolicyNotFound
	}
	if err != nil {
		return model.Policy{}, fmt.Errorf("shop policy lock: %w", err)
	}
	return value, nil
}

func insertOrReplayPolicyCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) (model.Policy, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_policy_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !policyDuplicateKey(err) {
		if err != nil {
			return model.Policy{}, false, fmt.Errorf("shop policy insert command: %w", err)
		}
		return model.Policy{}, false, nil
	}
	var stored []byte
	var responseVersion uint64
	var document []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_shop_policy_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &responseVersion, &document); err != nil {
		return model.Policy{}, false, fmt.Errorf("shop policy read command replay: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return model.Policy{}, false, model.ErrPolicyIdempotency
	}
	var replay model.Policy
	if responseVersion == 0 || len(document) == 0 || json.Unmarshal(document, &replay) != nil || replay.ID <= 0 || replay.Version != responseVersion {
		return model.Policy{}, false, fmt.Errorf("shop policy command response is incomplete")
	}
	return replay, true, nil
}

func completeAndCommitPolicy(ctx context.Context, tx *sql.Tx, key string, value model.Policy, operation string) (model.Policy, bool, error) {
	document, err := json.Marshal(value)
	if err != nil {
		return model.Policy{}, false, fmt.Errorf("shop policy encode %s response: %w", operation, err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_policy_command SET policy_id=?,response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, value.ID, value.Version, document, key)
	if err != nil {
		return model.Policy{}, false, fmt.Errorf("shop policy complete %s command: %w", operation, err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Policy{}, false, fmt.Errorf("shop policy %s command completion affected no row", operation)
	}
	if err := tx.Commit(); err != nil {
		return model.Policy{}, false, fmt.Errorf("shop policy commit %s: %w", operation, err)
	}
	return value, false, nil
}

func commitPolicyReplay(tx *sql.Tx, value model.Policy, operation string) (model.Policy, bool, error) {
	if err := tx.Commit(); err != nil {
		return model.Policy{}, false, fmt.Errorf("shop policy commit %s replay: %w", operation, err)
	}
	return value, true, nil
}

func policyDuplicateKey(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
