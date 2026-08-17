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
	"strings"
	"time"

	subscription "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
	model "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
)

type OrderRepository struct{ database *sql.DB }

func NewOrderRepository(database *sql.DB) *OrderRepository {
	return &OrderRepository{database: database}
}

var _ subscription.OrderRepository = (*OrderRepository)(nil)

type storedPurchase struct {
	Order      model.Order      `json:"order"`
	Assignment model.Assignment `json:"assignment,omitempty"`
}

func (r *OrderRepository) CreateOrder(ctx context.Context, command model.CreateOrderCommand) (model.Order, bool, error) {
	if r == nil || r.database == nil {
		return model.Order{}, false, model.ErrOrderInvalid
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Order{}, false, fmt.Errorf("subscription order begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	digest := command.RequestDigest()
	_, err = tx.ExecContext(ctx, `INSERT INTO subscription_order_command(command_key,request_hash) VALUES(?,?)`, command.CommandKey, digest[:])
	if assignmentDuplicate(err) {
		replay, err := readOrderCommand(ctx, tx, command.CommandKey, digest[:])
		if err != nil {
			return model.Order{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return model.Order{}, false, err
		}
		return replay.Order, true, nil
	}
	if err != nil {
		return model.Order{}, false, fmt.Errorf("subscription order command: %w", err)
	}
	pending, err := lockPendingOrder(ctx, tx, command.MerchantID, command.PlanID)
	if err != nil && !errors.Is(err, model.ErrOrderNotFound) {
		return model.Order{}, false, err
	}
	if errors.Is(err, model.ErrOrderNotFound) {
		pending, err = insertPendingOrder(ctx, tx, command)
		if err != nil {
			return model.Order{}, false, err
		}
	}
	if err := completeOrderCommand(ctx, tx, command.CommandKey, command.MerchantID, storedPurchase{Order: pending}); err != nil {
		return model.Order{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Order{}, false, fmt.Errorf("subscription order commit: %w", err)
	}
	return pending, false, nil
}

func (r *OrderRepository) GetOrder(ctx context.Context, merchantID int64, orderNo string) (model.Order, error) {
	if r == nil || r.database == nil {
		return model.Order{}, model.ErrOrderInvalid
	}
	value, err := readOrder(ctx, r.database, merchantID, orderNo)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Order{}, model.ErrOrderNotFound
	}
	return value, err
}

func (r *OrderRepository) ListOrders(ctx context.Context, query model.OrderQuery) (model.OrderPage, error) {
	if r == nil || r.database == nil {
		return model.OrderPage{}, model.ErrOrderInvalid
	}
	args := []any{query.MerchantID}
	filter := `o.merchant_id=?`
	if query.Status != "" {
		filter += ` AND o.status=?`
		args = append(args, string(query.Status))
	}
	var total int64
	if err := r.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscription_order o WHERE `+filter, args...).Scan(&total); err != nil {
		return model.OrderPage{}, fmt.Errorf("subscription order count: %w", err)
	}
	offset := (query.Page - 1) * query.PageSize
	rows, err := r.database.QueryContext(ctx, orderSelect+` AND `+filter+` ORDER BY o.created_at DESC, o.order_id DESC LIMIT ? OFFSET ?`,
		append(args, query.PageSize, offset)...)
	if err != nil {
		return model.OrderPage{}, fmt.Errorf("subscription order list: %w", err)
	}
	defer rows.Close()
	items := make([]model.Order, 0)
	for rows.Next() {
		value, err := scanOrderRow(rows)
		if err != nil {
			return model.OrderPage{}, err
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.OrderPage{}, fmt.Errorf("subscription order list rows: %w", err)
	}
	return model.OrderPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *OrderRepository) AttachPayment(ctx context.Context, command model.AttachPaymentCommand) (model.Order, error) {
	if r == nil || r.database == nil {
		return model.Order{}, model.ErrOrderInvalid
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Order{}, fmt.Errorf("subscription order attach begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	current, err := lockOrder(ctx, tx, command.MerchantID, command.OrderNo)
	if err != nil {
		return model.Order{}, err
	}
	if current.Status != model.OrderPending {
		return model.Order{}, model.ErrOrderConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE subscription_order SET pay_no=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE order_no=? AND merchant_id=? AND status='PENDING'`, command.PayNo, command.OrderNo, command.MerchantID); err != nil {
		return model.Order{}, fmt.Errorf("subscription order attach: %w", err)
	}
	saved, err := readOrder(ctx, tx, command.MerchantID, command.OrderNo)
	if err != nil {
		return model.Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Order{}, err
	}
	return saved, nil
}

func (r *OrderRepository) Activate(ctx context.Context, command model.ActivateOrderCommand) (model.Order, model.Assignment, bool, error) {
	if r == nil || r.database == nil {
		return model.Order{}, model.Assignment{}, false, model.ErrOrderInvalid
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Order{}, model.Assignment{}, false, fmt.Errorf("subscription activate begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	digest := command.RequestDigest()
	_, err = tx.ExecContext(ctx, `INSERT INTO subscription_order_command(command_key,request_hash) VALUES(?,?)`, command.CommandKey, digest[:])
	if assignmentDuplicate(err) {
		replay, err := readOrderCommand(ctx, tx, command.CommandKey, digest[:])
		if err != nil {
			return model.Order{}, model.Assignment{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return model.Order{}, model.Assignment{}, false, err
		}
		return replay.Order, replay.Assignment, true, nil
	}
	if err != nil {
		return model.Order{}, model.Assignment{}, false, fmt.Errorf("subscription activate command: %w", err)
	}
	current, err := lockOrder(ctx, tx, command.MerchantID, command.OrderNo)
	if err != nil {
		return model.Order{}, model.Assignment{}, false, err
	}
	if current.Status == model.OrderPaid {
		assignment, err := readAssignment(ctx, tx, command.MerchantID)
		if err != nil && !errors.Is(err, model.ErrAssignmentNotFound) {
			return model.Order{}, model.Assignment{}, false, err
		}
		if errors.Is(err, model.ErrAssignmentNotFound) {
			assignment = model.Assignment{MerchantID: command.MerchantID}
		}
		if err := completeOrderCommand(ctx, tx, command.CommandKey, command.MerchantID, storedPurchase{Order: current, Assignment: assignment}); err != nil {
			return model.Order{}, model.Assignment{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return model.Order{}, model.Assignment{}, false, err
		}
		return current, assignment, true, nil
	}
	if current.Status != model.OrderPending {
		return model.Order{}, model.Assignment{}, false, model.ErrOrderConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE subscription_order SET status='PAID',paid_at=CURRENT_TIMESTAMP(3),version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE order_no=? AND merchant_id=? AND status='PENDING'`, command.OrderNo, command.MerchantID); err != nil {
		return model.Order{}, model.Assignment{}, false, fmt.Errorf("subscription order pay: %w", err)
	}
	existing, err := lockAssignment(ctx, tx, command.MerchantID)
	if err != nil && !errors.Is(err, model.ErrAssignmentNotFound) {
		return model.Order{}, model.Assignment{}, false, err
	}
	expires := model.RenewExpiresAt(existing, current.PlanID, current.DurationDays, command.Now)
	if errors.Is(err, model.ErrAssignmentNotFound) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO subscription_merchant_assignment(merchant_id,plan_id,expires_at,version) VALUES(?,?,?,1)`,
			command.MerchantID, current.PlanID, nullableString(expires)); err != nil {
			return model.Order{}, model.Assignment{}, false, fmt.Errorf("subscription purchase assign insert: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE subscription_merchant_assignment SET plan_id=?,expires_at=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=?`, current.PlanID, nullableString(expires), command.MerchantID); err != nil {
			return model.Order{}, model.Assignment{}, false, fmt.Errorf("subscription purchase assign update: %w", err)
		}
	}
	savedOrder, err := readOrder(ctx, tx, command.MerchantID, command.OrderNo)
	if err != nil {
		return model.Order{}, model.Assignment{}, false, err
	}
	assignment, err := readAssignment(ctx, tx, command.MerchantID)
	if err != nil {
		return model.Order{}, model.Assignment{}, false, err
	}
	if err := completeOrderCommand(ctx, tx, command.CommandKey, command.MerchantID, storedPurchase{Order: savedOrder, Assignment: assignment}); err != nil {
		return model.Order{}, model.Assignment{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Order{}, model.Assignment{}, false, fmt.Errorf("subscription activate commit: %w", err)
	}
	return savedOrder, assignment, false, nil
}

func (r *OrderRepository) Close(ctx context.Context, command model.CloseOrderCommand) (model.Order, bool, error) {
	if r == nil || r.database == nil {
		return model.Order{}, false, model.ErrOrderInvalid
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Order{}, false, fmt.Errorf("subscription close begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	digest := command.RequestDigest()
	_, err = tx.ExecContext(ctx, `INSERT INTO subscription_order_command(command_key,request_hash) VALUES(?,?)`, command.CommandKey, digest[:])
	if assignmentDuplicate(err) {
		replay, err := readOrderCommand(ctx, tx, command.CommandKey, digest[:])
		if err != nil {
			return model.Order{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return model.Order{}, false, err
		}
		return replay.Order, true, nil
	}
	if err != nil {
		return model.Order{}, false, fmt.Errorf("subscription close command: %w", err)
	}
	current, err := lockOrder(ctx, tx, command.MerchantID, command.OrderNo)
	if err != nil {
		return model.Order{}, false, err
	}
	if current.Status == model.OrderCancelled {
		if err := completeOrderCommand(ctx, tx, command.CommandKey, command.MerchantID, storedPurchase{Order: current}); err != nil {
			return model.Order{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return model.Order{}, false, err
		}
		return current, true, nil
	}
	if current.Status != model.OrderPending {
		return model.Order{}, false, model.ErrOrderConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE subscription_order SET status='CANCELLED',version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE order_no=? AND merchant_id=? AND status='PENDING'`, command.OrderNo, command.MerchantID); err != nil {
		return model.Order{}, false, fmt.Errorf("subscription order close: %w", err)
	}
	saved, err := readOrder(ctx, tx, command.MerchantID, command.OrderNo)
	if err != nil {
		return model.Order{}, false, err
	}
	if err := completeOrderCommand(ctx, tx, command.CommandKey, command.MerchantID, storedPurchase{Order: saved}); err != nil {
		return model.Order{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Order{}, false, fmt.Errorf("subscription close commit: %w", err)
	}
	return saved, false, nil
}

func insertPendingOrder(ctx context.Context, tx *sql.Tx, command model.CreateOrderCommand) (model.Order, error) {
	orderNo, err := newOrderNo()
	if err != nil {
		return model.Order{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO subscription_order
(order_no,merchant_id,plan_id,plan_code,plan_name,price_minor,duration_days,status,channel_code,version)
VALUES(?,?,?,?,?,?,?,'PENDING',?,1)`,
		orderNo, command.MerchantID, command.PlanID, command.PlanCode, command.PlanName, command.PriceMinor, command.DurationDays, command.ChannelCode); err != nil {
		if assignmentDuplicate(err) {
			pending, pendingErr := lockPendingOrder(ctx, tx, command.MerchantID, command.PlanID)
			if pendingErr != nil {
				return model.Order{}, pendingErr
			}
			return pending, nil
		}
		return model.Order{}, fmt.Errorf("subscription order insert: %w", err)
	}
	return readOrder(ctx, tx, command.MerchantID, orderNo)
}

func newOrderNo() (string, error) {
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("subscription order no: %w", err)
	}
	return fmt.Sprintf("SUB%d-%s", time.Now().UTC().UnixMilli(), hex.EncodeToString(nonce[:])), nil
}

func lockPendingOrder(ctx context.Context, tx *sql.Tx, merchantID, planID int64) (model.Order, error) {
	var orderNo string
	err := tx.QueryRowContext(ctx, `SELECT order_no FROM subscription_order WHERE merchant_id=? AND plan_id=? AND status='PENDING' FOR UPDATE`, merchantID, planID).Scan(&orderNo)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Order{}, model.ErrOrderNotFound
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("subscription pending order: %w", err)
	}
	return lockOrder(ctx, tx, merchantID, orderNo)
}

func lockOrder(ctx context.Context, tx *sql.Tx, merchantID int64, orderNo string) (model.Order, error) {
	value, err := scanOrder(tx.QueryRowContext(ctx, orderSelect+` AND o.merchant_id=? AND o.order_no=? FOR UPDATE`, merchantID, orderNo))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Order{}, model.ErrOrderNotFound
	}
	return value, err
}

func readOrder(ctx context.Context, query interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, merchantID int64, orderNo string) (model.Order, error) {
	value, err := scanOrder(query.QueryRowContext(ctx, orderSelect+` AND o.merchant_id=? AND o.order_no=?`, merchantID, orderNo))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Order{}, model.ErrOrderNotFound
	}
	return value, err
}

const orderSelect = `SELECT o.order_no,o.merchant_id,o.plan_id,o.plan_code,o.plan_name,o.price_minor,o.duration_days,o.status,o.pay_no,o.channel_code,o.version,o.paid_at,o.created_at
FROM subscription_order o WHERE 1=1`

func scanOrder(row *sql.Row) (model.Order, error) {
	return scanOrderValues(row)
}

func scanOrderRow(row *sql.Rows) (model.Order, error) {
	return scanOrderValues(row)
}

func scanOrderValues(row interface{ Scan(dest ...any) error }) (model.Order, error) {
	var value model.Order
	var paid sql.NullTime
	var created sql.NullTime
	err := row.Scan(&value.OrderNo, &value.MerchantID, &value.PlanID, &value.PlanCode, &value.PlanName, &value.PriceMinor, &value.DurationDays, &value.Status, &value.PayNo, &value.ChannelCode, &value.Version, &paid, &created)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("subscription scan order: %w", err)
		}
		return model.Order{}, err
	}
	if paid.Valid {
		value.PaidAt = paid.Time.UTC().Format(time.RFC3339Nano)
	}
	if created.Valid {
		value.CreatedAt = created.Time.UTC().Format(time.RFC3339Nano)
	}
	return value, nil
}

func completeOrderCommand(ctx context.Context, tx *sql.Tx, commandKey string, merchantID int64, value storedPurchase) error {
	document, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE subscription_order_command SET merchant_id=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`,
		merchantID, document, commandKey); err != nil {
		return fmt.Errorf("subscription order complete: %w", err)
	}
	return nil
}

func readOrderCommand(ctx context.Context, tx *sql.Tx, commandKey string, want []byte) (storedPurchase, error) {
	var stored []byte
	var document []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_json FROM subscription_order_command WHERE command_key=? FOR UPDATE`, commandKey).
		Scan(&stored, &document); err != nil {
		return storedPurchase{}, fmt.Errorf("subscription order replay: %w", err)
	}
	if len(stored) != len(want) || subtle.ConstantTimeCompare(stored, want) != 1 {
		return storedPurchase{}, model.ErrOrderIdempotency
	}
	var replay storedPurchase
	if len(document) == 0 || json.Unmarshal(document, &replay) != nil || strings.TrimSpace(replay.Order.OrderNo) == "" {
		return storedPurchase{}, fmt.Errorf("subscription order command response is incomplete")
	}
	return replay, nil
}
