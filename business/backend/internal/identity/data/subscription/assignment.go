package mysql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"
	subscription "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
	model "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
)

type AssignmentRepository struct{ database *sql.DB }

func NewAssignmentRepository(database *sql.DB) *AssignmentRepository {
	return &AssignmentRepository{database: database}
}

var _ subscription.AssignmentRepository = (*AssignmentRepository)(nil)

func (r *AssignmentRepository) GetAssignment(ctx context.Context, merchantID int64) (model.Assignment, error) {
	if r == nil || r.database == nil {
		return model.Assignment{}, model.ErrPlanInvalid
	}
	value, err := readAssignment(ctx, r.database, merchantID)
	if errors.Is(err, model.ErrAssignmentNotFound) {
		return model.Assignment{MerchantID: merchantID}, nil
	}
	return value, err
}

func (r *AssignmentRepository) Assign(ctx context.Context, command model.AssignCommand) (model.Assignment, bool, error) {
	if r == nil || r.database == nil {
		return model.Assignment{}, false, model.ErrPlanInvalid
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Assignment{}, false, fmt.Errorf("subscription assignment begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO subscription_merchant_assignment_command(command_key,request_hash) VALUES(?,?)`, command.CommandKey, digest(command))
	if assignmentDuplicate(err) {
		var stored []byte
		var document []byte
		if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_json FROM subscription_merchant_assignment_command WHERE command_key=? FOR UPDATE`, command.CommandKey).
			Scan(&stored, &document); err != nil {
			return model.Assignment{}, false, fmt.Errorf("subscription assignment replay: %w", err)
		}
		want := digest(command)
		if len(stored) != len(want) || subtle.ConstantTimeCompare(stored, want) != 1 {
			return model.Assignment{}, false, model.ErrAssignmentIdempotency
		}
		var replay model.Assignment
		if len(document) == 0 || json.Unmarshal(document, &replay) != nil || replay.MerchantID <= 0 {
			return model.Assignment{}, false, fmt.Errorf("subscription assignment command response is incomplete")
		}
		if err := tx.Commit(); err != nil {
			return model.Assignment{}, false, err
		}
		return replay, true, nil
	}
	if err != nil {
		return model.Assignment{}, false, fmt.Errorf("subscription assignment command: %w", err)
	}
	var planCode, planName string
	var durationDays int
	if err := tx.QueryRowContext(ctx, `SELECT code,name,duration_days FROM subscription_plan WHERE plan_id=? AND status='ACTIVE'`, command.PlanID).
		Scan(&planCode, &planName, &durationDays); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Assignment{}, false, model.ErrPlanNotFound
		}
		return model.Assignment{}, false, fmt.Errorf("subscription assignment plan: %w", err)
	}
	current, err := lockAssignment(ctx, tx, command.MerchantID)
	if err != nil && !errors.Is(err, model.ErrAssignmentNotFound) {
		return model.Assignment{}, false, err
	}
	if errors.Is(err, model.ErrAssignmentNotFound) {
		if command.ExpectedVersion != 0 {
			return model.Assignment{}, false, model.ErrAssignmentConflict
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO subscription_merchant_assignment(merchant_id,plan_id,expires_at,version) VALUES(?,?,?,1)`,
			command.MerchantID, command.PlanID, nullableString(command.ExpiresAt)); err != nil {
			return model.Assignment{}, false, fmt.Errorf("subscription assignment insert: %w", err)
		}
	} else {
		if current.Version != command.ExpectedVersion {
			return model.Assignment{}, false, model.ErrAssignmentConflict
		}
		if _, err := tx.ExecContext(ctx, `UPDATE subscription_merchant_assignment SET plan_id=?,expires_at=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=? AND version=?`, command.PlanID, nullableString(command.ExpiresAt), command.MerchantID, command.ExpectedVersion); err != nil {
			return model.Assignment{}, false, fmt.Errorf("subscription assignment update: %w", err)
		}
	}
	saved, err := readAssignment(ctx, tx, command.MerchantID)
	if err != nil {
		return model.Assignment{}, false, err
	}
	document, err := json.Marshal(saved)
	if err != nil {
		return model.Assignment{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE subscription_merchant_assignment_command SET merchant_id=?,response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`,
		saved.MerchantID, saved.Version, document, command.CommandKey); err != nil {
		return model.Assignment{}, false, fmt.Errorf("subscription assignment complete: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Assignment{}, false, fmt.Errorf("subscription assignment commit: %w", err)
	}
	return saved, false, nil
}

func digest(command model.AssignCommand) []byte {
	value := command.RequestDigest()
	return value[:]
}

func nullableString(value *string) any {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}

func lockAssignment(ctx context.Context, tx *sql.Tx, merchantID int64) (model.Assignment, error) {
	var value model.Assignment
	var expires sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT a.merchant_id,a.plan_id,p.code,p.name,a.expires_at,a.version
FROM subscription_merchant_assignment a JOIN subscription_plan p ON p.plan_id=a.plan_id
WHERE a.merchant_id=? FOR UPDATE`, merchantID).
		Scan(&value.MerchantID, &value.PlanID, &value.PlanCode, &value.PlanName, &expires, &value.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Assignment{}, model.ErrAssignmentNotFound
	}
	if err != nil {
		return model.Assignment{}, fmt.Errorf("subscription assignment lock: %w", err)
	}
	if expires.Valid {
		value.ExpiresAt = expires.String
	}
	return value, nil
}

func readAssignment(ctx context.Context, query interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, merchantID int64) (model.Assignment, error) {
	var value model.Assignment
	var expires sql.NullString
	err := query.QueryRowContext(ctx, `SELECT a.merchant_id,a.plan_id,p.code,p.name,a.expires_at,a.version
FROM subscription_merchant_assignment a JOIN subscription_plan p ON p.plan_id=a.plan_id
WHERE a.merchant_id=?`, merchantID).
		Scan(&value.MerchantID, &value.PlanID, &value.PlanCode, &value.PlanName, &expires, &value.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Assignment{}, model.ErrAssignmentNotFound
	}
	if err != nil {
		return model.Assignment{}, fmt.Errorf("subscription assignment read: %w", err)
	}
	if expires.Valid {
		value.ExpiresAt = expires.String
	}
	return value, nil
}

func assignmentDuplicate(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
