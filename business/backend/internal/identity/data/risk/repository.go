package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/risk"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/risk/model"
)

type Repository struct{ database *sql.DB }

var _ risk.Repository = (*Repository)(nil)

func NewRepository(database *sql.DB) *Repository { return &Repository{database: database} }

func (r *Repository) List(ctx context.Context, query model.Query) (model.Page, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return model.Page{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureShopScope(ctx, tx, query.MerchantID, query.ShopID); err != nil {
		return model.Page{}, err
	}
	where := "e.merchant_id=? AND e.shop_id=?"
	args := []any{query.MerchantID, query.ShopID}
	if query.VisitorID != "" {
		where += " AND e.visitor_id=?"
		args = append(args, query.VisitorID)
	}
	if query.RoomID > 0 {
		where += " AND e.room_id=?"
		args = append(args, query.RoomID)
	}
	if query.Reason != "" {
		where += " AND e.reason LIKE ?"
		args = append(args, "%"+query.Reason+"%")
	}
	if query.VisitorStatus != "" {
		where += " AND v.status=?"
		args = append(args, query.VisitorStatus)
	}
	var total int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_risk_event e
LEFT JOIN identity_visitor_risk v
  ON v.merchant_id=e.merchant_id AND v.shop_id=e.shop_id AND v.visitor_id=e.visitor_id
WHERE `+where, args...).Scan(&total); err != nil {
		return model.Page{}, fmt.Errorf("risk event count: %w", err)
	}
	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := tx.QueryContext(ctx, `SELECT e.event_id,e.merchant_id,e.shop_id,e.visitor_id,e.nickname,e.room_id,e.reason,
e.score_before,e.score_after_decay,e.score_delta,e.score_after,e.created_at,
COALESCE(v.score, e.score_after),COALESCE(v.level, 'NONE'),COALESCE(v.status, 'NORMAL')
FROM identity_risk_event e
LEFT JOIN identity_visitor_risk v
  ON v.merchant_id=e.merchant_id AND v.shop_id=e.shop_id AND v.visitor_id=e.visitor_id
WHERE `+where+` ORDER BY e.created_at DESC, e.event_id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return model.Page{}, fmt.Errorf("risk event list: %w", err)
	}
	defer rows.Close()
	items := []model.Event{}
	for rows.Next() {
		value, err := scanEvent(rows)
		if err != nil {
			return model.Page{}, fmt.Errorf("risk event list scan: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.Page{}, fmt.Errorf("risk event list iterate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Page{}, fmt.Errorf("risk event list commit: %w", err)
	}
	return model.Page{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *Repository) begin(ctx context.Context) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("risk event begin: %w", err)
	}
	return tx, nil
}

func ensureShopScope(ctx context.Context, tx *sql.Tx, merchantID, shopID int64) error {
	var foundMerchantID, foundShopID int64
	err := tx.QueryRowContext(ctx, `SELECT merchant_id,shop_id FROM identity_shop WHERE merchant_id=? AND shop_id=? AND status<>'CLOSED'`, merchantID, shopID).
		Scan(&foundMerchantID, &foundShopID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("risk event shop scope: %w", err)
	}
	return nil
}

type scanner interface{ Scan(...any) error }

func scanEvent(row scanner) (model.Event, error) {
	var value model.Event
	var level, status string
	err := row.Scan(&value.ID, &value.MerchantID, &value.ShopID, &value.VisitorID, &value.Nickname, &value.RoomID, &value.Reason,
		&value.ScoreBefore, &value.ScoreAfterDecay, &value.ScoreDelta, &value.ScoreAfter, &value.CreatedAt,
		&value.CurrentScore, &level, &status)
	value.CurrentLevel = model.Level(level)
	value.VisitorStatus = model.Status(status)
	return value, err
}
