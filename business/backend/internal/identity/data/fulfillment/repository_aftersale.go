package mysql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

var _ fulfillment.AftersaleRepository = (*Repository)(nil)

const aftersaleSelect = `aftersale_id,merchant_id,shop_id,customer_subject,order_id,payment_no,type,requested_amount,amount,reason,status,return_status,handle_note,version,created_at,updated_at,reviewed_at,received_at`

func (r *Repository) ListAftersales(ctx context.Context, query model.AftersaleQuery) (model.AftersalePage, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.AftersalePage{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureShopScope(ctx, tx, query.MerchantID, query.ShopID, false); err != nil {
		return model.AftersalePage{}, mapAftersaleScope(err)
	}
	where := "merchant_id=? AND shop_id=?"
	args := []any{query.MerchantID, query.ShopID}
	if query.CustomerSubject != "" {
		where += " AND customer_subject=?"
		args = append(args, query.CustomerSubject)
	}
	if query.Status != "" {
		where += " AND status=?"
		args = append(args, query.Status)
	}
	if query.Type != "" {
		where += " AND type=?"
		args = append(args, query.Type)
	}
	var total int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM identity_aftersale WHERE "+where, args...).Scan(&total); err != nil {
		return model.AftersalePage{}, fmt.Errorf("aftersale count: %w", err)
	}
	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := tx.QueryContext(ctx, `SELECT `+aftersaleSelect+` FROM identity_aftersale WHERE `+where+` ORDER BY created_at DESC, aftersale_id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return model.AftersalePage{}, fmt.Errorf("aftersale list: %w", err)
	}
	defer rows.Close()
	items := []model.Aftersale{}
	for rows.Next() {
		value, err := scanAftersale(rows)
		if err != nil {
			return model.AftersalePage{}, fmt.Errorf("aftersale list scan: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.AftersalePage{}, fmt.Errorf("aftersale list iterate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.AftersalePage{}, fmt.Errorf("aftersale list commit: %w", err)
	}
	return model.AftersalePage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *Repository) GetAftersale(ctx context.Context, merchantID, shopID, aftersaleID int64) (model.Aftersale, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.Aftersale{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureShopScope(ctx, tx, merchantID, shopID, false); err != nil {
		return model.Aftersale{}, mapAftersaleScope(err)
	}
	value, err := readAftersale(ctx, tx, aftersaleID, merchantID, shopID)
	if err != nil {
		return model.Aftersale{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Aftersale{}, fmt.Errorf("aftersale get commit: %w", err)
	}
	return value, nil
}

func (r *Repository) ReviewAftersale(ctx context.Context, command model.ReviewAftersaleCommand) (model.Aftersale, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Aftersale{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayAftersaleCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Aftersale{}, false, err
	}
	if found {
		return commitAftersaleReplay(tx, replay)
	}
	if err := ensureShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Aftersale{}, false, mapAftersaleScope(err)
	}
	current, err := readAftersaleForUpdate(ctx, tx, command.AftersaleID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Aftersale{}, false, err
	}
	if current.Version != command.ExpectedVersion || current.Status != model.AftersalePending {
		return model.Aftersale{}, false, model.ErrAftersaleConflict
	}
	amount := current.Amount
	if command.Status == model.AftersaleApproved {
		if command.Amount > 0 {
			amount = command.Amount
		}
		if amount < 1 || amount > current.RequestedAmount {
			return model.Aftersale{}, false, model.ErrAftersaleInvalid
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_aftersale
SET status=?,amount=?,handle_note=?,reviewed_at=CURRENT_TIMESTAMP(3),version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE aftersale_id=? AND merchant_id=? AND shop_id=? AND version=? AND status=?`,
		command.Status, amount, command.HandleNote, current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion, model.AftersalePending)
	if err != nil {
		return model.Aftersale{}, false, fmt.Errorf("aftersale review: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Aftersale{}, false, model.ErrAftersaleConflict
	}
	saved, err := readAftersale(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.Aftersale{}, false, err
	}
	return completeAndCommitAftersale(ctx, tx, command.CommandKey, saved)
}

func (r *Repository) ReceiveAftersale(ctx context.Context, command model.ReceiveAftersaleCommand) (model.Aftersale, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Aftersale{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayAftersaleCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Aftersale{}, false, err
	}
	if found {
		return commitAftersaleReplay(tx, replay)
	}
	if err := ensureShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Aftersale{}, false, mapAftersaleScope(err)
	}
	current, err := readAftersaleForUpdate(ctx, tx, command.AftersaleID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Aftersale{}, false, err
	}
	if current.Version != command.ExpectedVersion || current.Type != model.AftersaleReturnRefund ||
		current.ReturnStatus != model.ReturnPending ||
		(current.Status != model.AftersaleApproved && current.Status != model.AftersaleRefunded) ||
		len(current.Items) == 0 {
		return model.Aftersale{}, false, model.ErrAftersaleConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_aftersale_item SET received_quantity=quantity,updated_at=CURRENT_TIMESTAMP(3)
WHERE aftersale_id=? AND merchant_id=? AND shop_id=?`, current.ID, current.MerchantID, current.ShopID); err != nil {
		return model.Aftersale{}, false, fmt.Errorf("aftersale receive items: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_aftersale
SET return_status=?,received_at=CURRENT_TIMESTAMP(3),version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE aftersale_id=? AND merchant_id=? AND shop_id=? AND version=? AND return_status=?`,
		model.ReturnReceived, current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion, model.ReturnPending)
	if err != nil {
		return model.Aftersale{}, false, fmt.Errorf("aftersale receive: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Aftersale{}, false, model.ErrAftersaleConflict
	}
	saved, err := readAftersale(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.Aftersale{}, false, err
	}
	return completeAndCommitAftersale(ctx, tx, command.CommandKey, saved)
}

func scanAftersale(row scanner) (model.Aftersale, error) {
	var value model.Aftersale
	var ticketType, status, returnStatus string
	var reviewedAt, receivedAt sql.NullTime
	err := row.Scan(&value.ID, &value.MerchantID, &value.ShopID, &value.CustomerSubject, &value.OrderID, &value.PaymentNo,
		&ticketType, &value.RequestedAmount, &value.Amount, &value.Reason, &status, &returnStatus, &value.HandleNote,
		&value.Version, &value.CreatedAt, &value.UpdatedAt, &reviewedAt, &receivedAt)
	value.Type = model.AftersaleType(ticketType)
	value.Status = model.AftersaleStatus(status)
	value.ReturnStatus = model.ReturnStatus(returnStatus)
	if reviewedAt.Valid {
		moment := reviewedAt.Time
		value.ReviewedAt = &moment
	}
	if receivedAt.Valid {
		moment := receivedAt.Time
		value.ReceivedAt = &moment
	}
	return value, err
}

func readAftersale(ctx context.Context, tx *sql.Tx, aftersaleID, merchantID, shopID int64) (model.Aftersale, error) {
	value, err := scanAftersale(tx.QueryRowContext(ctx, `SELECT `+aftersaleSelect+` FROM identity_aftersale WHERE aftersale_id=? AND merchant_id=? AND shop_id=?`, aftersaleID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Aftersale{}, model.ErrAftersaleNotFound
	}
	if err != nil {
		return model.Aftersale{}, fmt.Errorf("aftersale read: %w", err)
	}
	items, err := listAftersaleItems(ctx, tx, aftersaleID, merchantID, shopID)
	if err != nil {
		return model.Aftersale{}, err
	}
	value.Items = items
	return value, nil
}

func readAftersaleForUpdate(ctx context.Context, tx *sql.Tx, aftersaleID, merchantID, shopID int64) (model.Aftersale, error) {
	value, err := scanAftersale(tx.QueryRowContext(ctx, `SELECT `+aftersaleSelect+` FROM identity_aftersale WHERE aftersale_id=? AND merchant_id=? AND shop_id=? FOR UPDATE`, aftersaleID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Aftersale{}, model.ErrAftersaleNotFound
	}
	if err != nil {
		return model.Aftersale{}, fmt.Errorf("aftersale lock: %w", err)
	}
	items, err := listAftersaleItems(ctx, tx, aftersaleID, merchantID, shopID)
	if err != nil {
		return model.Aftersale{}, err
	}
	value.Items = items
	return value, nil
}

func listAftersaleItems(ctx context.Context, tx *sql.Tx, aftersaleID, merchantID, shopID int64) ([]model.AftersaleItem, error) {
	rows, err := tx.QueryContext(ctx, `SELECT item_id,sku_id,title,quantity,refund_amount,received_quantity
FROM identity_aftersale_item WHERE aftersale_id=? AND merchant_id=? AND shop_id=? ORDER BY item_id ASC`, aftersaleID, merchantID, shopID)
	if err != nil {
		return nil, fmt.Errorf("aftersale items: %w", err)
	}
	defer rows.Close()
	items := []model.AftersaleItem{}
	for rows.Next() {
		var item model.AftersaleItem
		if err := rows.Scan(&item.ID, &item.SKUID, &item.Title, &item.Quantity, &item.RefundAmount, &item.ReceivedQuantity); err != nil {
			return nil, fmt.Errorf("aftersale item scan: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("aftersale items iterate: %w", err)
	}
	return items, nil
}

func insertOrReplayAftersaleCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) (model.Aftersale, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_aftersale_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !complaintDuplicateKey(err) {
		if err != nil {
			return model.Aftersale{}, false, fmt.Errorf("aftersale insert command: %w", err)
		}
		return model.Aftersale{}, false, nil
	}
	var stored []byte
	var responseVersion uint64
	var document []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_aftersale_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &responseVersion, &document); err != nil {
		return model.Aftersale{}, false, fmt.Errorf("aftersale read command replay: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return model.Aftersale{}, false, model.ErrAftersaleIdempotency
	}
	var replay model.Aftersale
	if responseVersion == 0 || len(document) == 0 || json.Unmarshal(document, &replay) != nil || replay.ID <= 0 || replay.Version != responseVersion {
		return model.Aftersale{}, false, fmt.Errorf("aftersale command response is incomplete")
	}
	return replay, true, nil
}

func completeAndCommitAftersale(ctx context.Context, tx *sql.Tx, key string, value model.Aftersale) (model.Aftersale, bool, error) {
	document, err := json.Marshal(value)
	if err != nil {
		return model.Aftersale{}, false, fmt.Errorf("aftersale encode response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_aftersale_command SET aftersale_id=?,response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, value.ID, value.Version, document, key)
	if err != nil {
		return model.Aftersale{}, false, fmt.Errorf("aftersale complete command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Aftersale{}, false, fmt.Errorf("aftersale command completion affected no row")
	}
	if err := tx.Commit(); err != nil {
		return model.Aftersale{}, false, fmt.Errorf("aftersale commit: %w", err)
	}
	return value, false, nil
}

func commitAftersaleReplay(tx *sql.Tx, value model.Aftersale) (model.Aftersale, bool, error) {
	if err := tx.Commit(); err != nil {
		return model.Aftersale{}, false, fmt.Errorf("aftersale commit replay: %w", err)
	}
	return value, true, nil
}

func mapAftersaleScope(err error) error {
	if errors.Is(err, model.ErrNotFound) {
		return model.ErrAftersaleNotFound
	}
	return err
}
