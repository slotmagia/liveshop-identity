package mysql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/auth"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/auth/model"
)

type Repository struct{ database *sql.DB }

var _ auth.Repository = (*Repository)(nil)

func NewRepository(database *sql.DB) *Repository { return &Repository{database: database} }

func (r *Repository) CreatePending(ctx context.Context, record model.Record) (model.Record, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Record{}, err
	}
	defer func() { _ = tx.Rollback() }()
	err = tx.QueryRowContext(ctx, `SELECT s.merchant_id,s.shop_id,s.code
FROM identity_shop s
JOIN identity_merchant m ON m.merchant_id=s.merchant_id AND m.status='ACTIVE'
WHERE s.code=? AND s.status='ACTIVE' FOR UPDATE`, record.ShopCode).
		Scan(&record.MerchantID, &record.ShopID, &record.ShopCode)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Record{}, model.ErrInvalid
	}
	if err != nil {
		return model.Record{}, fmt.Errorf("auth otp resolve shop: %w", err)
	}
	var lastCreated time.Time
	err = tx.QueryRowContext(ctx, `SELECT created_at FROM identity_auth_otp_challenge
WHERE shop_id=? AND phone=? AND email=?
ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, record.ShopID, record.Phone, record.Email).Scan(&lastCreated)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.Record{}, fmt.Errorf("auth otp last send: %w", err)
	}
	if err == nil {
		if remaining := model.RemainingResendSeconds(lastCreated, record.CreatedAt); remaining > 0 {
			return model.Record{}, &model.ResendCooldownError{
				ResendAfterSeconds: remaining,
				NextSendAt:         model.NextSendAt(lastCreated),
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_auth_otp_challenge
SET status='EXPIRED' WHERE shop_id=? AND phone=? AND email=? AND status='PENDING'`, record.ShopID, record.Phone, record.Email); err != nil {
		return model.Record{}, fmt.Errorf("auth otp expire previous: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_auth_otp_challenge
(challenge_id,merchant_id,shop_id,shop_code,phone,email,code_hash,ttl_seconds,status,attempt_count,expires_at,created_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		record.ID, record.MerchantID, record.ShopID, record.ShopCode, record.Phone, record.Email, record.CodeHash,
		record.TTLSeconds, model.StatusPending, 0, record.ExpiresAt, record.CreatedAt); err != nil {
		return model.Record{}, fmt.Errorf("auth otp insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Record{}, fmt.Errorf("auth otp create commit: %w", err)
	}
	record.Status = model.StatusPending
	return record, nil
}

func (r *Repository) Consume(ctx context.Context, command model.VerifyCommand, codeHash string, now time.Time) error {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var record model.Record
	var status string
	err = tx.QueryRowContext(ctx, `SELECT challenge_id,merchant_id,shop_id,shop_code,phone,email,code_hash,ttl_seconds,status,attempt_count,expires_at,created_at
FROM identity_auth_otp_challenge WHERE challenge_id=? FOR UPDATE`, command.ChallengeID).
		Scan(&record.ID, &record.MerchantID, &record.ShopID, &record.ShopCode, &record.Phone, &record.Email,
			&record.CodeHash, &record.TTLSeconds, &status, &record.AttemptCount, &record.ExpiresAt, &record.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("auth otp lock: %w", err)
	}
	record.Status = status
	if record.ShopCode != command.ShopCode {
		return model.ErrInvalid
	}
	if record.Status != model.StatusPending {
		if record.Status == model.StatusExpired || !record.ExpiresAt.After(now) {
			return model.ErrExpired
		}
		return model.ErrInvalid
	}
	if !record.ExpiresAt.After(now) {
		if _, err := tx.ExecContext(ctx, `UPDATE identity_auth_otp_challenge SET status='EXPIRED' WHERE challenge_id=?`, record.ID); err != nil {
			return fmt.Errorf("auth otp expire: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("auth otp expire commit: %w", err)
		}
		return model.ErrExpired
	}
	if subtle.ConstantTimeCompare([]byte(record.CodeHash), []byte(codeHash)) != 1 {
		next := record.AttemptCount + 1
		nextStatus := model.StatusPending
		if next >= model.MaxAttempts {
			nextStatus = model.StatusExpired
		}
		if _, err := tx.ExecContext(ctx, `UPDATE identity_auth_otp_challenge SET attempt_count=?,status=? WHERE challenge_id=?`, next, nextStatus, record.ID); err != nil {
			return fmt.Errorf("auth otp attempt: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("auth otp attempt commit: %w", err)
		}
		if nextStatus == model.StatusExpired {
			return model.ErrExpired
		}
		return model.ErrInvalid
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_auth_otp_challenge
SET status='CONSUMED',consumed_at=? WHERE challenge_id=? AND status='PENDING'`, now, record.ID)
	if err != nil {
		return fmt.Errorf("auth otp consume: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.ErrInvalid
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("auth otp consume commit: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, challengeID string) (model.Record, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.Record{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var record model.Record
	var status string
	err = tx.QueryRowContext(ctx, `SELECT challenge_id,merchant_id,shop_id,shop_code,phone,email,code_hash,ttl_seconds,status,attempt_count,expires_at,created_at
FROM identity_auth_otp_challenge WHERE challenge_id=?`, challengeID).
		Scan(&record.ID, &record.MerchantID, &record.ShopID, &record.ShopCode, &record.Phone, &record.Email,
			&record.CodeHash, &record.TTLSeconds, &status, &record.AttemptCount, &record.ExpiresAt, &record.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Record{}, model.ErrNotFound
	}
	if err != nil {
		return model.Record{}, fmt.Errorf("auth otp get: %w", err)
	}
	record.Status = status
	if err := tx.Commit(); err != nil {
		return model.Record{}, fmt.Errorf("auth otp get commit: %w", err)
	}
	return record, nil
}

func (r *Repository) begin(ctx context.Context, readOnly bool) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: readOnly})
	if err != nil {
		return nil, fmt.Errorf("auth otp begin: %w", err)
	}
	return tx, nil
}
