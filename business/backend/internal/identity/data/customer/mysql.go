package customer

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer/model"
)

type Repository struct{ database *sql.DB }

var _ customer.Repository = (*Repository)(nil)

func NewRepository(database *sql.DB) *Repository { return &Repository{database: database} }

func (r *Repository) ListAddresses(ctx context.Context, tenant model.Tenant, subject string) ([]model.Address, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	rows, err := r.database.QueryContext(ctx, `SELECT address_id,recipient,phone,country,province,city,district,detail,postal_code,is_default,version
FROM identity_customer_address WHERE merchant_id=? AND shop_id=? AND customer_subject=?
ORDER BY is_default DESC, updated_at DESC, address_id DESC`, tenant.MerchantID, tenant.ShopID, subject)
	if err != nil {
		return nil, fmt.Errorf("customer address list: %w", err)
	}
	defer rows.Close()
	items := make([]model.Address, 0)
	for rows.Next() {
		item, err := scanAddress(rows)
		if err != nil {
			return nil, fmt.Errorf("customer address list scan: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("customer address list iterate: %w", err)
	}
	return items, nil
}

func (r *Repository) SaveAddress(ctx context.Context, command model.SaveAddressCommand) (model.Address, bool, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return model.Address{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replayID, found, err := insertOrReplayAddressCommand(ctx, tx, command.Tenant, command.Subject, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Address{}, false, err
	}
	if found {
		saved, err := readAddress(ctx, tx, command.Tenant, command.Subject, replayID)
		return saved, true, err
	}
	address := command.Address
	if address.ID == 0 {
		result, err := tx.ExecContext(ctx, `INSERT INTO identity_customer_address(
merchant_id,shop_id,customer_subject,recipient,phone,country,province,city,district,detail,postal_code,is_default,version)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,1)`,
			command.Tenant.MerchantID, command.Tenant.ShopID, command.Subject,
			address.Recipient, address.Phone, address.Country, address.Province, address.City, address.District,
			address.Detail, address.PostalCode, boolToInt(address.IsDefault))
		if err != nil {
			return model.Address{}, false, fmt.Errorf("customer address insert: %w", err)
		}
		address.ID, err = result.LastInsertId()
		if err != nil {
			return model.Address{}, false, fmt.Errorf("customer address inserted id: %w", err)
		}
	} else {
		current, err := readAddressForUpdate(ctx, tx, command.Tenant, command.Subject, address.ID)
		if err != nil {
			return model.Address{}, false, err
		}
		if current.Version != command.ExpectedVersion {
			return model.Address{}, false, model.ErrConflict
		}
		result, err := tx.ExecContext(ctx, `UPDATE identity_customer_address
SET recipient=?,phone=?,country=?,province=?,city=?,district=?,detail=?,postal_code=?,is_default=?,version=version+1
WHERE address_id=? AND merchant_id=? AND shop_id=? AND customer_subject=? AND version=?`,
			address.Recipient, address.Phone, address.Country, address.Province, address.City, address.District,
			address.Detail, address.PostalCode, boolToInt(address.IsDefault),
			address.ID, command.Tenant.MerchantID, command.Tenant.ShopID, command.Subject, command.ExpectedVersion)
		if err != nil {
			return model.Address{}, false, fmt.Errorf("customer address update: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return model.Address{}, false, model.ErrConflict
		}
	}
	if address.IsDefault {
		if err := clearOtherDefaults(ctx, tx, command.Tenant, command.Subject, address.ID); err != nil {
			return model.Address{}, false, err
		}
	}
	saved, err := readAddress(ctx, tx, command.Tenant, command.Subject, address.ID)
	if err != nil {
		return model.Address{}, false, err
	}
	if err := completeAddressCommand(ctx, tx, command.Tenant, command.Subject, command.CommandKey, saved.ID); err != nil {
		return model.Address{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Address{}, false, fmt.Errorf("customer address save commit: %w", err)
	}
	return saved, false, nil
}

func (r *Repository) DeleteAddress(ctx context.Context, command model.DeleteAddressCommand) (bool, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	replayID, found, err := insertOrReplayAddressCommand(ctx, tx, command.Tenant, command.Subject, command.CommandKey, command.RequestDigest())
	if err != nil {
		return false, err
	}
	if found {
		return true, completeNoop(tx, replayID == command.AddressID)
	}
	current, err := readAddressForUpdate(ctx, tx, command.Tenant, command.Subject, command.AddressID)
	if err != nil {
		return false, err
	}
	if current.Version != command.ExpectedVersion {
		return false, model.ErrConflict
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM identity_customer_address
WHERE address_id=? AND merchant_id=? AND shop_id=? AND customer_subject=? AND version=?`,
		command.AddressID, command.Tenant.MerchantID, command.Tenant.ShopID, command.Subject, command.ExpectedVersion)
	if err != nil {
		return false, fmt.Errorf("customer address delete: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return false, model.ErrConflict
	}
	if err := completeAddressCommand(ctx, tx, command.Tenant, command.Subject, command.CommandKey, command.AddressID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("customer address delete commit: %w", err)
	}
	return false, nil
}

func (r *Repository) ReplaceDefault(ctx context.Context, command model.ReplaceDefaultCommand) (model.Address, bool, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return model.Address{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replayID, found, err := insertOrReplayAddressCommand(ctx, tx, command.Tenant, command.Subject, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Address{}, false, err
	}
	if found {
		saved, err := readAddress(ctx, tx, command.Tenant, command.Subject, replayID)
		return saved, true, err
	}
	current, err := readAddressForUpdate(ctx, tx, command.Tenant, command.Subject, command.AddressID)
	if err != nil {
		return model.Address{}, false, err
	}
	if current.Version != command.ExpectedVersion {
		return model.Address{}, false, model.ErrConflict
	}
	if err := clearOtherDefaults(ctx, tx, command.Tenant, command.Subject, command.AddressID); err != nil {
		return model.Address{}, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_customer_address SET is_default=1,version=version+1
WHERE address_id=? AND merchant_id=? AND shop_id=? AND customer_subject=? AND version=?`,
		command.AddressID, command.Tenant.MerchantID, command.Tenant.ShopID, command.Subject, command.ExpectedVersion)
	if err != nil {
		return model.Address{}, false, fmt.Errorf("customer address default: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Address{}, false, model.ErrConflict
	}
	saved, err := readAddress(ctx, tx, command.Tenant, command.Subject, command.AddressID)
	if err != nil {
		return model.Address{}, false, err
	}
	if err := completeAddressCommand(ctx, tx, command.Tenant, command.Subject, command.CommandKey, saved.ID); err != nil {
		return model.Address{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Address{}, false, fmt.Errorf("customer address default commit: %w", err)
	}
	return saved, false, nil
}

func (r *Repository) ListWishlist(ctx context.Context, tenant model.Tenant, subject string, cursor int64, limit int) ([]model.WishlistItem, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	query := `SELECT product_id,created_at FROM identity_customer_wishlist
WHERE merchant_id=? AND shop_id=? AND customer_subject=?`
	args := []any{tenant.MerchantID, tenant.ShopID, subject}
	if cursor > 0 {
		query += ` AND product_id<?`
		args = append(args, cursor)
	}
	query += ` ORDER BY created_at DESC, product_id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("customer wishlist list: %w", err)
	}
	defer rows.Close()
	items := make([]model.WishlistItem, 0)
	for rows.Next() {
		var item model.WishlistItem
		var created time.Time
		if err := rows.Scan(&item.ProductID, &created); err != nil {
			return nil, fmt.Errorf("customer wishlist list scan: %w", err)
		}
		item.CreatedAt = created.UTC().UnixMilli()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("customer wishlist list iterate: %w", err)
	}
	return items, nil
}

func (r *Repository) AddWishlist(ctx context.Context, command model.AddWishlistCommand) (model.WishlistItem, bool, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return model.WishlistItem{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replayID, found, err := insertOrReplayWishlistCommand(ctx, tx, command.Tenant, command.Subject, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.WishlistItem{}, false, err
	}
	if found {
		item, err := readWishlist(ctx, tx, command.Tenant, command.Subject, replayID)
		return item, true, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO identity_customer_wishlist(merchant_id,shop_id,customer_subject,product_id)
VALUES(?,?,?,?)`, command.Tenant.MerchantID, command.Tenant.ShopID, command.Subject, command.ProductID)
	if duplicateKey(err) {
		item, readErr := readWishlist(ctx, tx, command.Tenant, command.Subject, command.ProductID)
		if readErr != nil {
			return model.WishlistItem{}, false, readErr
		}
		if err := completeWishlistCommand(ctx, tx, command.Tenant, command.Subject, command.CommandKey, command.ProductID); err != nil {
			return model.WishlistItem{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return model.WishlistItem{}, false, fmt.Errorf("customer wishlist add commit: %w", err)
		}
		return item, true, nil
	}
	if err != nil {
		return model.WishlistItem{}, false, fmt.Errorf("customer wishlist insert: %w", err)
	}
	item, err := readWishlist(ctx, tx, command.Tenant, command.Subject, command.ProductID)
	if err != nil {
		return model.WishlistItem{}, false, err
	}
	if err := completeWishlistCommand(ctx, tx, command.Tenant, command.Subject, command.CommandKey, command.ProductID); err != nil {
		return model.WishlistItem{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.WishlistItem{}, false, fmt.Errorf("customer wishlist add commit: %w", err)
	}
	return item, false, nil
}

func (r *Repository) RemoveWishlist(ctx context.Context, command model.RemoveWishlistCommand) error {
	if r == nil || r.database == nil {
		return model.ErrUnavailable
	}
	result, err := r.database.ExecContext(ctx, `DELETE FROM identity_customer_wishlist
WHERE merchant_id=? AND shop_id=? AND customer_subject=? AND product_id=?`,
		command.Tenant.MerchantID, command.Tenant.ShopID, command.Subject, command.ProductID)
	if err != nil {
		return fmt.Errorf("customer wishlist delete: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *Repository) begin(ctx context.Context) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("customer begin: %w", err)
	}
	return tx, nil
}

func insertOrReplayAddressCommand(ctx context.Context, tx *sql.Tx, tenant model.Tenant, subject, key string, digest [32]byte) (int64, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_customer_address_command(merchant_id,shop_id,customer_subject,command_key,request_hash)
VALUES(?,?,?,?,?)`, tenant.MerchantID, tenant.ShopID, subject, key, digest[:])
	if !duplicateKey(err) {
		if err != nil {
			return 0, false, fmt.Errorf("customer address insert command: %w", err)
		}
		return 0, false, nil
	}
	var stored []byte
	var addressID int64
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,address_id FROM identity_customer_address_command
WHERE merchant_id=? AND shop_id=? AND customer_subject=? AND command_key=? FOR UPDATE`,
		tenant.MerchantID, tenant.ShopID, subject, key).Scan(&stored, &addressID); err != nil {
		return 0, false, fmt.Errorf("customer address read command: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return 0, false, model.ErrIdempotency
	}
	if addressID <= 0 {
		return 0, false, fmt.Errorf("customer address command response is incomplete")
	}
	return addressID, true, nil
}

func completeAddressCommand(ctx context.Context, tx *sql.Tx, tenant model.Tenant, subject, key string, addressID int64) error {
	result, err := tx.ExecContext(ctx, `UPDATE identity_customer_address_command SET address_id=?
WHERE merchant_id=? AND shop_id=? AND customer_subject=? AND command_key=?`,
		addressID, tenant.MerchantID, tenant.ShopID, subject, key)
	if err != nil {
		return fmt.Errorf("customer address complete command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("customer address command completion affected no row")
	}
	return nil
}

func insertOrReplayWishlistCommand(ctx context.Context, tx *sql.Tx, tenant model.Tenant, subject, key string, digest [32]byte) (int64, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_customer_wishlist_command(merchant_id,shop_id,customer_subject,command_key,request_hash)
VALUES(?,?,?,?,?)`, tenant.MerchantID, tenant.ShopID, subject, key, digest[:])
	if !duplicateKey(err) {
		if err != nil {
			return 0, false, fmt.Errorf("customer wishlist insert command: %w", err)
		}
		return 0, false, nil
	}
	var stored []byte
	var productID int64
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,product_id FROM identity_customer_wishlist_command
WHERE merchant_id=? AND shop_id=? AND customer_subject=? AND command_key=? FOR UPDATE`,
		tenant.MerchantID, tenant.ShopID, subject, key).Scan(&stored, &productID); err != nil {
		return 0, false, fmt.Errorf("customer wishlist read command: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return 0, false, model.ErrIdempotency
	}
	if productID <= 0 {
		return 0, false, fmt.Errorf("customer wishlist command response is incomplete")
	}
	return productID, true, nil
}

func completeWishlistCommand(ctx context.Context, tx *sql.Tx, tenant model.Tenant, subject, key string, productID int64) error {
	result, err := tx.ExecContext(ctx, `UPDATE identity_customer_wishlist_command SET product_id=?
WHERE merchant_id=? AND shop_id=? AND customer_subject=? AND command_key=?`,
		productID, tenant.MerchantID, tenant.ShopID, subject, key)
	if err != nil {
		return fmt.Errorf("customer wishlist complete command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("customer wishlist command completion affected no row")
	}
	return nil
}

func clearOtherDefaults(ctx context.Context, tx *sql.Tx, tenant model.Tenant, subject string, keepID int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE identity_customer_address SET is_default=0
WHERE merchant_id=? AND shop_id=? AND customer_subject=? AND address_id<>? AND is_default=1`,
		tenant.MerchantID, tenant.ShopID, subject, keepID)
	if err != nil {
		return fmt.Errorf("customer address clear default: %w", err)
	}
	return nil
}

const addressColumns = `address_id,recipient,phone,country,province,city,district,detail,postal_code,is_default,version`

func readAddress(ctx context.Context, tx *sql.Tx, tenant model.Tenant, subject string, addressID int64) (model.Address, error) {
	return scanAddress(tx.QueryRowContext(ctx, `SELECT `+addressColumns+`
FROM identity_customer_address WHERE address_id=? AND merchant_id=? AND shop_id=? AND customer_subject=?`,
		addressID, tenant.MerchantID, tenant.ShopID, subject))
}

func readAddressForUpdate(ctx context.Context, tx *sql.Tx, tenant model.Tenant, subject string, addressID int64) (model.Address, error) {
	return scanAddress(tx.QueryRowContext(ctx, `SELECT `+addressColumns+`
FROM identity_customer_address WHERE address_id=? AND merchant_id=? AND shop_id=? AND customer_subject=? FOR UPDATE`,
		addressID, tenant.MerchantID, tenant.ShopID, subject))
}

func readWishlist(ctx context.Context, tx *sql.Tx, tenant model.Tenant, subject string, productID int64) (model.WishlistItem, error) {
	var item model.WishlistItem
	var created time.Time
	err := tx.QueryRowContext(ctx, `SELECT product_id,created_at FROM identity_customer_wishlist
WHERE merchant_id=? AND shop_id=? AND customer_subject=? AND product_id=?`,
		tenant.MerchantID, tenant.ShopID, subject, productID).Scan(&item.ProductID, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return model.WishlistItem{}, model.ErrNotFound
	}
	if err != nil {
		return model.WishlistItem{}, fmt.Errorf("customer wishlist read: %w", err)
	}
	item.CreatedAt = created.UTC().UnixMilli()
	return item, nil
}

type scanner interface{ Scan(dest ...any) error }

func scanAddress(row scanner) (model.Address, error) {
	var item model.Address
	var isDefault int
	err := row.Scan(&item.ID, &item.Recipient, &item.Phone, &item.Country, &item.Province, &item.City, &item.District,
		&item.Detail, &item.PostalCode, &isDefault, &item.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Address{}, model.ErrNotFound
	}
	if err != nil {
		return model.Address{}, err
	}
	item.IsDefault = isDefault == 1
	return item, nil
}

func completeNoop(tx *sql.Tx, ok bool) error {
	if !ok {
		return model.ErrIdempotency
	}
	return tx.Commit()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func duplicateKey(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
