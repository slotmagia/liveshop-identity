package mysql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

func (r *Repository) CreateShop(ctx context.Context, command model.CreateCommand) (model.Shop, bool, error) {
	tx, err := r.beginShopWrite(ctx)
	if err != nil {
		return model.Shop{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShopCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Shop{}, false, err
	}
	if found {
		return commitShopReplay(tx, replay)
	}
	if err := requireMerchantWritable(ctx, tx, command.MerchantID, true); err != nil {
		return model.Shop{}, false, err
	}
	if err := requireActiveCategory(ctx, tx, command.CategoryCode); err != nil {
		return model.Shop{}, false, err
	}
	shopID, err := nextShopID(ctx, tx)
	if err != nil {
		return model.Shop{}, false, err
	}
	category := sql.NullString{String: command.CategoryCode, Valid: command.CategoryCode != ""}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_shop(shop_id,merchant_id,code,subdomain,name,default_locale,currency,category_code,status,version)
VALUES(?,?,?,?,?,?,?,?,?,1)`, shopID, command.MerchantID, model.ShopCodeForID(shopID), command.Subdomain, command.Name,
		command.DefaultLocale, command.Currency, category, command.Status); err != nil {
		if shopDuplicate(err) {
			return model.Shop{}, false, model.ErrConflict
		}
		return model.Shop{}, false, fmt.Errorf("identity shop insert: %w", err)
	}
	if err := grantOwnerShop(ctx, tx, command.MerchantID, shopID); err != nil {
		return model.Shop{}, false, err
	}
	saved, err := readShop(ctx, tx, command.MerchantID, shopID, false)
	if err != nil {
		return model.Shop{}, false, err
	}
	if err := completeShopCommand(ctx, tx, command.CommandKey, saved); err != nil {
		return model.Shop{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Shop{}, false, fmt.Errorf("identity shop create commit: %w", err)
	}
	return saved, false, nil
}

func (r *Repository) UpdateShop(ctx context.Context, command model.UpdateCommand) (model.Shop, bool, error) {
	tx, err := r.beginShopWrite(ctx)
	if err != nil {
		return model.Shop{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShopCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Shop{}, false, err
	}
	if found {
		return commitShopReplay(tx, replay)
	}
	current, err := lockManagedShop(ctx, tx, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Shop{}, false, err
	}
	if current.Version != command.ExpectedVersion {
		return model.Shop{}, false, model.ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop SET name=?,subdomain=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE shop_id=? AND merchant_id=? AND status<>'CLOSED' AND version=?`, command.Name, command.Subdomain, current.ID, current.MerchantID, command.ExpectedVersion)
	if shopDuplicate(err) {
		return model.Shop{}, false, model.ErrConflict
	}
	if err != nil {
		return model.Shop{}, false, fmt.Errorf("identity shop update: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Shop{}, false, model.ErrConflict
	}
	saved, err := readShop(ctx, tx, current.MerchantID, current.ID, false)
	if err != nil {
		return model.Shop{}, false, err
	}
	if err := completeShopCommand(ctx, tx, command.CommandKey, saved); err != nil {
		return model.Shop{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Shop{}, false, fmt.Errorf("identity shop update commit: %w", err)
	}
	return saved, false, nil
}

func (r *Repository) SetShopEnabled(ctx context.Context, command model.SetEnabledCommand) (model.Shop, bool, error) {
	tx, err := r.beginShopWrite(ctx)
	if err != nil {
		return model.Shop{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShopCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Shop{}, false, err
	}
	if found {
		return commitShopReplay(tx, replay)
	}
	if command.Enabled {
		if err := requireMerchantWritable(ctx, tx, command.MerchantID, false); err != nil {
			return model.Shop{}, false, err
		}
	}
	current, err := lockManagedShop(ctx, tx, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Shop{}, false, err
	}
	if current.Version != command.ExpectedVersion {
		return model.Shop{}, false, model.ErrConflict
	}
	target := model.StatusDisabled
	if command.Enabled {
		target = model.StatusActive
	}
	if current.Status != target {
		result, err := tx.ExecContext(ctx, `UPDATE identity_shop SET status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE shop_id=? AND merchant_id=? AND status<>'CLOSED' AND version=?`, target, current.ID, current.MerchantID, command.ExpectedVersion)
		if err != nil {
			return model.Shop{}, false, fmt.Errorf("identity shop enable: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return model.Shop{}, false, model.ErrConflict
		}
	}
	saved, err := readShop(ctx, tx, current.MerchantID, current.ID, false)
	if err != nil {
		return model.Shop{}, false, err
	}
	if err := completeShopCommand(ctx, tx, command.CommandKey, saved); err != nil {
		return model.Shop{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Shop{}, false, fmt.Errorf("identity shop enable commit: %w", err)
	}
	return saved, false, nil
}

func (r *Repository) CloseShop(ctx context.Context, command model.CloseCommand) (model.Shop, bool, error) {
	tx, err := r.beginShopWrite(ctx)
	if err != nil {
		return model.Shop{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShopCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Shop{}, false, err
	}
	if found {
		return commitClosedShopReplay(tx, replay)
	}
	if _, err := requireMerchantExists(ctx, tx, command.MerchantID); err != nil {
		return model.Shop{}, false, err
	}
	open, err := lockMerchantOpenShops(ctx, tx, command.MerchantID)
	if err != nil {
		return model.Shop{}, false, err
	}
	current, ok := open[command.ShopID]
	if !ok {
		return model.Shop{}, false, model.ErrNotFound
	}
	if current.Version != command.ExpectedVersion {
		return model.Shop{}, false, model.ErrConflict
	}
	if len(open) == 1 {
		return model.Shop{}, false, model.ErrLastShop
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop SET status='CLOSED',closed_at=CURRENT_TIMESTAMP(3),version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE shop_id=? AND merchant_id=? AND status<>'CLOSED' AND version=?`, current.ID, current.MerchantID, command.ExpectedVersion)
	if err != nil {
		return model.Shop{}, false, fmt.Errorf("identity shop close: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Shop{}, false, model.ErrConflict
	}
	saved, err := readShop(ctx, tx, current.MerchantID, current.ID, true)
	if err != nil {
		return model.Shop{}, false, err
	}
	if err := completeShopCommand(ctx, tx, command.CommandKey, saved); err != nil {
		return model.Shop{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Shop{}, false, fmt.Errorf("identity shop close commit: %w", err)
	}
	return saved, false, nil
}

func (r *Repository) beginShopWrite(ctx context.Context) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("identity shop begin: %w", err)
	}
	return tx, nil
}

func nextShopID(ctx context.Context, tx *sql.Tx) (int64, error) {
	var current sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT shop_id FROM identity_shop ORDER BY shop_id DESC LIMIT 1 FOR UPDATE`).Scan(&current); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("identity shop allocate id: %w", err)
	}
	if !current.Valid || current.Int64 <= 0 {
		return 1, nil
	}
	return current.Int64 + 1, nil
}

func requireMerchantExists(ctx context.Context, tx *sql.Tx, merchantID int64) (string, error) {
	var status string
	err := tx.QueryRowContext(ctx, `SELECT status FROM identity_merchant WHERE merchant_id=? FOR UPDATE`, merchantID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", model.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("identity shop lock merchant: %w", err)
	}
	return status, nil
}

func requireMerchantWritable(ctx context.Context, tx *sql.Tx, merchantID int64, _ bool) error {
	status, err := requireMerchantExists(ctx, tx, merchantID)
	if err != nil {
		return err
	}
	if status == string(model.StatusClosed) {
		return model.ErrMerchantClosed
	}
	return nil
}

func requireActiveCategory(ctx context.Context, tx *sql.Tx, code string) error {
	if code == "" {
		return nil
	}
	var status string
	err := tx.QueryRowContext(ctx, `SELECT status FROM identity_shop_category WHERE code=? FOR UPDATE`, code).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrCategoryInactive
	}
	if err != nil {
		return fmt.Errorf("identity shop lock category: %w", err)
	}
	if status != string(model.CategoryActive) {
		return model.ErrCategoryInactive
	}
	return nil
}

func grantOwnerShop(ctx context.Context, tx *sql.Tx, merchantID, shopID int64) error {
	var memberID int64
	err := tx.QueryRowContext(ctx, `SELECT member_id FROM identity_workforce_member
WHERE merchant_id=? AND member_type='OWNER' AND status='ACTIVE' ORDER BY member_id LIMIT 1 FOR UPDATE`, merchantID).Scan(&memberID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrUnavailable
	}
	if err != nil {
		return fmt.Errorf("identity shop lock owner: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_member_shop(member_id,shop_id,assignment_kind,status)
VALUES(?,?,'OPERATE','ACTIVE')`, memberID, shopID); err != nil {
		if shopDuplicate(err) {
			return model.ErrConflict
		}
		return fmt.Errorf("identity shop owner assignment: %w", err)
	}
	return nil
}

func lockManagedShop(ctx context.Context, tx *sql.Tx, merchantID, shopID int64) (model.Shop, error) {
	value, err := scanShop(tx.QueryRowContext(ctx, `SELECT `+shopSelect+` FROM identity_shop WHERE merchant_id=? AND shop_id=? AND status<>'CLOSED' FOR UPDATE`, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Shop{}, model.ErrNotFound
	}
	if err != nil {
		return model.Shop{}, fmt.Errorf("identity shop lock: %w", err)
	}
	return value, nil
}

func lockMerchantOpenShops(ctx context.Context, tx *sql.Tx, merchantID int64) (map[int64]model.Shop, error) {
	rows, err := tx.QueryContext(ctx, `SELECT `+shopSelect+` FROM identity_shop WHERE merchant_id=? AND status<>'CLOSED' ORDER BY shop_id FOR UPDATE`, merchantID)
	if err != nil {
		return nil, fmt.Errorf("identity shop lock open shops: %w", err)
	}
	defer rows.Close()
	open := map[int64]model.Shop{}
	for rows.Next() {
		value, err := scanShop(rows)
		if err != nil {
			return nil, fmt.Errorf("identity shop lock open shops scan: %w", err)
		}
		open[value.ID] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity shop lock open shops iterate: %w", err)
	}
	return open, nil
}

func readShop(ctx context.Context, tx *sql.Tx, merchantID, shopID int64, includeClosed bool) (model.Shop, error) {
	query := `SELECT ` + shopSelect + ` FROM identity_shop WHERE merchant_id=? AND shop_id=?`
	if !includeClosed {
		query += ` AND status<>'CLOSED'`
	}
	value, err := scanShop(tx.QueryRowContext(ctx, query, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Shop{}, model.ErrNotFound
	}
	if err != nil {
		return model.Shop{}, fmt.Errorf("identity shop read: %w", err)
	}
	return value, nil
}

func insertOrReplayShopCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) ([]byte, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !shopDuplicate(err) {
		if err != nil {
			return nil, false, fmt.Errorf("identity shop insert command: %w", err)
		}
		return nil, false, nil
	}
	var stored, document []byte
	var version uint64
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_shop_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &version, &document); err != nil {
		return nil, false, fmt.Errorf("identity shop read command: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return nil, false, model.ErrIdempotency
	}
	if version == 0 || len(document) == 0 {
		return nil, false, fmt.Errorf("identity shop command response is incomplete")
	}
	return document, true, nil
}

func completeShopCommand(ctx context.Context, tx *sql.Tx, key string, value model.Shop) error {
	document, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("identity shop encode response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_command
SET shop_id=?,response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, value.ID, value.Version, document, key)
	if err != nil {
		return fmt.Errorf("identity shop complete command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("identity shop command completion affected no row")
	}
	return nil
}

func commitShopReplay(tx *sql.Tx, document []byte) (model.Shop, bool, error) {
	var value model.Shop
	if json.Unmarshal(document, &value) != nil || value.Validate() != nil {
		return model.Shop{}, false, fmt.Errorf("identity shop command response is incomplete")
	}
	if err := tx.Commit(); err != nil {
		return model.Shop{}, false, fmt.Errorf("identity shop replay commit: %w", err)
	}
	return value, true, nil
}

func commitClosedShopReplay(tx *sql.Tx, document []byte) (model.Shop, bool, error) {
	var value model.Shop
	if json.Unmarshal(document, &value) != nil || value.ValidatePersisted() != nil || value.Status != model.StatusClosed {
		return model.Shop{}, false, fmt.Errorf("identity shop command response is incomplete")
	}
	if err := tx.Commit(); err != nil {
		return model.Shop{}, false, fmt.Errorf("identity shop close replay commit: %w", err)
	}
	return value, true, nil
}

func shopDuplicate(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
