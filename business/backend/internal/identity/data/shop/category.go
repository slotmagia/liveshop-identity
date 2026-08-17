package mysql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type CategoryRepository struct{ database *sql.DB }

var _ shop.CategoryRepository = (*CategoryRepository)(nil)

func NewCategoryRepository(database *sql.DB) *CategoryRepository {
	return &CategoryRepository{database: database}
}

func (r *CategoryRepository) ListCategories(ctx context.Context) ([]model.Category, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	rows, err := r.database.QueryContext(ctx, `SELECT c.category_id,c.code,c.name,c.icon,c.sort_order,c.status,c.version,
  (SELECT COUNT(*) FROM identity_shop s WHERE s.category_code=c.code)
FROM identity_shop_category c WHERE c.status<>'RETIRED' ORDER BY c.sort_order,c.category_id`)
	if err != nil {
		return nil, fmt.Errorf("shop category list: %w", err)
	}
	defer rows.Close()
	values := []model.Category{}
	for rows.Next() {
		value, err := scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("shop category list scan: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("shop category list iterate: %w", err)
	}
	return values, nil
}

func (r *CategoryRepository) SaveCategory(ctx context.Context, command model.SaveCategoryCommand) (model.Category, bool, error) {
	tx, err := r.begin(ctx, "save")
	if err != nil {
		return model.Category{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayCategoryCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Category{}, false, err
	}
	if found {
		return commitCategoryReplay(tx, replay, "save")
	}

	category := command.Category
	if category.ID == 0 {
		result, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_category(code,name,icon,sort_order,status,version)
VALUES(?,?,?,?,?,1)`, category.Code, category.Name, category.Icon, category.Sort, category.Status)
		if categoryDuplicateKey(err) {
			return model.Category{}, false, model.ErrCategoryConflict
		}
		if err != nil {
			return model.Category{}, false, fmt.Errorf("shop category insert: %w", err)
		}
		category.ID, err = result.LastInsertId()
		if err != nil {
			return model.Category{}, false, fmt.Errorf("shop category inserted id: %w", err)
		}
	} else {
		current, err := readCategoryForUpdate(ctx, tx, category.ID)
		if err != nil {
			return model.Category{}, false, err
		}
		if current.Version != command.ExpectedVersion || current.Status == model.CategoryRetired || current.Code != category.Code {
			return model.Category{}, false, model.ErrCategoryConflict
		}
		result, err := tx.ExecContext(ctx, `UPDATE identity_shop_category SET name=?,icon=?,sort_order=?,status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE category_id=? AND version=? AND status<>'RETIRED'`, category.Name, category.Icon, category.Sort, category.Status, category.ID, command.ExpectedVersion)
		if err != nil {
			return model.Category{}, false, fmt.Errorf("shop category update: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return model.Category{}, false, model.ErrCategoryConflict
		}
	}

	saved, err := readCategory(ctx, tx, category.ID)
	if err != nil {
		return model.Category{}, false, err
	}
	return completeAndCommitCategory(ctx, tx, command.CommandKey, saved, "save")
}

func (r *CategoryRepository) SetCategoryEnabled(ctx context.Context, command model.SetCategoryEnabledCommand) (model.Category, bool, error) {
	tx, err := r.begin(ctx, "set enabled")
	if err != nil {
		return model.Category{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayCategoryCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Category{}, false, err
	}
	if found {
		return commitCategoryReplay(tx, replay, "set enabled")
	}
	current, err := readCategoryForUpdate(ctx, tx, command.CategoryID)
	if err != nil {
		return model.Category{}, false, err
	}
	if current.Version != command.ExpectedVersion || current.Status == model.CategoryRetired {
		return model.Category{}, false, model.ErrCategoryConflict
	}
	target := model.CategoryDisabled
	if command.Enabled {
		target = model.CategoryActive
	}
	if current.Status != target {
		result, err := tx.ExecContext(ctx, `UPDATE identity_shop_category SET status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE category_id=? AND version=? AND status<>'RETIRED'`, target, command.CategoryID, command.ExpectedVersion)
		if err != nil {
			return model.Category{}, false, fmt.Errorf("shop category set enabled: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return model.Category{}, false, model.ErrCategoryConflict
		}
	}
	saved, err := readCategory(ctx, tx, command.CategoryID)
	if err != nil {
		return model.Category{}, false, err
	}
	return completeAndCommitCategory(ctx, tx, command.CommandKey, saved, "set enabled")
}

func (r *CategoryRepository) RetireCategory(ctx context.Context, command model.RetireCategoryCommand) (model.Category, bool, error) {
	tx, err := r.begin(ctx, "retire")
	if err != nil {
		return model.Category{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayCategoryCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Category{}, false, err
	}
	if found {
		return commitCategoryReplay(tx, replay, "retire")
	}
	current, err := readCategoryForUpdate(ctx, tx, command.CategoryID)
	if err != nil {
		return model.Category{}, false, err
	}
	if current.Version != command.ExpectedVersion || current.Status == model.CategoryRetired {
		return model.Category{}, false, model.ErrCategoryConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_category SET status='RETIRED',version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE category_id=? AND version=? AND status<>'RETIRED'`, command.CategoryID, command.ExpectedVersion)
	if err != nil {
		return model.Category{}, false, fmt.Errorf("shop category retire: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Category{}, false, model.ErrCategoryConflict
	}
	retired, err := readCategory(ctx, tx, command.CategoryID)
	if err != nil {
		return model.Category{}, false, err
	}
	return completeAndCommitCategory(ctx, tx, command.CommandKey, retired, "retire")
}

func (r *CategoryRepository) begin(ctx context.Context, operation string) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("shop category begin %s: %w", operation, err)
	}
	return tx, nil
}

type categoryScanner interface{ Scan(...any) error }

func scanCategory(row categoryScanner) (model.Category, error) {
	var value model.Category
	err := row.Scan(&value.ID, &value.Code, &value.Name, &value.Icon, &value.Sort, &value.Status, &value.Version, &value.UsedShopCount)
	return value, err
}

func readCategory(ctx context.Context, tx *sql.Tx, categoryID int64) (model.Category, error) {
	value, err := scanCategory(tx.QueryRowContext(ctx, `SELECT c.category_id,c.code,c.name,c.icon,c.sort_order,c.status,c.version,
  (SELECT COUNT(*) FROM identity_shop s WHERE s.category_code=c.code)
FROM identity_shop_category c WHERE c.category_id=?`, categoryID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Category{}, model.ErrCategoryNotFound
	}
	if err != nil {
		return model.Category{}, fmt.Errorf("shop category read: %w", err)
	}
	return value, nil
}

func readCategoryForUpdate(ctx context.Context, tx *sql.Tx, categoryID int64) (model.Category, error) {
	var value model.Category
	err := tx.QueryRowContext(ctx, `SELECT category_id,code,name,icon,sort_order,status,version,
  (SELECT COUNT(*) FROM identity_shop s WHERE s.category_code=identity_shop_category.code)
FROM identity_shop_category WHERE category_id=? FOR UPDATE`, categoryID).Scan(
		&value.ID, &value.Code, &value.Name, &value.Icon, &value.Sort, &value.Status, &value.Version, &value.UsedShopCount)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Category{}, model.ErrCategoryNotFound
	}
	if err != nil {
		return model.Category{}, fmt.Errorf("shop category lock: %w", err)
	}
	return value, nil
}

func insertOrReplayCategoryCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) (model.Category, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_category_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !categoryDuplicateKey(err) {
		if err != nil {
			return model.Category{}, false, fmt.Errorf("shop category insert command: %w", err)
		}
		return model.Category{}, false, nil
	}
	var stored []byte
	var responseVersion uint64
	var document []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_shop_category_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &responseVersion, &document); err != nil {
		return model.Category{}, false, fmt.Errorf("shop category read command replay: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return model.Category{}, false, model.ErrCategoryIdempotency
	}
	var replay model.Category
	if responseVersion == 0 || len(document) == 0 || json.Unmarshal(document, &replay) != nil || replay.ID <= 0 || replay.Version != responseVersion {
		return model.Category{}, false, fmt.Errorf("shop category command response is incomplete")
	}
	return replay, true, nil
}

func completeAndCommitCategory(ctx context.Context, tx *sql.Tx, key string, value model.Category, operation string) (model.Category, bool, error) {
	document, err := json.Marshal(value)
	if err != nil {
		return model.Category{}, false, fmt.Errorf("shop category encode %s response: %w", operation, err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_category_command SET category_id=?,response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, value.ID, value.Version, document, key)
	if err != nil {
		return model.Category{}, false, fmt.Errorf("shop category complete %s command: %w", operation, err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Category{}, false, fmt.Errorf("shop category %s command completion affected no row", operation)
	}
	if err := tx.Commit(); err != nil {
		return model.Category{}, false, fmt.Errorf("shop category commit %s: %w", operation, err)
	}
	return value, false, nil
}

func commitCategoryReplay(tx *sql.Tx, value model.Category, operation string) (model.Category, bool, error) {
	if err := tx.Commit(); err != nil {
		return model.Category{}, false, fmt.Errorf("shop category commit %s replay: %w", operation, err)
	}
	return value, true, nil
}

func categoryDuplicateKey(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
