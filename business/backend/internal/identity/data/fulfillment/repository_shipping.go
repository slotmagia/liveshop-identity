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

var _ fulfillment.ShippingRepository = (*Repository)(nil)

const shippingRuleSelect = `shipping_rule_id,merchant_id,shop_id,name,regions,fee_fen,free_over_fen,min_days,max_days,sort_order,status,version,created_at,updated_at`

const shippingPresetSelect = `shipping_preset_id,merchant_id,shop_id,name,is_default,product_scope,product_ids_json,origin_name,origin_region_code,origin_region_name,origin_country_code,origin_country_name,origin_subdivision_code,origin_subdivision_name,status,zones_json,version,created_at,updated_at`

func (r *Repository) ListRules(ctx context.Context, query model.ShippingQuery) (model.ShippingRulePage, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.ShippingRulePage{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureShopScope(ctx, tx, query.MerchantID, query.ShopID, false); err != nil {
		return model.ShippingRulePage{}, mapShippingScope(err)
	}
	where := "merchant_id=? AND shop_id=? AND status<>?"
	args := []any{query.MerchantID, query.ShopID, model.ShippingRetired}
	if query.Status != "" {
		where += " AND status=?"
		args = append(args, query.Status)
	}
	var total int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM identity_shipping_rule WHERE "+where, args...).Scan(&total); err != nil {
		return model.ShippingRulePage{}, fmt.Errorf("shipping rule count: %w", err)
	}
	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := tx.QueryContext(ctx, `SELECT `+shippingRuleSelect+` FROM identity_shipping_rule WHERE `+where+` ORDER BY sort_order ASC, shipping_rule_id ASC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return model.ShippingRulePage{}, fmt.Errorf("shipping rule list: %w", err)
	}
	defer rows.Close()
	items := []model.ShippingRule{}
	for rows.Next() {
		value, err := scanShippingRule(rows)
		if err != nil {
			return model.ShippingRulePage{}, fmt.Errorf("shipping rule list scan: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.ShippingRulePage{}, fmt.Errorf("shipping rule list iterate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.ShippingRulePage{}, fmt.Errorf("shipping rule list commit: %w", err)
	}
	return model.ShippingRulePage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *Repository) SaveRule(ctx context.Context, command model.SaveShippingRuleCommand) (model.ShippingRule, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.ShippingRule{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShippingRuleCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.ShippingRule{}, false, err
	}
	if found {
		return commitShippingRuleReplay(tx, replay)
	}
	if err := ensureShopScope(ctx, tx, command.Rule.MerchantID, command.Rule.ShopID, true); err != nil {
		return model.ShippingRule{}, false, mapShippingScope(err)
	}
	var id int64
	if command.Rule.ID == 0 {
		result, err := tx.ExecContext(ctx, `INSERT INTO identity_shipping_rule
(merchant_id,shop_id,name,regions,fee_fen,free_over_fen,min_days,max_days,sort_order,status,version)
VALUES(?,?,?,?,?,?,?,?,?,?,1)`,
			command.Rule.MerchantID, command.Rule.ShopID, command.Rule.Name, command.Rule.Regions,
			command.Rule.FeeFen, command.Rule.FreeOverFen, command.Rule.MinDays, command.Rule.MaxDays,
			command.Rule.SortOrder, command.Rule.Status)
		if err != nil {
			return model.ShippingRule{}, false, fmt.Errorf("shipping rule insert: %w", err)
		}
		id, err = result.LastInsertId()
		if err != nil {
			return model.ShippingRule{}, false, fmt.Errorf("shipping rule insert id: %w", err)
		}
	} else {
		current, err := readShippingRuleForUpdate(ctx, tx, command.Rule.ID, command.Rule.MerchantID, command.Rule.ShopID)
		if err != nil {
			return model.ShippingRule{}, false, err
		}
		if current.Version != command.ExpectedVersion || current.Status == model.ShippingRetired {
			return model.ShippingRule{}, false, model.ErrShippingConflict
		}
		result, err := tx.ExecContext(ctx, `UPDATE identity_shipping_rule
SET name=?,regions=?,fee_fen=?,free_over_fen=?,min_days=?,max_days=?,sort_order=?,status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE shipping_rule_id=? AND merchant_id=? AND shop_id=? AND version=? AND status<>?`,
			command.Rule.Name, command.Rule.Regions, command.Rule.FeeFen, command.Rule.FreeOverFen,
			command.Rule.MinDays, command.Rule.MaxDays, command.Rule.SortOrder, command.Rule.Status,
			current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion, model.ShippingRetired)
		if err != nil {
			return model.ShippingRule{}, false, fmt.Errorf("shipping rule update: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return model.ShippingRule{}, false, model.ErrShippingConflict
		}
		id = current.ID
	}
	saved, err := readShippingRule(ctx, tx, id, command.Rule.MerchantID, command.Rule.ShopID)
	if err != nil {
		return model.ShippingRule{}, false, err
	}
	return completeAndCommitShippingRule(ctx, tx, command.CommandKey, saved)
}

func (r *Repository) RetireRule(ctx context.Context, command model.RetireShippingCommand) (model.ShippingRule, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.ShippingRule{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShippingRuleCommand(ctx, tx, command.CommandKey, command.RequestDigest("RETIRE-RULE"))
	if err != nil {
		return model.ShippingRule{}, false, err
	}
	if found {
		return commitShippingRuleReplay(tx, replay)
	}
	if err := ensureShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.ShippingRule{}, false, mapShippingScope(err)
	}
	current, err := readShippingRuleForUpdate(ctx, tx, command.ID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.ShippingRule{}, false, err
	}
	if current.Version != command.ExpectedVersion || current.Status == model.ShippingRetired {
		return model.ShippingRule{}, false, model.ErrShippingConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shipping_rule
SET status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE shipping_rule_id=? AND merchant_id=? AND shop_id=? AND version=? AND status<>?`,
		model.ShippingRetired, current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion, model.ShippingRetired)
	if err != nil {
		return model.ShippingRule{}, false, fmt.Errorf("shipping rule retire: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.ShippingRule{}, false, model.ErrShippingConflict
	}
	saved, err := readShippingRule(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.ShippingRule{}, false, err
	}
	return completeAndCommitShippingRule(ctx, tx, command.CommandKey, saved)
}

func (r *Repository) ListPresets(ctx context.Context, query model.ShippingQuery) (model.ShippingPresetPage, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.ShippingPresetPage{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureShopScope(ctx, tx, query.MerchantID, query.ShopID, false); err != nil {
		return model.ShippingPresetPage{}, mapShippingScope(err)
	}
	where := "merchant_id=? AND shop_id=? AND status<>?"
	args := []any{query.MerchantID, query.ShopID, model.ShippingRetired}
	if query.Status != "" {
		where += " AND status=?"
		args = append(args, query.Status)
	}
	var total int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM identity_shipping_preset WHERE "+where, args...).Scan(&total); err != nil {
		return model.ShippingPresetPage{}, fmt.Errorf("shipping preset count: %w", err)
	}
	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := tx.QueryContext(ctx, `SELECT `+shippingPresetSelect+` FROM identity_shipping_preset WHERE `+where+` ORDER BY is_default DESC, shipping_preset_id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return model.ShippingPresetPage{}, fmt.Errorf("shipping preset list: %w", err)
	}
	defer rows.Close()
	items := []model.ShippingPreset{}
	for rows.Next() {
		value, err := scanShippingPreset(rows)
		if err != nil {
			return model.ShippingPresetPage{}, fmt.Errorf("shipping preset list scan: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.ShippingPresetPage{}, fmt.Errorf("shipping preset list iterate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.ShippingPresetPage{}, fmt.Errorf("shipping preset list commit: %w", err)
	}
	return model.ShippingPresetPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *Repository) GetPreset(ctx context.Context, merchantID, shopID, presetID int64) (model.ShippingPreset, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.ShippingPreset{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureShopScope(ctx, tx, merchantID, shopID, false); err != nil {
		return model.ShippingPreset{}, mapShippingScope(err)
	}
	value, err := readShippingPreset(ctx, tx, presetID, merchantID, shopID)
	if err != nil {
		return model.ShippingPreset{}, err
	}
	if value.Status == model.ShippingRetired {
		return model.ShippingPreset{}, model.ErrShippingNotFound
	}
	if err := tx.Commit(); err != nil {
		return model.ShippingPreset{}, fmt.Errorf("shipping preset get commit: %w", err)
	}
	return value, nil
}

func (r *Repository) SavePreset(ctx context.Context, command model.SaveShippingPresetCommand) (model.ShippingPreset, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShippingPresetCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	if found {
		return commitShippingPresetReplay(tx, replay)
	}
	if err := ensureShopScope(ctx, tx, command.Preset.MerchantID, command.Preset.ShopID, true); err != nil {
		return model.ShippingPreset{}, false, mapShippingScope(err)
	}
	productIDs, err := json.Marshal(command.Preset.ProductIDs)
	if err != nil {
		return model.ShippingPreset{}, false, fmt.Errorf("shipping preset product ids: %w", err)
	}
	zones, err := json.Marshal(command.Preset.Zones)
	if err != nil {
		return model.ShippingPreset{}, false, fmt.Errorf("shipping preset zones: %w", err)
	}
	var id int64
	if command.Preset.ID == 0 {
		result, err := tx.ExecContext(ctx, `INSERT INTO identity_shipping_preset
(merchant_id,shop_id,name,is_default,product_scope,product_ids_json,origin_name,origin_region_code,origin_region_name,origin_country_code,origin_country_name,origin_subdivision_code,origin_subdivision_name,status,zones_json,version)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`,
			command.Preset.MerchantID, command.Preset.ShopID, command.Preset.Name, command.Preset.IsDefault,
			command.Preset.ProductScope, productIDs, command.Preset.OriginName, command.Preset.OriginRegionCode,
			command.Preset.OriginRegionName, command.Preset.OriginCountryCode, command.Preset.OriginCountryName,
			command.Preset.OriginSubdivisionCode, command.Preset.OriginSubdivisionName, command.Preset.Status, zones)
		if err != nil {
			return model.ShippingPreset{}, false, fmt.Errorf("shipping preset insert: %w", err)
		}
		id, err = result.LastInsertId()
		if err != nil {
			return model.ShippingPreset{}, false, fmt.Errorf("shipping preset insert id: %w", err)
		}
	} else {
		current, err := readShippingPresetForUpdate(ctx, tx, command.Preset.ID, command.Preset.MerchantID, command.Preset.ShopID)
		if err != nil {
			return model.ShippingPreset{}, false, err
		}
		if current.Version != command.ExpectedVersion || current.Status == model.ShippingRetired {
			return model.ShippingPreset{}, false, model.ErrShippingConflict
		}
		result, err := tx.ExecContext(ctx, `UPDATE identity_shipping_preset
SET name=?,is_default=?,product_scope=?,product_ids_json=?,origin_name=?,origin_region_code=?,origin_region_name=?,origin_country_code=?,origin_country_name=?,origin_subdivision_code=?,origin_subdivision_name=?,status=?,zones_json=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE shipping_preset_id=? AND merchant_id=? AND shop_id=? AND version=? AND status<>?`,
			command.Preset.Name, command.Preset.IsDefault, command.Preset.ProductScope, productIDs,
			command.Preset.OriginName, command.Preset.OriginRegionCode, command.Preset.OriginRegionName,
			command.Preset.OriginCountryCode, command.Preset.OriginCountryName, command.Preset.OriginSubdivisionCode,
			command.Preset.OriginSubdivisionName, command.Preset.Status, zones,
			current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion, model.ShippingRetired)
		if err != nil {
			return model.ShippingPreset{}, false, fmt.Errorf("shipping preset update: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return model.ShippingPreset{}, false, model.ErrShippingConflict
		}
		id = current.ID
	}
	if command.Preset.IsDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE identity_shipping_preset
SET is_default=0,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=? AND shop_id=? AND shipping_preset_id<>? AND is_default=1 AND status<>?`,
			command.Preset.MerchantID, command.Preset.ShopID, id, model.ShippingRetired); err != nil {
			return model.ShippingPreset{}, false, fmt.Errorf("shipping preset clear default: %w", err)
		}
	}
	saved, err := readShippingPreset(ctx, tx, id, command.Preset.MerchantID, command.Preset.ShopID)
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	return completeAndCommitShippingPreset(ctx, tx, command.CommandKey, saved)
}

func (r *Repository) SetPresetEnabled(ctx context.Context, command model.SetShippingPresetEnabledCommand) (model.ShippingPreset, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShippingPresetCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	if found {
		return commitShippingPresetReplay(tx, replay)
	}
	if err := ensureShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.ShippingPreset{}, false, mapShippingScope(err)
	}
	current, err := readShippingPresetForUpdate(ctx, tx, command.PresetID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	if current.Version != command.ExpectedVersion || current.Status == model.ShippingRetired {
		return model.ShippingPreset{}, false, model.ErrShippingConflict
	}
	status := model.ShippingDisabled
	if command.Enabled {
		status = model.ShippingActive
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shipping_preset
SET status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE shipping_preset_id=? AND merchant_id=? AND shop_id=? AND version=? AND status<>?`,
		status, current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion, model.ShippingRetired)
	if err != nil {
		return model.ShippingPreset{}, false, fmt.Errorf("shipping preset enable: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.ShippingPreset{}, false, model.ErrShippingConflict
	}
	saved, err := readShippingPreset(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	return completeAndCommitShippingPreset(ctx, tx, command.CommandKey, saved)
}

func (r *Repository) RetirePreset(ctx context.Context, command model.RetireShippingCommand) (model.ShippingPreset, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayShippingPresetCommand(ctx, tx, command.CommandKey, command.RequestDigest("RETIRE-PRESET"))
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	if found {
		return commitShippingPresetReplay(tx, replay)
	}
	if err := ensureShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.ShippingPreset{}, false, mapShippingScope(err)
	}
	current, err := readShippingPresetForUpdate(ctx, tx, command.ID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	if current.Version != command.ExpectedVersion || current.Status == model.ShippingRetired {
		return model.ShippingPreset{}, false, model.ErrShippingConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shipping_preset
SET status=?,is_default=0,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE shipping_preset_id=? AND merchant_id=? AND shop_id=? AND version=? AND status<>?`,
		model.ShippingRetired, current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion, model.ShippingRetired)
	if err != nil {
		return model.ShippingPreset{}, false, fmt.Errorf("shipping preset retire: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.ShippingPreset{}, false, model.ErrShippingConflict
	}
	saved, err := readShippingPreset(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.ShippingPreset{}, false, err
	}
	return completeAndCommitShippingPreset(ctx, tx, command.CommandKey, saved)
}

func scanShippingRule(row scanner) (model.ShippingRule, error) {
	var value model.ShippingRule
	var status string
	err := row.Scan(&value.ID, &value.MerchantID, &value.ShopID, &value.Name, &value.Regions, &value.FeeFen,
		&value.FreeOverFen, &value.MinDays, &value.MaxDays, &value.SortOrder, &status, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	value.Status = model.ShippingStatus(status)
	return value, err
}

func readShippingRule(ctx context.Context, tx *sql.Tx, ruleID, merchantID, shopID int64) (model.ShippingRule, error) {
	value, err := scanShippingRule(tx.QueryRowContext(ctx, `SELECT `+shippingRuleSelect+` FROM identity_shipping_rule WHERE shipping_rule_id=? AND merchant_id=? AND shop_id=?`, ruleID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.ShippingRule{}, model.ErrShippingNotFound
	}
	if err != nil {
		return model.ShippingRule{}, fmt.Errorf("shipping rule read: %w", err)
	}
	return value, nil
}

func readShippingRuleForUpdate(ctx context.Context, tx *sql.Tx, ruleID, merchantID, shopID int64) (model.ShippingRule, error) {
	value, err := scanShippingRule(tx.QueryRowContext(ctx, `SELECT `+shippingRuleSelect+` FROM identity_shipping_rule WHERE shipping_rule_id=? AND merchant_id=? AND shop_id=? FOR UPDATE`, ruleID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.ShippingRule{}, model.ErrShippingNotFound
	}
	if err != nil {
		return model.ShippingRule{}, fmt.Errorf("shipping rule lock: %w", err)
	}
	return value, nil
}

func scanShippingPreset(row scanner) (model.ShippingPreset, error) {
	var value model.ShippingPreset
	var productScope, status string
	var productIDs, zones []byte
	err := row.Scan(&value.ID, &value.MerchantID, &value.ShopID, &value.Name, &value.IsDefault, &productScope, &productIDs,
		&value.OriginName, &value.OriginRegionCode, &value.OriginRegionName, &value.OriginCountryCode, &value.OriginCountryName,
		&value.OriginSubdivisionCode, &value.OriginSubdivisionName, &status, &zones, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	value.ProductScope = model.ProductScope(productScope)
	value.Status = model.ShippingStatus(status)
	if len(productIDs) == 0 || string(productIDs) == "null" {
		value.ProductIDs = []int64{}
	} else if json.Unmarshal(productIDs, &value.ProductIDs) != nil {
		return model.ShippingPreset{}, fmt.Errorf("shipping preset product ids json")
	}
	if value.ProductIDs == nil {
		value.ProductIDs = []int64{}
	}
	if len(zones) == 0 || json.Unmarshal(zones, &value.Zones) != nil {
		return model.ShippingPreset{}, fmt.Errorf("shipping preset zones json")
	}
	return value, err
}

func readShippingPreset(ctx context.Context, tx *sql.Tx, presetID, merchantID, shopID int64) (model.ShippingPreset, error) {
	value, err := scanShippingPreset(tx.QueryRowContext(ctx, `SELECT `+shippingPresetSelect+` FROM identity_shipping_preset WHERE shipping_preset_id=? AND merchant_id=? AND shop_id=?`, presetID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.ShippingPreset{}, model.ErrShippingNotFound
	}
	if err != nil {
		return model.ShippingPreset{}, fmt.Errorf("shipping preset read: %w", err)
	}
	return value, nil
}

func readShippingPresetForUpdate(ctx context.Context, tx *sql.Tx, presetID, merchantID, shopID int64) (model.ShippingPreset, error) {
	value, err := scanShippingPreset(tx.QueryRowContext(ctx, `SELECT `+shippingPresetSelect+` FROM identity_shipping_preset WHERE shipping_preset_id=? AND merchant_id=? AND shop_id=? FOR UPDATE`, presetID, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.ShippingPreset{}, model.ErrShippingNotFound
	}
	if err != nil {
		return model.ShippingPreset{}, fmt.Errorf("shipping preset lock: %w", err)
	}
	return value, nil
}

func insertOrReplayShippingRuleCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) (model.ShippingRule, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_shipping_rule_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !complaintDuplicateKey(err) {
		if err != nil {
			return model.ShippingRule{}, false, fmt.Errorf("shipping rule insert command: %w", err)
		}
		return model.ShippingRule{}, false, nil
	}
	var stored []byte
	var responseVersion uint64
	var document []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_shipping_rule_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &responseVersion, &document); err != nil {
		return model.ShippingRule{}, false, fmt.Errorf("shipping rule read command replay: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return model.ShippingRule{}, false, model.ErrShippingIdempotency
	}
	var replay model.ShippingRule
	if responseVersion == 0 || len(document) == 0 || json.Unmarshal(document, &replay) != nil || replay.ID <= 0 || replay.Version != responseVersion {
		return model.ShippingRule{}, false, fmt.Errorf("shipping rule command response is incomplete")
	}
	return replay, true, nil
}

func completeAndCommitShippingRule(ctx context.Context, tx *sql.Tx, key string, value model.ShippingRule) (model.ShippingRule, bool, error) {
	document, err := json.Marshal(value)
	if err != nil {
		return model.ShippingRule{}, false, fmt.Errorf("shipping rule encode response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shipping_rule_command SET shipping_rule_id=?,response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, value.ID, value.Version, document, key)
	if err != nil {
		return model.ShippingRule{}, false, fmt.Errorf("shipping rule complete command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.ShippingRule{}, false, fmt.Errorf("shipping rule command completion affected no row")
	}
	if err := tx.Commit(); err != nil {
		return model.ShippingRule{}, false, fmt.Errorf("shipping rule commit: %w", err)
	}
	return value, false, nil
}

func commitShippingRuleReplay(tx *sql.Tx, value model.ShippingRule) (model.ShippingRule, bool, error) {
	if err := tx.Commit(); err != nil {
		return model.ShippingRule{}, false, fmt.Errorf("shipping rule commit replay: %w", err)
	}
	return value, true, nil
}

func insertOrReplayShippingPresetCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) (model.ShippingPreset, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_shipping_preset_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !complaintDuplicateKey(err) {
		if err != nil {
			return model.ShippingPreset{}, false, fmt.Errorf("shipping preset insert command: %w", err)
		}
		return model.ShippingPreset{}, false, nil
	}
	var stored []byte
	var responseVersion uint64
	var document []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_shipping_preset_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &responseVersion, &document); err != nil {
		return model.ShippingPreset{}, false, fmt.Errorf("shipping preset read command replay: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return model.ShippingPreset{}, false, model.ErrShippingIdempotency
	}
	var replay model.ShippingPreset
	if responseVersion == 0 || len(document) == 0 || json.Unmarshal(document, &replay) != nil || replay.ID <= 0 || replay.Version != responseVersion {
		return model.ShippingPreset{}, false, fmt.Errorf("shipping preset command response is incomplete")
	}
	return replay, true, nil
}

func completeAndCommitShippingPreset(ctx context.Context, tx *sql.Tx, key string, value model.ShippingPreset) (model.ShippingPreset, bool, error) {
	document, err := json.Marshal(value)
	if err != nil {
		return model.ShippingPreset{}, false, fmt.Errorf("shipping preset encode response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shipping_preset_command SET shipping_preset_id=?,response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, value.ID, value.Version, document, key)
	if err != nil {
		return model.ShippingPreset{}, false, fmt.Errorf("shipping preset complete command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.ShippingPreset{}, false, fmt.Errorf("shipping preset command completion affected no row")
	}
	if err := tx.Commit(); err != nil {
		return model.ShippingPreset{}, false, fmt.Errorf("shipping preset commit: %w", err)
	}
	return value, false, nil
}

func commitShippingPresetReplay(tx *sql.Tx, value model.ShippingPreset) (model.ShippingPreset, bool, error) {
	if err := tx.Commit(); err != nil {
		return model.ShippingPreset{}, false, fmt.Errorf("shipping preset commit replay: %w", err)
	}
	return value, true, nil
}

func mapShippingScope(err error) error {
	if errors.Is(err, model.ErrNotFound) {
		return model.ErrShippingNotFound
	}
	return err
}
