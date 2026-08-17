package mysql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	subscription "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
	model "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
)

type PlanRepository struct{ database *sql.DB }

func NewPlanRepository(database *sql.DB) *PlanRepository { return &PlanRepository{database: database} }

var _ subscription.PlanRepository = (*PlanRepository)(nil)

func (r *PlanRepository) ListPlans(ctx context.Context) ([]model.Plan, error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, fmt.Errorf("subscription begin plan list: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT plan_id,code,name,level,price_minor,duration_days,description,is_default,sort_order,status,version
FROM subscription_plan WHERE status<>'RETIRED' ORDER BY sort_order,level,plan_id`)
	if err != nil {
		return nil, fmt.Errorf("subscription list plans: %w", err)
	}
	defer rows.Close()
	plans := []model.Plan{}
	for rows.Next() {
		plan, err := scanPlan(rows)
		if err != nil {
			return nil, fmt.Errorf("subscription scan plan: %w", err)
		}
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("subscription iterate plans: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("subscription commit plan list: %w", err)
	}
	return plans, nil
}

func (r *PlanRepository) SavePlan(ctx context.Context, command model.SavePlanCommand) (model.Plan, bool, error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return model.Plan{}, false, fmt.Errorf("subscription begin save plan: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockPlanGuard(ctx, tx); err != nil {
		return model.Plan{}, false, err
	}
	digest := command.RequestDigest()
	replay, found, err := insertOrReplayPlanCommand(ctx, tx, command.CommandKey, digest)
	if err != nil {
		return model.Plan{}, false, err
	}
	if found {
		if err := tx.Commit(); err != nil {
			return model.Plan{}, false, fmt.Errorf("subscription commit plan replay: %w", err)
		}
		return replay, true, nil
	}
	plan := command.Plan
	var existing model.Plan
	if plan.ID > 0 {
		existing, err = readPlanForUpdate(ctx, tx, plan.ID)
		if err != nil {
			return model.Plan{}, false, err
		}
		if existing.Version != command.ExpectedVersion || existing.Status == model.PlanRetired || existing.Code != plan.Code {
			return model.Plan{}, false, model.ErrPlanConflict
		}
		if existing.Default && (!plan.Default || plan.Status != model.PlanActive) {
			return model.Plan{}, false, model.ErrPlanDefaultRequired
		}
	}
	if plan.Default {
		if _, err := tx.ExecContext(ctx, `UPDATE subscription_plan SET is_default=0,updated_at=CURRENT_TIMESTAMP(3)
WHERE is_default=1 AND plan_id<>?`, plan.ID); err != nil {
			return model.Plan{}, false, fmt.Errorf("subscription clear previous default plan: %w", err)
		}
	}
	if plan.ID == 0 {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscription_plan WHERE status<>'RETIRED'`).Scan(&count); err != nil {
			return model.Plan{}, false, fmt.Errorf("subscription count plans: %w", err)
		}
		if count == 0 && !plan.Default {
			return model.Plan{}, false, model.ErrPlanDefaultRequired
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO subscription_plan
  (code,name,level,price_minor,duration_days,description,is_default,sort_order,status,version,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,1,CURRENT_TIMESTAMP(3))`, plan.Code, plan.Name, plan.Level, plan.PriceMinor, plan.DurationDays, plan.Description, plan.Default, plan.Sort, plan.Status)
		if duplicateKey(err) {
			return model.Plan{}, false, model.ErrPlanConflict
		}
		if err != nil {
			return model.Plan{}, false, fmt.Errorf("subscription insert plan: %w", err)
		}
		plan.ID, err = result.LastInsertId()
		if err != nil {
			return model.Plan{}, false, fmt.Errorf("subscription read inserted plan id: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO subscription_plan_quota(plan_id,quota_code,limit_value) VALUES(?,?,NULL)`, plan.ID, model.CatalogProductsQuota); err != nil {
			return model.Plan{}, false, fmt.Errorf("subscription initialize plan policy quota: %w", err)
		}
	} else {
		result, err := tx.ExecContext(ctx, `UPDATE subscription_plan SET
name=?,level=?,price_minor=?,duration_days=?,description=?,is_default=?,sort_order=?,status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE plan_id=? AND version=? AND status<>'RETIRED'`, plan.Name, plan.Level, plan.PriceMinor, plan.DurationDays, plan.Description, plan.Default, plan.Sort, plan.Status, plan.ID, command.ExpectedVersion)
		if err != nil {
			return model.Plan{}, false, fmt.Errorf("subscription update plan: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return model.Plan{}, false, model.ErrPlanConflict
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE subscription_plan_guard SET version=version+1 WHERE singleton_id=1`); err != nil {
		return model.Plan{}, false, fmt.Errorf("subscription advance plan guard: %w", err)
	}
	saved, err := readPlan(ctx, tx, plan.ID)
	if err != nil {
		return model.Plan{}, false, err
	}
	if err := completePlanCommand(ctx, tx, command.CommandKey, saved); err != nil {
		return model.Plan{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Plan{}, false, fmt.Errorf("subscription commit save plan: %w", err)
	}
	return saved, false, nil
}

func (r *PlanRepository) RetirePlan(ctx context.Context, command model.RetirePlanCommand) (model.Plan, bool, error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return model.Plan{}, false, fmt.Errorf("subscription begin retire plan: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockPlanGuard(ctx, tx); err != nil {
		return model.Plan{}, false, err
	}
	digest := command.RequestDigest()
	replay, found, err := insertOrReplayPlanCommand(ctx, tx, command.CommandKey, digest)
	if err != nil {
		return model.Plan{}, false, err
	}
	if found {
		if err := tx.Commit(); err != nil {
			return model.Plan{}, false, fmt.Errorf("subscription commit retire replay: %w", err)
		}
		return replay, true, nil
	}
	plan, err := readPlanForUpdate(ctx, tx, command.PlanID)
	if err != nil {
		return model.Plan{}, false, err
	}
	if plan.Version != command.ExpectedVersion || plan.Status == model.PlanRetired {
		return model.Plan{}, false, model.ErrPlanConflict
	}
	if plan.Default {
		return model.Plan{}, false, model.ErrPlanDefaultRequired
	}
	result, err := tx.ExecContext(ctx, `UPDATE subscription_plan SET status='RETIRED',is_default=0,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE plan_id=? AND version=? AND status<>'RETIRED'`, plan.ID, command.ExpectedVersion)
	if err != nil {
		return model.Plan{}, false, fmt.Errorf("subscription retire plan: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Plan{}, false, model.ErrPlanConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE subscription_plan_guard SET version=version+1 WHERE singleton_id=1`); err != nil {
		return model.Plan{}, false, fmt.Errorf("subscription advance plan guard: %w", err)
	}
	retired, err := readPlan(ctx, tx, plan.ID)
	if err != nil {
		return model.Plan{}, false, err
	}
	if err := completePlanCommand(ctx, tx, command.CommandKey, retired); err != nil {
		return model.Plan{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Plan{}, false, fmt.Errorf("subscription commit retire plan: %w", err)
	}
	return retired, false, nil
}

func (r *PlanRepository) GetPlanPolicy(ctx context.Context, planID int64) (model.PlanPolicy, error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return model.PlanPolicy{}, fmt.Errorf("subscription begin plan policy read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	policy, err := readPlanPolicy(ctx, tx, planID, false)
	if err != nil {
		return model.PlanPolicy{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.PlanPolicy{}, fmt.Errorf("subscription commit plan policy read: %w", err)
	}
	return policy, nil
}

func (r *PlanRepository) SavePlanPolicy(ctx context.Context, command model.SavePlanPolicyCommand) (model.PlanPolicy, bool, error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return model.PlanPolicy{}, false, fmt.Errorf("subscription begin save plan policy: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	digest := command.RequestDigest()
	replay, found, err := insertOrReplayPlanPolicyCommand(ctx, tx, command.CommandKey, digest)
	if err != nil {
		return model.PlanPolicy{}, false, err
	}
	if found {
		if err := tx.Commit(); err != nil {
			return model.PlanPolicy{}, false, fmt.Errorf("subscription commit plan policy replay: %w", err)
		}
		return replay, true, nil
	}
	current, err := readPlanPolicy(ctx, tx, command.Policy.PlanID, true)
	if err != nil {
		return model.PlanPolicy{}, false, err
	}
	if current.Revision != command.ExpectedRevision {
		return model.PlanPolicy{}, false, model.ErrPlanConflict
	}
	if err := validateActivePermissions(ctx, tx, command.Policy.PermissionCodes); err != nil {
		return model.PlanPolicy{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM subscription_plan_permission WHERE plan_id=?`, command.Policy.PlanID); err != nil {
		return model.PlanPolicy{}, false, fmt.Errorf("subscription clear plan permissions: %w", err)
	}
	for _, code := range command.Policy.PermissionCodes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO subscription_plan_permission(plan_id,permission_code) VALUES(?,?)`, command.Policy.PlanID, code); err != nil {
			return model.PlanPolicy{}, false, fmt.Errorf("subscription insert plan permission %s: %w", code, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO subscription_plan_quota(plan_id,quota_code,limit_value) VALUES(?,?,?)
ON DUPLICATE KEY UPDATE limit_value=VALUES(limit_value)`, command.Policy.PlanID, model.CatalogProductsQuota, command.Policy.ProductLimit); err != nil {
		return model.PlanPolicy{}, false, fmt.Errorf("subscription write plan policy quota: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE subscription_plan SET policy_revision=policy_revision+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE plan_id=? AND policy_revision=? AND status<>'RETIRED'`, command.Policy.PlanID, command.ExpectedRevision)
	if err != nil {
		return model.PlanPolicy{}, false, fmt.Errorf("subscription advance plan policy revision: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.PlanPolicy{}, false, model.ErrPlanConflict
	}
	saved, err := readPlanPolicy(ctx, tx, command.Policy.PlanID, false)
	if err != nil {
		return model.PlanPolicy{}, false, err
	}
	if err := completePlanPolicyCommand(ctx, tx, command.CommandKey, saved); err != nil {
		return model.PlanPolicy{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.PlanPolicy{}, false, fmt.Errorf("subscription commit save plan policy: %w", err)
	}
	return saved, false, nil
}

type planScanner interface{ Scan(...any) error }

func scanPlan(row planScanner) (model.Plan, error) {
	var plan model.Plan
	err := row.Scan(&plan.ID, &plan.Code, &plan.Name, &plan.Level, &plan.PriceMinor, &plan.DurationDays, &plan.Description, &plan.Default, &plan.Sort, &plan.Status, &plan.Version)
	return plan, err
}

func readPlan(ctx context.Context, query *sql.Tx, planID int64) (model.Plan, error) {
	plan, err := scanPlan(query.QueryRowContext(ctx, `SELECT plan_id,code,name,level,price_minor,duration_days,description,is_default,sort_order,status,version
FROM subscription_plan WHERE plan_id=?`, planID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Plan{}, model.ErrPlanNotFound
	}
	if err != nil {
		return model.Plan{}, fmt.Errorf("subscription read plan: %w", err)
	}
	return plan, nil
}

func readPlanForUpdate(ctx context.Context, tx *sql.Tx, planID int64) (model.Plan, error) {
	plan, err := scanPlan(tx.QueryRowContext(ctx, `SELECT plan_id,code,name,level,price_minor,duration_days,description,is_default,sort_order,status,version
FROM subscription_plan WHERE plan_id=? FOR UPDATE`, planID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Plan{}, model.ErrPlanNotFound
	}
	if err != nil {
		return model.Plan{}, fmt.Errorf("subscription lock plan: %w", err)
	}
	return plan, nil
}

func readPlanPolicy(ctx context.Context, query *sql.Tx, planID int64, lock bool) (model.PlanPolicy, error) {
	statement := `SELECT plan_id,code,name,policy_revision FROM subscription_plan WHERE plan_id=? AND status<>'RETIRED'`
	if lock {
		statement += ` FOR UPDATE`
	}
	var policy model.PlanPolicy
	if err := query.QueryRowContext(ctx, statement, planID).Scan(&policy.PlanID, &policy.PlanCode, &policy.PlanName, &policy.Revision); errors.Is(err, sql.ErrNoRows) {
		return model.PlanPolicy{}, model.ErrPlanNotFound
	} else if err != nil {
		return model.PlanPolicy{}, fmt.Errorf("subscription read plan policy: %w", err)
	}
	rows, err := query.QueryContext(ctx, `SELECT permission_code FROM subscription_plan_permission WHERE plan_id=? ORDER BY permission_code`, planID)
	if err != nil {
		return model.PlanPolicy{}, fmt.Errorf("subscription read plan permissions: %w", err)
	}
	policy.PermissionCodes = []string{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			_ = rows.Close()
			return model.PlanPolicy{}, fmt.Errorf("subscription scan plan permission: %w", err)
		}
		policy.PermissionCodes = append(policy.PermissionCodes, code)
	}
	if err := rows.Close(); err != nil {
		return model.PlanPolicy{}, fmt.Errorf("subscription close plan permissions: %w", err)
	}
	var limit sql.NullInt64
	err = query.QueryRowContext(ctx, `SELECT limit_value FROM subscription_plan_quota WHERE plan_id=? AND quota_code=?`, planID, model.CatalogProductsQuota).Scan(&limit)
	if errors.Is(err, sql.ErrNoRows) {
		return model.PlanPolicy{}, fmt.Errorf("subscription plan product quota is missing")
	}
	if err != nil {
		return model.PlanPolicy{}, fmt.Errorf("subscription read plan quota: %w", err)
	}
	if limit.Valid {
		value := limit.Int64
		policy.ProductLimit = &value
	} else {
		policy.ProductLimit = nil
	}
	return policy, nil
}

func lockPlanGuard(ctx context.Context, tx *sql.Tx) error {
	var version uint64
	if err := tx.QueryRowContext(ctx, `SELECT version FROM subscription_plan_guard WHERE singleton_id=1 FOR UPDATE`).Scan(&version); err != nil {
		return fmt.Errorf("subscription lock plan guard: %w", err)
	}
	return nil
}

func validateActivePermissions(ctx context.Context, tx *sql.Tx, codes []string) error {
	if len(codes) == 0 {
		return nil
	}
	markers := strings.TrimSuffix(strings.Repeat("?,", len(codes)), ",")
	args := make([]any, len(codes))
	for index, code := range codes {
		args[index] = code
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_permission_projection WHERE active=1 AND permission_code IN (`+markers+`)`, args...).Scan(&count); err != nil {
		return fmt.Errorf("subscription validate active permissions: %w", err)
	}
	if count != len(codes) {
		return model.ErrPlanPermissionInactive
	}
	return nil
}

func insertOrReplayPlanCommand(ctx context.Context, tx *sql.Tx, commandKey string, requestDigest [32]byte) (model.Plan, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO subscription_plan_command(command_key,request_hash) VALUES(?,?)`, commandKey, requestDigest[:])
	if !duplicateKey(err) {
		if err != nil {
			return model.Plan{}, false, fmt.Errorf("subscription insert plan command: %w", err)
		}
		return model.Plan{}, false, nil
	}
	var storedDigest []byte
	var responseVersion uint64
	var responseJSON []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM subscription_plan_command WHERE command_key=? FOR UPDATE`, commandKey).Scan(&storedDigest, &responseVersion, &responseJSON); err != nil {
		return model.Plan{}, false, fmt.Errorf("subscription read plan command replay: %w", err)
	}
	if len(storedDigest) != len(requestDigest) || subtle.ConstantTimeCompare(storedDigest, requestDigest[:]) != 1 {
		return model.Plan{}, false, model.ErrPlanIdempotency
	}
	if responseVersion == 0 || len(responseJSON) == 0 {
		return model.Plan{}, false, fmt.Errorf("subscription plan command is incomplete")
	}
	var replay model.Plan
	if err := json.Unmarshal(responseJSON, &replay); err != nil || replay.ID <= 0 || replay.Version != responseVersion {
		return model.Plan{}, false, fmt.Errorf("subscription plan command response is invalid")
	}
	return replay, true, nil
}

func completePlanCommand(ctx context.Context, tx *sql.Tx, commandKey string, plan model.Plan) error {
	document, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("subscription encode plan command response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE subscription_plan_command SET plan_id=?,response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, plan.ID, plan.Version, document, commandKey)
	if err != nil {
		return fmt.Errorf("subscription complete plan command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("subscription plan command completion affected no row")
	}
	return nil
}

func insertOrReplayPlanPolicyCommand(ctx context.Context, tx *sql.Tx, commandKey string, requestDigest [32]byte) (model.PlanPolicy, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO subscription_plan_policy_command(command_key,request_hash) VALUES(?,?)`, commandKey, requestDigest[:])
	if !duplicateKey(err) {
		if err != nil {
			return model.PlanPolicy{}, false, fmt.Errorf("subscription insert plan policy command: %w", err)
		}
		return model.PlanPolicy{}, false, nil
	}
	var storedDigest []byte
	var responseRevision uint64
	var responseJSON []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_revision,response_json FROM subscription_plan_policy_command WHERE command_key=? FOR UPDATE`, commandKey).Scan(&storedDigest, &responseRevision, &responseJSON); err != nil {
		return model.PlanPolicy{}, false, fmt.Errorf("subscription read plan policy command replay: %w", err)
	}
	if len(storedDigest) != len(requestDigest) || subtle.ConstantTimeCompare(storedDigest, requestDigest[:]) != 1 {
		return model.PlanPolicy{}, false, model.ErrPlanIdempotency
	}
	if responseRevision == 0 || len(responseJSON) == 0 {
		return model.PlanPolicy{}, false, fmt.Errorf("subscription plan policy command is incomplete")
	}
	var replay model.PlanPolicy
	if err := json.Unmarshal(responseJSON, &replay); err != nil || replay.PlanID <= 0 || replay.Revision != responseRevision {
		return model.PlanPolicy{}, false, fmt.Errorf("subscription plan policy command response is invalid")
	}
	return replay, true, nil
}

func completePlanPolicyCommand(ctx context.Context, tx *sql.Tx, commandKey string, policy model.PlanPolicy) error {
	document, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("subscription encode plan policy response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE subscription_plan_policy_command SET plan_id=?,response_revision=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, policy.PlanID, policy.Revision, document, commandKey)
	if err != nil {
		return fmt.Errorf("subscription complete plan policy command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("subscription plan policy command completion affected no row")
	}
	return nil
}
