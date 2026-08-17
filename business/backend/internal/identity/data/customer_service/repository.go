package mysql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service/model"
)

type Repository struct{ database *sql.DB }

var _ customer_service.Repository = (*Repository)(nil)

func NewRepository(database *sql.DB) *Repository { return &Repository{database: database} }

func (r *Repository) List(ctx context.Context, query model.Query) (model.Page, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.Page{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if query.MerchantID > 0 && query.ShopID > 0 {
		if err := ensureShopScope(ctx, tx, query.MerchantID, query.ShopID, false); err != nil {
			return model.Page{}, err
		}
	}
	where := "1=1"
	args := []any{}
	if query.MerchantID > 0 {
		where += " AND merchant_id=?"
		args = append(args, query.MerchantID)
	}
	if query.ShopID > 0 {
		where += " AND shop_id=?"
		args = append(args, query.ShopID)
	}
	if query.Platform != "" {
		where += " AND platform=?"
		args = append(args, query.Platform)
	}
	if query.Account != "" {
		where += " AND account LIKE ?"
		args = append(args, "%"+query.Account+"%")
	}
	if query.Status != nil {
		where += " AND status=?"
		args = append(args, *query.Status)
	}
	var total int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM identity_customer_service_account WHERE "+where, args...).Scan(&total); err != nil {
		return model.Page{}, fmt.Errorf("customer service count: %w", err)
	}
	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := tx.QueryContext(ctx, `SELECT account_id,merchant_id,shop_id,platform,account,nickname,status,config,remark,version,created_at,updated_at
FROM identity_customer_service_account WHERE `+where+` ORDER BY account_id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return model.Page{}, fmt.Errorf("customer service list: %w", err)
	}
	defer rows.Close()
	items := []model.Account{}
	for rows.Next() {
		value, err := scanAccount(rows)
		if err != nil {
			return model.Page{}, fmt.Errorf("customer service list scan: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.Page{}, fmt.Errorf("customer service list iterate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Page{}, fmt.Errorf("customer service list commit: %w", err)
	}
	return model.Page{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *Repository) Save(ctx context.Context, command model.SaveCommand) (model.Account, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Account{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	document, found, err := insertOrReplay(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Account{}, false, err
	}
	if found {
		var replay model.Account
		if json.Unmarshal(document, &replay) != nil || replay.ValidatePersisted() != nil {
			return model.Account{}, false, fmt.Errorf("customer service command response is incomplete")
		}
		if err := tx.Commit(); err != nil {
			return model.Account{}, false, fmt.Errorf("customer service replay commit: %w", err)
		}
		return replay, true, nil
	}
	if err := ensureShopScope(ctx, tx, command.Account.MerchantID, command.Account.ShopID, true); err != nil {
		return model.Account{}, false, err
	}
	value := command.Account
	if value.ID == 0 {
		result, err := tx.ExecContext(ctx, `INSERT INTO identity_customer_service_account
(merchant_id,shop_id,platform,account,nickname,status,config,remark,version)
VALUES(?,?,?,?,?,?,?,?,1)`, value.MerchantID, value.ShopID, value.Platform, value.Account,
			value.Nickname, value.Status, value.Config, value.Remark)
		if duplicate(err) {
			return model.Account{}, false, model.ErrConflict
		}
		if err != nil {
			return model.Account{}, false, fmt.Errorf("customer service insert: %w", err)
		}
		value.ID, err = result.LastInsertId()
		if err != nil {
			return model.Account{}, false, fmt.Errorf("customer service inserted id: %w", err)
		}
	} else {
		current, err := readAccountForUpdate(ctx, tx, value.ID, value.MerchantID, value.ShopID)
		if err != nil {
			return model.Account{}, false, err
		}
		if current.Version != command.ExpectedVersion {
			return model.Account{}, false, model.ErrConflict
		}
		result, err := tx.ExecContext(ctx, `UPDATE identity_customer_service_account
SET platform=?,account=?,nickname=?,status=?,config=?,remark=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE account_id=? AND merchant_id=? AND shop_id=? AND version=?`, value.Platform, value.Account, value.Nickname, value.Status,
			value.Config, value.Remark, value.ID, value.MerchantID, value.ShopID, command.ExpectedVersion)
		if duplicate(err) {
			return model.Account{}, false, model.ErrConflict
		}
		if err != nil {
			return model.Account{}, false, fmt.Errorf("customer service update: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return model.Account{}, false, model.ErrConflict
		}
	}
	saved, err := readAccount(ctx, tx, value.ID, value.MerchantID, value.ShopID)
	if err != nil {
		return model.Account{}, false, err
	}
	if err := completeCommand(ctx, tx, command.CommandKey, saved.Version, saved); err != nil {
		return model.Account{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Account{}, false, fmt.Errorf("customer service save commit: %w", err)
	}
	return saved, false, nil
}

func (r *Repository) Delete(ctx context.Context, command model.DeleteCommand) (model.DeleteResult, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.DeleteResult{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	document, found, err := insertOrReplay(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.DeleteResult{}, false, err
	}
	if found {
		var replay model.DeleteResult
		if json.Unmarshal(document, &replay) != nil || replay.ID <= 0 || !replay.Deleted || replay.Version == 0 {
			return model.DeleteResult{}, false, fmt.Errorf("customer service delete response is incomplete")
		}
		if err := tx.Commit(); err != nil {
			return model.DeleteResult{}, false, fmt.Errorf("customer service delete replay commit: %w", err)
		}
		return replay, true, nil
	}
	if err := ensureShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.DeleteResult{}, false, err
	}
	current, err := readAccountForUpdate(ctx, tx, command.AccountID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.DeleteResult{}, false, err
	}
	if current.Version != command.ExpectedVersion {
		return model.DeleteResult{}, false, model.ErrConflict
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM identity_customer_service_account
WHERE account_id=? AND merchant_id=? AND shop_id=? AND version=?`, command.AccountID, command.MerchantID, command.ShopID, command.ExpectedVersion)
	if err != nil {
		return model.DeleteResult{}, false, fmt.Errorf("customer service delete: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.DeleteResult{}, false, model.ErrConflict
	}
	deleted := model.DeleteResult{ID: command.AccountID, Deleted: true, Version: command.ExpectedVersion}
	if err := completeCommand(ctx, tx, command.CommandKey, deleted.Version, deleted); err != nil {
		return model.DeleteResult{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.DeleteResult{}, false, fmt.Errorf("customer service delete commit: %w", err)
	}
	return deleted, false, nil
}

func (r *Repository) begin(ctx context.Context, readOnly bool) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: readOnly})
	if err != nil {
		return nil, fmt.Errorf("customer service begin: %w", err)
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
		return fmt.Errorf("customer service shop scope: %w", err)
	}
	return nil
}

type scanner interface{ Scan(...any) error }

func scanAccount(row scanner) (model.Account, error) {
	var value model.Account
	err := row.Scan(&value.ID, &value.MerchantID, &value.ShopID, &value.Platform,
		&value.Account, &value.Nickname, &value.Status, &value.Config, &value.Remark, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func readAccount(ctx context.Context, tx *sql.Tx, accountID, merchantID, shopID int64) (model.Account, error) {
	value, err := scanAccount(tx.QueryRowContext(ctx, `SELECT account_id,merchant_id,shop_id,platform,account,nickname,status,config,remark,version,created_at,updated_at
FROM identity_customer_service_account WHERE account_id=? AND merchant_id=? AND shop_id=?`, accountID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Account{}, model.ErrNotFound
	}
	if err != nil {
		return model.Account{}, fmt.Errorf("customer service read: %w", err)
	}
	return value, nil
}

func readAccountForUpdate(ctx context.Context, tx *sql.Tx, accountID, merchantID, shopID int64) (model.Account, error) {
	value, err := scanAccount(tx.QueryRowContext(ctx, `SELECT account_id,merchant_id,shop_id,platform,account,nickname,status,config,remark,version,created_at,updated_at
FROM identity_customer_service_account WHERE account_id=? AND merchant_id=? AND shop_id=? FOR UPDATE`, accountID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Account{}, model.ErrNotFound
	}
	if err != nil {
		return model.Account{}, fmt.Errorf("customer service lock: %w", err)
	}
	return value, nil
}

func insertOrReplay(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) ([]byte, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_customer_service_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !duplicate(err) {
		if err != nil {
			return nil, false, fmt.Errorf("customer service insert command: %w", err)
		}
		return nil, false, nil
	}
	var stored, document []byte
	var version uint64
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_customer_service_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &version, &document); err != nil {
		return nil, false, fmt.Errorf("customer service read command: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return nil, false, model.ErrIdempotency
	}
	if version == 0 || len(document) == 0 {
		return nil, false, fmt.Errorf("customer service command response is incomplete")
	}
	return document, true, nil
}

func completeCommand(ctx context.Context, tx *sql.Tx, key string, version uint64, response any) error {
	document, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("customer service encode response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_customer_service_command
SET response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, version, document, key)
	if err != nil {
		return fmt.Errorf("customer service complete command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("customer service command completion affected no row")
	}
	return nil
}

func duplicate(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
