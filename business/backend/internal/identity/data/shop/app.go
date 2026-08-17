package mysql

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type AppRepository struct{ database *sql.DB }

var _ shop.AppRepository = (*AppRepository)(nil)

func NewAppRepository(database *sql.DB) *AppRepository {
	return &AppRepository{database: database}
}

type appCommandResponse struct {
	App          model.App `json:"app"`
	ClientSecret string    `json:"clientSecret,omitempty"`
}

func (r *AppRepository) ListApps(ctx context.Context, query model.AppQuery) (model.AppPage, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.AppPage{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureAppShopScope(ctx, tx, query.MerchantID, query.ShopID, false); err != nil {
		return model.AppPage{}, err
	}
	where := "merchant_id=? AND shop_id=?"
	args := []any{query.MerchantID, query.ShopID}
	if query.Status != "" {
		where += " AND status=?"
		args = append(args, query.Status)
	}
	var total int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM identity_shop_app WHERE "+where, args...).Scan(&total); err != nil {
		return model.AppPage{}, fmt.Errorf("shop app count: %w", err)
	}
	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := tx.QueryContext(ctx, `SELECT private_app_id,merchant_id,shop_id,name,client_id,secret_hint,scopes,status,version,created_at,updated_at
FROM identity_shop_app WHERE `+where+` ORDER BY private_app_id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return model.AppPage{}, fmt.Errorf("shop app list: %w", err)
	}
	defer rows.Close()
	items := []model.App{}
	for rows.Next() {
		value, err := scanApp(rows)
		if err != nil {
			return model.AppPage{}, fmt.Errorf("shop app list scan: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.AppPage{}, fmt.Errorf("shop app list iterate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.AppPage{}, fmt.Errorf("shop app list commit: %w", err)
	}
	return model.AppPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *AppRepository) CreateApp(ctx context.Context, command model.CreateAppCommand) (model.AppMutation, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.AppMutation{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayAppCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.AppMutation{}, false, err
	}
	if found {
		return commitAppReplay(tx, replay, "create")
	}
	if err := ensureAppShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.AppMutation{}, false, err
	}
	if err := requireAppOverlayActive(ctx, tx, command.MerchantID, command.ShopID); err != nil {
		return model.AppMutation{}, false, err
	}
	clientID, err := randomAppCredential("app_", 12)
	if err != nil {
		return model.AppMutation{}, false, err
	}
	secret, err := randomAppCredential("sec_", 24)
	if err != nil {
		return model.AppMutation{}, false, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_app
(merchant_id,shop_id,name,client_id,client_secret_hash,secret_hint,scopes,status,version)
VALUES(?,?,?,?,?,?,?,?,1)`, command.MerchantID, command.ShopID, command.Name, clientID, hashAppSecret(secret),
		secret[len(secret)-model.AppHintLength:], command.Scopes, model.AppActive)
	if appDuplicate(err) {
		return model.AppMutation{}, false, model.ErrAppConflict
	}
	if err != nil {
		return model.AppMutation{}, false, fmt.Errorf("shop app insert: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return model.AppMutation{}, false, fmt.Errorf("shop app inserted id: %w", err)
	}
	saved, err := readApp(ctx, tx, id, command.MerchantID, command.ShopID)
	if err != nil {
		return model.AppMutation{}, false, err
	}
	mutation := model.AppMutation{App: saved, ClientSecret: secret}
	if err := appendAppOutbox(ctx, tx, saved); err != nil {
		return model.AppMutation{}, false, err
	}
	if err := completeAppCommand(ctx, tx, command.CommandKey, saved.Version, appCommandResponse{App: saved, ClientSecret: secret}); err != nil {
		return model.AppMutation{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.AppMutation{}, false, fmt.Errorf("shop app create commit: %w", err)
	}
	return mutation, false, nil
}

func (r *AppRepository) ResetAppSecret(ctx context.Context, command model.ResetAppSecretCommand) (model.AppMutation, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.AppMutation{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayAppCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.AppMutation{}, false, err
	}
	if found {
		return commitAppReplay(tx, replay, "reset")
	}
	if err := ensureAppShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.AppMutation{}, false, err
	}
	if err := requireAppOverlayActive(ctx, tx, command.MerchantID, command.ShopID); err != nil {
		return model.AppMutation{}, false, err
	}
	current, err := readAppForUpdate(ctx, tx, command.AppID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.AppMutation{}, false, err
	}
	if current.Version != command.ExpectedVersion {
		return model.AppMutation{}, false, model.ErrAppConflict
	}
	secret, err := randomAppCredential("sec_", 24)
	if err != nil {
		return model.AppMutation{}, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_app
SET client_secret_hash=?,secret_hint=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE private_app_id=? AND merchant_id=? AND shop_id=? AND version=?`, hashAppSecret(secret),
		secret[len(secret)-model.AppHintLength:], current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion)
	if err != nil {
		return model.AppMutation{}, false, fmt.Errorf("shop app reset: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.AppMutation{}, false, model.ErrAppConflict
	}
	saved, err := readApp(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.AppMutation{}, false, err
	}
	mutation := model.AppMutation{App: saved, ClientSecret: secret}
	if err := appendAppOutbox(ctx, tx, saved); err != nil {
		return model.AppMutation{}, false, err
	}
	if err := completeAppCommand(ctx, tx, command.CommandKey, saved.Version, appCommandResponse{App: saved, ClientSecret: secret}); err != nil {
		return model.AppMutation{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.AppMutation{}, false, fmt.Errorf("shop app reset commit: %w", err)
	}
	return mutation, false, nil
}

func (r *AppRepository) SetAppEnabled(ctx context.Context, command model.SetAppEnabledCommand) (model.App, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.App{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayAppCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.App{}, false, err
	}
	if found {
		mutation, replayed, err := commitAppReplay(tx, replay, "enable")
		return mutation.App, replayed, err
	}
	if err := ensureAppShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.App{}, false, err
	}
	if command.Enabled {
		if err := requireAppOverlayActive(ctx, tx, command.MerchantID, command.ShopID); err != nil {
			return model.App{}, false, err
		}
	}
	current, err := readAppForUpdate(ctx, tx, command.AppID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.App{}, false, err
	}
	if current.Version != command.ExpectedVersion {
		return model.App{}, false, model.ErrAppConflict
	}
	target := model.AppDisabled
	required := model.AppActive
	if command.Enabled {
		target = model.AppActive
		required = model.AppDisabled
	}
	if current.Status != required {
		return model.App{}, false, model.ErrAppConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_app
SET status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE private_app_id=? AND merchant_id=? AND shop_id=? AND status=? AND version=?`, target,
		current.ID, current.MerchantID, current.ShopID, required, command.ExpectedVersion)
	if err != nil {
		return model.App{}, false, fmt.Errorf("shop app enable: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.App{}, false, model.ErrAppConflict
	}
	saved, err := readApp(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.App{}, false, err
	}
	if err := appendAppOutbox(ctx, tx, saved); err != nil {
		return model.App{}, false, err
	}
	if err := completeAppCommand(ctx, tx, command.CommandKey, saved.Version, appCommandResponse{App: saved}); err != nil {
		return model.App{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.App{}, false, fmt.Errorf("shop app enable commit: %w", err)
	}
	return saved, false, nil
}

func (r *AppRepository) begin(ctx context.Context, readOnly bool) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: readOnly})
	if err != nil {
		return nil, fmt.Errorf("shop app begin: %w", err)
	}
	return tx, nil
}

func ensureAppShopScope(ctx context.Context, tx *sql.Tx, merchantID, shopID int64, lock bool) error {
	query := `SELECT merchant_id,shop_id FROM identity_shop WHERE merchant_id=? AND shop_id=? AND status<>'CLOSED'`
	if lock {
		query += " FOR UPDATE"
	}
	var storedMerchant, storedShop int64
	err := tx.QueryRowContext(ctx, query, merchantID, shopID).Scan(&storedMerchant, &storedShop)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrAppNotFound
	}
	if err != nil {
		return fmt.Errorf("shop app scope: %w", err)
	}
	return nil
}

func requireAppOverlayActive(ctx context.Context, tx *sql.Tx, merchantID, shopID int64) error {
	var status string
	err := tx.QueryRowContext(ctx, `SELECT platform_status FROM identity_merchant_capability
WHERE merchant_id=? AND shop_id=? AND module='apps' FOR UPDATE`, merchantID, shopID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("shop app overlay: %w", err)
	}
	if status != "active" {
		return model.ErrAppRestricted
	}
	return nil
}

type appScanner interface{ Scan(...any) error }

func scanApp(row appScanner) (model.App, error) {
	var value model.App
	err := row.Scan(&value.ID, &value.MerchantID, &value.ShopID, &value.Name, &value.ClientID, &value.SecretHint,
		&value.Scopes, &value.Status, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func readApp(ctx context.Context, tx *sql.Tx, id, merchantID, shopID int64) (model.App, error) {
	value, err := scanApp(tx.QueryRowContext(ctx, `SELECT private_app_id,merchant_id,shop_id,name,client_id,secret_hint,scopes,status,version,created_at,updated_at
FROM identity_shop_app WHERE private_app_id=? AND merchant_id=? AND shop_id=?`, id, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.App{}, model.ErrAppNotFound
	}
	if err != nil {
		return model.App{}, fmt.Errorf("shop app read: %w", err)
	}
	return value, nil
}

func readAppForUpdate(ctx context.Context, tx *sql.Tx, id, merchantID, shopID int64) (model.App, error) {
	value, err := scanApp(tx.QueryRowContext(ctx, `SELECT private_app_id,merchant_id,shop_id,name,client_id,secret_hint,scopes,status,version,created_at,updated_at
FROM identity_shop_app WHERE private_app_id=? AND merchant_id=? AND shop_id=? FOR UPDATE`, id, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.App{}, model.ErrAppNotFound
	}
	if err != nil {
		return model.App{}, fmt.Errorf("shop app lock: %w", err)
	}
	return value, nil
}

func insertOrReplayAppCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) ([]byte, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_app_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !appDuplicate(err) {
		if err != nil {
			return nil, false, fmt.Errorf("shop app insert command: %w", err)
		}
		return nil, false, nil
	}
	var stored, document []byte
	var version uint64
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_shop_app_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &version, &document); err != nil {
		return nil, false, fmt.Errorf("shop app read command: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return nil, false, model.ErrAppIdempotency
	}
	if version == 0 || len(document) == 0 {
		return nil, false, fmt.Errorf("shop app command response is incomplete")
	}
	return document, true, nil
}

func completeAppCommand(ctx context.Context, tx *sql.Tx, key string, version uint64, response any) error {
	document, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("shop app encode response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_app_command
SET response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, version, document, key)
	if err != nil {
		return fmt.Errorf("shop app complete command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("shop app command completion affected no row")
	}
	return nil
}

func commitAppReplay(tx *sql.Tx, document []byte, action string) (model.AppMutation, bool, error) {
	var replay appCommandResponse
	if json.Unmarshal(document, &replay) != nil || replay.App.ValidatePersisted() != nil {
		return model.AppMutation{}, false, fmt.Errorf("shop app %s command response is incomplete", action)
	}
	if err := tx.Commit(); err != nil {
		return model.AppMutation{}, false, fmt.Errorf("shop app %s replay commit: %w", action, err)
	}
	return model.AppMutation{App: replay.App, ClientSecret: replay.ClientSecret}, true, nil
}

func appendAppOutbox(ctx context.Context, tx *sql.Tx, value model.App) error {
	encoded, err := json.Marshal(map[string]any{
		"appId": value.ID, "merchantId": value.MerchantID, "shopId": value.ShopID,
		"clientId": value.ClientID, "status": value.Status, "version": value.Version,
	})
	if err != nil {
		return err
	}
	id := make([]byte, 18)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO identity_outbox (event_id, aggregate_type, aggregate_id, aggregate_version, event_type, payload_json) VALUES (?, ?, ?, ?, ?, ?)`,
		hex.EncodeToString(id), "shop_app", fmt.Sprintf("%d", value.ID), value.Version, "identity.shop.app.changed", encoded)
	return err
}

func randomAppCredential(prefix string, n int) (string, error) {
	buffer := make([]byte, n)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("shop app credential: %w", err)
	}
	return prefix + hex.EncodeToString(buffer), nil
}

func hashAppSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func appDuplicate(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
