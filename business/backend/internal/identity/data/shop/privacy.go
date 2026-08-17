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
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type PrivacyRepository struct{ database *sql.DB }

var _ shop.PrivacyRepository = (*PrivacyRepository)(nil)

func NewPrivacyRepository(database *sql.DB) *PrivacyRepository {
	return &PrivacyRepository{database: database}
}

func (r *PrivacyRepository) GetPrivacy(ctx context.Context, merchantID, shopID int64) (model.Privacy, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.Privacy{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensurePrivacyShopScope(ctx, tx, merchantID, shopID, false); err != nil {
		return model.Privacy{}, err
	}
	value, err := readPrivacy(ctx, tx, merchantID, shopID)
	if errors.Is(err, model.ErrPrivacyNotFound) {
		if err := tx.Commit(); err != nil {
			return model.Privacy{}, fmt.Errorf("shop privacy default commit: %w", err)
		}
		return model.DefaultPrivacy(merchantID, shopID), nil
	}
	if err != nil {
		return model.Privacy{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Privacy{}, fmt.Errorf("shop privacy get commit: %w", err)
	}
	return value, nil
}

func (r *PrivacyRepository) SavePrivacy(ctx context.Context, command model.SavePrivacyCommand) (model.Privacy, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Privacy{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	document, found, err := insertOrReplayPrivacyCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Privacy{}, false, err
	}
	if found {
		var replay model.Privacy
		if json.Unmarshal(document, &replay) != nil || replay.ValidatePersisted() != nil {
			return model.Privacy{}, false, fmt.Errorf("shop privacy command response is incomplete")
		}
		if err := tx.Commit(); err != nil {
			return model.Privacy{}, false, fmt.Errorf("shop privacy replay commit: %w", err)
		}
		return replay, true, nil
	}
	if err := ensurePrivacyShopScope(ctx, tx, command.Privacy.MerchantID, command.Privacy.ShopID, true); err != nil {
		return model.Privacy{}, false, err
	}
	current, err := readPrivacyForUpdate(ctx, tx, command.Privacy.MerchantID, command.Privacy.ShopID)
	exists := err == nil
	if err != nil && !errors.Is(err, model.ErrPrivacyNotFound) {
		return model.Privacy{}, false, err
	}
	if exists && current.Version != command.ExpectedVersion {
		return model.Privacy{}, false, model.ErrPrivacyConflict
	}
	if !exists && command.ExpectedVersion != 0 {
		return model.Privacy{}, false, model.ErrPrivacyConflict
	}
	value := command.Privacy
	if !exists {
		result, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_privacy
(merchant_id,shop_id,collect_consent,marketing_consent,cookie_banner,data_retention_days,contact_email,version)
VALUES(?,?,?,?,?,?,?,1)`, value.MerchantID, value.ShopID, value.CollectConsent, value.MarketingConsent,
			value.CookieBanner, value.DataRetentionDays, value.ContactEmail)
		if privacyDuplicate(err) {
			return model.Privacy{}, false, model.ErrPrivacyConflict
		}
		if err != nil {
			return model.Privacy{}, false, fmt.Errorf("shop privacy insert: %w", err)
		}
		if _, err := result.LastInsertId(); err != nil {
			return model.Privacy{}, false, fmt.Errorf("shop privacy inserted id: %w", err)
		}
	} else {
		result, err := tx.ExecContext(ctx, `UPDATE identity_shop_privacy
SET collect_consent=?,marketing_consent=?,cookie_banner=?,data_retention_days=?,contact_email=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE privacy_id=? AND merchant_id=? AND shop_id=? AND version=?`, value.CollectConsent, value.MarketingConsent, value.CookieBanner,
			value.DataRetentionDays, value.ContactEmail, current.ID, value.MerchantID, value.ShopID, command.ExpectedVersion)
		if err != nil {
			return model.Privacy{}, false, fmt.Errorf("shop privacy update: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return model.Privacy{}, false, model.ErrPrivacyConflict
		}
	}
	saved, err := readPrivacy(ctx, tx, value.MerchantID, value.ShopID)
	if err != nil {
		return model.Privacy{}, false, err
	}
	if err := appendPrivacyOutbox(ctx, tx, saved); err != nil {
		return model.Privacy{}, false, err
	}
	if err := completePrivacyCommand(ctx, tx, command.CommandKey, saved.Version, saved); err != nil {
		return model.Privacy{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Privacy{}, false, fmt.Errorf("shop privacy save commit: %w", err)
	}
	return saved, false, nil
}

func (r *PrivacyRepository) begin(ctx context.Context, readOnly bool) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: readOnly})
	if err != nil {
		return nil, fmt.Errorf("shop privacy begin: %w", err)
	}
	return tx, nil
}

func ensurePrivacyShopScope(ctx context.Context, tx *sql.Tx, merchantID, shopID int64, lock bool) error {
	query := `SELECT merchant_id,shop_id FROM identity_shop WHERE merchant_id=? AND shop_id=? AND status<>'CLOSED'`
	if lock {
		query += " FOR UPDATE"
	}
	var foundMerchantID, foundShopID int64
	err := tx.QueryRowContext(ctx, query, merchantID, shopID).Scan(&foundMerchantID, &foundShopID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrPrivacyNotFound
	}
	if err != nil {
		return fmt.Errorf("shop privacy shop scope: %w", err)
	}
	return nil
}

type privacyScanner interface{ Scan(...any) error }

func scanPrivacy(row privacyScanner) (model.Privacy, error) {
	var value model.Privacy
	var collect, marketing, cookie bool
	err := row.Scan(&value.ID, &value.MerchantID, &value.ShopID, &collect, &marketing, &cookie,
		&value.DataRetentionDays, &value.ContactEmail, &value.Version)
	value.CollectConsent = collect
	value.MarketingConsent = marketing
	value.CookieBanner = cookie
	return value, err
}

func readPrivacy(ctx context.Context, tx *sql.Tx, merchantID, shopID int64) (model.Privacy, error) {
	value, err := scanPrivacy(tx.QueryRowContext(ctx, `SELECT privacy_id,merchant_id,shop_id,collect_consent,marketing_consent,cookie_banner,data_retention_days,contact_email,version
FROM identity_shop_privacy WHERE merchant_id=? AND shop_id=?`, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Privacy{}, model.ErrPrivacyNotFound
	}
	if err != nil {
		return model.Privacy{}, fmt.Errorf("shop privacy read: %w", err)
	}
	return value, nil
}

func readPrivacyForUpdate(ctx context.Context, tx *sql.Tx, merchantID, shopID int64) (model.Privacy, error) {
	value, err := scanPrivacy(tx.QueryRowContext(ctx, `SELECT privacy_id,merchant_id,shop_id,collect_consent,marketing_consent,cookie_banner,data_retention_days,contact_email,version
FROM identity_shop_privacy WHERE merchant_id=? AND shop_id=? FOR UPDATE`, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Privacy{}, model.ErrPrivacyNotFound
	}
	if err != nil {
		return model.Privacy{}, fmt.Errorf("shop privacy lock: %w", err)
	}
	return value, nil
}

func insertOrReplayPrivacyCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) ([]byte, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_privacy_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !privacyDuplicate(err) {
		if err != nil {
			return nil, false, fmt.Errorf("shop privacy insert command: %w", err)
		}
		return nil, false, nil
	}
	var stored, document []byte
	var version uint64
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_shop_privacy_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &version, &document); err != nil {
		return nil, false, fmt.Errorf("shop privacy read command: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return nil, false, model.ErrPrivacyIdempotency
	}
	if version == 0 || len(document) == 0 {
		return nil, false, fmt.Errorf("shop privacy command response is incomplete")
	}
	return document, true, nil
}

func completePrivacyCommand(ctx context.Context, tx *sql.Tx, key string, version uint64, response any) error {
	document, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("shop privacy encode response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_privacy_command
SET response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, version, document, key)
	if err != nil {
		return fmt.Errorf("shop privacy complete command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("shop privacy command completion affected no row")
	}
	return nil
}

func appendPrivacyOutbox(ctx context.Context, tx *sql.Tx, value model.Privacy) error {
	encoded, err := json.Marshal(map[string]any{
		"privacyId": value.ID, "merchantId": value.MerchantID, "shopId": value.ShopID,
		"collectConsent": value.CollectConsent, "marketingConsent": value.MarketingConsent,
		"cookieBanner": value.CookieBanner, "dataRetentionDays": value.DataRetentionDays,
		"version": value.Version,
	})
	if err != nil {
		return err
	}
	id := make([]byte, 18)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO identity_outbox (event_id, aggregate_type, aggregate_id, aggregate_version, event_type, payload_json) VALUES (?, ?, ?, ?, ?, ?)`,
		hex.EncodeToString(id), "shop_privacy", fmt.Sprintf("%d", value.ID), value.Version, "identity.shop.privacy.changed", encoded)
	return err
}

func privacyDuplicate(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
