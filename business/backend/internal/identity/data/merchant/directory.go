package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
)

type Repository struct{ database *sql.DB }

var _ merchant.Repository = (*Repository)(nil)

func NewRepository(database *sql.DB) *Repository { return &Repository{database: database} }

func (r *Repository) ListMerchants(ctx context.Context) ([]model.Merchant, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	rows, err := r.database.QueryContext(ctx, `SELECT merchant_id,name,COALESCE(external_id,''),status,version
FROM identity_merchant WHERE status<>'CLOSED' ORDER BY merchant_id DESC`)
	if err != nil {
		return nil, fmt.Errorf("identity merchant directory: %w", err)
	}
	defer rows.Close()
	values := []model.Merchant{}
	for rows.Next() {
		var value model.Merchant
		if err := rows.Scan(&value.ID, &value.Name, &value.ExternalID, &value.Status, &value.Version); err != nil {
			return nil, fmt.Errorf("identity merchant directory scan: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity merchant directory iterate: %w", err)
	}
	return values, nil
}
