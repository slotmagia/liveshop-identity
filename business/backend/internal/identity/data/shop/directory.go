package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type Repository struct{ database *sql.DB }

var _ shop.Repository = (*Repository)(nil)

func NewRepository(database *sql.DB) *Repository { return &Repository{database: database} }

const shopSelect = `shop_id,merchant_id,code,COALESCE(subdomain,''),name,default_locale,currency,COALESCE(category_code,''),status,version`

func (r *Repository) ListShops(ctx context.Context, merchantID int64) ([]model.Shop, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	query := `SELECT ` + shopSelect + ` FROM identity_shop WHERE status<>'CLOSED'`
	args := []any{}
	if merchantID > 0 {
		query += ` AND merchant_id=?`
		args = append(args, merchantID)
	}
	query += ` ORDER BY shop_id DESC`
	rows, err := r.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("identity shop directory: %w", err)
	}
	defer rows.Close()
	values := []model.Shop{}
	for rows.Next() {
		value, err := scanShop(rows)
		if err != nil {
			return nil, fmt.Errorf("identity shop directory scan: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity shop directory iterate: %w", err)
	}
	return values, nil
}

func (r *Repository) ListShopsByMerchant(ctx context.Context, merchantID int64) ([]model.Shop, error) {
	if merchantID <= 0 {
		return nil, model.ErrInvalidMerchantID
	}
	return r.ListShops(ctx, merchantID)
}

func (r *Repository) ListManagedShops(ctx context.Context, query model.Query) (model.Page, error) {
	if r == nil || r.database == nil {
		return model.Page{}, model.ErrUnavailable
	}
	where := "merchant_id=? AND status<>'CLOSED'"
	args := []any{query.MerchantID}
	if query.Status != "" {
		where += " AND status=?"
		args = append(args, query.Status)
	}
	if query.Keyword != "" {
		like := "%" + query.Keyword + "%"
		where += " AND (name LIKE ? OR code LIKE ? OR COALESCE(subdomain,'') LIKE ? OR CAST(shop_id AS CHAR) LIKE ?)"
		args = append(args, like, like, like, like)
	}
	var total int64
	if err := r.database.QueryRowContext(ctx, "SELECT COUNT(*) FROM identity_shop WHERE "+where, args...).Scan(&total); err != nil {
		return model.Page{}, fmt.Errorf("identity shop managed count: %w", err)
	}
	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := r.database.QueryContext(ctx, `SELECT `+shopSelect+` FROM identity_shop WHERE `+where+` ORDER BY shop_id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return model.Page{}, fmt.Errorf("identity shop managed list: %w", err)
	}
	defer rows.Close()
	items := []model.Shop{}
	for rows.Next() {
		value, err := scanShop(rows)
		if err != nil {
			return model.Page{}, fmt.Errorf("identity shop managed scan: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.Page{}, fmt.Errorf("identity shop managed iterate: %w", err)
	}
	return model.Page{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *Repository) GetManagedShop(ctx context.Context, merchantID, shopID int64) (model.Shop, error) {
	if r == nil || r.database == nil {
		return model.Shop{}, model.ErrUnavailable
	}
	value, err := scanShop(r.database.QueryRowContext(ctx, `SELECT `+shopSelect+` FROM identity_shop WHERE merchant_id=? AND shop_id=? AND status<>'CLOSED'`, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Shop{}, model.ErrNotFound
	}
	if err != nil {
		return model.Shop{}, fmt.Errorf("identity shop get managed: %w", err)
	}
	return value, nil
}

type shopScanner interface{ Scan(...any) error }

func scanShop(row shopScanner) (model.Shop, error) {
	var value model.Shop
	err := row.Scan(&value.ID, &value.MerchantID, &value.Code, &value.Subdomain, &value.Name,
		&value.DefaultLocale, &value.Currency, &value.CategoryCode, &value.Status, &value.Version)
	return value, err
}
