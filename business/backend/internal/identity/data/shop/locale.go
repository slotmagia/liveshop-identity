package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

func (r *Repository) GetLanguages(ctx context.Context, merchantID, shopID int64) (model.Languages, error) {
	if r == nil || r.database == nil {
		return model.Languages{}, model.ErrUnavailable
	}
	shop, err := r.GetManagedShop(ctx, merchantID, shopID)
	if err != nil {
		return model.Languages{}, err
	}
	rows, err := listLocaleRows(ctx, r.database, merchantID, shopID)
	if err != nil {
		return model.Languages{}, err
	}
	return assembleLanguages(shop, rows), nil
}

func (r *Repository) PublishedLocales(ctx context.Context, shopID int64) (string, []string, error) {
	if r == nil || r.database == nil {
		return "", nil, model.ErrUnavailable
	}
	shop, err := scanShop(r.database.QueryRowContext(ctx, `SELECT `+shopSelect+` FROM identity_shop WHERE shop_id=? AND status<>'CLOSED'`, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, model.ErrNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("identity shop published locales: %w", err)
	}
	rows, err := listLocaleRows(ctx, r.database, shop.MerchantID, shop.ID)
	if err != nil {
		return "", nil, err
	}
	defaultLocale, published := model.PublishedFromRows(shop.DefaultLocale, rows)
	return defaultLocale, published, nil
}

func (r *Repository) ReplaceLanguages(ctx context.Context, command model.ReplaceLanguagesCommand) (model.Languages, bool, error) {
	tx, err := r.beginShopWrite(ctx)
	if err != nil {
		return model.Languages{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShopCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Languages{}, false, err
	}
	if found {
		shop, replayed, err := commitShopReplay(tx, replay)
		if err != nil {
			return model.Languages{}, false, err
		}
		value, err := r.GetLanguages(ctx, shop.MerchantID, shop.ID)
		return value, replayed, err
	}
	current, err := lockManagedShop(ctx, tx, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Languages{}, false, err
	}
	if current.Version != command.ExpectedVersion {
		return model.Languages{}, false, model.ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop SET default_locale=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE shop_id=? AND merchant_id=? AND status<>'CLOSED' AND version=?`, command.DefaultLocale, current.ID, current.MerchantID, command.ExpectedVersion)
	if err != nil {
		return model.Languages{}, false, fmt.Errorf("identity shop languages update: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Languages{}, false, model.ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM identity_shop_locale WHERE merchant_id=? AND shop_id=?`, current.MerchantID, current.ID); err != nil {
		return model.Languages{}, false, fmt.Errorf("identity shop languages replace: %w", err)
	}
	for index, locale := range command.PublishedLocales {
		if err := insertShopLocale(ctx, tx, current.MerchantID, current.ID, locale, index); err != nil {
			return model.Languages{}, false, err
		}
	}
	saved, err := readShop(ctx, tx, current.MerchantID, current.ID, false)
	if err != nil {
		return model.Languages{}, false, err
	}
	rows, err := listLocaleRowsTx(ctx, tx, saved.MerchantID, saved.ID)
	if err != nil {
		return model.Languages{}, false, err
	}
	if err := completeShopCommand(ctx, tx, command.CommandKey, saved); err != nil {
		return model.Languages{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Languages{}, false, fmt.Errorf("identity shop languages commit: %w", err)
	}
	return assembleLanguages(saved, rows), false, nil
}

func insertShopLocale(ctx context.Context, tx *sql.Tx, merchantID, shopID int64, locale string, sortOrder int) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_locale(merchant_id,shop_id,locale,published,sort_order) VALUES(?,?,?,1,?)`,
		merchantID, shopID, locale, sortOrder); err != nil {
		return fmt.Errorf("identity shop locale insert: %w", err)
	}
	return nil
}

func listLocaleRows(ctx context.Context, db *sql.DB, merchantID, shopID int64) ([]model.LocaleRow, error) {
	rows, err := db.QueryContext(ctx, `SELECT locale,published,sort_order FROM identity_shop_locale WHERE merchant_id=? AND shop_id=? ORDER BY sort_order, locale`, merchantID, shopID)
	if err != nil {
		return nil, fmt.Errorf("identity shop locales: %w", err)
	}
	defer rows.Close()
	return scanLocaleRows(rows)
}

func listLocaleRowsTx(ctx context.Context, tx *sql.Tx, merchantID, shopID int64) ([]model.LocaleRow, error) {
	rows, err := tx.QueryContext(ctx, `SELECT locale,published,sort_order FROM identity_shop_locale WHERE merchant_id=? AND shop_id=? ORDER BY sort_order, locale`, merchantID, shopID)
	if err != nil {
		return nil, fmt.Errorf("identity shop locales: %w", err)
	}
	defer rows.Close()
	return scanLocaleRows(rows)
}

func scanLocaleRows(rows *sql.Rows) ([]model.LocaleRow, error) {
	out := []model.LocaleRow{}
	for rows.Next() {
		var item model.LocaleRow
		var published int
		if err := rows.Scan(&item.Locale, &published, &item.SortOrder); err != nil {
			return nil, fmt.Errorf("identity shop locale scan: %w", err)
		}
		item.Published = published == 1
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity shop locale iterate: %w", err)
	}
	return out, nil
}

func assembleLanguages(shop model.Shop, rows []model.LocaleRow) model.Languages {
	if len(rows) == 0 {
		rows = model.DefaultLanguageRows(shop.DefaultLocale)
	}
	return model.Languages{
		MerchantID: shop.MerchantID, ShopID: shop.ID, DefaultLocale: shop.DefaultLocale, Version: shop.Version, Items: rows,
	}
}
