package mysql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lvtuopen-ai/kernel-go/principal"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

type DirectoryRepository struct{ database *sql.DB }

var _ biz.DirectoryRepository = (*DirectoryRepository)(nil)

func NewDirectoryRepository(database *sql.DB) (*DirectoryRepository, error) {
	if database == nil {
		return nil, model.ErrUnavailable
	}
	return &DirectoryRepository{database: database}, nil
}

func (r *DirectoryRepository) ListOrganizationDirectory(ctx context.Context, organizationID, merchantID int64) (biz.OrganizationDirectory, error) {
	var result biz.OrganizationDirectory
	var orgType string
	err := r.database.QueryRowContext(ctx, `SELECT organization_id,organization_type,COALESCE(merchant_id,0),name,status,version FROM identity_organization WHERE organization_id=? AND (COALESCE(merchant_id,0)=? OR ?=0)`, organizationID, merchantID, merchantID).Scan(&result.Organization.ID, &orgType, &result.Organization.MerchantID, &result.Organization.Name, &result.Organization.Status, &result.Organization.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return result, model.ErrNotFound
	}
	if err != nil {
		return result, err
	}
	result.Organization.Type = model.OrganizationType(orgType)
	rows, err := r.database.QueryContext(ctx, `SELECT organization_unit_id,COALESCE(parent_unit_id,0),name,status,version FROM identity_organization_unit WHERE organization_id=? AND status<>'CLOSED' ORDER BY organization_unit_id`, organizationID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item biz.OrganizationUnit
		if err := rows.Scan(&item.ID, &item.ParentID, &item.Name, &item.Status, &item.Version); err != nil {
			rows.Close()
			return result, err
		}
		result.Units = append(result.Units, item)
	}
	rows.Close()
	rows, err = r.database.QueryContext(ctx, `SELECT m.member_id,m.organization_id,COALESCE(m.merchant_id,0),m.subject,m.member_type,m.status,m.access_version,COALESCE(m.legacy_staff_id,0),s.display_name,s.principal_type,s.status,s.version,COALESCE(c.credential_id,0),COALESCE(c.version,0),COALESCE(c.credential_kind,''),COALESCE(c.normalized_identifier,''),COALESCE(c.status,'') FROM identity_workforce_member m JOIN identity_subject s ON s.subject=m.subject LEFT JOIN identity_credential c ON c.credential_id=(SELECT MIN(c2.credential_id) FROM identity_credential c2 WHERE c2.subject=s.subject AND c2.status<>'CLOSED') WHERE m.organization_id=? AND m.status<>'REVOKED' ORDER BY m.member_id`, organizationID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item biz.MemberDirectoryItem
		var pt string
		if err := rows.Scan(&item.Member.ID, &item.Member.OrganizationID, &item.Member.MerchantID, &item.Member.Subject, &item.Member.Type, &item.Member.Status, &item.Member.AccessVersion, &item.Member.LegacyStaffID, &item.DisplayName, &pt, &item.SubjectStatus, &item.SubjectVersion, &item.Credential.ID, &item.Credential.Version, &item.Credential.Kind, &item.Credential.Identifier, &item.Credential.Status); err != nil {
			rows.Close()
			return result, err
		}
		item.PrincipalType = principal.Type(pt)
		item.ShopIDs, _ = queryIDs(ctx, r.database, `SELECT shop_id FROM identity_member_shop WHERE member_id=? AND status='ACTIVE' ORDER BY shop_id`, item.Member.ID)
		item.UnitIDs, _ = queryIDs(ctx, r.database, `SELECT organization_unit_id FROM identity_organization_membership WHERE member_id=? AND status='ACTIVE' ORDER BY organization_unit_id`, item.Member.ID)
		result.Members = append(result.Members, item)
	}
	rows.Close()
	if merchantID > 0 {
		rows, err = r.database.QueryContext(ctx, `SELECT merchant_id,shop_id,name,code,status,version FROM identity_shop WHERE merchant_id=? AND status<>'CLOSED' ORDER BY shop_id`, merchantID)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var item biz.ShopDirectoryItem
			if err := rows.Scan(&item.Context.MerchantID, &item.Context.ShopID, &item.Name, &item.Code, &item.Status, &item.Version); err != nil {
				rows.Close()
				return result, err
			}
			result.Shops = append(result.Shops, item)
		}
		rows.Close()
	}
	return result, nil
}

func (r *DirectoryRepository) ResolvePrincipalContext(ctx context.Context, subject string) (model.PrincipalContext, error) {
	var resolved model.PrincipalContext
	var realm, principalType, subjectStatus string
	var memberID, organizationID, merchantID, legacyStaffID sql.NullInt64
	var memberType, memberStatus, organizationType, organizationName, organizationStatus sql.NullString
	var accessVersion, organizationVersion sql.NullInt64
	err := r.database.QueryRowContext(ctx, `
SELECT s.subject, s.realm, s.principal_type, s.display_name, COALESCE(s.legacy_uid, 0), s.status, s.version,
       m.member_id, m.organization_id, m.merchant_id, m.member_type, m.status, m.access_version,
       m.legacy_staff_id,
       o.organization_type, o.name, o.status, o.version
FROM identity_subject s
LEFT JOIN identity_workforce_member m ON m.subject = s.subject AND m.status <> 'REVOKED'
LEFT JOIN identity_organization o ON o.organization_id = m.organization_id
WHERE s.subject = ?`, subject).Scan(
		&resolved.Subject.ID, &realm, &principalType, &resolved.Subject.DisplayName, &resolved.Subject.LegacyUID,
		&subjectStatus, &resolved.Subject.Version, &memberID, &organizationID, &merchantID,
		&memberType, &memberStatus, &accessVersion, &legacyStaffID,
		&organizationType, &organizationName, &organizationStatus, &organizationVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.PrincipalContext{}, model.ErrNotFound
	}
	if err != nil {
		return model.PrincipalContext{}, fmt.Errorf("identity: resolve principal: %w", err)
	}
	resolved.Subject.Realm = principal.Realm(realm)
	resolved.Subject.PrincipalType = principal.Type(principalType)
	resolved.Subject.Status = model.Status(subjectStatus)
	if memberID.Valid {
		resolved.Member.ID = memberID.Int64
		resolved.Member.OrganizationID = organizationID.Int64
		resolved.Member.MerchantID = merchantID.Int64
		resolved.Member.Subject = subject
		resolved.Member.Type = model.MemberType(memberType.String)
		resolved.Member.Status = model.MemberStatus(memberStatus.String)
		resolved.Member.AccessVersion = uint64(accessVersion.Int64)
		resolved.Member.LegacyStaffID = legacyStaffID.Int64
		resolved.Organization.ID = organizationID.Int64
		resolved.Organization.Type = model.OrganizationType(organizationType.String)
		resolved.Organization.MerchantID = merchantID.Int64
		resolved.Organization.Name = organizationName.String
		resolved.Organization.Status = model.Status(organizationStatus.String)
		resolved.Organization.Version = uint64(organizationVersion.Int64)
	}
	if err := resolved.Subject.Validate(); err != nil {
		return model.PrincipalContext{}, model.ErrInactive
	}
	if memberID.Valid {
		units, err := queryIDs(ctx, r.database, `
SELECT organization_unit_id FROM identity_organization_membership
WHERE member_id = ? AND status = 'ACTIVE' ORDER BY organization_unit_id`, memberID.Int64)
		if err != nil {
			return model.PrincipalContext{}, err
		}
		resolved.OrganizationUnitIDs = units
	}
	shops, err := r.principalShops(ctx, resolved)
	if err != nil {
		return model.PrincipalContext{}, err
	}
	resolved.AvailableShops = shops
	return resolved, nil
}

func (r *DirectoryRepository) ValidateActiveSession(ctx context.Context, sessionID, subject string, selected model.SelectedContext, expectedContextVersion uint64) error {
	var found int
	err := r.database.QueryRowContext(ctx, `SELECT 1 FROM identity_session
WHERE session_id=? AND subject=? AND status='ACTIVE' AND expires_at>CURRENT_TIMESTAMP(3)
AND context_version=?
AND COALESCE(selected_organization_id,0)=? AND COALESCE(selected_merchant_id,0)=?
AND COALESCE(selected_shop_id,0)=?`,
		sessionID, subject, expectedContextVersion, selected.OrganizationID, selected.MerchantID,
		selected.ShopID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrInactive
	}
	if err != nil {
		return fmt.Errorf("identity: validate active session: %w", err)
	}
	return nil
}

func (r *DirectoryRepository) principalShops(ctx context.Context, resolved model.PrincipalContext) ([]model.ShopContext, error) {
	if resolved.Subject.PrincipalType == principal.TypeCustomer {
		rows, err := r.database.QueryContext(ctx, `SELECT s.merchant_id,s.shop_id
FROM identity_credential c JOIN identity_shop s ON s.shop_id=c.shop_id
WHERE c.subject=? AND c.namespace_type='SHOP' AND c.status='ACTIVE' AND s.status='ACTIVE' ORDER BY s.shop_id`, resolved.Subject.ID)
		if err != nil {
			return nil, fmt.Errorf("identity: list customer shops: %w", err)
		}
		defer rows.Close()
		var shops []model.ShopContext
		for rows.Next() {
			var shop model.ShopContext
			if err := rows.Scan(&shop.MerchantID, &shop.ShopID); err != nil {
				return nil, err
			}
			shops = append(shops, shop)
		}
		return shops, rows.Err()
	}
	if resolved.Subject.PrincipalType == principal.TypeGuest {
		return nil, nil
	}
	if resolved.Member.Type == model.MemberOperator {
		return nil, nil
	}
	query := `SELECT s.merchant_id, s.shop_id
FROM identity_shop s WHERE s.merchant_id = ? AND s.status = 'ACTIVE' ORDER BY s.shop_id`
	args := []any{resolved.Member.MerchantID}
	if resolved.Member.Type != model.MemberOwner {
		query = `SELECT s.merchant_id, s.shop_id
FROM identity_member_shop ms JOIN identity_shop s ON s.shop_id = ms.shop_id
WHERE ms.member_id = ? AND ms.status = 'ACTIVE' AND s.status = 'ACTIVE' ORDER BY s.shop_id`
		args = []any{resolved.Member.ID}
	}
	rows, err := r.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("identity: list principal shops: %w", err)
	}
	defer rows.Close()
	var shops []model.ShopContext
	for rows.Next() {
		var shop model.ShopContext
		if err := rows.Scan(&shop.MerchantID, &shop.ShopID); err != nil {
			return nil, fmt.Errorf("identity: scan principal shop: %w", err)
		}
		shops = append(shops, shop)
	}
	return shops, rows.Err()
}

func (r *DirectoryRepository) ResolveShopByID(ctx context.Context, shopID int64) (model.ShopResolution, error) {
	return r.resolveShop(ctx, "s.shop_id = ?", shopID)
}

func (r *DirectoryRepository) resolveShop(ctx context.Context, condition string, args ...any) (model.ShopResolution, error) {
	var resolved model.ShopResolution
	err := r.database.QueryRowContext(ctx, `SELECT s.merchant_id, s.shop_id, s.currency, s.status, s.version
FROM identity_shop s WHERE `+condition, args...).Scan(&resolved.Context.MerchantID, &resolved.Context.ShopID, &resolved.Currency, &resolved.Status, &resolved.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ShopResolution{}, model.ErrNotFound
	}
	if err != nil {
		return model.ShopResolution{}, fmt.Errorf("identity: resolve shop: %w", err)
	}
	if err := resolved.Validate(); err != nil {
		return model.ShopResolution{}, fmt.Errorf("identity: resolve shop contract: %w", err)
	}
	return resolved, nil
}

func (r *DirectoryRepository) ListOrganizationSubtree(ctx context.Context, organizationID, rootUnitID int64) ([]int64, uint64, error) {
	ids, err := queryIDs(ctx, r.database, `WITH RECURSIVE subtree AS (
 SELECT organization_unit_id FROM identity_organization_unit
 WHERE organization_id = ? AND organization_unit_id = ? AND status = 'ACTIVE'
 UNION ALL
 SELECT u.organization_unit_id FROM identity_organization_unit u JOIN subtree p ON u.parent_unit_id = p.organization_unit_id
 WHERE u.organization_id = ? AND u.status = 'ACTIVE'
) SELECT organization_unit_id FROM subtree ORDER BY organization_unit_id`, organizationID, rootUnitID, organizationID)
	if err != nil {
		return nil, 0, err
	}
	var version uint64
	if err := r.database.QueryRowContext(ctx, `SELECT version FROM identity_organization WHERE organization_id = ?`, organizationID).Scan(&version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, model.ErrNotFound
		}
		return nil, 0, fmt.Errorf("identity: organization version: %w", err)
	}
	return ids, version, nil
}

func (r *DirectoryRepository) BatchGetSubjects(ctx context.Context, subjects []string) ([]model.Subject, error) {
	if len(subjects) == 0 {
		return []model.Subject{}, nil
	}
	if len(subjects) > 200 {
		return nil, model.ErrConflict
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(subjects)), ",")
	args := make([]any, len(subjects))
	for i := range subjects {
		args[i] = subjects[i]
	}
	rows, err := r.database.QueryContext(ctx, `SELECT subject, realm, principal_type, display_name, COALESCE(legacy_uid, 0), status, version
FROM identity_subject WHERE subject IN (`+placeholders+`) ORDER BY subject`, args...)
	if err != nil {
		return nil, fmt.Errorf("identity: batch subjects: %w", err)
	}
	defer rows.Close()
	var result []model.Subject
	for rows.Next() {
		var item model.Subject
		if err := rows.Scan(&item.ID, &item.Realm, &item.PrincipalType, &item.DisplayName, &item.LegacyUID, &item.Status, &item.Version); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *DirectoryRepository) ResolveLegacySubjects(ctx context.Context, realm principal.Realm, legacyUIDs []int64) ([]model.Subject, error) {
	if len(legacyUIDs) == 0 {
		return []model.Subject{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(legacyUIDs)), ",")
	args := make([]any, 0, len(legacyUIDs)+1)
	args = append(args, realm.String())
	for _, legacyUID := range legacyUIDs {
		args = append(args, legacyUID)
	}
	rows, err := r.database.QueryContext(ctx, `SELECT subject, realm, principal_type, display_name, legacy_uid, status, version
FROM identity_subject WHERE realm=? AND legacy_uid IN (`+placeholders+`) ORDER BY legacy_uid`, args...)
	if err != nil {
		return nil, fmt.Errorf("identity: resolve legacy subjects: %w", err)
	}
	defer rows.Close()
	result := make([]model.Subject, 0, len(legacyUIDs))
	for rows.Next() {
		var item model.Subject
		if err := rows.Scan(&item.ID, &item.Realm, &item.PrincipalType, &item.DisplayName, &item.LegacyUID, &item.Status, &item.Version); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *DirectoryRepository) CreateOrganizationUnit(ctx context.Context, command biz.CreateOrganizationUnit) (biz.OrganizationUnitResult, error) {
	hash, err := biz.CommandHash(command)
	if err != nil {
		return biz.OrganizationUnitResult{}, err
	}
	result := biz.OrganizationUnitResult{OrganizationID: command.OrganizationID, UnitID: command.UnitID, Version: command.ExpectedVersion + 1}
	err = r.transaction(ctx, func(tx *sql.Tx) error {
		replayed, err := reserveIdempotency(ctx, tx, "create_organization_unit", command.IdempotencyKey, hash[:], &result)
		if err != nil || replayed {
			return err
		}
		update, err := tx.ExecContext(ctx, `UPDATE identity_organization SET version = version + 1, updated_at = CURRENT_TIMESTAMP(3)
WHERE organization_id = ? AND version = ? AND status = 'ACTIVE'`, command.OrganizationID, command.ExpectedVersion)
		if err != nil {
			return err
		}
		if affected, _ := update.RowsAffected(); affected != 1 {
			return model.ErrConflict
		}
		var parent any
		if command.ParentUnitID > 0 {
			parent = command.ParentUnitID
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_organization_unit
(organization_id, organization_unit_id, parent_unit_id, name, status, version) VALUES (?, ?, ?, ?, 'ACTIVE', 1)`, command.OrganizationID, command.UnitID, parent, command.Name); err != nil {
			return mapConflict(err)
		}
		if err := appendOutbox(ctx, tx, "organization", fmt.Sprint(command.OrganizationID), result.Version, "identity.organization.changed", result); err != nil {
			return err
		}
		return completeIdempotency(ctx, tx, "create_organization_unit", command.IdempotencyKey, result)
	})
	return result, err
}

func (r *DirectoryRepository) ProvisionMember(ctx context.Context, command biz.ProvisionMember) (biz.ProvisionMemberResult, error) {
	result := biz.ProvisionMemberResult{Subject: command.Subject, Status: model.MemberActive, AccessVersion: 1, OperationID: command.OperationID}
	err := r.transaction(ctx, func(tx *sql.Tx) error {
		replayed, err := reserveIdempotency(ctx, tx, "provision_member", command.IdempotencyKey, command.RequestHash[:], &result)
		if err != nil || replayed {
			return err
		}
		// Password hashing is intentionally after the durable idempotency
		// reservation. A replay returns the original result without generating a
		// fresh salted hash, while a rolled-back first attempt can safely retry.
		secretHash, err := HashPassword(command.Password)
		if err != nil {
			return err
		}
		var entitlementRevision uint64
		if err := tx.QueryRowContext(ctx, `SELECT entitlement_revision FROM identity_authorization_domain WHERE domain_type='MERCHANT' AND domain_id=? FOR UPDATE`, command.MerchantID).Scan(&entitlementRevision); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			return model.ErrEntitlementUnavailable
		}
		if entitlementRevision == 0 {
			return model.ErrEntitlementUnavailable
		}
		for _, roleID := range command.RoleIDs {
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_authorization_role ar WHERE ar.domain_type='MERCHANT' AND ar.domain_id=? AND ar.role_id=? AND ar.status='ACTIVE' AND NOT EXISTS (SELECT 1 FROM identity_authorization_role_permission rp WHERE rp.domain_type=ar.domain_type AND rp.domain_id=ar.domain_id AND rp.role_id=ar.role_id AND NOT EXISTS (SELECT 1 FROM identity_entitlement_projection ep WHERE ep.merchant_id=ar.domain_id AND ep.permission_code=rp.permission_code AND ep.status='ACTIVE' AND ep.entitlement_revision=?))`, command.MerchantID, roleID, entitlementRevision).Scan(&count); err != nil {
				return err
			}
			if count != 1 {
				return model.ErrAuthorizationInvalid
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_subject
(subject, realm, principal_type, display_name, status, version) VALUES (?, ?, ?, ?, 'ACTIVE', 1)`, command.Subject, command.Realm, command.PrincipalType, command.DisplayName); err != nil {
			return mapConflict(err)
		}
		memberResult, err := tx.ExecContext(ctx, `INSERT INTO identity_workforce_member
(organization_id, merchant_id, subject, member_type, status, access_version)
VALUES (?, NULLIF(?, 0), ?, ?, 'ACTIVE', 1)`, command.OrganizationID, command.MerchantID, command.Subject, command.MemberType)
		if err != nil {
			return mapConflict(err)
		}
		result.MemberID, err = memberResult.LastInsertId()
		if err != nil {
			return err
		}
		for _, unitID := range command.OrganizationUnitIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO identity_organization_membership
(organization_id, organization_unit_id, member_id, is_primary, status) VALUES (?, ?, ?, ?, 'ACTIVE')`, command.OrganizationID, unitID, result.MemberID, unitID == command.OrganizationUnitIDs[0]); err != nil {
				return mapConflict(err)
			}
		}
		if err := insertAssignments(ctx, tx, result.MemberID, command.MerchantID, command.ShopIDs, command.AssignmentKind); err != nil {
			return err
		}
		var credentialMerchantID, credentialShopID any
		if command.CredentialNamespace == "SHOP" {
			var shopID int64
			if err := tx.QueryRowContext(ctx, `SELECT shop_id FROM identity_shop WHERE shop_id=? AND merchant_id=? AND status='ACTIVE'`, command.ShopIDs[0], command.MerchantID).Scan(&shopID); err != nil {
				return model.ErrInvalidAssignment
			}
			credentialMerchantID = command.MerchantID
			credentialShopID = shopID
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_credential
(subject, namespace_type, merchant_id, shop_id, credential_kind, normalized_identifier, secret_hash, status, version)
VALUES (?, ?, ?, ?, ?, ?, ?, 'ACTIVE', 1)`, command.Subject, command.CredentialNamespace, credentialMerchantID, credentialShopID, command.CredentialKind, command.NormalizedIdentifier, secretHash); err != nil {
			return mapConflict(err)
		}
		for _, roleID := range command.RoleIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO identity_subject_grant(grant_id,domain_type,domain_id,subject,role_id,status,access_version,operation_id) VALUES(?,'MERCHANT',?,?,?,'ACTIVE',1,?)`, randomID(), command.MerchantID, command.Subject, roleID, command.OperationID); err != nil {
				return mapConflict(err)
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE identity_authorization_domain SET revision=revision+1,updated_at=CURRENT_TIMESTAMP(3) WHERE domain_type='MERCHANT' AND domain_id=?`, command.MerchantID); err != nil {
			return err
		}
		event := map[string]any{"operationId": command.OperationID, "memberId": result.MemberID, "subject": command.Subject, "accessVersion": 1}
		if err := appendOutbox(ctx, tx, "workforce", fmt.Sprint(result.MemberID), 1, "identity.workforce.changed", event); err != nil {
			return err
		}
		return completeIdempotency(ctx, tx, "provision_member", command.IdempotencyKey, result)
	})
	return result, err
}

func (r *DirectoryRepository) ReplaceMemberAccess(ctx context.Context, command biz.ReplaceMemberAccess) (biz.ProvisionMemberResult, error) {
	hash, err := biz.CommandHash(command)
	if err != nil {
		return biz.ProvisionMemberResult{}, err
	}
	result := biz.ProvisionMemberResult{MemberID: command.MemberID, Status: model.MemberActive, AccessVersion: command.ExpectedAccessVersion + 1, OperationID: command.OperationID}
	err = r.transaction(ctx, func(tx *sql.Tx) error {
		replayed, err := reserveIdempotency(ctx, tx, "replace_member_access", command.IdempotencyKey, hash[:], &result)
		if err != nil || replayed {
			return err
		}
		var merchantID int64
		var memberType model.MemberType
		var subject string
		if err := tx.QueryRowContext(ctx, `SELECT merchant_id, member_type, subject FROM identity_workforce_member WHERE member_id = ? FOR UPDATE`, command.MemberID).Scan(&merchantID, &memberType, &subject); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return model.ErrNotFound
			}
			return err
		}
		if memberType == model.MemberOwner {
			return model.ErrProtectedOwner
		}
		update, err := tx.ExecContext(ctx, `UPDATE identity_workforce_member SET access_version = access_version + 1, updated_at = CURRENT_TIMESTAMP(3)
WHERE member_id = ? AND access_version = ? AND status = 'ACTIVE'`, command.MemberID, command.ExpectedAccessVersion)
		if err != nil {
			return err
		}
		if affected, _ := update.RowsAffected(); affected != 1 {
			return model.ErrConflict
		}
		if _, err := tx.ExecContext(ctx, `UPDATE identity_organization_membership SET status = 'REVOKED', revoked_at = CURRENT_TIMESTAMP(3) WHERE member_id = ? AND status = 'ACTIVE'`, command.MemberID); err != nil {
			return err
		}
		var organizationID int64
		if err := tx.QueryRowContext(ctx, `SELECT organization_id FROM identity_workforce_member WHERE member_id = ?`, command.MemberID).Scan(&organizationID); err != nil {
			return err
		}
		for i, unitID := range command.OrganizationUnitIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO identity_organization_membership (organization_id, organization_unit_id, member_id, is_primary, status) VALUES (?, ?, ?, ?, 'ACTIVE') ON DUPLICATE KEY UPDATE is_primary=VALUES(is_primary), status='ACTIVE', revoked_at=NULL`, organizationID, unitID, command.MemberID, i == 0); err != nil {
				return mapConflict(err)
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE identity_member_shop SET status='REVOKED', revoked_at=CURRENT_TIMESTAMP(3) WHERE member_id=? AND status='ACTIVE'`, command.MemberID); err != nil {
			return err
		}
		if err := insertAssignments(ctx, tx, command.MemberID, merchantID, command.ShopIDs, command.AssignmentKind); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE identity_session SET status='REVOKED', revoked_at=CURRENT_TIMESTAMP(3), revoke_reason='ACCESS_CHANGED' WHERE subject=? AND status='ACTIVE'`, subject); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE identity_refresh_token rt JOIN identity_session s ON s.session_id=rt.session_id SET rt.status='REVOKED' WHERE s.subject=? AND rt.status='ACTIVE'`, subject); err != nil {
			return err
		}
		grants, err := tx.ExecContext(ctx, `UPDATE identity_subject_grant SET access_version=? WHERE domain_type='MERCHANT' AND domain_id=? AND subject=? AND status='ACTIVE' AND access_version=?`, result.AccessVersion, merchantID, subject, command.ExpectedAccessVersion)
		if err != nil {
			return err
		}
		if affected, _ := grants.RowsAffected(); affected > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE identity_authorization_domain SET revision=revision+1,updated_at=CURRENT_TIMESTAMP(3) WHERE domain_type='MERCHANT' AND domain_id=?`, merchantID); err != nil {
				return err
			}
		}
		event := map[string]any{"operationId": command.OperationID, "memberId": command.MemberID, "subject": subject, "accessVersion": result.AccessVersion}
		if err := appendOutbox(ctx, tx, "workforce", fmt.Sprint(command.MemberID), result.AccessVersion, "identity.workforce.changed", event); err != nil {
			return err
		}
		return completeIdempotency(ctx, tx, "replace_member_access", command.IdempotencyKey, result)
	})
	return result, err
}

func (r *DirectoryRepository) UpdateMember(ctx context.Context, command biz.UpdateMember) (biz.UpdateMemberResult, error) {
	result := biz.UpdateMemberResult{
		Subject: command.Subject, Status: model.MemberActive,
		IdentityVersion: command.ExpectedIdentityVersion, AccessVersion: command.ExpectedAccessVersion,
		OperationID: command.OperationID,
	}
	err := r.transaction(ctx, func(tx *sql.Tx) error {
		replayed, err := reserveIdempotency(ctx, tx, "update_member", command.IdempotencyKey, command.RequestHash[:], &result)
		if err != nil || replayed {
			return err
		}
		var memberID, organizationID, merchantID int64
		var memberType model.MemberType
		var memberStatus model.MemberStatus
		var accessVersion uint64
		var subject string
		err = tx.QueryRowContext(ctx, `SELECT member_id, organization_id, merchant_id, member_type, status, access_version, subject FROM identity_workforce_member WHERE merchant_id=? AND subject=? AND member_type IN ('STAFF','ANCHOR') AND status<>'REVOKED' FOR UPDATE`, command.MerchantID, command.Subject).Scan(&memberID, &organizationID, &merchantID, &memberType, &memberStatus, &accessVersion, &subject)
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrNotFound
		}
		if err != nil {
			return err
		}
		if memberType != command.MemberType {
			return model.ErrConflict
		}
		var displayName string
		var identityVersion uint64
		if err := tx.QueryRowContext(ctx, `SELECT display_name, version FROM identity_subject WHERE subject=? FOR UPDATE`, subject).Scan(&displayName, &identityVersion); err != nil {
			return err
		}
		if identityVersion != command.ExpectedIdentityVersion || accessVersion != command.ExpectedAccessVersion {
			return model.ErrConflict
		}
		currentUnits, err := queryIDs(ctx, tx, `SELECT organization_unit_id FROM identity_organization_membership WHERE member_id=? AND status='ACTIVE' ORDER BY organization_unit_id`, memberID)
		if err != nil {
			return err
		}
		currentShops, err := queryIDs(ctx, tx, `SELECT shop_id FROM identity_member_shop WHERE member_id=? AND status='ACTIVE' ORDER BY shop_id`, memberID)
		if err != nil {
			return err
		}
		currentRoles, err := queryIDs(ctx, tx, `SELECT role_id FROM identity_subject_grant WHERE domain_type='MERCHANT' AND domain_id=? AND subject=? AND status='ACTIVE' ORDER BY role_id`, merchantID, subject)
		if err != nil {
			return err
		}
		nameChanged := displayName != command.DisplayName
		unitsChanged := !sameIDs(currentUnits, command.OrganizationUnitIDs)
		shopsChanged := !sameIDs(currentShops, command.ShopIDs)
		rolesChanged := !sameIDs(currentRoles, command.RoleIDs)
		accessChanged := unitsChanged || shopsChanged || rolesChanged
		nextIdentity, nextAccess := identityVersion, accessVersion
		if nameChanged {
			update, err := tx.ExecContext(ctx, `UPDATE identity_subject SET display_name=?, version=version+1, updated_at=CURRENT_TIMESTAMP(3) WHERE subject=? AND version=?`, command.DisplayName, subject, identityVersion)
			if err != nil {
				return err
			}
			if affected, _ := update.RowsAffected(); affected != 1 {
				return model.ErrConflict
			}
			nextIdentity++
		}
		if accessChanged {
			update, err := tx.ExecContext(ctx, `UPDATE identity_workforce_member SET access_version=access_version+1, updated_at=CURRENT_TIMESTAMP(3) WHERE member_id=? AND access_version=? AND status<>'REVOKED'`, memberID, accessVersion)
			if err != nil {
				return err
			}
			if affected, _ := update.RowsAffected(); affected != 1 {
				return model.ErrConflict
			}
			nextAccess++
		}
		if unitsChanged {
			if _, err := tx.ExecContext(ctx, `UPDATE identity_organization_membership SET status='REVOKED', revoked_at=CURRENT_TIMESTAMP(3) WHERE member_id=? AND status='ACTIVE'`, memberID); err != nil {
				return err
			}
			for i, unitID := range command.OrganizationUnitIDs {
				if _, err := tx.ExecContext(ctx, `INSERT INTO identity_organization_membership (organization_id, organization_unit_id, member_id, is_primary, status) VALUES (?, ?, ?, ?, 'ACTIVE') ON DUPLICATE KEY UPDATE is_primary=VALUES(is_primary), status='ACTIVE', revoked_at=NULL`, organizationID, unitID, memberID, i == 0); err != nil {
					return mapConflict(err)
				}
			}
		}
		if shopsChanged {
			if _, err := tx.ExecContext(ctx, `UPDATE identity_member_shop SET status='REVOKED', revoked_at=CURRENT_TIMESTAMP(3) WHERE member_id=? AND status='ACTIVE'`, memberID); err != nil {
				return err
			}
			if err := insertAssignments(ctx, tx, memberID, merchantID, command.ShopIDs, command.AssignmentKind); err != nil {
				return err
			}
		}
		if rolesChanged {
			var entitlementRevision uint64
			if err := tx.QueryRowContext(ctx, `SELECT entitlement_revision FROM identity_authorization_domain WHERE domain_type='MERCHANT' AND domain_id=? FOR UPDATE`, merchantID).Scan(&entitlementRevision); err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					return err
				}
				return model.ErrEntitlementUnavailable
			}
			if entitlementRevision == 0 {
				return model.ErrEntitlementUnavailable
			}
			for _, roleID := range command.RoleIDs {
				var count int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_authorization_role ar WHERE ar.domain_type='MERCHANT' AND ar.domain_id=? AND ar.role_id=? AND ar.status='ACTIVE' AND NOT EXISTS (SELECT 1 FROM identity_authorization_role_permission rp WHERE rp.domain_type=ar.domain_type AND rp.domain_id=ar.domain_id AND rp.role_id=ar.role_id AND NOT EXISTS (SELECT 1 FROM identity_entitlement_projection ep WHERE ep.merchant_id=ar.domain_id AND ep.permission_code=rp.permission_code AND ep.status='ACTIVE' AND ep.entitlement_revision=?))`, merchantID, roleID, entitlementRevision).Scan(&count); err != nil {
					return err
				}
				if count != 1 {
					return model.ErrAuthorizationInvalid
				}
			}
			if _, err := tx.ExecContext(ctx, `UPDATE identity_subject_grant SET status='REVOKED', revoked_at=CURRENT_TIMESTAMP(3) WHERE domain_type='MERCHANT' AND domain_id=? AND subject=? AND status='ACTIVE'`, merchantID, subject); err != nil {
				return err
			}
			for _, roleID := range command.RoleIDs {
				if _, err := tx.ExecContext(ctx, `INSERT INTO identity_subject_grant(grant_id,domain_type,domain_id,subject,role_id,status,access_version,operation_id) VALUES(?,'MERCHANT',?,?,?,'ACTIVE',?,?)`, randomID(), merchantID, subject, roleID, nextAccess, command.OperationID); err != nil {
					return mapConflict(err)
				}
			}
			if _, err := tx.ExecContext(ctx, `UPDATE identity_authorization_domain SET revision=revision+1,updated_at=CURRENT_TIMESTAMP(3) WHERE domain_type='MERCHANT' AND domain_id=?`, merchantID); err != nil {
				return err
			}
		} else if accessChanged {
			grants, err := tx.ExecContext(ctx, `UPDATE identity_subject_grant SET access_version=? WHERE domain_type='MERCHANT' AND domain_id=? AND subject=? AND status='ACTIVE' AND access_version=?`, nextAccess, merchantID, subject, accessVersion)
			if err != nil {
				return err
			}
			if affected, _ := grants.RowsAffected(); affected > 0 {
				if _, err := tx.ExecContext(ctx, `UPDATE identity_authorization_domain SET revision=revision+1,updated_at=CURRENT_TIMESTAMP(3) WHERE domain_type='MERCHANT' AND domain_id=?`, merchantID); err != nil {
					return err
				}
			}
		}
		if accessChanged {
			if _, err := tx.ExecContext(ctx, `UPDATE identity_session SET status='REVOKED', revoked_at=CURRENT_TIMESTAMP(3), revoke_reason='ACCESS_CHANGED' WHERE subject=? AND status='ACTIVE'`, subject); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE identity_refresh_token rt JOIN identity_session s ON s.session_id=rt.session_id SET rt.status='REVOKED' WHERE s.subject=? AND rt.status='ACTIVE'`, subject); err != nil {
				return err
			}
		}
		result = biz.UpdateMemberResult{MemberID: memberID, Subject: subject, Status: memberStatus, IdentityVersion: nextIdentity, AccessVersion: nextAccess, OperationID: command.OperationID}
		event := map[string]any{"operationId": command.OperationID, "memberId": memberID, "subject": subject, "accessVersion": nextAccess, "identityVersion": nextIdentity}
		if err := appendOutbox(ctx, tx, "workforce", fmt.Sprint(memberID), nextAccess, "identity.workforce.changed", event); err != nil {
			return err
		}
		return completeIdempotency(ctx, tx, "update_member", command.IdempotencyKey, result)
	})
	return result, err
}

func sameIDs(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func insertAssignments(ctx context.Context, tx *sql.Tx, memberID, merchantID int64, shopIDs []int64, kind model.AssignmentKind) error {
	for _, shopID := range shopIDs {
		result, err := tx.ExecContext(ctx, `INSERT INTO identity_member_shop (member_id, shop_id, assignment_kind, status)
SELECT ?, shop_id, ?, 'ACTIVE' FROM identity_shop WHERE shop_id=? AND merchant_id=? AND status='ACTIVE'
ON DUPLICATE KEY UPDATE status='ACTIVE', revoked_at=NULL`, memberID, kind, shopID, merchantID)
		if err != nil {
			return mapConflict(err)
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			return model.ErrInvalidAssignment
		}
	}
	return nil
}

func (r *DirectoryRepository) transaction(ctx context.Context, work func(*sql.Tx) error) error {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return err
	}
	if err := work(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func reserveIdempotency(ctx context.Context, tx *sql.Tx, command, key string, hash []byte, result any) (bool, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_idempotency (command_name, idempotency_key, request_hash) VALUES (?, ?, ?)`, command, key, hash)
	if err == nil {
		return false, nil
	}
	var storedHash []byte
	var storedResult sql.NullString
	if scanErr := tx.QueryRowContext(ctx, `SELECT request_hash, result_json FROM identity_idempotency WHERE command_name=? AND idempotency_key=? FOR UPDATE`, command, key).Scan(&storedHash, &storedResult); scanErr != nil {
		return false, mapConflict(err)
	}
	if string(storedHash) != string(hash) {
		return false, model.ErrIdempotencyConflict
	}
	if !storedResult.Valid {
		return false, model.ErrConflict
	}
	if err := json.Unmarshal([]byte(storedResult.String), result); err != nil {
		return false, err
	}
	return true, nil
}

func completeIdempotency(ctx context.Context, tx *sql.Tx, command, key string, result any) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE identity_idempotency SET result_json=?, completed_at=CURRENT_TIMESTAMP(3) WHERE command_name=? AND idempotency_key=?`, payload, command, key)
	return err
}

func appendOutbox(ctx context.Context, tx *sql.Tx, aggregateType, aggregateID string, version uint64, eventType string, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO identity_outbox (event_id, aggregate_type, aggregate_id, aggregate_version, event_type, payload_json) VALUES (?, ?, ?, ?, ?, ?)`, randomID(), aggregateType, aggregateID, version, eventType, encoded)
	return err
}

func randomID() string {
	buffer := make([]byte, 18)
	if _, err := rand.Read(buffer); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer)
}

func mapConflict(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "foreign key constraint fails") {
		return model.ErrConflict
	}
	return err
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func queryIDs(ctx context.Context, queryer queryer, query string, args ...any) ([]int64, error) {
	rows, err := queryer.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("identity: query ids: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
