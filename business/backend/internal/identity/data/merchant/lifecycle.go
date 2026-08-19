package mysql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
	identitymysql "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/data/mysql"
)

func (r *Repository) ListMerchantPage(ctx context.Context, query model.Query) (model.Page, error) {
	if r == nil || r.database == nil {
		return model.Page{}, model.ErrUnavailable
	}
	query = query.Normalize()
	where := []string{"m.status<>'CLOSED'"}
	args := []any{}
	if query.Keyword != "" {
		like := "%" + query.Keyword + "%"
		where = append(where, "(m.name LIKE ? OR COALESCE(m.external_id,'') LIKE ? OR COALESCE(c.normalized_identifier,'') LIKE ?)")
		args = append(args, like, like, like)
	}
	if query.Status == string(model.StatusActive) || query.Status == string(model.StatusDisabled) {
		where = append(where, "m.status=?")
		args = append(args, query.Status)
	}
	predicate := strings.Join(where, " AND ")
	var total int64
	if err := r.database.QueryRowContext(ctx, `SELECT COUNT(*)
FROM identity_merchant m
LEFT JOIN identity_workforce_member wm ON wm.merchant_id=m.merchant_id AND wm.member_type='OWNER' AND wm.status<>'REVOKED'
LEFT JOIN identity_credential c ON c.credential_id=(SELECT MIN(c2.credential_id) FROM identity_credential c2 WHERE c2.subject=wm.subject AND c2.status<>'CLOSED')
WHERE `+predicate, args...).Scan(&total); err != nil {
		return model.Page{}, fmt.Errorf("identity merchant page count: %w", err)
	}
	offset := (query.Page - 1) * query.PageSize
	rows, err := r.database.QueryContext(ctx, `SELECT m.merchant_id,m.name,COALESCE(m.external_id,''),COALESCE(m.contact_name,''),COALESCE(m.contact_phone,''),m.status,m.version,
COALESCE(c.normalized_identifier,''),COALESCE(s.shop_id,0),COALESCE(s.code,'')
FROM identity_merchant m
LEFT JOIN identity_workforce_member wm ON wm.merchant_id=m.merchant_id AND wm.member_type='OWNER' AND wm.status<>'REVOKED'
LEFT JOIN identity_credential c ON c.credential_id=(SELECT MIN(c2.credential_id) FROM identity_credential c2 WHERE c2.subject=wm.subject AND c2.status<>'CLOSED')
LEFT JOIN identity_shop s ON s.shop_id=(SELECT MIN(s2.shop_id) FROM identity_shop s2 WHERE s2.merchant_id=m.merchant_id AND s2.status<>'CLOSED')
WHERE `+predicate+` ORDER BY m.merchant_id DESC LIMIT ? OFFSET ?`, append(append([]any{}, args...), query.PageSize, offset)...)
	if err != nil {
		return model.Page{}, fmt.Errorf("identity merchant page: %w", err)
	}
	defer rows.Close()
	items := []model.Record{}
	for rows.Next() {
		var item model.Record
		if err := rows.Scan(&item.ID, &item.Name, &item.ExternalID, &item.ContactName, &item.ContactPhone, &item.Status, &item.Version, &item.Account, &item.ShopID, &item.ShopCode); err != nil {
			return model.Page{}, fmt.Errorf("identity merchant page scan: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return model.Page{}, fmt.Errorf("identity merchant page iterate: %w", err)
	}
	return model.Page{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

func (r *Repository) GetMerchant(ctx context.Context, merchantID int64) (model.Record, error) {
	if r == nil || r.database == nil {
		return model.Record{}, model.ErrUnavailable
	}
	return readMerchantRecord(ctx, r.database, merchantID)
}

func (r *Repository) CreateMerchant(ctx context.Context, command model.CreateCommand) (model.CreateResult, bool, error) {
	tx, err := r.begin(ctx, "create")
	if err != nil {
		return model.CreateResult{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayMerchantCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.CreateResult{}, false, err
	}
	if found {
		return commitMerchantCreateReplay(tx, replay)
	}
	secretHash, err := identitymysql.HashPassword(command.Password)
	if err != nil {
		return model.CreateResult{}, false, model.ErrInvalid
	}
	merchantID, err := nextID(ctx, tx, `SELECT merchant_id FROM identity_merchant ORDER BY merchant_id DESC LIMIT 1 FOR UPDATE`)
	if err != nil {
		return model.CreateResult{}, false, err
	}
	shopID, err := nextID(ctx, tx, `SELECT shop_id FROM identity_shop ORDER BY shop_id DESC LIMIT 1 FOR UPDATE`)
	if err != nil {
		return model.CreateResult{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_merchant(merchant_id,name,contact_name,contact_phone,status,version)
VALUES(?,?,?,?,'ACTIVE',1)`, merchantID, command.Name, command.ContactName, command.ContactPhone); err != nil {
		if duplicateKey(err) {
			return model.CreateResult{}, false, model.ErrConflict
		}
		return model.CreateResult{}, false, fmt.Errorf("identity merchant insert: %w", err)
	}
	org, err := tx.ExecContext(ctx, `INSERT INTO identity_organization(organization_type,merchant_id,name,status,version)
VALUES('MERCHANT',?,?,'ACTIVE',1)`, merchantID, command.Name)
	if err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant organization: %w", err)
	}
	organizationID, err := org.LastInsertId()
	if err != nil || organizationID <= 0 {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant organization id: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_organization_unit(organization_id,organization_unit_id,parent_unit_id,name,status,version)
VALUES(?,1,NULL,?,'ACTIVE',1)`, organizationID, command.Name); err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant organization unit: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_authorization_domain(domain_type,domain_id,revision,entitlement_revision,platform_boundary_revision)
VALUES('MERCHANT',?,1,0,1)`, merchantID); err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant authorization domain: %w", err)
	}
	shopCode := model.ShopCodeForMerchant(merchantID)
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_shop(shop_id,merchant_id,code,name,default_locale,currency,status,version)
VALUES(?,?,?,?,'zh-CN','CNY','ACTIVE',1)`, shopID, merchantID, shopCode, command.Name); err != nil {
		if duplicateKey(err) {
			return model.CreateResult{}, false, model.ErrConflict
		}
		return model.CreateResult{}, false, fmt.Errorf("identity merchant default shop: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_shop_locale(merchant_id,shop_id,locale,published,sort_order) VALUES(?,?,'zh-CN',1,0)`, merchantID, shopID); err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant default shop locale: %w", err)
	}
	subject := model.SubjectForMerchant(merchantID)
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_subject(subject,realm,principal_type,display_name,status,version)
VALUES(?,'MERCHANT','MERCHANT_OWNER',?,'ACTIVE',1)`, subject, command.Name); err != nil {
		if duplicateKey(err) {
			return model.CreateResult{}, false, model.ErrConflict
		}
		return model.CreateResult{}, false, fmt.Errorf("identity merchant subject: %w", err)
	}
	member, err := tx.ExecContext(ctx, `INSERT INTO identity_workforce_member(organization_id,merchant_id,subject,member_type,status,access_version)
VALUES(?,?,?,'OWNER','ACTIVE',1)`, organizationID, merchantID, subject)
	if err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant owner: %w", err)
	}
	memberID, err := member.LastInsertId()
	if err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant owner id: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_organization_membership(organization_id,organization_unit_id,member_id,is_primary,status)
VALUES(?,1,?,1,'ACTIVE')`, organizationID, memberID); err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant membership: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_member_shop(member_id,shop_id,assignment_kind,status)
VALUES(?,?,'OPERATE','ACTIVE')`, memberID, shopID); err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant owner shop: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_credential(subject,namespace_type,credential_kind,normalized_identifier,secret_hash,status,verified_at,version)
VALUES(?,'GLOBAL','USERNAME',?,?,'ACTIVE',CURRENT_TIMESTAMP(3),1)`, subject, command.Account, secretHash); err != nil {
		if duplicateKey(err) {
			return model.CreateResult{}, false, model.ErrConflict
		}
		return model.CreateResult{}, false, fmt.Errorf("identity merchant credential: %w", err)
	}
	result := model.CreateResult{
		Merchant: model.Record{
			ID: merchantID, Name: command.Name, Account: command.Account,
			ContactName: command.ContactName, ContactPhone: command.ContactPhone,
			Status: model.StatusActive, Version: 1, ShopID: shopID, ShopCode: shopCode,
		},
		ShopID: shopID, ShopCode: shopCode, Account: command.Account,
	}
	return completeAndCommitMerchantCreate(ctx, tx, command.CommandKey, result)
}

func (r *Repository) UpdateMerchant(ctx context.Context, command model.UpdateCommand) (model.Record, bool, error) {
	tx, err := r.begin(ctx, "update")
	if err != nil {
		return model.Record{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayMerchantRecordCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Record{}, false, err
	}
	if found {
		return commitMerchantRecordReplay(tx, replay)
	}
	current, err := lockMerchant(ctx, tx, command.MerchantID)
	if err != nil {
		return model.Record{}, false, err
	}
	if current.Status == model.StatusClosed {
		return model.Record{}, false, model.ErrClosed
	}
	if current.Version != command.ExpectedVersion {
		return model.Record{}, false, model.ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_merchant SET name=?,contact_name=?,contact_phone=?,status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=? AND version=? AND status<>'CLOSED'`, command.Name, command.ContactName, command.ContactPhone, command.Status, command.MerchantID, command.ExpectedVersion)
	if err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant update: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Record{}, false, model.ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_organization SET name=?,status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=? AND organization_type='MERCHANT' AND status<>'CLOSED'`, command.Name, command.Status, command.MerchantID); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant organization update: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_subject s JOIN identity_workforce_member wm ON wm.subject=s.subject
SET s.display_name=?,s.updated_at=CURRENT_TIMESTAMP(3) WHERE wm.merchant_id=? AND wm.member_type='OWNER' AND wm.status<>'REVOKED'`, command.Name, command.MerchantID); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant owner name: %w", err)
	}
	saved, err := readMerchantRecord(ctx, tx, command.MerchantID)
	if err != nil {
		return model.Record{}, false, err
	}
	return completeAndCommitMerchantRecord(ctx, tx, command.CommandKey, saved)
}

func (r *Repository) UpdateProfile(ctx context.Context, command model.UpdateProfileCommand) (model.Record, bool, error) {
	tx, err := r.begin(ctx, "update profile")
	if err != nil {
		return model.Record{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayMerchantRecordCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Record{}, false, err
	}
	if found {
		return commitMerchantRecordReplay(tx, replay)
	}
	current, err := lockMerchant(ctx, tx, command.MerchantID)
	if err != nil {
		return model.Record{}, false, err
	}
	if current.Status == model.StatusClosed {
		return model.Record{}, false, model.ErrClosed
	}
	if current.Version != command.ExpectedVersion {
		return model.Record{}, false, model.ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE identity_merchant SET external_id=?,contact_name=?,contact_phone=?,marketing_email_opt_in=?,marketing_sms_opt_in=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=? AND version=? AND status<>'CLOSED'`, command.ExternalID, command.ContactName, command.ContactPhone, command.MarketingEmailOptIn, command.MarketingSMSOptIn, command.MerchantID, command.ExpectedVersion)
	if err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant update profile: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.Record{}, false, model.ErrConflict
	}
	saved, err := readMerchantRecord(ctx, tx, command.MerchantID)
	if err != nil {
		return model.Record{}, false, err
	}
	return completeAndCommitMerchantRecord(ctx, tx, command.CommandKey, saved)
}

func (r *Repository) ResetOwnerPassword(ctx context.Context, command model.ResetPasswordCommand) (bool, error) {
	tx, err := r.begin(ctx, "reset password")
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	if replayed, found, err := insertOrReplayEmptyCommand(ctx, tx, command.CommandKey, command.RequestDigest()); err != nil {
		return false, err
	} else if found {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("identity merchant reset password replay: %w", err)
		}
		return replayed, nil
	}
	current, err := lockMerchant(ctx, tx, command.MerchantID)
	if err != nil {
		return false, err
	}
	if current.Status == model.StatusClosed {
		return false, model.ErrClosed
	}
	var subject string
	var credentialID int64
	err = tx.QueryRowContext(ctx, `SELECT wm.subject,c.credential_id
FROM identity_workforce_member wm
JOIN identity_credential c ON c.subject=wm.subject AND c.status<>'CLOSED'
WHERE wm.merchant_id=? AND wm.member_type='OWNER' AND wm.status<>'REVOKED'
ORDER BY c.credential_id LIMIT 1 FOR UPDATE`, command.MerchantID).Scan(&subject, &credentialID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, model.ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("identity merchant owner credential: %w", err)
	}
	secretHash, err := identitymysql.HashPassword(command.Password)
	if err != nil {
		return false, model.ErrInvalid
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_credential SET secret_hash=?,failed_login_count=0,locked_until=NULL,status='ACTIVE',version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE credential_id=?`, secretHash, credentialID); err != nil {
		return false, fmt.Errorf("identity merchant reset password: %w", err)
	}
	if err := revokeSubjectSessions(ctx, tx, subject, "CREDENTIAL_RESET"); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_merchant_command SET merchant_id=?,response_version=1,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`,
		command.MerchantID, []byte(`{"ok":true}`), command.CommandKey); err != nil {
		return false, fmt.Errorf("identity merchant complete reset password: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("identity merchant commit reset password: %w", err)
	}
	return false, nil
}

func (r *Repository) CloseMerchant(ctx context.Context, command model.CloseCommand) (model.Record, bool, error) {
	tx, err := r.begin(ctx, "close")
	if err != nil {
		return model.Record{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	replay, found, err := insertOrReplayMerchantRecordCommand(ctx, tx, command.CommandKey, command.RequestDigest())
	if err != nil {
		return model.Record{}, false, err
	}
	if found {
		return commitMerchantRecordReplay(tx, replay)
	}
	current, err := lockMerchant(ctx, tx, command.MerchantID)
	if err != nil {
		return model.Record{}, false, err
	}
	if current.Status == model.StatusClosed {
		return model.Record{}, false, model.ErrClosed
	}
	if current.Version != command.ExpectedVersion {
		return model.Record{}, false, model.ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_merchant SET status='CLOSED',closed_at=CURRENT_TIMESTAMP(3),version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=? AND version=? AND status<>'CLOSED'`, command.MerchantID, command.ExpectedVersion); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant close: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_shop SET status='CLOSED',closed_at=CURRENT_TIMESTAMP(3),version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=? AND status<>'CLOSED'`, command.MerchantID); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant close shops: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_organization SET status='CLOSED',closed_at=CURRENT_TIMESTAMP(3),version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=? AND organization_type='MERCHANT' AND status<>'CLOSED'`, command.MerchantID); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant close organization: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_organization_unit u JOIN identity_organization o ON o.organization_id=u.organization_id
SET u.status='CLOSED',u.version=u.version+1,u.updated_at=CURRENT_TIMESTAMP(3)
WHERE o.merchant_id=? AND u.status<>'CLOSED'`, command.MerchantID); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant close units: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT subject FROM identity_workforce_member WHERE merchant_id=? AND status<>'REVOKED'`, command.MerchantID)
	if err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant close members: %w", err)
	}
	subjects := []string{}
	for rows.Next() {
		var subject string
		if err := rows.Scan(&subject); err != nil {
			rows.Close()
			return model.Record{}, false, err
		}
		subjects = append(subjects, subject)
	}
	rows.Close()
	if _, err := tx.ExecContext(ctx, `UPDATE identity_member_shop ms JOIN identity_workforce_member wm ON wm.member_id=ms.member_id
SET ms.status='REVOKED',ms.revoked_at=CURRENT_TIMESTAMP(3) WHERE wm.merchant_id=? AND ms.status='ACTIVE'`, command.MerchantID); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant revoke shops: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_workforce_member SET status='REVOKED',revoked_at=CURRENT_TIMESTAMP(3),updated_at=CURRENT_TIMESTAMP(3)
WHERE merchant_id=? AND status<>'REVOKED'`, command.MerchantID); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant revoke members: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_credential c JOIN identity_workforce_member wm ON wm.subject=c.subject
SET c.status='CLOSED',c.updated_at=CURRENT_TIMESTAMP(3) WHERE wm.merchant_id=? AND c.status<>'CLOSED'`, command.MerchantID); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant close credentials: %w", err)
	}
	for _, subject := range subjects {
		if err := revokeSubjectSessions(ctx, tx, subject, "MERCHANT_CLOSED"); err != nil {
			return model.Record{}, false, err
		}
	}
	saved, err := readMerchantRecord(ctx, tx, command.MerchantID)
	if err != nil {
		return model.Record{}, false, err
	}
	return completeAndCommitMerchantRecord(ctx, tx, command.CommandKey, saved)
}

func (r *Repository) begin(ctx context.Context, operation string) (*sql.Tx, error) {
	if r == nil || r.database == nil {
		return nil, model.ErrUnavailable
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("identity merchant begin %s: %w", operation, err)
	}
	return tx, nil
}

func nextID(ctx context.Context, tx *sql.Tx, query string) (int64, error) {
	var current sql.NullInt64
	if err := tx.QueryRowContext(ctx, query).Scan(&current); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("identity merchant allocate id: %w", err)
	}
	if !current.Valid || current.Int64 <= 0 {
		return 1, nil
	}
	return current.Int64 + 1, nil
}

func lockMerchant(ctx context.Context, tx *sql.Tx, merchantID int64) (model.Record, error) {
	var value model.Record
	err := tx.QueryRowContext(ctx, `SELECT merchant_id,name,COALESCE(external_id,''),COALESCE(contact_name,''),COALESCE(contact_phone,''),marketing_email_opt_in,marketing_sms_opt_in,status,version
FROM identity_merchant WHERE merchant_id=? FOR UPDATE`, merchantID).
		Scan(&value.ID, &value.Name, &value.ExternalID, &value.ContactName, &value.ContactPhone, &value.MarketingEmailOptIn, &value.MarketingSMSOptIn, &value.Status, &value.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Record{}, model.ErrNotFound
	}
	if err != nil {
		return model.Record{}, fmt.Errorf("identity merchant lock: %w", err)
	}
	return value, nil
}

func readMerchantRecord(ctx context.Context, query interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, merchantID int64) (model.Record, error) {
	var value model.Record
	err := query.QueryRowContext(ctx, `SELECT m.merchant_id,m.name,COALESCE(m.external_id,''),COALESCE(m.contact_name,''),COALESCE(m.contact_phone,''),m.marketing_email_opt_in,m.marketing_sms_opt_in,m.status,m.version,
COALESCE(c.normalized_identifier,''),COALESCE(s.shop_id,0),COALESCE(s.code,'')
FROM identity_merchant m
LEFT JOIN identity_workforce_member wm ON wm.merchant_id=m.merchant_id AND wm.member_type='OWNER' AND wm.status<>'REVOKED'
LEFT JOIN identity_credential c ON c.credential_id=(SELECT MIN(c2.credential_id) FROM identity_credential c2 WHERE c2.subject=wm.subject AND c2.status<>'CLOSED')
LEFT JOIN identity_shop s ON s.shop_id=(SELECT MIN(s2.shop_id) FROM identity_shop s2 WHERE s2.merchant_id=m.merchant_id AND s2.status<>'CLOSED')
WHERE m.merchant_id=?`, merchantID).
		Scan(&value.ID, &value.Name, &value.ExternalID, &value.ContactName, &value.ContactPhone, &value.MarketingEmailOptIn, &value.MarketingSMSOptIn, &value.Status, &value.Version, &value.Account, &value.ShopID, &value.ShopCode)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Record{}, model.ErrNotFound
	}
	if err != nil {
		return model.Record{}, fmt.Errorf("identity merchant read: %w", err)
	}
	return value, nil
}

func insertOrReplayMerchantCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) (model.CreateResult, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_merchant_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !duplicateKey(err) {
		if err != nil {
			return model.CreateResult{}, false, fmt.Errorf("identity merchant insert command: %w", err)
		}
		return model.CreateResult{}, false, nil
	}
	var stored []byte
	var document []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_json FROM identity_merchant_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &document); err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant read command: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return model.CreateResult{}, false, model.ErrIdempotency
	}
	var replay model.CreateResult
	if len(document) == 0 || json.Unmarshal(document, &replay) != nil || replay.Merchant.ID <= 0 {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant command response is incomplete")
	}
	return replay, true, nil
}

func insertOrReplayMerchantRecordCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) (model.Record, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_merchant_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !duplicateKey(err) {
		if err != nil {
			return model.Record{}, false, fmt.Errorf("identity merchant insert command: %w", err)
		}
		return model.Record{}, false, nil
	}
	var stored []byte
	var document []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash,response_json FROM identity_merchant_command WHERE command_key=? FOR UPDATE`, key).
		Scan(&stored, &document); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant read command: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return model.Record{}, false, model.ErrIdempotency
	}
	var replay model.Record
	if len(document) == 0 || json.Unmarshal(document, &replay) != nil || replay.ID <= 0 {
		return model.Record{}, false, fmt.Errorf("identity merchant command response is incomplete")
	}
	return replay, true, nil
}

func insertOrReplayEmptyCommand(ctx context.Context, tx *sql.Tx, key string, digest [32]byte) (bool, bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_merchant_command(command_key,request_hash) VALUES(?,?)`, key, digest[:])
	if !duplicateKey(err) {
		if err != nil {
			return false, false, fmt.Errorf("identity merchant insert command: %w", err)
		}
		return false, false, nil
	}
	var stored []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_hash FROM identity_merchant_command WHERE command_key=? FOR UPDATE`, key).Scan(&stored); err != nil {
		return false, false, fmt.Errorf("identity merchant read command: %w", err)
	}
	if len(stored) != len(digest) || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return false, false, model.ErrIdempotency
	}
	return true, true, nil
}

func completeAndCommitMerchantCreate(ctx context.Context, tx *sql.Tx, key string, value model.CreateResult) (model.CreateResult, bool, error) {
	document, err := json.Marshal(value)
	if err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant encode create: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_merchant_command SET merchant_id=?,response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`,
		value.Merchant.ID, value.Merchant.Version, document, key); err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant complete create: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant commit create: %w", err)
	}
	return value, false, nil
}

func completeAndCommitMerchantRecord(ctx context.Context, tx *sql.Tx, key string, value model.Record) (model.Record, bool, error) {
	document, err := json.Marshal(value)
	if err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant encode record: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_merchant_command SET merchant_id=?,response_version=?,response_json=?,completed_at=CURRENT_TIMESTAMP(3) WHERE command_key=?`,
		value.ID, value.Version, document, key); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant complete record: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant commit record: %w", err)
	}
	return value, false, nil
}

func commitMerchantCreateReplay(tx *sql.Tx, value model.CreateResult) (model.CreateResult, bool, error) {
	if err := tx.Commit(); err != nil {
		return model.CreateResult{}, false, fmt.Errorf("identity merchant commit create replay: %w", err)
	}
	return value, true, nil
}

func commitMerchantRecordReplay(tx *sql.Tx, value model.Record) (model.Record, bool, error) {
	if err := tx.Commit(); err != nil {
		return model.Record{}, false, fmt.Errorf("identity merchant commit record replay: %w", err)
	}
	return value, true, nil
}

func revokeSubjectSessions(ctx context.Context, tx *sql.Tx, subject, reason string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE identity_session SET status='REVOKED',revoked_at=CURRENT_TIMESTAMP(3),revoke_reason=? WHERE subject=? AND status='ACTIVE'`, reason, subject); err != nil {
		return fmt.Errorf("identity merchant revoke sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_refresh_token rt JOIN identity_session s ON s.session_id=rt.session_id SET rt.status='REVOKED' WHERE s.subject=? AND rt.status='ACTIVE'`, subject); err != nil {
		return fmt.Errorf("identity merchant revoke refresh: %w", err)
	}
	return nil
}

func duplicateKey(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
