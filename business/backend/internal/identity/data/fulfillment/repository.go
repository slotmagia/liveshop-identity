package mysql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
)

type Repository struct{ database *sql.DB }

var _ fulfillment.Repository = (*Repository)(nil)
var _ fulfillment.AftersaleRepository = (*Repository)(nil)

func NewRepository(database *sql.DB) *Repository { return &Repository{database: database} }

const complaintSelect = `complaint_id,merchant_id,shop_id,customer_subject,target_type,target_id,reason_code,content,status,handle_note,version,created_at,updated_at,handled_at`

func (r *Repository) List(ctx context.Context, query model.Query) (model.Page, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.Page{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureShopScope(ctx, tx, query.MerchantID, query.ShopID, false); err != nil {
		return model.Page{}, err
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
	if query.TargetType != "" {
		where += " AND target_type=?"
		args = append(args, query.TargetType)
	}
	var total int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM identity_complaint WHERE "+where, args...).Scan(&total); err != nil {
		return model.Page{}, fmt.Errorf("complaint count: %w", err)
	}
	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := tx.QueryContext(ctx, `SELECT `+complaintSelect+` FROM identity_complaint WHERE `+where+` ORDER BY created_at DESC, complaint_id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return model.Page{}, fmt.Errorf("complaint list: %w", err)
	}
	defer rows.Close()
	items := []model.Complaint{}
	for rows.Next() {
		value, err := scanComplaint(rows)
		if err != nil {
			return model.Page{}, fmt.Errorf("complaint list scan: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.Page{}, fmt.Errorf("complaint list iterate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Page{}, fmt.Errorf("complaint list commit: %w", err)
	}
	return model.Page{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *Repository) Get(ctx context.Context, merchantID, shopID, complaintID int64) (model.Complaint, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.Complaint{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureShopScope(ctx, tx, merchantID, shopID, false); err != nil {
		return model.Complaint{}, err
	}
	value, err := readComplaint(ctx, tx, complaintID, merchantID, shopID)
	if err != nil {
		return model.Complaint{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Complaint{}, fmt.Errorf("complaint get commit: %w", err)
	}
	return value, nil
}

func (r *Repository) Review(ctx context.Context, command model.ReviewCommand) (model.Complaint, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Complaint{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayComplaintCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Complaint{}, false, err
	}
	if found {
		return commitComplaintReplay(tx, replay)
	}
	if err := ensureShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Complaint{}, false, err
	}
	current, err := readComplaintForUpdate(ctx, tx, command.ComplaintID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Complaint{}, false, err
	}
	if current.Version != command.ExpectedVersion || current.Status != model.StatusOpen {
		return model.Complaint{}, false, model.ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_complaint
SET status=?,handle_note=?,handled_at=CURRENT_TIMESTAMP(3),version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE complaint_id=? AND merchant_id=? AND shop_id=? AND version=? AND status=?`,
		command.Status, command.HandleNote, current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion, model.StatusOpen)
	if err != nil {
		return model.Complaint{}, false, fmt.Errorf("complaint review: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Complaint{}, false, model.ErrConflict
	}
	saved, err := readComplaint(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.Complaint{}, false, err
	}
	return completeAndCommitComplaint(ctx, tx, command.CommandKey, saved)
}

func (r *Repository) begin(ctx context.Context, readOnly bool) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: readOnly})
	if err != nil {
		return nil, fmt.Errorf("complaint begin: %w", err)
	}
	return tx, nil
}

func ensureShopScope(ctx context.Context, tx *sql.Tx, merchantID, shopID int64, lock bool) error {
	query := `SELECT merchant_id,shop_id FROM identity_shop WHERE merchant_id=? AND shop_id=? AND status<>'CLOSED'`
	if lock {
		query += " FOR UPDATE"
	}
	var storedMerchant, storedShop int64
	err := tx.QueryRowContext(ctx, query, merchantID, shopID).Scan(&storedMerchant, &storedShop)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("complaint shop scope: %w", err)
	}
	return nil
}

type scanner interface{ Scan(...any) error }

func scanComplaint(row scanner) (model.Complaint, error) {
	var value model.Complaint
	var targetType, status string
	var handledAt sql.NullTime
	err := row.Scan(&value.ID, &value.MerchantID, &value.ShopID, &value.CustomerSubject, &targetType, &value.TargetID,
		&value.ReasonCode, &value.Content, &status, &value.HandleNote, &value.Version, &value.CreatedAt, &value.UpdatedAt, &handledAt)
	value.TargetType = model.TargetType(targetType)
	value.Status = model.Status(status)
	if handledAt.Valid {
		moment := handledAt.Time
		value.HandledAt = &moment
	}
	return value, err
}

func readComplaint(ctx context.Context, tx *sql.Tx, complaintID, merchantID, shopID int64) (model.Complaint, error) {
	value, err := scanComplaint(tx.QueryRowContext(ctx, `SELECT `+complaintSelect+` FROM identity_complaint WHERE complaint_id=? AND merchant_id=? AND shop_id=?`, complaintID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Complaint{}, model.ErrNotFound
	}
	if err != nil {
		return model.Complaint{}, fmt.Errorf("complaint read: %w", err)
	}
	return value, nil
}

func readComplaintForUpdate(ctx context.Context, tx *sql.Tx, complaintID, merchantID, shopID int64) (model.Complaint, error) {
	value, err := scanComplaint(tx.QueryRowContext(ctx, `SELECT `+complaintSelect+` FROM identity_complaint WHERE complaint_id=? AND merchant_id=? AND shop_id=? FOR UPDATE`, complaintID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Complaint{}, model.ErrNotFound
	}
	if err != nil {
		return model.Complaint{}, fmt.Errorf("complaint lock: %w", err)
	}
	return value, nil
}

func insertOrReplayComplaintCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) (model.Complaint, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_complaint_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !complaintDuplicateKey(err) {
		if err != nil {
			return model.Complaint{}, false, fmt.Errorf("complaint insert command: %w", err)
		}
		return model.Complaint{}, false, nil
	}
	var stored []byte
	var responseVersion uint64
	var document []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_complaint_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &responseVersion, &document); err != nil {
		return model.Complaint{}, false, fmt.Errorf("complaint read command replay: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return model.Complaint{}, false, model.ErrIdempotency
	}
	var replay model.Complaint
	if responseVersion == 0 || len(document) == 0 || json.Unmarshal(document, &replay) != nil || replay.ID <= 0 || replay.Version != responseVersion {
		return model.Complaint{}, false, fmt.Errorf("complaint command response is incomplete")
	}
	return replay, true, nil
}

func completeAndCommitComplaint(ctx context.Context, tx *sql.Tx, key string, value model.Complaint) (model.Complaint, bool, error) {
	document, err := json.Marshal(value)
	if err != nil {
		return model.Complaint{}, false, fmt.Errorf("complaint encode review response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_complaint_command SET complaint_id=?,response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, value.ID, value.Version, document, key)
	if err != nil {
		return model.Complaint{}, false, fmt.Errorf("complaint complete review command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Complaint{}, false, fmt.Errorf("complaint review command completion affected no row")
	}
	if err := tx.Commit(); err != nil {
		return model.Complaint{}, false, fmt.Errorf("complaint commit review: %w", err)
	}
	return value, false, nil
}

func commitComplaintReplay(tx *sql.Tx, value model.Complaint) (model.Complaint, bool, error) {
	if err := tx.Commit(); err != nil {
		return model.Complaint{}, false, fmt.Errorf("complaint commit review replay: %w", err)
	}
	return value, true, nil
}

func complaintDuplicateKey(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
