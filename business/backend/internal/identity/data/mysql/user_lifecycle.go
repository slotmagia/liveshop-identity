package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

type UserLifecycleRepository struct {
	database                *sql.DB
	directory               *DirectoryRepository
	entitlementMaxStaleness time.Duration
	registryMaxStaleness    time.Duration
}

var _ biz.UserLifecycleRepository = (*UserLifecycleRepository)(nil)

func NewUserLifecycleRepository(database *sql.DB, directory *DirectoryRepository, entitlementMaxStaleness, registryMaxStaleness time.Duration) (*UserLifecycleRepository, error) {
	if database == nil || directory == nil || entitlementMaxStaleness <= 0 || registryMaxStaleness <= 0 {
		return nil, model.ErrUnavailable
	}
	return &UserLifecycleRepository{
		database:                database,
		directory:               directory,
		entitlementMaxStaleness: entitlementMaxStaleness,
		registryMaxStaleness:    registryMaxStaleness,
	}, nil
}

const managedUserSelect = `SELECT s.subject,s.realm,s.principal_type,s.display_name,s.status,s.version,
wm.member_id,wm.organization_id,COALESCE(wm.merchant_id,0),wm.member_type,wm.status,wm.access_version,
COALESCE(c.credential_id,0),COALESCE(c.version,0),COALESCE(c.credential_kind,''),COALESCE(c.normalized_identifier,''),COALESCE(c.status,''),
(SELECT COUNT(*) FROM identity_session se WHERE se.subject=s.subject AND se.status='ACTIVE' AND se.expires_at>CURRENT_TIMESTAMP(3))
FROM identity_subject s JOIN identity_workforce_member wm ON wm.subject=s.subject
LEFT JOIN identity_credential c ON c.credential_id=(SELECT MIN(c2.credential_id) FROM identity_credential c2 WHERE c2.subject=s.subject AND c2.status<>'CLOSED')`

func scopePredicate() string {
	return ` wm.organization_id=? AND ((?=0 AND wm.merchant_id IS NULL AND wm.member_type='OPERATOR') OR (? > 0 AND wm.merchant_id=? AND wm.member_type IN ('STAFF','ANCHOR')))`
}

func scopeArgs(scope biz.UserScope) []any {
	return []any{scope.OrganizationID, scope.MerchantID, scope.MerchantID, scope.MerchantID}
}

func (r *UserLifecycleRepository) ListUsers(ctx context.Context, scope biz.UserScope) ([]biz.ManagedUser, error) {
	rows, err := r.database.QueryContext(ctx, managedUserSelect+` WHERE `+scopePredicate()+` ORDER BY s.display_name,s.subject`, scopeArgs(scope)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []biz.ManagedUser
	for rows.Next() {
		value, err := scanManagedUser(rows)
		if err != nil {
			return nil, err
		}
		if err := r.attachScope(ctx, &value); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

func (r *UserLifecycleRepository) ListMembers(ctx context.Context, query biz.MemberQuery) (biz.MemberPage, error) {
	where := ` WHERE ` + scopePredicate()
	args := scopeArgs(query.Scope)
	if query.Keyword != "" {
		like := "%" + query.Keyword + "%"
		where += ` AND (s.display_name LIKE ? OR COALESCE(c.normalized_identifier,'') LIKE ?)`
		args = append(args, like, like)
	}
	if query.MemberType != "" {
		where += ` AND wm.member_type=?`
		args = append(args, query.MemberType)
	}
	if query.Status != "" {
		where += ` AND s.status=?`
		args = append(args, query.Status)
	}
	if query.ShopID > 0 {
		where += ` AND EXISTS (SELECT 1 FROM identity_member_shop ms WHERE ms.member_id=wm.member_id AND ms.shop_id=? AND ms.status='ACTIVE')`
		args = append(args, query.ShopID)
	}
	from := ` FROM identity_subject s JOIN identity_workforce_member wm ON wm.subject=s.subject LEFT JOIN identity_credential c ON c.credential_id=(SELECT MIN(c2.credential_id) FROM identity_credential c2 WHERE c2.subject=s.subject AND c2.status<>'CLOSED')`
	var total int64
	if err := r.database.QueryRowContext(ctx, `SELECT COUNT(*)`+from+where, args...).Scan(&total); err != nil {
		return biz.MemberPage{}, err
	}
	pageArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := r.database.QueryContext(ctx, managedUserSelect+where+` ORDER BY s.display_name,s.subject LIMIT ? OFFSET ?`, pageArgs...)
	if err != nil {
		return biz.MemberPage{}, err
	}
	defer rows.Close()
	items := make([]biz.ManagedUser, 0)
	for rows.Next() {
		value, err := scanManagedUser(rows)
		if err != nil {
			return biz.MemberPage{}, err
		}
		if err := r.attachScope(ctx, &value); err != nil {
			return biz.MemberPage{}, err
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return biz.MemberPage{}, err
	}
	return biz.MemberPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

type rowScanner interface{ Scan(...any) error }

func scanManagedUser(row rowScanner) (biz.ManagedUser, error) {
	var value biz.ManagedUser
	err := row.Scan(&value.Subject.ID, &value.Subject.Realm, &value.Subject.PrincipalType, &value.Subject.DisplayName, &value.Subject.Status, &value.Subject.Version,
		&value.Member.ID, &value.Member.OrganizationID, &value.Member.MerchantID, &value.Member.Type, &value.Member.Status, &value.Member.AccessVersion,
		&value.Credential.ID, &value.Credential.Version, &value.Credential.Kind, &value.Credential.Identifier, &value.Credential.Status, &value.ActiveSessions)
	return value, err
}

func (r *UserLifecycleRepository) GetUser(ctx context.Context, scope biz.UserScope, subject string) (biz.ManagedUser, error) {
	args := append(scopeArgs(scope), subject)
	value, err := scanManagedUser(r.database.QueryRowContext(ctx, managedUserSelect+` WHERE `+scopePredicate()+` AND s.subject=?`, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return biz.ManagedUser{}, model.ErrNotFound
	}
	if err != nil {
		return biz.ManagedUser{}, err
	}
	if err := r.attachScope(ctx, &value); err != nil {
		return biz.ManagedUser{}, err
	}
	return value, nil
}

func (r *UserLifecycleRepository) OwnAccount(ctx context.Context, scope biz.UserScope, subject string) (biz.ManagedUser, error) {
	if !scope.Valid() || scope.MerchantID <= 0 || strings.TrimSpace(subject) == "" {
		return biz.ManagedUser{}, model.ErrInvalidContext
	}
	value, err := scanManagedUser(r.database.QueryRowContext(ctx, managedUserSelect+` WHERE wm.organization_id=? AND wm.merchant_id=? AND wm.member_type IN ('OWNER','STAFF','ANCHOR') AND wm.status<>'REVOKED' AND s.subject=?`, scope.OrganizationID, scope.MerchantID, subject))
	if errors.Is(err, sql.ErrNoRows) {
		return biz.ManagedUser{}, model.ErrNotFound
	}
	if err != nil {
		return biz.ManagedUser{}, err
	}
	return value, nil
}

func (r *UserLifecycleRepository) attachScope(ctx context.Context, value *biz.ManagedUser) error {
	var err error
	value.RoleIDs, err = r.roleIDs(ctx, value.Member, value.Subject.ID)
	if err != nil {
		return err
	}
	value.ShopIDs, err = queryIDs(ctx, r.database, `SELECT shop_id FROM identity_member_shop WHERE member_id=? AND status='ACTIVE' ORDER BY shop_id`, value.Member.ID)
	if err != nil {
		return err
	}
	value.UnitIDs, err = queryIDs(ctx, r.database, `SELECT organization_unit_id FROM identity_organization_membership WHERE member_id=? AND status='ACTIVE' ORDER BY organization_unit_id`, value.Member.ID)
	return err
}

func (r *UserLifecycleRepository) roleIDs(ctx context.Context, member model.WorkforceMember, subject string) ([]int64, error) {
	domainType, domainID := "PLATFORM_ORG", member.OrganizationID
	if member.MerchantID > 0 {
		domainType, domainID = "MERCHANT", member.MerchantID
	}
	return queryIDs(ctx, r.database, `SELECT role_id FROM identity_subject_grant WHERE domain_type=? AND domain_id=? AND subject=? AND status='ACTIVE' ORDER BY role_id`, domainType, domainID, subject)
}

func (r *UserLifecycleRepository) CreatePlatformOperator(ctx context.Context, command biz.CreateOperator) (biz.ManagedUser, error) {
	var result biz.ManagedUser
	err := r.directory.transaction(ctx, func(tx *sql.Tx) error {
		replayed, err := reserveIdempotency(ctx, tx, "create_platform_operator", command.IdempotencyKey, command.RequestHash[:], &result)
		if err != nil || replayed {
			return err
		}
		var orgStatus string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM identity_organization WHERE organization_id=? AND organization_type='PLATFORM' FOR UPDATE`, command.OrganizationID).Scan(&orgStatus); err != nil {
			return userLifecycleMapNotFound(err)
		}
		if orgStatus != "ACTIVE" {
			return model.ErrInactive
		}
		for _, roleID := range command.RoleIDs {
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_authorization_role WHERE domain_type='PLATFORM_ORG' AND domain_id=? AND role_id=? AND status='ACTIVE'`, command.OrganizationID, roleID).Scan(&count); err != nil {
				return err
			}
			if count != 1 {
				return model.ErrAuthorizationInvalid
			}
		}
		secretHash, err := HashPassword(command.Password)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO identity_subject(subject,realm,principal_type,display_name,status,version) VALUES(?,'PLATFORM','PLATFORM_OPERATOR',?,'ACTIVE',1)`, command.Subject, command.DisplayName); err != nil {
			return mapConflict(err)
		}
		member, err := tx.ExecContext(ctx, `INSERT INTO identity_workforce_member(organization_id,merchant_id,subject,member_type,status,access_version) VALUES(?,NULL,?,'OPERATOR','ACTIVE',1)`, command.OrganizationID, command.Subject)
		if err != nil {
			return mapConflict(err)
		}
		memberID, err := member.LastInsertId()
		if err != nil {
			return err
		}
		credential, err := tx.ExecContext(ctx, `INSERT INTO identity_credential(subject,namespace_type,credential_kind,normalized_identifier,secret_hash,status,version) VALUES(?,'GLOBAL','USERNAME',?,?,'ACTIVE',1)`, command.Subject, command.Username, secretHash)
		if err != nil {
			return mapConflict(err)
		}
		credentialID, err := credential.LastInsertId()
		if err != nil {
			return err
		}
		for _, roleID := range command.RoleIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO identity_subject_grant(grant_id,domain_type,domain_id,subject,role_id,status,access_version,operation_id) VALUES(?,'PLATFORM_ORG',?,?,?,'ACTIVE',1,?)`, randomID(), command.OrganizationID, command.Subject, roleID, command.OperationID); err != nil {
				return mapConflict(err)
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE identity_authorization_domain SET revision=revision+1,updated_at=CURRENT_TIMESTAMP(3) WHERE domain_type='PLATFORM_ORG' AND domain_id=?`, command.OrganizationID); err != nil {
			return err
		}
		result = biz.ManagedUser{Subject: model.Subject{ID: command.Subject, Realm: principal.RealmPlatform, PrincipalType: principal.TypePlatformOperator, DisplayName: command.DisplayName, Status: model.StatusActive, Version: 1}, Member: model.WorkforceMember{ID: memberID, OrganizationID: command.OrganizationID, Subject: command.Subject, Type: model.MemberOperator, Status: model.MemberActive, AccessVersion: 1}, Credential: biz.ManagedCredential{ID: uint64(credentialID), Version: 1, Kind: "USERNAME", Identifier: command.Username, Status: model.StatusActive}, RoleIDs: command.RoleIDs}
		if err := appendOutbox(ctx, tx, "subject", command.Subject, 1, "identity.subject.changed", map[string]any{"operationId": command.OperationID, "subject": command.Subject, "status": "ACTIVE"}); err != nil {
			return err
		}
		return completeIdempotency(ctx, tx, "create_platform_operator", command.IdempotencyKey, result)
	})
	return result, err
}

func (r *UserLifecycleRepository) ChangeUserStatus(ctx context.Context, command biz.ChangeUserStatus) (biz.ManagedUser, error) {
	var result biz.ManagedUser
	err := r.directory.transaction(ctx, func(tx *sql.Tx) error {
		replayed, err := reserveIdempotency(ctx, tx, "change_user_status", command.IdempotencyKey, command.RequestHash[:], &result)
		if err != nil || replayed {
			return err
		}
		args := append(scopeArgs(command.Scope), command.Subject)
		result, err = scanManagedUser(tx.QueryRowContext(ctx, managedUserSelect+` WHERE `+scopePredicate()+` AND s.subject=? FOR UPDATE`, args...))
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := validateSingleMembershipTx(ctx, tx, command.Subject); err != nil {
			return err
		}
		if err := model.ValidateSubjectTransition(result.Subject.Status, command.Target); err != nil {
			return err
		}
		if result.Subject.Version != command.ExpectedIdentityVersion || result.Member.AccessVersion != command.ExpectedAccessVersion {
			return model.ErrConflict
		}
		if result.Subject.Status == command.Target {
			return completeIdempotency(ctx, tx, "change_user_status", command.IdempotencyKey, result)
		}
		if command.Target == model.StatusDisabled && command.Scope.MerchantID == 0 {
			protected, err := hasSystemRoleTx(ctx, tx, command.Scope, command.Subject)
			if err != nil {
				return err
			}
			if protected {
				return model.ErrSystemRoleProtected
			}
			var active int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_workforce_member WHERE organization_id=? AND merchant_id IS NULL AND member_type='OPERATOR' AND status='ACTIVE'`, command.Scope.OrganizationID).Scan(&active); err != nil {
				return err
			}
			if active <= 1 {
				return model.ErrLastActiveOperator
			}
		}
		memberTarget := model.MemberSuspended
		if command.Target == model.StatusActive {
			memberTarget = model.MemberActive
		}
		updated, err := tx.ExecContext(ctx, `UPDATE identity_subject SET status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3) WHERE subject=? AND version=? AND status=?`, command.Target, command.Subject, command.ExpectedIdentityVersion, result.Subject.Status)
		if err != nil {
			return err
		}
		if n, _ := updated.RowsAffected(); n != 1 {
			return model.ErrConflict
		}
		updated, err = tx.ExecContext(ctx, `UPDATE identity_workforce_member SET status=?,access_version=access_version+1,updated_at=CURRENT_TIMESTAMP(3) WHERE member_id=? AND access_version=? AND status=?`, memberTarget, result.Member.ID, command.ExpectedAccessVersion, result.Member.Status)
		if err != nil {
			return err
		}
		if n, _ := updated.RowsAffected(); n != 1 {
			return model.ErrConflict
		}
		result.Subject.Status = command.Target
		result.Subject.Version++
		result.Member.Status = memberTarget
		result.Member.AccessVersion++
		if command.Target == model.StatusDisabled {
			if err := revokeAllSubjectSessions(ctx, tx, command.Subject, "ACCOUNT_DISABLED"); err != nil {
				return err
			}
			result.ActiveSessions = 0
		}
		domainType, domainID := "PLATFORM_ORG", command.Scope.OrganizationID
		if command.Scope.MerchantID > 0 {
			domainType, domainID = "MERCHANT", command.Scope.MerchantID
		}
		grantUpdate, err := tx.ExecContext(ctx, `UPDATE identity_subject_grant SET access_version=? WHERE domain_type=? AND domain_id=? AND subject=? AND status='ACTIVE' AND access_version=?`, result.Member.AccessVersion, domainType, domainID, command.Subject, command.ExpectedAccessVersion)
		if err != nil {
			return err
		}
		if n, _ := grantUpdate.RowsAffected(); n > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE identity_authorization_domain SET revision=revision+1,updated_at=CURRENT_TIMESTAMP(3) WHERE domain_type=? AND domain_id=?`, domainType, domainID); err != nil {
				return err
			}
		}
		if err := appendOutbox(ctx, tx, "subject", command.Subject, result.Subject.Version, "identity.subject.changed", map[string]any{"operationId": command.OperationID, "subject": command.Subject, "status": command.Target}); err != nil {
			return err
		}
		return completeIdempotency(ctx, tx, "change_user_status", command.IdempotencyKey, result)
	})
	return result, err
}

func (r *UserLifecycleRepository) ResetCredential(ctx context.Context, command biz.ResetCredential) (biz.ManagedCredential, error) {
	var result biz.ManagedCredential
	err := r.directory.transaction(ctx, func(tx *sql.Tx) error {
		replayed, err := reserveIdempotency(ctx, tx, "reset_credential", command.IdempotencyKey, command.RequestHash[:], &result)
		if err != nil || replayed {
			return err
		}
		if err := validateTargetTx(ctx, tx, command.Scope, command.Subject); err != nil {
			return err
		}
		if err := validateSingleMembershipTx(ctx, tx, command.Subject); err != nil {
			return err
		}
		if command.Scope.MerchantID == 0 {
			protected, err := hasSystemRoleTx(ctx, tx, command.Scope, command.Subject)
			if err != nil {
				return err
			}
			if protected {
				return model.ErrSystemRoleProtected
			}
		}
		var currentStatus model.Status
		if err := tx.QueryRowContext(ctx, `SELECT credential_kind,normalized_identifier,status,version FROM identity_credential WHERE credential_id=? AND subject=? FOR UPDATE`, command.CredentialID, command.Subject).Scan(&result.Kind, &result.Identifier, &currentStatus, &result.Version); errors.Is(err, sql.ErrNoRows) {
			return model.ErrNotFound
		} else if err != nil {
			return err
		}
		if result.Version != command.ExpectedCredentialVersion || currentStatus == model.StatusClosed {
			return model.ErrConflict
		}
		hash, err := HashPassword(command.Password)
		if err != nil {
			return err
		}
		updated, err := tx.ExecContext(ctx, `UPDATE identity_credential SET secret_hash=?,failed_login_count=0,locked_until=NULL,status='ACTIVE',version=version+1,updated_at=CURRENT_TIMESTAMP(3) WHERE credential_id=? AND subject=? AND version=?`, hash, command.CredentialID, command.Subject, command.ExpectedCredentialVersion)
		if err != nil {
			return err
		}
		if n, _ := updated.RowsAffected(); n != 1 {
			return model.ErrConflict
		}
		result.ID = command.CredentialID
		result.Version++
		result.Status = model.StatusActive
		if err := revokeAllSubjectSessions(ctx, tx, command.Subject, "CREDENTIAL_RESET"); err != nil {
			return err
		}
		if err := appendOutbox(ctx, tx, "credential", fmt.Sprint(command.CredentialID), result.Version, "identity.credential.changed", map[string]any{"operationId": command.OperationID, "subject": command.Subject, "credentialId": command.CredentialID, "version": result.Version}); err != nil {
			return err
		}
		return completeIdempotency(ctx, tx, "reset_credential", command.IdempotencyKey, result)
	})
	return result, err
}

func (r *UserLifecycleRepository) ChangeOwnCredential(ctx context.Context, command biz.ChangeOwnCredential) (biz.ChangeOwnCredentialResult, error) {
	var result biz.ChangeOwnCredentialResult
	err := r.directory.transaction(ctx, func(tx *sql.Tx) error {
		replayed, err := reserveIdempotency(ctx, tx, "change_own_credential", command.IdempotencyKey, command.RequestHash[:], &result)
		if err != nil {
			return err
		}
		if replayed {
			result.Replayed = true
			return nil
		}
		if err := validateSingleMembershipTx(ctx, tx, command.Subject); err != nil {
			return err
		}
		var memberType, memberStatus, subjectStatus string
		if err := tx.QueryRowContext(ctx, `SELECT wm.member_type,wm.status,s.status FROM identity_workforce_member wm JOIN identity_subject s ON s.subject=wm.subject WHERE wm.subject=? AND wm.organization_id=? AND wm.merchant_id=? AND wm.status<>'REVOKED' FOR UPDATE`, command.Subject, command.Scope.OrganizationID, command.Scope.MerchantID).Scan(&memberType, &memberStatus, &subjectStatus); errors.Is(err, sql.ErrNoRows) {
			return model.ErrNotFound
		} else if err != nil {
			return err
		}
		if memberStatus != string(model.MemberActive) || subjectStatus != string(model.StatusActive) {
			return model.ErrInactive
		}
		if memberType != string(model.MemberOwner) && memberType != string(model.MemberStaff) && memberType != string(model.MemberAnchor) {
			return model.ErrInvalidContext
		}
		var secretHash string
		var currentStatus model.Status
		if err := tx.QueryRowContext(ctx, `SELECT credential_id,credential_kind,normalized_identifier,secret_hash,status,version FROM identity_credential WHERE subject=? AND status<>'CLOSED' ORDER BY credential_id LIMIT 1 FOR UPDATE`, command.Subject).Scan(&result.Credential.ID, &result.Credential.Kind, &result.Credential.Identifier, &secretHash, &currentStatus, &result.Credential.Version); errors.Is(err, sql.ErrNoRows) {
			return model.ErrNotFound
		} else if err != nil {
			return err
		}
		if result.Credential.Version != command.ExpectedCredentialVersion || currentStatus == model.StatusClosed {
			return model.ErrConflict
		}
		if err := ComparePassword(secretHash, command.OldPassword); err != nil {
			return err
		}
		hash, err := HashPassword(command.Password)
		if err != nil {
			return err
		}
		updated, err := tx.ExecContext(ctx, `UPDATE identity_credential SET secret_hash=?,failed_login_count=0,locked_until=NULL,status='ACTIVE',version=version+1,updated_at=CURRENT_TIMESTAMP(3) WHERE credential_id=? AND subject=? AND version=?`, hash, result.Credential.ID, command.Subject, command.ExpectedCredentialVersion)
		if err != nil {
			return err
		}
		if n, _ := updated.RowsAffected(); n != 1 {
			return model.ErrConflict
		}
		result.Credential.Version++
		result.Credential.Status = model.StatusActive
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_session WHERE subject=? AND status='ACTIVE' AND session_id<>?`, command.Subject, command.SessionID).Scan(&result.RevokedSessions); err != nil {
			return err
		}
		if err := revokeOtherSubjectSessions(ctx, tx, command.Subject, command.SessionID, "CREDENTIAL_CHANGED"); err != nil {
			return err
		}
		var retained int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_session WHERE session_id=? AND subject=? AND status='ACTIVE'`, command.SessionID, command.Subject).Scan(&retained); err != nil {
			return err
		}
		result.CurrentRetained = retained == 1
		if err := appendOutbox(ctx, tx, "credential", fmt.Sprint(result.Credential.ID), result.Credential.Version, "identity.credential.changed", map[string]any{"operationId": command.OperationID, "subject": command.Subject, "credentialId": result.Credential.ID, "version": result.Credential.Version}); err != nil {
			return err
		}
		return completeIdempotency(ctx, tx, "change_own_credential", command.IdempotencyKey, result)
	})
	return result, err
}

func (r *UserLifecycleRepository) ListSessions(ctx context.Context, scope biz.UserScope, subject string) ([]biz.ManagedSession, error) {
	if err := validateTargetDB(ctx, r.database, scope, subject); err != nil {
		return nil, err
	}
	return r.listSessionsBySubject(ctx, subject)
}

func (r *UserLifecycleRepository) ListOwnSessions(ctx context.Context, scope biz.UserScope, subject string) ([]biz.ManagedSession, error) {
	if err := validateOwnAccount(ctx, r.database, scope, subject); err != nil {
		return nil, err
	}
	return r.listSessionsBySubject(ctx, subject)
}

func (r *UserLifecycleRepository) listSessionsBySubject(ctx context.Context, subject string) ([]biz.ManagedSession, error) {
	rows, err := r.database.QueryContext(ctx, `SELECT session_id,device_name,ip_address,user_agent,status,created_at,last_refreshed_at,expires_at FROM identity_session WHERE subject=? ORDER BY created_at DESC`, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []biz.ManagedSession
	for rows.Next() {
		var v biz.ManagedSession
		var created, refreshed, expires time.Time
		if err := rows.Scan(&v.ID, &v.DeviceName, &v.IPAddress, &v.UserAgent, &v.Status, &created, &refreshed, &expires); err != nil {
			return nil, err
		}
		v.CreatedAt = created.UTC().Format(time.RFC3339Nano)
		v.LastRefreshedAt = refreshed.UTC().Format(time.RFC3339Nano)
		v.ExpiresAt = expires.UTC().Format(time.RFC3339Nano)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *UserLifecycleRepository) RevokeSessions(ctx context.Context, command biz.RevokeSessions) error {
	return r.revokeSessions(ctx, command, validateTargetTx)
}

func (r *UserLifecycleRepository) RevokeOwnSessions(ctx context.Context, command biz.RevokeSessions) error {
	return r.revokeSessions(ctx, command, validateOwnAccountTx)
}

func (r *UserLifecycleRepository) revokeSessions(ctx context.Context, command biz.RevokeSessions, validate func(context.Context, *sql.Tx, biz.UserScope, string) error) error {
	return r.directory.transaction(ctx, func(tx *sql.Tx) error {
		var result struct{}
		replayed, err := reserveIdempotency(ctx, tx, "revoke_sessions", command.IdempotencyKey, command.RequestHash[:], &result)
		if err != nil || replayed {
			return err
		}
		if err := validate(ctx, tx, command.Scope, command.Subject); err != nil {
			return err
		}
		reason := command.Reason
		if reason == "" {
			reason = "ADMIN_REVOKED"
		}
		if command.SessionID == "" {
			if err := revokeAllSubjectSessions(ctx, tx, command.Subject, reason); err != nil {
				return err
			}
		} else {
			var found string
			if err := tx.QueryRowContext(ctx, `SELECT session_id FROM identity_session WHERE session_id=? AND subject=? FOR UPDATE`, command.SessionID, command.Subject).Scan(&found); errors.Is(err, sql.ErrNoRows) {
				return model.ErrNotFound
			} else if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE identity_session SET status='REVOKED',revoked_at=COALESCE(revoked_at,CURRENT_TIMESTAMP(3)),revoke_reason=COALESCE(revoke_reason,?) WHERE session_id=? AND status='ACTIVE'`, reason, command.SessionID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE identity_refresh_token SET status='REVOKED' WHERE session_id=? AND status='ACTIVE'`, command.SessionID); err != nil {
				return err
			}
		}
		if err := appendOutbox(ctx, tx, "session", command.Subject, 1, "identity.session.changed", map[string]any{"operationId": command.OperationID, "subject": command.Subject, "sessionId": command.SessionID, "status": "REVOKED"}); err != nil {
			return err
		}
		return completeIdempotency(ctx, tx, "revoke_sessions", command.IdempotencyKey, result)
	})
}

func (r *UserLifecycleRepository) ValidateCurrentAuthorization(ctx context.Context, claims modulesession.Claims) error {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var subjectRealm, subjectPrincipalType, subjectStatus, memberStatus, organizationStatus string
	var identityVersion, accessVersion, organizationVersion uint64
	var organizationID, merchantID int64
	err = tx.QueryRowContext(ctx, `SELECT s.realm,s.principal_type,s.status,s.version,m.status,m.access_version,m.organization_id,COALESCE(m.merchant_id,0),o.status,o.version FROM identity_subject s JOIN identity_workforce_member m ON m.subject=s.subject AND m.status<>'REVOKED' JOIN identity_organization o ON o.organization_id=m.organization_id WHERE s.subject=? AND m.organization_id=? AND COALESCE(m.merchant_id,0)=?`, claims.Subject, claims.OrganizationID, claims.MerchantID).Scan(&subjectRealm, &subjectPrincipalType, &subjectStatus, &identityVersion, &memberStatus, &accessVersion, &organizationID, &merchantID, &organizationStatus, &organizationVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrAuthorizationDenied
	}
	if err != nil {
		return err
	}
	if subjectRealm != string(claims.Realm) || subjectPrincipalType != string(claims.PrincipalType) || subjectStatus != "ACTIVE" || memberStatus != "ACTIVE" || organizationStatus != "ACTIVE" || identityVersion != claims.IdentityVersion || organizationVersion != claims.OrganizationVersion || organizationID != claims.OrganizationID || merchantID != claims.MerchantID {
		return model.ErrAuthorizationDenied
	}
	selected := model.SelectedContext{OrganizationID: claims.OrganizationID, ShopContext: model.ShopContext{MerchantID: claims.MerchantID, ShopID: claims.ShopID}}
	var sessionFound int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM identity_session WHERE session_id=? AND subject=? AND status='ACTIVE' AND expires_at>CURRENT_TIMESTAMP(3) AND context_version=? AND COALESCE(selected_organization_id,0)=? AND COALESCE(selected_merchant_id,0)=? AND COALESCE(selected_shop_id,0)=?`, claims.SessionID, claims.Subject, claims.ContextVersion, selected.OrganizationID, selected.MerchantID, selected.ShopID).Scan(&sessionFound); err != nil {
		return model.ErrAuthorizationDenied
	}
	domainType, domainID := "PLATFORM_ORG", organizationID
	var currentAuthorization, currentEntitlement uint64
	if merchantID > 0 {
		domainType, domainID = "MERCHANT", merchantID
	}
	if merchantID == 0 {
		if err := tx.QueryRowContext(ctx, `SELECT revision,platform_boundary_revision FROM identity_authorization_domain WHERE domain_type=? AND domain_id=?`, domainType, domainID).Scan(&currentAuthorization, &currentEntitlement); err != nil {
			return model.ErrAuthorizationDenied
		}
	} else {
		if err := tx.QueryRowContext(ctx, `SELECT revision,entitlement_revision FROM identity_authorization_domain WHERE domain_type=? AND domain_id=?`, domainType, domainID).Scan(&currentAuthorization, &currentEntitlement); err != nil {
			return model.ErrAuthorizationDenied
		}
		var projectedRevision uint64
		var projectedAt time.Time
		if err := tx.QueryRowContext(ctx, `SELECT entitlement_revision,projected_at FROM identity_entitlement_projection_state WHERE merchant_id=?`, merchantID).Scan(&projectedRevision, &projectedAt); err != nil || projectedRevision != currentEntitlement || time.Since(projectedAt) > r.entitlementMaxStaleness {
			return model.ErrAuthorizationDenied
		}
	}
	if currentAuthorization != claims.AuthorizationRevision || currentEntitlement != claims.EntitlementRevision {
		return model.ErrAuthorizationDenied
	}
	var registryRevision uint64
	var projectedAt time.Time
	if err := tx.QueryRowContext(ctx, `SELECT registry_revision,projected_at FROM identity_registry_projection_state WHERE singleton_id=1`).Scan(&registryRevision, &projectedAt); err != nil || registryRevision != claims.RegistryRevision || time.Since(projectedAt) > r.registryMaxStaleness {
		return model.ErrAuthorizationDenied
	}
	return tx.Commit()
}

func revokeAllSubjectSessions(ctx context.Context, tx *sql.Tx, subject, reason string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE identity_session SET status='REVOKED',revoked_at=CURRENT_TIMESTAMP(3),revoke_reason=? WHERE subject=? AND status='ACTIVE'`, reason, subject); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE identity_refresh_token rt JOIN identity_session s ON s.session_id=rt.session_id SET rt.status='REVOKED' WHERE s.subject=? AND rt.status='ACTIVE'`, subject)
	return err
}

func revokeOtherSubjectSessions(ctx context.Context, tx *sql.Tx, subject, sessionID, reason string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE identity_session SET status='REVOKED',revoked_at=CURRENT_TIMESTAMP(3),revoke_reason=? WHERE subject=? AND status='ACTIVE' AND session_id<>?`, reason, subject, sessionID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE identity_refresh_token rt JOIN identity_session s ON s.session_id=rt.session_id SET rt.status='REVOKED' WHERE s.subject=? AND rt.status='ACTIVE' AND s.session_id<>?`, subject, sessionID)
	return err
}
func validateTargetDB(ctx context.Context, q *sql.DB, scope biz.UserScope, subject string) error {
	return validateTarget(ctx, q, scope, subject)
}
func validateTargetTx(ctx context.Context, q *sql.Tx, scope biz.UserScope, subject string) error {
	return validateTarget(ctx, q, scope, subject)
}
func validateOwnAccountTx(ctx context.Context, q *sql.Tx, scope biz.UserScope, subject string) error {
	return validateOwnAccount(ctx, q, scope, subject)
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func validateTarget(ctx context.Context, q queryRower, scope biz.UserScope, subject string) error {
	args := append(scopeArgs(scope), subject)
	var found string
	err := q.QueryRowContext(ctx, `SELECT s.subject FROM identity_subject s JOIN identity_workforce_member wm ON wm.subject=s.subject WHERE `+scopePredicate()+` AND s.subject=?`, args...).Scan(&found)
	return userLifecycleMapNotFound(err)
}

func validateOwnAccount(ctx context.Context, q queryRower, scope biz.UserScope, subject string) error {
	if !scope.Valid() || scope.MerchantID <= 0 || strings.TrimSpace(subject) == "" {
		return model.ErrInvalidContext
	}
	var found string
	err := q.QueryRowContext(ctx, `SELECT s.subject FROM identity_subject s JOIN identity_workforce_member wm ON wm.subject=s.subject WHERE wm.organization_id=? AND wm.merchant_id=? AND wm.member_type IN ('OWNER','STAFF','ANCHOR') AND wm.status<>'REVOKED' AND s.subject=?`, scope.OrganizationID, scope.MerchantID, subject).Scan(&found)
	return userLifecycleMapNotFound(err)
}
func userLifecycleMapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	return err
}
func validateSingleMembershipTx(ctx context.Context, tx *sql.Tx, subject string) error {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_workforce_member WHERE subject=? AND status<>'REVOKED'`, subject).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return model.ErrConflict
	}
	return nil
}
func hasSystemRoleTx(ctx context.Context, tx *sql.Tx, scope biz.UserScope, subject string) (bool, error) {
	domainType, domainID := "PLATFORM_ORG", scope.OrganizationID
	if scope.MerchantID > 0 {
		domainType, domainID = "MERCHANT", scope.MerchantID
	}
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_subject_grant g JOIN identity_authorization_role r ON r.domain_type=g.domain_type AND r.domain_id=g.domain_id AND r.role_id=g.role_id WHERE g.domain_type=? AND g.domain_id=? AND g.subject=? AND g.status='ACTIVE' AND r.status='ACTIVE' AND r.system_role=1`, domainType, domainID, subject).Scan(&count)
	return count > 0, err
}
