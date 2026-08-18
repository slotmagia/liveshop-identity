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

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type DomainRepository struct{ database *sql.DB }

var _ shop.DomainRepository = (*DomainRepository)(nil)

func NewDomainRepository(database *sql.DB) *DomainRepository {
	return &DomainRepository{database: database}
}

type domainCommandResponse struct {
	Domain model.Domain `json:"domain"`
}

func (r *DomainRepository) ListDomains(ctx context.Context, query model.DomainQuery) (model.DomainPage, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.DomainPage{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureDomainShopScope(ctx, tx, query.MerchantID, query.ShopID, false); err != nil {
		return model.DomainPage{}, err
	}
	where := "merchant_id=? AND shop_id=? AND scene=? AND status<>'DELETED'"
	args := []any{query.MerchantID, query.ShopID, query.Scene}
	if query.Status != "" {
		where += " AND status=?"
		args = append(args, query.Status)
	}
	var total int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM identity_shop_domain WHERE "+where, args...).Scan(&total); err != nil {
		return model.DomainPage{}, fmt.Errorf("shop domain count: %w", err)
	}
	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := tx.QueryContext(ctx, `SELECT domain_id,merchant_id,shop_id,host,scene,status,is_primary,txt_name,txt_value,version,created_at,updated_at
FROM identity_shop_domain WHERE `+where+` ORDER BY domain_id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return model.DomainPage{}, fmt.Errorf("shop domain list: %w", err)
	}
	defer rows.Close()
	items := []model.Domain{}
	for rows.Next() {
		value, err := scanDomain(rows)
		if err != nil {
			return model.DomainPage{}, fmt.Errorf("shop domain list scan: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return model.DomainPage{}, fmt.Errorf("shop domain list iterate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.DomainPage{}, fmt.Errorf("shop domain list commit: %w", err)
	}
	return model.DomainPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *DomainRepository) GetDomain(ctx context.Context, id, merchantID, shopID int64) (model.Domain, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.Domain{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureDomainShopScope(ctx, tx, merchantID, shopID, false); err != nil {
		return model.Domain{}, err
	}
	value, err := readDomain(ctx, tx, id, merchantID, shopID)
	if err != nil {
		return model.Domain{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Domain{}, fmt.Errorf("shop domain get commit: %w", err)
	}
	return value, nil
}

func (r *DomainRepository) GetDomainByHost(ctx context.Context, host string) (model.Domain, error) {
	tx, err := r.begin(ctx, true)
	if err != nil {
		return model.Domain{}, err
	}
	defer func() { _ = tx.Rollback() }()
	value, err := scanDomain(tx.QueryRowContext(ctx, `SELECT domain_id,merchant_id,shop_id,host,scene,status,is_primary,txt_name,txt_value,version,created_at,updated_at
FROM identity_shop_domain WHERE active_host=?`, host))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Domain{}, model.ErrDomainNotFound
	}
	if err != nil {
		return model.Domain{}, fmt.Errorf("shop domain by host: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Domain{}, fmt.Errorf("shop domain by host commit: %w", err)
	}
	return value, nil
}

func (r *DomainRepository) CreateDomain(ctx context.Context, command model.CreateDomainCommand) (model.Domain, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Domain{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayDomainCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Domain{}, false, err
	}
	if found {
		return commitDomainReplay(tx, replay, "create")
	}
	if err := ensureDomainShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Domain{}, false, err
	}
	if err := requireDomainOverlayActive(ctx, tx, command.MerchantID, command.ShopID); err != nil {
		return model.Domain{}, false, err
	}
	token, err := randomDomainToken(16)
	if err != nil {
		return model.Domain{}, false, err
	}
	txtName := model.ChallengeName(command.Host)
	result, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_domain
(merchant_id,shop_id,host,scene,status,is_primary,txt_name,txt_value,version)
VALUES(?,?,?,?,?,?,?,?,1)`, command.MerchantID, command.ShopID, command.Host, command.Scene, model.DomainPending, 0, txtName, token)
	if domainDuplicate(err) {
		return model.Domain{}, false, model.ErrDomainConflict
	}
	if err != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain insert: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain inserted id: %w", err)
	}
	saved, err := readDomain(ctx, tx, id, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Domain{}, false, err
	}
	if err := appendDomainOutbox(ctx, tx, saved); err != nil {
		return model.Domain{}, false, err
	}
	if err := completeDomainCommand(ctx, tx, command.CommandKey, saved.Version, domainCommandResponse{Domain: saved}); err != nil {
		return model.Domain{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain create commit: %w", err)
	}
	return saved, false, nil
}

func (r *DomainRepository) TestDomain(ctx context.Context, command model.DomainWriteCommand, matched bool) (model.Domain, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Domain{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayDomainCommand(ctx, tx, command.CommandKey, command.RequestDigest("TEST"))
	if err != nil {
		return model.Domain{}, false, err
	}
	if found {
		return commitDomainReplay(tx, replay, "test")
	}
	if err := ensureDomainShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Domain{}, false, err
	}
	if err := requireDomainOverlayActive(ctx, tx, command.MerchantID, command.ShopID); err != nil {
		return model.Domain{}, false, err
	}
	current, err := readDomainForUpdate(ctx, tx, command.DomainID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Domain{}, false, err
	}
	if current.Status == model.DomainDeleted {
		return model.Domain{}, false, model.ErrDomainNotFound
	}
	if err := rejectDomainSceneMismatch(current, command); err != nil {
		return model.Domain{}, false, err
	}
	if current.Version != command.ExpectedVersion {
		return model.Domain{}, false, model.ErrDomainConflict
	}
	target := model.DomainFailed
	if matched {
		target = model.DomainVerified
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_domain
SET status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE domain_id=? AND merchant_id=? AND shop_id=? AND status<>'DELETED' AND version=?`, target,
		current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion)
	if err != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain test: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Domain{}, false, model.ErrDomainConflict
	}
	saved, err := readDomain(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.Domain{}, false, err
	}
	if err := appendDomainOutbox(ctx, tx, saved); err != nil {
		return model.Domain{}, false, err
	}
	if err := completeDomainCommand(ctx, tx, command.CommandKey, saved.Version, domainCommandResponse{Domain: saved}); err != nil {
		return model.Domain{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain test commit: %w", err)
	}
	return saved, false, nil
}

func (r *DomainRepository) ActivateDomain(ctx context.Context, command model.DomainWriteCommand) (model.Domain, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Domain{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayDomainCommand(ctx, tx, command.CommandKey, command.RequestDigest("ACTIVATE"))
	if err != nil {
		return model.Domain{}, false, err
	}
	if found {
		return commitDomainReplay(tx, replay, "activate")
	}
	if err := ensureDomainShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Domain{}, false, err
	}
	if err := requireDomainOverlayActive(ctx, tx, command.MerchantID, command.ShopID); err != nil {
		return model.Domain{}, false, err
	}
	lockRows, err := tx.QueryContext(ctx, `SELECT domain_id FROM identity_shop_domain
WHERE merchant_id=? AND shop_id=? AND status<>'DELETED' FOR UPDATE`, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain activate lock: %w", err)
	}
	for lockRows.Next() {
	}
	scanErr := lockRows.Err()
	_ = lockRows.Close()
	if scanErr != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain activate lock iterate: %w", scanErr)
	}
	current, err := readDomainForUpdate(ctx, tx, command.DomainID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Domain{}, false, err
	}
	if current.Status == model.DomainDeleted {
		return model.Domain{}, false, model.ErrDomainNotFound
	}
	if err := rejectDomainSceneMismatch(current, command); err != nil {
		return model.Domain{}, false, err
	}
	if current.Status != model.DomainVerified || current.Version != command.ExpectedVersion {
		return model.Domain{}, false, model.ErrDomainConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_shop_domain
SET is_primary=0,updated_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=? AND shop_id=? AND scene=? AND status<>'DELETED' AND domain_id<>?`, current.MerchantID, current.ShopID, current.Scene, current.ID); err != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain clear primary: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_domain
SET is_primary=1,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE domain_id=? AND merchant_id=? AND shop_id=? AND status='VERIFIED' AND version=?`, current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion)
	if domainDuplicate(err) {
		return model.Domain{}, false, model.ErrDomainConflict
	}
	if err != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain activate: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Domain{}, false, model.ErrDomainConflict
	}
	saved, err := readDomain(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.Domain{}, false, err
	}
	if err := appendDomainOutbox(ctx, tx, saved); err != nil {
		return model.Domain{}, false, err
	}
	if err := completeDomainCommand(ctx, tx, command.CommandKey, saved.Version, domainCommandResponse{Domain: saved}); err != nil {
		return model.Domain{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain activate commit: %w", err)
	}
	return saved, false, nil
}

func (r *DomainRepository) DeleteDomain(ctx context.Context, command model.DomainWriteCommand) (model.Domain, bool, error) {
	tx, err := r.begin(ctx, false)
	if err != nil {
		return model.Domain{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayDomainCommand(ctx, tx, command.CommandKey, command.RequestDigest("DELETE"))
	if err != nil {
		return model.Domain{}, false, err
	}
	if found {
		return commitDomainReplay(tx, replay, "delete")
	}
	if err := ensureDomainShopScope(ctx, tx, command.MerchantID, command.ShopID, true); err != nil {
		return model.Domain{}, false, err
	}
	current, err := readDomainForUpdate(ctx, tx, command.DomainID, command.MerchantID, command.ShopID)
	if err != nil {
		return model.Domain{}, false, err
	}
	if current.Status == model.DomainDeleted {
		return model.Domain{}, false, model.ErrDomainNotFound
	}
	if err := rejectDomainSceneMismatch(current, command); err != nil {
		return model.Domain{}, false, err
	}
	if current.Version != command.ExpectedVersion {
		return model.Domain{}, false, model.ErrDomainConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_domain
SET status='DELETED',is_primary=0,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE domain_id=? AND merchant_id=? AND shop_id=? AND status<>'DELETED' AND version=?`, current.ID, current.MerchantID, current.ShopID, command.ExpectedVersion)
	if err != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain delete: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Domain{}, false, model.ErrDomainConflict
	}
	saved, err := readDomain(ctx, tx, current.ID, current.MerchantID, current.ShopID)
	if err != nil {
		return model.Domain{}, false, err
	}
	if err := appendDomainOutbox(ctx, tx, saved); err != nil {
		return model.Domain{}, false, err
	}
	if err := completeDomainCommand(ctx, tx, command.CommandKey, saved.Version, domainCommandResponse{Domain: saved}); err != nil {
		return model.Domain{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain delete commit: %w", err)
	}
	return saved, false, nil
}

func (r *DomainRepository) begin(ctx context.Context, readOnly bool) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: readOnly})
	if err != nil {
		return nil, fmt.Errorf("shop domain begin: %w", err)
	}
	return tx, nil
}

func ensureDomainShopScope(ctx context.Context, tx *sql.Tx, merchantID, shopID int64, lock bool) error {
	query := `SELECT merchant_id,shop_id FROM identity_shop WHERE merchant_id=? AND shop_id=? AND status<>'CLOSED'`
	if lock {
		query += " FOR UPDATE"
	}
	var storedMerchant, storedShop int64
	err := tx.QueryRowContext(ctx, query, merchantID, shopID).Scan(&storedMerchant, &storedShop)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrDomainNotFound
	}
	if err != nil {
		return fmt.Errorf("shop domain scope: %w", err)
	}
	return nil
}

func requireDomainOverlayActive(ctx context.Context, tx *sql.Tx, merchantID, shopID int64) error {
	var status string
	err := tx.QueryRowContext(ctx, `SELECT platform_status FROM identity_merchant_capability
WHERE merchant_id=? AND shop_id=? AND module='domains' FOR UPDATE`, merchantID, shopID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("shop domain overlay: %w", err)
	}
	if status != "active" {
		return model.ErrDomainRestricted
	}
	return nil
}

type domainScanner interface{ Scan(...any) error }

func scanDomain(row domainScanner) (model.Domain, error) {
	var value model.Domain
	var primary int
	err := row.Scan(&value.ID, &value.MerchantID, &value.ShopID, &value.Host, &value.Scene, &value.Status, &primary,
		&value.TxtName, &value.TxtValue, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	value.IsPrimary = primary == 1
	return value, err
}

func readDomain(ctx context.Context, tx *sql.Tx, id, merchantID, shopID int64) (model.Domain, error) {
	value, err := scanDomain(tx.QueryRowContext(ctx, `SELECT domain_id,merchant_id,shop_id,host,scene,status,is_primary,txt_name,txt_value,version,created_at,updated_at
FROM identity_shop_domain WHERE domain_id=? AND merchant_id=? AND shop_id=?`, id, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Domain{}, model.ErrDomainNotFound
	}
	if err != nil {
		return model.Domain{}, fmt.Errorf("shop domain read: %w", err)
	}
	return value, nil
}

func readDomainForUpdate(ctx context.Context, tx *sql.Tx, id, merchantID, shopID int64) (model.Domain, error) {
	value, err := scanDomain(tx.QueryRowContext(ctx, `SELECT domain_id,merchant_id,shop_id,host,scene,status,is_primary,txt_name,txt_value,version,created_at,updated_at
FROM identity_shop_domain WHERE domain_id=? AND merchant_id=? AND shop_id=? FOR UPDATE`, id, merchantID, shopID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Domain{}, model.ErrDomainNotFound
	}
	if err != nil {
		return model.Domain{}, fmt.Errorf("shop domain lock: %w", err)
	}
	return value, nil
}

func insertOrReplayDomainCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) ([]byte, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_domain_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !domainDuplicate(err) {
		if err != nil {
			return nil, false, fmt.Errorf("shop domain insert command: %w", err)
		}
		return nil, false, nil
	}
	var stored, document []byte
	var version uint64
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_version,response_json FROM identity_shop_domain_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &version, &document); err != nil {
		return nil, false, fmt.Errorf("shop domain read command: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return nil, false, model.ErrDomainIdempotency
	}
	if version == 0 || len(document) == 0 {
		return nil, false, fmt.Errorf("shop domain command response is incomplete")
	}
	return document, true, nil
}

func completeDomainCommand(ctx context.Context, tx *sql.Tx, key string, version uint64, response any) error {
	document, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("shop domain encode response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_shop_domain_command
SET response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`, version, document, key)
	if err != nil {
		return fmt.Errorf("shop domain complete command: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("shop domain command completion affected no row")
	}
	return nil
}

func commitDomainReplay(tx *sql.Tx, document []byte, action string) (model.Domain, bool, error) {
	var replay domainCommandResponse
	if json.Unmarshal(document, &replay) != nil || replay.Domain.ValidatePersisted() != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain %s command response is incomplete", action)
	}
	if err := tx.Commit(); err != nil {
		return model.Domain{}, false, fmt.Errorf("shop domain %s replay commit: %w", action, err)
	}
	return replay.Domain, true, nil
}

func appendDomainOutbox(ctx context.Context, tx *sql.Tx, value model.Domain) error {
	encoded, err := json.Marshal(map[string]any{
		"domainId": value.ID, "merchantId": value.MerchantID, "shopId": value.ShopID,
		"host": value.Host, "scene": value.Scene, "status": value.Status,
		"isPrimary": value.IsPrimary, "version": value.Version,
	})
	if err != nil {
		return err
	}
	id := make([]byte, 18)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO identity_outbox (event_id, aggregate_type, aggregate_id, aggregate_version, event_type, payload_json) VALUES (?, ?, ?, ?, ?, ?)`,
		hex.EncodeToString(id), "shop_domain", fmt.Sprintf("%d", value.ID), value.Version, "identity.shop.domain.changed", encoded)
	return err
}

func randomDomainToken(n int) (string, error) {
	buffer := make([]byte, n)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("shop domain token: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func rejectDomainSceneMismatch(current model.Domain, command model.DomainWriteCommand) error {
	if command.Scene != "" && current.Scene != command.Scene {
		return model.ErrDomainNotFound
	}
	return nil
}

func domainDuplicate(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
