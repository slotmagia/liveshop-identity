package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"
	biz "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
)

type QuotaRepository struct{ database *sql.DB }

func NewQuotaRepository(database *sql.DB) *QuotaRepository {
	return &QuotaRepository{database: database}
}

func (r *QuotaRepository) Ready(ctx context.Context) error { return r.database.PingContext(ctx) }

func (r *QuotaRepository) GetEffective(ctx context.Context, merchantID int64, code string, at time.Time) (biz.QuotaLimit, error) {
	return getEffective(ctx, r.database, merchantID, code, at)
}

func (r *QuotaRepository) GetEffectiveMany(ctx context.Context, merchantID int64, codes []string, at time.Time) ([]biz.QuotaLimit, error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, fmt.Errorf("subscription begin quota snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result := make([]biz.QuotaLimit, 0, len(codes))
	for _, code := range codes {
		quota, err := getEffective(ctx, tx, merchantID, code, at)
		if err != nil {
			return nil, err
		}
		result = append(result, quota)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("subscription commit quota snapshot: %w", err)
	}
	return result, nil
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getEffective(ctx context.Context, query queryer, merchantID int64, code string, at time.Time) (biz.QuotaLimit, error) {
	var quota biz.QuotaLimit
	var limit sql.NullInt64
	var until sql.NullTime
	err := query.QueryRowContext(ctx, `
SELECT merchant_id, quota_code, limit_value, revision, effective_from, effective_until
FROM subscription_quota_entitlement
WHERE merchant_id = ? AND quota_code = ?
  AND effective_from <= ?
  AND (effective_until IS NULL OR effective_until > ?)`, merchantID, code, at, at).
		Scan(&quota.MerchantID, &quota.Code, &limit, &quota.Revision, &quota.EffectiveFrom, &until)
	if errors.Is(err, sql.ErrNoRows) {
		return biz.QuotaLimit{}, biz.ErrNotConfigured
	}
	if err != nil {
		return biz.QuotaLimit{}, fmt.Errorf("subscription read quota: %w", err)
	}
	if limit.Valid {
		value := limit.Int64
		quota.Limit = &value
	}
	if until.Valid {
		value := until.Time.UTC()
		quota.EffectiveUntil = &value
	}
	quota.EffectiveFrom = quota.EffectiveFrom.UTC()
	return quota, nil
}

func (r *QuotaRepository) Apply(ctx context.Context, command biz.ApplyQuotaCommand) (biz.QuotaLimit, bool, error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return biz.QuotaLimit{}, false, fmt.Errorf("subscription begin quota command: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	hash := command.RequestHash()
	_, err = tx.ExecContext(ctx, `
INSERT INTO subscription_quota_command (merchant_id, command_key, request_hash, quota_code)
VALUES (?, ?, ?, ?)`, command.MerchantID, command.CommandKey, hash, command.Code)
	if duplicateKey(err) {
		var storedHash, storedCode string
		var revision int64
		if scanErr := tx.QueryRowContext(ctx, `
SELECT request_hash, quota_code, response_revision
FROM subscription_quota_command
WHERE merchant_id = ? AND command_key = ? FOR UPDATE`, command.MerchantID, command.CommandKey).
			Scan(&storedHash, &storedCode, &revision); scanErr != nil {
			return biz.QuotaLimit{}, false, fmt.Errorf("subscription read quota replay: %w", scanErr)
		}
		if storedHash != hash || storedCode != command.Code {
			return biz.QuotaLimit{}, false, biz.ErrIdempotencyConflict
		}
		quota, scanErr := getByRevision(ctx, tx, command.MerchantID, command.Code, revision)
		if scanErr != nil {
			return biz.QuotaLimit{}, false, scanErr
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return biz.QuotaLimit{}, false, fmt.Errorf("subscription commit quota replay: %w", commitErr)
		}
		return quota, true, nil
	}
	if err != nil {
		return biz.QuotaLimit{}, false, fmt.Errorf("subscription insert quota command: %w", err)
	}

	var currentRevision int64
	err = tx.QueryRowContext(ctx, `
SELECT revision FROM subscription_quota_entitlement
WHERE merchant_id = ? AND quota_code = ? FOR UPDATE`, command.MerchantID, command.Code).Scan(&currentRevision)
	switch {
	case errors.Is(err, sql.ErrNoRows) && command.ExpectedRevision == 0:
		currentRevision = 0
	case errors.Is(err, sql.ErrNoRows):
		return biz.QuotaLimit{}, false, biz.ErrVersionConflict
	case err != nil:
		return biz.QuotaLimit{}, false, fmt.Errorf("subscription lock quota: %w", err)
	case currentRevision != command.ExpectedRevision:
		return biz.QuotaLimit{}, false, biz.ErrVersionConflict
	}

	nextRevision := currentRevision + 1
	_, err = tx.ExecContext(ctx, `
INSERT INTO subscription_quota_entitlement
  (merchant_id, quota_code, limit_value, revision, effective_from, effective_until)
VALUES (?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  limit_value = VALUES(limit_value), revision = VALUES(revision),
  effective_from = VALUES(effective_from), effective_until = VALUES(effective_until)`,
		command.MerchantID, command.Code, nullableLimit(command.Limit), nextRevision,
		command.EffectiveFrom.UTC(), nullableTime(command.EffectiveUntil))
	if err != nil {
		return biz.QuotaLimit{}, false, fmt.Errorf("subscription write quota: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE subscription_quota_command SET response_revision = ?
WHERE merchant_id = ? AND command_key = ?`, nextRevision, command.MerchantID, command.CommandKey); err != nil {
		return biz.QuotaLimit{}, false, fmt.Errorf("subscription complete quota command: %w", err)
	}
	quota, err := getByRevision(ctx, tx, command.MerchantID, command.Code, nextRevision)
	if err != nil {
		return biz.QuotaLimit{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return biz.QuotaLimit{}, false, fmt.Errorf("subscription commit quota command: %w", err)
	}
	return quota, false, nil
}

func getByRevision(ctx context.Context, query queryer, merchantID int64, code string, revision int64) (biz.QuotaLimit, error) {
	var quota biz.QuotaLimit
	var limit sql.NullInt64
	var until sql.NullTime
	err := query.QueryRowContext(ctx, `
SELECT merchant_id, quota_code, limit_value, revision, effective_from, effective_until
FROM subscription_quota_entitlement
WHERE merchant_id = ? AND quota_code = ? AND revision = ?`, merchantID, code, revision).
		Scan(&quota.MerchantID, &quota.Code, &limit, &quota.Revision, &quota.EffectiveFrom, &until)
	if err != nil {
		return biz.QuotaLimit{}, fmt.Errorf("subscription read quota revision: %w", err)
	}
	if limit.Valid {
		value := limit.Int64
		quota.Limit = &value
	}
	if until.Valid {
		value := until.Time.UTC()
		quota.EffectiveUntil = &value
	}
	quota.EffectiveFrom = quota.EffectiveFrom.UTC()
	return quota, nil
}

func duplicateKey(err error) bool {
	var mysqlError *driver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

func nullableLimit(limit *int64) any {
	if limit == nil {
		return nil
	}
	return *limit
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func Open(dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("subscription database DSN is required")
	}
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(20)
	database.SetMaxIdleConns(5)
	database.SetConnMaxLifetime(30 * time.Minute)
	return database, nil
}
