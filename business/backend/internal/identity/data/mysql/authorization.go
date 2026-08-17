package mysql

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

type AuthorizationRepository struct {
	db                      *sql.DB
	entitlementMaxStaleness time.Duration
}

var _ biz.AuthorizationRepository = (*AuthorizationRepository)(nil)

func NewAuthorizationRepository(db *sql.DB, entitlementMaxStaleness time.Duration) (*AuthorizationRepository, error) {
	if db == nil {
		return nil, model.ErrUnavailable
	}
	if entitlementMaxStaleness <= 0 {
		return nil, model.ErrUnavailable
	}
	return &AuthorizationRepository{db: db, entitlementMaxStaleness: entitlementMaxStaleness}, nil
}

func (r *AuthorizationRepository) Permissions(ctx context.Context, d model.AuthorizationDomain) ([]model.Permission, error) {
	query := `SELECT p.module_id,p.permission_code,p.name,p.resource_code,p.action,p.description,p.registry_revision FROM identity_permission_projection p WHERE p.active=1 ORDER BY p.permission_code`
	args := []any{}
	if d.Type == model.AuthorizationMerchant {
		var revision uint64
		err := r.db.QueryRowContext(ctx, `SELECT entitlement_revision FROM identity_authorization_domain WHERE domain_type='MERCHANT' AND domain_id=?`, d.ID).Scan(&revision)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrEntitlementUnavailable
		}
		if err != nil {
			return nil, err
		}
		if revision == 0 {
			return nil, model.ErrEntitlementUnavailable
		}
		var projectedAt time.Time
		var projectedRevision uint64
		if err := r.db.QueryRowContext(ctx, `SELECT entitlement_revision,projected_at FROM identity_entitlement_projection_state WHERE merchant_id=?`, d.ID).Scan(&projectedRevision, &projectedAt); err != nil || projectedRevision != revision || time.Since(projectedAt) > r.entitlementMaxStaleness {
			return nil, model.ErrEntitlementUnavailable
		}
		query = `SELECT p.module_id,p.permission_code,p.name,p.resource_code,p.action,p.description,p.registry_revision
FROM identity_permission_projection p
JOIN identity_entitlement_projection e ON e.merchant_id=? AND e.permission_code=p.permission_code AND e.status='ACTIVE' AND e.entitlement_revision=?
WHERE p.active=1 ORDER BY p.permission_code`
		args = []any{d.ID, revision}
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Permission{}
	for rows.Next() {
		var p model.Permission
		if err := rows.Scan(&p.ModuleID, &p.Code, &p.Name, &p.Resource, &p.Action, &p.Description, &p.RegistryRevision); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (r *AuthorizationRepository) Roles(ctx context.Context, d model.AuthorizationDomain) ([]model.Role, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT role_id,code,name,status,system_role,version FROM identity_authorization_role WHERE domain_type=? AND domain_id=? AND status<>'DELETED' ORDER BY role_id`, d.Type, d.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Role{}
	for rows.Next() {
		var v model.Role
		if err := rows.Scan(&v.ID, &v.Code, &v.Name, &v.Status, &v.SystemRole, &v.Version); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *AuthorizationRepository) PutRole(ctx context.Context, d model.AuthorizationDomain, v model.Role, expected int64) (model.Role, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Role{}, err
	}
	defer tx.Rollback()
	if err = lockDomain(ctx, tx, d); err != nil {
		return model.Role{}, err
	}
	if expected == 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO identity_authorization_role(domain_type,domain_id,role_id,code,name,status,system_role,version) VALUES(?,?,?,?,?,?,0,1)`, d.Type, d.ID, v.ID, v.Code, v.Name, v.Status)
		v.Version = 1
	} else {
		res, e := tx.ExecContext(ctx, `UPDATE identity_authorization_role SET code=?,name=?,status=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3) WHERE domain_type=? AND domain_id=? AND role_id=? AND version=? AND system_role=0`, v.Code, v.Name, v.Status, d.Type, d.ID, v.ID, expected)
		err = e
		if err == nil {
			n, _ := res.RowsAffected()
			if n != 1 {
				err = model.ErrAuthorizationConflict
			}
		}
		v.Version = expected + 1
	}
	if err != nil {
		return model.Role{}, authorizationConflict(err)
	}
	if err = bumpDomain(ctx, tx, d); err != nil {
		return model.Role{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.Role{}, err
	}
	return v, nil
}
func (r *AuthorizationRepository) SetRolePolicy(ctx context.Context, d model.AuthorizationDomain, roleID, expected int64, p model.RolePolicy) (model.Role, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Role{}, err
	}
	defer tx.Rollback()
	if err = lockDomain(ctx, tx, d); err != nil {
		return model.Role{}, err
	}
	catalog, err := permissionMap(ctx, tx)
	if err != nil {
		return model.Role{}, err
	}
	if err = model.ValidatePolicy(p, catalog); err != nil {
		return model.Role{}, err
	}
	resources := map[string]bool{}
	for _, code := range p.Permissions {
		resources[catalog[code].Resource] = true
	}
	for _, scope := range p.Scopes {
		if !resources[scope.Resource] || (scope.Type == model.ScopeAll && d.Type != model.AuthorizationPlatform) || scope.Type == model.ScopeDelegatedBusiness || scope.Type == model.ScopeCustomReference || len(scope.ReferenceIDs) != 0 {
			return model.Role{}, model.ErrAuthorizationInvalid
		}
	}
	if d.Type == model.AuthorizationMerchant {
		var entitlementRevision uint64
		if err = tx.QueryRowContext(ctx, `SELECT entitlement_revision FROM identity_authorization_domain WHERE domain_type=? AND domain_id=?`, d.Type, d.ID).Scan(&entitlementRevision); err != nil {
			return model.Role{}, mapNotFound(err)
		}
		if entitlementRevision == 0 {
			return model.Role{}, model.ErrEntitlementUnavailable
		}
		for _, code := range p.Permissions {
			var count int
			if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_entitlement_projection WHERE merchant_id=? AND permission_code=? AND status='ACTIVE' AND entitlement_revision=?`, d.ID, code, entitlementRevision).Scan(&count); err != nil {
				return model.Role{}, err
			}
			if count != 1 {
				return model.Role{}, model.ErrAuthorizationInvalid
			}
		}
	}
	var role model.Role
	if err = tx.QueryRowContext(ctx, `SELECT role_id,code,name,status,system_role,version FROM identity_authorization_role WHERE domain_type=? AND domain_id=? AND role_id=? FOR UPDATE`, d.Type, d.ID, roleID).Scan(&role.ID, &role.Code, &role.Name, &role.Status, &role.SystemRole, &role.Version); err != nil {
		return model.Role{}, mapNotFound(err)
	}
	if role.SystemRole {
		return model.Role{}, model.ErrSystemRoleProtected
	}
	if role.Version != expected {
		return model.Role{}, model.ErrAuthorizationConflict
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM identity_authorization_role_permission WHERE domain_type=? AND domain_id=? AND role_id=?`, d.Type, d.ID, roleID); err != nil {
		return model.Role{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM identity_authorization_role_scope WHERE domain_type=? AND domain_id=? AND role_id=?`, d.Type, d.ID, roleID); err != nil {
		return model.Role{}, err
	}
	for _, code := range p.Permissions {
		if _, err = tx.ExecContext(ctx, `INSERT INTO identity_authorization_role_permission(domain_type,domain_id,role_id,permission_code) VALUES(?,?,?,?)`, d.Type, d.ID, roleID, code); err != nil {
			return model.Role{}, err
		}
	}
	for _, scope := range p.Scopes {
		refs, _ := json.Marshal(scope.ReferenceIDs)
		if _, err = tx.ExecContext(ctx, `INSERT INTO identity_authorization_role_scope(domain_type,domain_id,role_id,resource_code,scope_type,reference_json) VALUES(?,?,?,?,?,?)`, d.Type, d.ID, roleID, scope.Resource, scope.Type, refs); err != nil {
			return model.Role{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE identity_authorization_role SET version=version+1,updated_at=CURRENT_TIMESTAMP(3) WHERE domain_type=? AND domain_id=? AND role_id=? AND version=?`, d.Type, d.ID, roleID, expected); err != nil {
		return model.Role{}, err
	}
	role.Version++
	if err = bumpDomain(ctx, tx, d); err != nil {
		return model.Role{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.Role{}, err
	}
	return role, nil
}
func (r *AuthorizationRepository) ReplaceSubjectGrants(ctx context.Context, d model.AuthorizationDomain, subject string, roles []int64, operation string, accessVersion uint64) error {
	sort.Slice(roles, func(i, j int) bool { return roles[i] < roles[j] })
	payload, _ := json.Marshal(struct {
		D string
		S string
		R []int64
		V uint64
	}{d.Key(), subject, roles, accessVersion})
	hash := sha256.Sum256(payload)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var existing []byte
	err = tx.QueryRowContext(ctx, `SELECT request_hash FROM identity_authorization_operation WHERE operation_id=?`, operation).Scan(&existing)
	if err == nil {
		if string(existing) != string(hash[:]) {
			return model.ErrIdempotencyConflict
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err = lockDomain(ctx, tx, d); err != nil {
		return err
	}
	var memberVersion uint64
	memberQuery := `SELECT access_version FROM identity_workforce_member WHERE organization_id=? AND subject=? AND status='ACTIVE' AND member_type='OPERATOR' AND merchant_id IS NULL FOR UPDATE`
	memberArgs := []any{d.OrganizationID, subject}
	if d.Type == model.AuthorizationMerchant {
		memberQuery = `SELECT access_version FROM identity_workforce_member WHERE organization_id=? AND merchant_id=? AND subject=? AND status='ACTIVE' AND member_type IN ('STAFF','ANCHOR') FOR UPDATE`
		memberArgs = []any{d.OrganizationID, d.ID, subject}
	}
	if err = tx.QueryRowContext(ctx, memberQuery, memberArgs...).Scan(&memberVersion); err != nil {
		return mapNotFound(err)
	}
	if memberVersion != accessVersion {
		return model.ErrAuthorizationConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE identity_subject_grant SET status='REVOKED',revoked_at=CURRENT_TIMESTAMP(3) WHERE domain_type=? AND domain_id=? AND subject=? AND status='ACTIVE'`, d.Type, d.ID, subject); err != nil {
		return err
	}
	for _, roleID := range roles {
		var count int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_authorization_role WHERE domain_type=? AND domain_id=? AND role_id=? AND status='ACTIVE'`, d.Type, d.ID, roleID).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return model.ErrAuthorizationInvalid
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO identity_subject_grant(grant_id,domain_type,domain_id,subject,role_id,status,access_version,operation_id) VALUES(?,?,?,?,?,'ACTIVE',?,?)`, authorizationRandomID(), d.Type, d.ID, subject, roleID, accessVersion, operation); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO identity_authorization_operation(operation_id,domain_type,domain_id,subject,access_version,request_hash) VALUES(?,?,?,?,?,?)`, operation, d.Type, d.ID, subject, accessVersion, hash[:]); err != nil {
		return err
	}
	if err = bumpDomain(ctx, tx, d); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *AuthorizationRepository) Effective(ctx context.Context, d model.AuthorizationDomain, p model.PrincipalContext) (model.Authorization, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return model.Authorization{}, err
	}
	defer tx.Rollback()
	a, err := effective(ctx, tx, d, p, r.entitlementMaxStaleness)
	if err != nil {
		return model.Authorization{}, err
	}
	return a, tx.Commit()
}
func (r *AuthorizationRepository) CapabilitySnapshot(ctx context.Context, d model.AuthorizationDomain, p model.PrincipalContext, moduleID, version, contributionID, surface string, maxStaleness time.Duration) (model.RegistryContribution, model.Authorization, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return model.RegistryContribution{}, model.Authorization{}, err
	}
	defer tx.Rollback()
	var raw []byte
	var revision uint64
	var projected sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT c.capability_json,s.registry_revision,s.projected_at FROM identity_contribution_projection c JOIN identity_registry_projection_state s ON s.singleton_id=1 WHERE c.module_id=? AND c.module_version=? AND c.contribution_id=? AND c.surface=? AND c.active=1 AND c.registry_revision=s.registry_revision`, moduleID, version, contributionID, surface).Scan(&raw, &revision, &projected)
	if err != nil {
		return model.RegistryContribution{}, model.Authorization{}, model.ErrRegistryProjectionStale
	}
	if !projected.Valid || maxStaleness <= 0 || time.Since(projected.Time) > maxStaleness {
		return model.RegistryContribution{}, model.Authorization{}, model.ErrRegistryProjectionStale
	}
	var c model.RegistryContribution
	if json.Unmarshal(raw, &c) != nil {
		return model.RegistryContribution{}, model.Authorization{}, model.ErrRegistryProjectionStale
	}
	c.RegistryRevision = revision
	for _, code := range append(append([]string{}, c.RequiredPermissions...), routePermissions(c.AllowedRoutes)...) {
		var active bool
		var pr uint64
		if err = tx.QueryRowContext(ctx, `SELECT active,registry_revision FROM identity_permission_projection WHERE permission_code=?`, code).Scan(&active, &pr); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return model.RegistryContribution{}, model.Authorization{}, err
			}
			return model.RegistryContribution{}, model.Authorization{}, model.ErrRegistryProjectionStale
		}
		if !active || pr != revision {
			return model.RegistryContribution{}, model.Authorization{}, model.ErrRegistryProjectionStale
		}
	}
	a, err := effective(ctx, tx, d, p, r.entitlementMaxStaleness)
	if err != nil {
		return model.RegistryContribution{}, model.Authorization{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.RegistryContribution{}, model.Authorization{}, err
	}
	return c, a, nil
}

func (r *AuthorizationRepository) RuntimeSnapshot(ctx context.Context, d model.AuthorizationDomain, principal model.PrincipalContext, surface string, maxStaleness time.Duration) (uint64, []model.RegistryContribution, model.Authorization, error) {
	tx, err := r.freshRegistryTx(ctx, maxStaleness)
	if err != nil {
		return 0, nil, model.Authorization{}, err
	}
	defer tx.Rollback()
	var revision uint64
	if err = tx.QueryRowContext(ctx, `SELECT registry_revision FROM identity_registry_projection_state WHERE singleton_id=1`).Scan(&revision); err != nil {
		return 0, nil, model.Authorization{}, model.ErrRegistryProjectionStale
	}
	rows, err := tx.QueryContext(ctx, `SELECT capability_json FROM identity_contribution_projection WHERE surface=? AND active=1 AND registry_revision=? ORDER BY module_id,module_version,contribution_id`, surface, revision)
	if err != nil {
		return 0, nil, model.Authorization{}, err
	}
	defer rows.Close()
	items := []model.RegistryContribution{}
	for rows.Next() {
		var raw []byte
		var item model.RegistryContribution
		if scanErr := rows.Scan(&raw); scanErr != nil {
			return 0, nil, model.Authorization{}, scanErr
		}
		if jsonErr := json.Unmarshal(raw, &item); jsonErr != nil {
			return 0, nil, model.Authorization{}, model.ErrRegistryProjectionStale
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return 0, nil, model.Authorization{}, err
	}
	authorization, err := effective(ctx, tx, d, principal, r.entitlementMaxStaleness)
	if err != nil {
		return 0, nil, model.Authorization{}, err
	}
	return revision, items, authorization, tx.Commit()
}

func (r *AuthorizationRepository) CatalogSnapshot(ctx context.Context, d model.AuthorizationDomain, principal model.PrincipalContext, maxStaleness time.Duration) (uint64, []model.RegistryModule, model.Authorization, error) {
	tx, err := r.freshRegistryTx(ctx, maxStaleness)
	if err != nil {
		return 0, nil, model.Authorization{}, err
	}
	defer tx.Rollback()
	var revision uint64
	if err = tx.QueryRowContext(ctx, `SELECT registry_revision FROM identity_registry_projection_state WHERE singleton_id=1`).Scan(&revision); err != nil {
		return 0, nil, model.Authorization{}, model.ErrRegistryProjectionStale
	}
	rows, err := tx.QueryContext(ctx, `SELECT module_id,module_name,module_version,release_json FROM identity_module_projection WHERE active=1 AND registry_revision=? ORDER BY module_id`, revision)
	if err != nil {
		return 0, nil, model.Authorization{}, err
	}
	defer rows.Close()
	items := []model.RegistryModule{}
	for rows.Next() {
		var item model.RegistryModule
		if err = rows.Scan(&item.ID, &item.Name, &item.Version, &item.Release); err != nil {
			return 0, nil, model.Authorization{}, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return 0, nil, model.Authorization{}, err
	}
	authorization, err := effective(ctx, tx, d, principal, r.entitlementMaxStaleness)
	if err != nil {
		return 0, nil, model.Authorization{}, err
	}
	return revision, items, authorization, tx.Commit()
}

func (r *AuthorizationRepository) freshRegistryTx(ctx context.Context, maxStaleness time.Duration) (*sql.Tx, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	var revision uint64
	var projected time.Time
	if err = tx.QueryRowContext(ctx, `SELECT registry_revision,projected_at FROM identity_registry_projection_state WHERE singleton_id=1`).Scan(&revision, &projected); err != nil {
		_ = tx.Rollback()
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, model.ErrRegistryProjectionStale
	}
	if revision == 0 || maxStaleness <= 0 || time.Since(projected) > maxStaleness {
		_ = tx.Rollback()
		return nil, model.ErrRegistryProjectionStale
	}
	return tx, nil
}
func effective(ctx context.Context, tx *sql.Tx, d model.AuthorizationDomain, p model.PrincipalContext, entitlementMaxStaleness time.Duration) (model.Authorization, error) {
	var revision, entitlement, platformBoundary uint64
	if err := tx.QueryRowContext(ctx, `SELECT revision,entitlement_revision,platform_boundary_revision FROM identity_authorization_domain WHERE domain_type=? AND domain_id=?`, d.Type, d.ID).Scan(&revision, &entitlement, &platformBoundary); err != nil {
		return model.Authorization{}, mapNotFound(err)
	}
	if d.Type == model.AuthorizationPlatform {
		if platformBoundary == 0 {
			return model.Authorization{}, model.ErrEntitlementUnavailable
		}
		// The shared wire claim is named entitlementRevision. For PLATFORM_ORG
		// it carries this explicit authorization-boundary revision instead;
		// it never implies subscription ownership or grants any permission.
		entitlement = platformBoundary
	} else if err := requireFreshEntitlement(ctx, tx, d.ID, entitlement, entitlementMaxStaleness); err != nil {
		return model.Authorization{}, err
	}
	if d.Type == model.AuthorizationMerchant && p.Member.Type == model.MemberOwner {
		return merchantOwnerAuthorization(ctx, tx, d, p, revision, entitlement)
	}
	query := `SELECT DISTINCT pp.permission_code,pp.resource_code FROM identity_subject_grant sg JOIN identity_authorization_role ar ON ar.domain_type=sg.domain_type AND ar.domain_id=sg.domain_id AND ar.role_id=sg.role_id AND ar.status='ACTIVE' JOIN identity_authorization_role_permission rp ON rp.domain_type=ar.domain_type AND rp.domain_id=ar.domain_id AND rp.role_id=ar.role_id JOIN identity_permission_projection pp ON pp.permission_code=rp.permission_code AND pp.active=1 WHERE sg.domain_type=? AND sg.domain_id=? AND sg.subject=? AND sg.status='ACTIVE' AND sg.access_version=?`
	queryArgs := []any{d.Type, d.ID, p.Subject.ID, p.Member.AccessVersion}
	if d.Type == model.AuthorizationMerchant {
		query = `SELECT DISTINCT pp.permission_code,pp.resource_code FROM identity_subject_grant sg JOIN identity_authorization_role ar ON ar.domain_type=sg.domain_type AND ar.domain_id=sg.domain_id AND ar.role_id=sg.role_id AND ar.status='ACTIVE' JOIN identity_authorization_role_permission rp ON rp.domain_type=ar.domain_type AND rp.domain_id=ar.domain_id AND rp.role_id=ar.role_id JOIN identity_permission_projection pp ON pp.permission_code=rp.permission_code AND pp.active=1 JOIN identity_entitlement_projection ep ON ep.merchant_id=? AND ep.permission_code=pp.permission_code AND ep.status='ACTIVE' AND ep.entitlement_revision=? WHERE sg.domain_type=? AND sg.domain_id=? AND sg.subject=? AND sg.status='ACTIVE' AND sg.access_version=?`
		queryArgs = []any{d.ID, entitlement, d.Type, d.ID, p.Subject.ID, p.Member.AccessVersion}
	}
	rows, err := tx.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return model.Authorization{}, err
	}
	permissions := []string{}
	resources := map[string]bool{}
	for rows.Next() {
		var code, res string
		if scanErr := rows.Scan(&code, &res); scanErr != nil {
			rows.Close()
			return model.Authorization{}, scanErr
		}
		permissions = append(permissions, code)
		resources[res] = true
	}
	rows.Close()
	sort.Strings(permissions)
	scopes := []modulesession.DataScope{}
	srows, err := tx.QueryContext(ctx, `SELECT DISTINCT rs.resource_code,rs.scope_type,rs.reference_json FROM identity_subject_grant sg JOIN identity_authorization_role ar ON ar.domain_type=sg.domain_type AND ar.domain_id=sg.domain_id AND ar.role_id=sg.role_id AND ar.status='ACTIVE' JOIN identity_authorization_role_scope rs ON rs.domain_type=ar.domain_type AND rs.domain_id=ar.domain_id AND rs.role_id=ar.role_id WHERE sg.domain_type=? AND sg.domain_id=? AND sg.subject=? AND sg.status='ACTIVE' AND sg.access_version=?`, d.Type, d.ID, p.Subject.ID, p.Member.AccessVersion)
	if err != nil {
		return model.Authorization{}, err
	}
	defer srows.Close()
	for srows.Next() {
		var resource, kind string
		var raw []byte
		if err = srows.Scan(&resource, &kind, &raw); err != nil {
			return model.Authorization{}, err
		}
		if !resources[resource] {
			continue
		}
		scope := modulesession.DataScope{Resource: resource, Type: kind}
		switch kind {
		case model.ScopeSelf:
			scope.Subject = p.Subject.ID
		case model.ScopeCurrentOrgUnit, model.ScopeOrgUnitSubtree:
			scope.OrganizationUnitIDs = append([]int64{}, p.OrganizationUnitIDs...)
		case model.ScopeCurrentShop:
			if p.Selected.ShopContext.Complete() {
				scope.ShopScopes = []modulesession.ShopScope{{MerchantID: p.Selected.MerchantID, ShopID: p.Selected.ShopID}}
			}
		case model.ScopeAssignedShops:
			for _, shop := range p.AvailableShops {
				scope.ShopScopes = append(scope.ShopScopes, modulesession.ShopScope{MerchantID: shop.MerchantID, ShopID: shop.ShopID})
			}
		case model.ScopeCustomReference:
			_ = json.Unmarshal(raw, &scope.ReferenceIDs)
		}
		scopes = append(scopes, scope)
	}
	return model.Authorization{Revision: revision, IdentityVersion: p.Subject.Version, OrganizationVersion: p.Organization.Version, EntitlementRevision: entitlement, Permissions: permissions, DataScopes: scopes}, nil
}

func merchantOwnerAuthorization(ctx context.Context, tx *sql.Tx, d model.AuthorizationDomain, p model.PrincipalContext, revision, entitlement uint64) (model.Authorization, error) {
	rows, err := tx.QueryContext(ctx, `SELECT ep.permission_code,pp.resource_code FROM identity_entitlement_projection ep JOIN identity_permission_projection pp ON pp.permission_code=ep.permission_code AND pp.active=1 WHERE ep.merchant_id=? AND ep.status='ACTIVE' AND ep.entitlement_revision=? ORDER BY ep.permission_code`, d.ID, entitlement)
	if err != nil {
		return model.Authorization{}, err
	}
	defer rows.Close()
	permissions := []string{}
	resources := map[string]bool{}
	for rows.Next() {
		var code, resource string
		if err = rows.Scan(&code, &resource); err != nil {
			return model.Authorization{}, err
		}
		permissions = append(permissions, code)
		resources[resource] = true
	}
	if err = rows.Err(); err != nil {
		return model.Authorization{}, err
	}
	scopes := []modulesession.DataScope{}
	for resource := range resources {
		scope := modulesession.DataScope{Resource: resource, Type: model.ScopeAssignedShops}
		for _, shop := range p.AvailableShops {
			if shop.MerchantID == d.ID && shop.Complete() {
				scope.ShopScopes = append(scope.ShopScopes, modulesession.ShopScope{MerchantID: shop.MerchantID, ShopID: shop.ShopID})
			}
		}
		scopes = append(scopes, scope)
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].Resource < scopes[j].Resource })
	return model.Authorization{Revision: revision, IdentityVersion: p.Subject.Version, OrganizationVersion: p.Organization.Version, EntitlementRevision: entitlement, Permissions: permissions, DataScopes: scopes}, nil
}
func lockDomain(ctx context.Context, tx *sql.Tx, d model.AuthorizationDomain) error {
	var x int64
	err := tx.QueryRowContext(ctx, `SELECT revision FROM identity_authorization_domain WHERE domain_type=? AND domain_id=? FOR UPDATE`, d.Type, d.ID).Scan(&x)
	return mapNotFound(err)
}
func bumpDomain(ctx context.Context, tx *sql.Tx, d model.AuthorizationDomain) error {
	_, err := tx.ExecContext(ctx, `UPDATE identity_authorization_domain SET revision=revision+1,updated_at=CURRENT_TIMESTAMP(3) WHERE domain_type=? AND domain_id=?`, d.Type, d.ID)
	return err
}
func permissionMap(ctx context.Context, tx *sql.Tx) (map[string]model.Permission, error) {
	rows, err := tx.QueryContext(ctx, `SELECT module_id,permission_code,name,resource_code,action,description,registry_revision FROM identity_permission_projection WHERE active=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]model.Permission{}
	for rows.Next() {
		var p model.Permission
		if scanErr := rows.Scan(&p.ModuleID, &p.Code, &p.Name, &p.Resource, &p.Action, &p.Description, &p.RegistryRevision); scanErr != nil {
			return nil, scanErr
		}
		out[p.Code] = p
	}
	return out, rows.Err()
}
func routePermissions(routes []model.RegistryAllowedRoute) []string {
	out := []string{}
	for _, r := range routes {
		out = append(out, r.RequiredPermissions...)
	}
	return out
}
func authorizationRandomID() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrAuthorizationNotFound
	}
	return err
}
func authorizationConflict(err error) error {
	if err == nil {
		return nil
	}
	if fmt.Sprintf("%v", err) != "" {
		return model.ErrAuthorizationConflict
	}
	return err
}
