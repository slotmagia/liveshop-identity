package mysql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

var _ fulfillment.ShipmentRepository = (*Repository)(nil)

const shipmentSelect = `shipment_id,merchant_id,shop_id,order_id,carrier,tracking_no,status,traces,version,created_at,updated_at`

func (r *Repository) ListShipments(ctx context.Context, query model.ShipmentQuery) (model.ShipmentPage, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.ShipmentPage{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureShopScope(ctx, tx, query.MerchantID, query.ShopID, false); err != nil {
		return model.ShipmentPage{}, err
	}
	where := "merchant_id=? AND shop_id=?"
	args := []any{query.MerchantID, query.ShopID}
	if query.OrderID > 0 {
		where += " AND order_id=?"
		args = append(args, query.OrderID)
	}
	if query.Status != "" {
		where += " AND status=?"
		args = append(args, query.Status)
	}
	var total int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM identity_shipment WHERE "+where, args...).Scan(&total); err != nil {
		return model.ShipmentPage{}, fmt.Errorf("shipment count: %w", err)
	}
	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := tx.QueryContext(ctx, `SELECT `+shipmentSelect+` FROM identity_shipment WHERE `+where+` ORDER BY created_at DESC, shipment_id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return model.ShipmentPage{}, fmt.Errorf("shipment list: %w", err)
	}
	defer rows.Close()
	items := []model.Shipment{}
	for rows.Next() {
		value, err := scanShipment(rows)
		if err != nil {
			return model.ShipmentPage{}, fmt.Errorf("shipment list scan: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.ShipmentPage{}, fmt.Errorf("shipment list iterate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.ShipmentPage{}, fmt.Errorf("shipment list commit: %w", err)
	}
	return model.ShipmentPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *Repository) GetShipment(ctx context.Context, merchantID, shopID, shipmentID int64) (model.Shipment, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.Shipment{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureShopScope(ctx, tx, merchantID, shopID, false); err != nil {
		return model.Shipment{}, err
	}
	value, err := readShipment(ctx, tx, shipmentID, merchantID, shopID)
	if err != nil {
		return model.Shipment{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Shipment{}, fmt.Errorf("shipment get commit: %w", err)
	}
	return value, nil
}

func (r *Repository) Ship(ctx context.Context, command model.ShipCommand) (model.Shipment, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Shipment{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShipmentCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Shipment{}, false, err
	}
	if found {
		return commitShipmentReplay(tx, replay)
	}
	if err := ensureShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Shipment{}, false, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO identity_shipment(merchant_id,shop_id,order_id,carrier,tracking_no,status,traces,version)
VALUES (?,?,?,?,?,'SHIPPED','[]',1)`, command.MerchantID, command.ShopID, command.OrderID, command.Carrier, command.TrackingNo)
	if err != nil {
		if complaintDuplicateKey(err) {
			return model.Shipment{}, false, model.ErrConflict
		}
		return model.Shipment{}, false, fmt.Errorf("shipment insert: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil || id <= 0 {
		return model.Shipment{}, false, fmt.Errorf("shipment insert id: %w", err)
	}
	saved, err := readShipment(ctx, tx, id, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Shipment{}, false, err
	}
	return completeAndCommitShipment(ctx, tx, command.CommandKey, saved)
}

func (r *Repository) AddTrace(ctx context.Context, command model.TraceCommand) (model.Shipment, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Shipment{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShipmentCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Shipment{}, false, err
	}
	if found {
		return commitShipmentReplay(tx, replay)
	}
	if err := ensureShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Shipment{}, false, err
	}
	current, err := readShipmentForUpdate(ctx, tx, command.ShipmentID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Shipment{}, false, err
	}
	if current.Version != command.ExpectedVersion || current.Status != model.ShipmentShipped || len(current.Traces) >= model.MaxShipmentTraces {
		return model.Shipment{}, false, model.ErrConflict
	}
	traces := append(append([]model.Trace{}, current.Traces...), model.Trace{OccurredAt: time.Now().UTC(), Node: command.Node})
	document, err := json.Marshal(traces)
	if err != nil {
		return model.Shipment{}, false, fmt.Errorf("shipment encode traces: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shipment
SET traces=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE shipment_id=? AND merchant_id=? AND shop_id=? AND version=? AND status=?`,
		document, current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion, model.ShipmentShipped)
	if err != nil {
		return model.Shipment{}, false, fmt.Errorf("shipment add trace: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Shipment{}, false, model.ErrConflict
	}
	saved, err := readShipment(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.Shipment{}, false, err
	}
	return completeAndCommitShipment(ctx, tx, command.CommandKey, saved)
}

func (r *Repository) CloseShipment(ctx context.Context, command model.CloseShipmentCommand) (model.Shipment, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Shipment{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShipmentCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Shipment{}, false, err
	}
	if found {
		return commitShipmentReplay(tx, replay)
	}
	if err := ensureShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Shipment{}, false, err
	}
	current, err := readShipmentForUpdate(ctx, tx, command.ShipmentID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Shipment{}, false, err
	}
	if current.Version != command.ExpectedVersion || current.Status != model.ShipmentShipped {
		return model.Shipment{}, false, model.ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shipment
SET status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE shipment_id=? AND merchant_id=? AND shop_id=? AND version=? AND status=?`,
		model.ShipmentDelivered, current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion, model.ShipmentShipped)
	if err != nil {
		return model.Shipment{}, false, fmt.Errorf("shipment close: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Shipment{}, false, model.ErrConflict
	}
	saved, err := readShipment(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.Shipment{}, false, err
	}
	return completeAndCommitShipment(ctx, tx, command.CommandKey, saved)
}

func scanShipment(row scanner) (model.Shipment, error) {
	var value model.Shipment
	var status string
	var traces []byte
	err := row.Scan(&value.ID, &value.MerchantID, &value.ShopID, &value.OrderID, &value.Carrier, &value.TrackingNo,
		&status, &traces, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	value.Status = model.ShipmentStatus(status)
	if len(traces) == 0 {
		value.Traces = []model.Trace{}
		return value, err
	}
	if json.Unmarshal(traces, &value.Traces) != nil {
		return model.Shipment{}, fmt.Errorf("shipment traces json")
	}
	if value.Traces == nil {
		value.Traces = []model.Trace{}
	}
	return value, err
}

func readShipment(ctx context.Context, tx *sql.Tx, shipmentID, merchantID, shopID int64) (model.Shipment, error) {
	value, err := scanShipment(tx.QueryRowContext(ctx, `SELECT `+shipmentSelect+` FROM identity_shipment WHERE shipment_id=? AND merchant_id=? AND shop_id=?`, shipmentID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Shipment{}, model.ErrNotFound
	}
	if err != nil {
		return model.Shipment{}, fmt.Errorf("shipment read: %w", err)
	}
	return value, nil
}

func readShipmentForUpdate(ctx context.Context, tx *sql.Tx, shipmentID, merchantID, shopID int64) (model.Shipment, error) {
	value, err := scanShipment(tx.QueryRowContext(ctx, `SELECT `+shipmentSelect+` FROM identity_shipment WHERE shipment_id=? AND merchant_id=? AND shop_id=? FOR UPDATE`, shipmentID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Shipment{}, model.ErrNotFound
	}
	if err != nil {
		return model.Shipment{}, fmt.Errorf("shipment lock: %w", err)
	}
	return value, nil
}

func insertOrReplayShipmentCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) (model.Shipment, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_shipment_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !complaintDuplicateKey(err) {
		if err != nil {
			return model.Shipment{}, false, fmt.Errorf("shipment insert command: %w", err)
		}
		return model.Shipment{}, false, nil
	}
	var stored []byte
	var responseVersion uint64
	var document []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_shipment_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &responseVersion, &document); err != nil {
		return model.Shipment{}, false, fmt.Errorf("shipment read command replay: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return model.Shipment{}, false, model.ErrIdempotency
	}
	var replay model.Shipment
	if responseVersion == 0 || len(document) == 0 || json.Unmarshal(document, &replay) != nil || replay.ID <= 0 || replay.Version != responseVersion {
		return model.Shipment{}, false, fmt.Errorf("shipment command response is incomplete")
	}
	if replay.Traces == nil {
		replay.Traces = []model.Trace{}
	}
	return replay, true, nil
}

func completeAndCommitShipment(ctx context.Context, tx *sql.Tx, key string, value model.Shipment) (model.Shipment, bool, error) {
	document, err := json.Marshal(value)
	if err != nil {
		return model.Shipment{}, false, fmt.Errorf("shipment encode response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shipment_command SET shipment_id=?,response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, value.ID, value.Version, document, key)
	if err != nil {
		return model.Shipment{}, false, fmt.Errorf("shipment complete command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Shipment{}, false, fmt.Errorf("shipment command completion affected no row")
	}
	if err := tx.Commit(); err != nil {
		return model.Shipment{}, false, fmt.Errorf("shipment commit: %w", err)
	}
	return value, false, nil
}

func commitShipmentReplay(tx *sql.Tx, value model.Shipment) (model.Shipment, bool, error) {
	if err := tx.Commit(); err != nil {
		return model.Shipment{}, false, fmt.Errorf("shipment commit replay: %w", err)
	}
	return value, true, nil
}
