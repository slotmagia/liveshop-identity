//go:build integration

package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

const integrationDSNEnv = "LIVESHOP_IDENTITY_TEST_MYSQL_DSN"

func TestResolveCustomerPrincipalWithoutWorkforceOrganization(t *testing.T) {
	database := integrationDatabase(t)
	for _, statement := range []string{
		`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(7,'Merchant Seven','ACTIVE',1)`,
		`INSERT INTO identity_shop(shop_id,merchant_id,name,code,status,version) VALUES(13,7,'Customer Shop','customer-shop','ACTIVE',1)`,
		`INSERT INTO identity_subject(subject,realm,principal_type,display_name,status,version) VALUES('customer-no-workforce','CUSTOMER','CUSTOMER','Customer','ACTIVE',1)`,
		`INSERT INTO identity_credential(credential_id,subject,namespace_type,merchant_id,shop_id,credential_kind,normalized_identifier,secret_hash,status,version) VALUES(19,'customer-no-workforce','SHOP',7,13,'USERNAME','customer','unused','ACTIVE',1)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	repository, err := NewDirectoryRepository(database)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := repository.ResolvePrincipalContext(context.Background(), "customer-no-workforce")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Member.ID != 0 || resolved.Organization.ID != 0 {
		t.Fatalf("customer unexpectedly resolved workforce context: %+v", resolved)
	}
	if len(resolved.AvailableShops) != 1 || resolved.AvailableShops[0].ShopID != 13 {
		t.Fatalf("available shops=%+v", resolved.AvailableShops)
	}
}

func TestCustomerEffectiveAuthorizationUsesMerchantEntitlement(t *testing.T) {
	database := integrationDatabase(t)
	seedAuthorizationFixture(t, database)
	for _, statement := range []string{
		`INSERT INTO identity_subject(subject,realm,principal_type,display_name,status,version) VALUES('customer-authorized','CUSTOMER','CUSTOMER','Authorized Customer','ACTIVE',1)`,
		`INSERT INTO identity_subject_grant(grant_id,domain_type,domain_id,subject,role_id,status,access_version,operation_id) VALUES('customer-grant','MERCHANT',7,'customer-authorized',1,'ACTIVE',0,'customer-bootstrap')`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := database.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	authorization, err := effective(context.Background(), tx, model.AuthorizationDomain{Type: model.AuthorizationMerchant, ID: 7}, model.PrincipalContext{
		Subject: model.Subject{ID: "customer-authorized", Realm: principal.RealmCustomer, PrincipalType: principal.TypeCustomer, Status: model.StatusActive, Version: 1},
	}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(authorization.Permissions) != 1 || authorization.Permissions[0] != "identity.directory.read" {
		t.Fatalf("permissions=%v", authorization.Permissions)
	}
}

func TestProvisionMemberTransactionAndIdempotency(t *testing.T) {
	database := integrationDatabase(t)
	seedAuthorizationFixture(t, database)
	repository, err := NewDirectoryRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	directory := biz.NewDirectory(repository)
	command := biz.ProvisionMember{
		OperationID: "provision-staff-1", IdempotencyKey: "provision-key-1", Subject: "subject-staff-1",
		Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff, DisplayName: "Staff One",
		OrganizationID: 9, MerchantID: 7, MemberType: model.MemberStaff, AssignmentKind: model.AssignmentOperate,
		CredentialKind: "USERNAME", CredentialNamespace: "GLOBAL", NormalizedIdentifier: "staff-one",
		Password: "password-one", RoleIDs: []int64{1},
	}
	first, err := directory.ProvisionMember(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	second, err := directory.ProvisionMember(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if second.MemberID != first.MemberID || second.Subject != first.Subject {
		t.Fatalf("replay diverged: first=%+v second=%+v", first, second)
	}
	assertCounts(t, database, map[string]int{
		"identity_subject": 1, "identity_workforce_member": 1, "identity_credential": 1,
		"identity_subject_grant": 1, "identity_outbox": 1, "identity_idempotency": 1,
	})

	changed := command
	changed.Password = "different-password"
	if _, err := directory.ProvisionMember(context.Background(), changed); !errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("changed payload must conflict, got %v", err)
	}
	assertCounts(t, database, map[string]int{
		"identity_subject": 1, "identity_workforce_member": 1, "identity_credential": 1,
		"identity_subject_grant": 1, "identity_outbox": 1, "identity_idempotency": 1,
	})
}

func TestUpdateMemberTransactionAndIdempotency(t *testing.T) {
	database := integrationDatabase(t)
	seedAuthorizationFixture(t, database)
	if _, err := database.Exec(`INSERT INTO identity_shop(shop_id,merchant_id,app_id,commercial_id,code,name,status,version) VALUES(101,7,1,101,'shop-seven-101','Shop 101','ACTIVE',1),(102,7,1,102,'shop-seven-102','Shop 102','ACTIVE',1)`); err != nil {
		t.Fatal(err)
	}
	repository, err := NewDirectoryRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	directory := biz.NewDirectory(repository)
	created, err := directory.ProvisionMember(context.Background(), biz.ProvisionMember{
		OperationID: "provision-staff-3", IdempotencyKey: "provision-key-3", Subject: "subject-staff-3",
		Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff, DisplayName: "Staff Three",
		OrganizationID: 9, MerchantID: 7, MemberType: model.MemberStaff, AssignmentKind: model.AssignmentOperate,
		ShopIDs: []int64{101}, CredentialKind: "USERNAME", CredentialNamespace: "GLOBAL", NormalizedIdentifier: "staff-three",
		Password: "password-one", RoleIDs: []int64{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO identity_session(session_id,session_family_id,subject,selected_organization_id,context_version,status,expires_at) VALUES('staff-session','staff-family','subject-staff-3',9,1,'ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY))`); err != nil {
		t.Fatal(err)
	}
	command := biz.UpdateMember{
		OperationID: "update-staff-3", IdempotencyKey: "update-key-3", Subject: "subject-staff-3", MerchantID: 7,
		DisplayName: "Staff Three Updated", MemberType: model.MemberStaff, ExpectedIdentityVersion: 1, ExpectedAccessVersion: created.AccessVersion,
		ShopIDs: []int64{101, 102}, RoleIDs: []int64{1, 2}, AssignmentKind: model.AssignmentOperate,
	}
	first, err := directory.UpdateMember(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if first.IdentityVersion != 2 || first.AccessVersion != created.AccessVersion+1 {
		t.Fatalf("versions=%+v", first)
	}
	second, err := directory.UpdateMember(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if second.MemberID != first.MemberID || second.IdentityVersion != first.IdentityVersion || second.AccessVersion != first.AccessVersion {
		t.Fatalf("replay diverged: first=%+v second=%+v", first, second)
	}
	if got := scalarInt(t, database, fmt.Sprintf(`SELECT COUNT(*) FROM identity_member_shop WHERE member_id=%d AND status='ACTIVE'`, created.MemberID)); got != 2 {
		t.Fatalf("active shops=%d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_subject_grant WHERE subject='subject-staff-3' AND status='ACTIVE'`); got != 2 {
		t.Fatalf("active grants=%d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_session WHERE subject='subject-staff-3' AND status='ACTIVE'`); got != 0 {
		t.Fatalf("active sessions=%d", got)
	}
	if got := scalarInt(t, database, fmt.Sprintf(`SELECT COUNT(*) FROM identity_outbox WHERE aggregate_id='%d'`, created.MemberID)); got != 2 {
		t.Fatalf("outbox count=%d", got)
	}
	changed := command
	changed.DisplayName = "Different"
	if _, err := directory.UpdateMember(context.Background(), changed); !errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("changed payload must conflict, got %v", err)
	}
}

func TestPlatformUserLifecycleTransactionIdempotencyAndSessionRevocation(t *testing.T) {
	database := integrationDatabase(t)
	for _, statement := range []string{
		`INSERT INTO identity_organization(organization_id,organization_type,name,status,version) VALUES(1,'PLATFORM','Platform','ACTIVE',1)`,
		`INSERT INTO identity_authorization_domain(domain_type,domain_id,revision,entitlement_revision,platform_boundary_revision) VALUES('PLATFORM_ORG',1,1,0,1)`,
		`INSERT INTO identity_authorization_role(domain_type,domain_id,role_id,code,name,status,system_role,version) VALUES('PLATFORM_ORG',1,1,'SYSTEM_ADMIN','System administrator','ACTIVE',1,1),('PLATFORM_ORG',1,2,'OPERATOR','Operator','ACTIVE',0,1)`,
		`INSERT INTO identity_subject(subject,realm,principal_type,display_name,status,version) VALUES('protected-admin','PLATFORM','PLATFORM_OPERATOR','Protected','ACTIVE',1)`,
		`INSERT INTO identity_workforce_member(organization_id,subject,member_type,status,access_version) VALUES(1,'protected-admin','OPERATOR','ACTIVE',1)`,
		`INSERT INTO identity_subject_grant(grant_id,domain_type,domain_id,subject,role_id,status,access_version,operation_id) VALUES('protected-grant','PLATFORM_ORG',1,'protected-admin',1,'ACTIVE',1,'bootstrap')`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	directory, _ := NewDirectoryRepository(database)
	repository, _ := NewUserLifecycleRepository(database, directory, 2*time.Minute, 3*time.Minute)
	service := biz.NewUserLifecycle(repository)
	create := biz.CreateOperator{IdempotencyKey: "operator-key", OperationID: "operator-operation", Subject: "created-operator", DisplayName: "Created Operator", Username: "created", Password: "password-one", OrganizationID: 1, RoleIDs: []int64{2}}
	first, err := service.CreateOperator(context.Background(), create)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateOperator(context.Background(), create)
	if err != nil {
		t.Fatal(err)
	}
	if first.Subject.ID != second.Subject.ID || first.Member.ID != second.Member.ID {
		t.Fatalf("replay diverged: first=%+v second=%+v", first, second)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_subject WHERE subject='created-operator'`); got != 1 {
		t.Fatalf("subject count=%d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_outbox WHERE aggregate_id='created-operator'`); got != 1 {
		t.Fatalf("outbox count=%d", got)
	}
	if _, err := database.Exec(`INSERT INTO identity_registry_projection_state(singleton_id,registry_revision,snapshot_digest,projected_at) VALUES(1,1,UNHEX(SHA2('registry',256)),CURRENT_TIMESTAMP(3))`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO identity_session(session_id,session_family_id,subject,selected_organization_id,context_version,status,expires_at) VALUES('current-session','current-family','created-operator',1,1,'ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY))`); err != nil {
		t.Fatal(err)
	}
	claims := modulesession.Claims{Subject: "created-operator", Realm: principal.RealmPlatform, PrincipalType: principal.TypePlatformOperator, SessionID: "current-session", OrganizationID: 1, IdentityVersion: 1, OrganizationVersion: 1, AuthorizationRevision: 2, EntitlementRevision: 1, RegistryRevision: 1, ContextVersion: 1}
	if err := service.ValidateCurrentAuthorization(context.Background(), claims); err != nil {
		t.Fatalf("current authorization rejected: %v", err)
	}
	if _, err := database.Exec(`UPDATE identity_registry_projection_state SET projected_at=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 150 SECOND) WHERE singleton_id=1`); err != nil {
		t.Fatal(err)
	}
	if err := service.ValidateCurrentAuthorization(context.Background(), claims); err != nil {
		t.Fatalf("registry projection inside registry freshness rejected: %v", err)
	}
	if _, err := database.Exec(`UPDATE identity_registry_projection_state SET projected_at=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 181 SECOND) WHERE singleton_id=1`); err != nil {
		t.Fatal(err)
	}
	if err := service.ValidateCurrentAuthorization(context.Background(), claims); !errors.Is(err, model.ErrAuthorizationDenied) {
		t.Fatalf("stale registry projection accepted: %v", err)
	}
	if _, err := database.Exec(`UPDATE identity_registry_projection_state SET projected_at=CURRENT_TIMESTAMP(3) WHERE singleton_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE identity_session SET context_version=2 WHERE session_id='current-session'`); err != nil {
		t.Fatal(err)
	}
	if err := service.ValidateCurrentAuthorization(context.Background(), claims); !errors.Is(err, model.ErrAuthorizationDenied) {
		t.Fatalf("capability survived session context change: %v", err)
	}
	if _, err := database.Exec(`UPDATE identity_session SET context_version=1 WHERE session_id='current-session'`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE identity_authorization_domain SET platform_boundary_revision=2 WHERE domain_type='PLATFORM_ORG' AND domain_id=1`); err != nil {
		t.Fatal(err)
	}
	if err := service.ValidateCurrentAuthorization(context.Background(), claims); !errors.Is(err, model.ErrAuthorizationDenied) {
		t.Fatalf("capability survived entitlement boundary change: %v", err)
	}
	if _, err := database.Exec(`UPDATE identity_authorization_domain SET platform_boundary_revision=1 WHERE domain_type='PLATFORM_ORG' AND domain_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE identity_authorization_domain SET revision=revision+1 WHERE domain_type='PLATFORM_ORG' AND domain_id=1`); err != nil {
		t.Fatal(err)
	}
	if err := service.ValidateCurrentAuthorization(context.Background(), claims); !errors.Is(err, model.ErrAuthorizationDenied) {
		t.Fatalf("stale authorization accepted: %v", err)
	}
	if _, err := database.Exec(`UPDATE identity_authorization_domain SET revision=2 WHERE domain_type='PLATFORM_ORG' AND domain_id=1`); err != nil {
		t.Fatal(err)
	}
	changed := create
	changed.Username = "other"
	if _, err := service.CreateOperator(context.Background(), changed); !errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("changed replay=%v", err)
	}
	invalid := create
	invalid.IdempotencyKey = "invalid-role"
	invalid.OperationID = "invalid-role"
	invalid.Subject = "invalid-role"
	invalid.Username = "invalid-role"
	invalid.RoleIDs = []int64{999}
	if _, err := service.CreateOperator(context.Background(), invalid); !errors.Is(err, model.ErrAuthorizationInvalid) {
		t.Fatalf("invalid role create=%v", err)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_subject WHERE subject='invalid-role'`); got != 0 {
		t.Fatalf("failed create leaked subject=%d", got)
	}

	if _, err := service.ChangeStatus(context.Background(), biz.ChangeUserStatus{IdempotencyKey: "protect", OperationID: "protect", Subject: "protected-admin", ActorSubject: "created-operator", Scope: biz.UserScope{OrganizationID: 1}, ExpectedIdentityVersion: 1, ExpectedAccessVersion: 1, Target: model.StatusDisabled}); !errors.Is(err, model.ErrSystemRoleProtected) {
		t.Fatalf("system operator disable=%v", err)
	}
	disabled, err := service.ChangeStatus(context.Background(), biz.ChangeUserStatus{IdempotencyKey: "disable", OperationID: "disable", Subject: "created-operator", ActorSubject: "protected-admin", Scope: biz.UserScope{OrganizationID: 1}, ExpectedIdentityVersion: 1, ExpectedAccessVersion: 1, Target: model.StatusDisabled})
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Subject.Status != model.StatusDisabled || disabled.Member.Status != model.MemberSuspended {
		t.Fatalf("disabled=%+v", disabled)
	}
	if _, err := service.ChangeStatus(context.Background(), biz.ChangeUserStatus{IdempotencyKey: "stale-enable", OperationID: "stale-enable", Subject: "created-operator", ActorSubject: "protected-admin", Scope: biz.UserScope{OrganizationID: 1}, ExpectedIdentityVersion: 1, ExpectedAccessVersion: 1, Target: model.StatusActive}); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("stale status version accepted=%v", err)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_subject WHERE subject='created-operator' AND status='DISABLED' AND version=2`); got != 1 {
		t.Fatalf("stale status update changed subject=%d", got)
	}
	if _, err := database.Exec(`UPDATE identity_authorization_role SET system_role=0 WHERE domain_type='PLATFORM_ORG' AND domain_id=1 AND role_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeStatus(context.Background(), biz.ChangeUserStatus{IdempotencyKey: "last", OperationID: "last", Subject: "protected-admin", ActorSubject: "protected-admin", Scope: biz.UserScope{OrganizationID: 1}, ExpectedIdentityVersion: 1, ExpectedAccessVersion: 1, Target: model.StatusDisabled}); !errors.Is(err, model.ErrLastActiveOperator) {
		t.Fatalf("last active operator disable=%v", err)
	}
	enabled, err := service.ChangeStatus(context.Background(), biz.ChangeUserStatus{IdempotencyKey: "enable", OperationID: "enable", Subject: "created-operator", ActorSubject: "protected-admin", Scope: biz.UserScope{OrganizationID: 1}, ExpectedIdentityVersion: 2, ExpectedAccessVersion: 2, Target: model.StatusActive})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := database.Exec(`INSERT INTO identity_session(session_id,session_family_id,subject,context_version,status,expires_at) VALUES('session-one','family-one','created-operator',1,'ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY))`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO identity_refresh_token(token_hash,session_id,status,expires_at) VALUES(UNHEX(SHA2('token-one',256)),'session-one','ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY))`); err != nil {
		t.Fatal(err)
	}
	reset := biz.ResetCredential{IdempotencyKey: "reset", OperationID: "reset", Subject: "created-operator", ActorSubject: "protected-admin", Scope: biz.UserScope{OrganizationID: 1}, CredentialID: enabled.Credential.ID, ExpectedCredentialVersion: 1, Password: "password-two"}
	credential, err := service.ResetCredential(context.Background(), reset)
	if err != nil {
		t.Fatal(err)
	}
	if credential.Version != 2 {
		t.Fatalf("credential=%+v", credential)
	}
	if _, err := service.ResetCredential(context.Background(), reset); err != nil {
		t.Fatal(err)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_session WHERE subject='created-operator' AND status='ACTIVE'`); got != 0 {
		t.Fatalf("active sessions=%d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_refresh_token WHERE status='ACTIVE'`); got != 0 {
		t.Fatalf("active refresh tokens=%d", got)
	}
	changedReset := reset
	changedReset.Password = "password-three"
	if _, err := service.ResetCredential(context.Background(), changedReset); !errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("changed reset replay=%v", err)
	}
	for _, statement := range []string{
		`INSERT INTO identity_session(session_id,session_family_id,subject,context_version,status,expires_at) VALUES('session-two','family-two','created-operator',1,'ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY)),('session-three','family-three','created-operator',1,'ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY))`,
		`INSERT INTO identity_refresh_token(token_hash,session_id,status,expires_at) VALUES(UNHEX(SHA2('token-two',256)),'session-two','ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY)),(UNHEX(SHA2('token-three',256)),'session-three','ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY))`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	revokeOne := biz.RevokeSessions{IdempotencyKey: "revoke-one", OperationID: "revoke-one", Subject: "created-operator", ActorSubject: "protected-admin", Scope: biz.UserScope{OrganizationID: 1}, SessionID: "session-two"}
	if err := service.RevokeSessions(context.Background(), revokeOne); err != nil {
		t.Fatal(err)
	}
	if err := service.RevokeSessions(context.Background(), revokeOne); err != nil {
		t.Fatalf("specific session replay=%v", err)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_session WHERE subject='created-operator' AND status='ACTIVE'`); got != 1 {
		t.Fatalf("specific revoke active sessions=%d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_refresh_token rt JOIN identity_session s ON s.session_id=rt.session_id WHERE s.session_id='session-two' AND rt.status='ACTIVE'`); got != 0 {
		t.Fatalf("specific revoke active refresh=%d", got)
	}
	changedRevoke := revokeOne
	changedRevoke.SessionID = "session-three"
	if err := service.RevokeSessions(context.Background(), changedRevoke); !errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("changed revoke replay=%v", err)
	}
	revokeAll := biz.RevokeSessions{IdempotencyKey: "revoke-all", OperationID: "revoke-all", Subject: "created-operator", ActorSubject: "protected-admin", Scope: biz.UserScope{OrganizationID: 1}}
	if err := service.RevokeSessions(context.Background(), revokeAll); err != nil {
		t.Fatal(err)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_session WHERE subject='created-operator' AND status='ACTIVE'`); got != 0 {
		t.Fatalf("revoke all active sessions=%d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_refresh_token rt JOIN identity_session s ON s.session_id=rt.session_id WHERE s.subject='created-operator' AND rt.status='ACTIVE'`); got != 0 {
		t.Fatalf("revoke all active refresh=%d", got)
	}
}

func TestReplaceSubjectGrantsTransactionIsolationAndReplay(t *testing.T) {
	database := integrationDatabase(t)
	seedAuthorizationFixture(t, database)
	directoryRepository, _ := NewDirectoryRepository(database)
	directory := biz.NewDirectory(directoryRepository)
	_, err := directory.ProvisionMember(context.Background(), biz.ProvisionMember{
		OperationID: "provision-staff-2", IdempotencyKey: "provision-key-2", Subject: "subject-staff-2",
		Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff, DisplayName: "Staff Two",
		OrganizationID: 9, MerchantID: 7, MemberType: model.MemberStaff, AssignmentKind: model.AssignmentOperate,
		CredentialKind: "USERNAME", CredentialNamespace: "GLOBAL", NormalizedIdentifier: "staff-two",
		Password: "password-two", RoleIDs: []int64{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	authorizationRepository, _ := NewAuthorizationRepository(database, 2*time.Minute)
	service := biz.NewAuthorization(authorizationRepository)
	domain := model.AuthorizationDomain{Type: model.AuthorizationMerchant, ID: 7, OrganizationID: 9}
	otherDomain := model.AuthorizationDomain{Type: model.AuthorizationMerchant, ID: 8, OrganizationID: 10}

	baselineRevision := scalarInt(t, database, `SELECT revision FROM identity_authorization_domain WHERE domain_type='MERCHANT' AND domain_id=7`)
	baselineGrants := scalarInt(t, database, `SELECT COUNT(*) FROM identity_subject_grant`)
	for name, attempt := range map[string]func() error{
		"stale access version": func() error {
			return service.ReplaceSubjectGrants(context.Background(), domain, "subject-staff-2", []int64{2}, "grant-stale", 9)
		},
		"unknown role": func() error {
			return service.ReplaceSubjectGrants(context.Background(), domain, "subject-staff-2", []int64{999}, "grant-invalid", 1)
		},
		"cross merchant": func() error {
			return service.ReplaceSubjectGrants(context.Background(), otherDomain, "subject-staff-2", []int64{3}, "grant-cross", 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := attempt(); err == nil {
				t.Fatal("invalid grant replacement succeeded")
			}
			if got := scalarInt(t, database, `SELECT revision FROM identity_authorization_domain WHERE domain_type='MERCHANT' AND domain_id=7`); got != baselineRevision {
				t.Fatalf("failed operation changed revision: %d -> %d", baselineRevision, got)
			}
			if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_subject_grant`); got != baselineGrants {
				t.Fatalf("failed operation changed grants: %d -> %d", baselineGrants, got)
			}
		})
	}

	if err := service.ReplaceSubjectGrants(context.Background(), domain, "subject-staff-2", []int64{2}, "grant-valid", 1); err != nil {
		t.Fatal(err)
	}
	if got := scalarInt(t, database, `SELECT revision FROM identity_authorization_domain WHERE domain_type='MERCHANT' AND domain_id=7`); got != baselineRevision+1 {
		t.Fatalf("valid replacement did not increment revision once: %d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_subject_grant WHERE subject='subject-staff-2' AND status='ACTIVE' AND role_id=2`); got != 1 {
		t.Fatalf("valid replacement active grants=%d", got)
	}
	afterGrants := scalarInt(t, database, `SELECT COUNT(*) FROM identity_subject_grant`)
	if err := service.ReplaceSubjectGrants(context.Background(), domain, "subject-staff-2", []int64{2}, "grant-valid", 1); err != nil {
		t.Fatal(err)
	}
	if got := scalarInt(t, database, `SELECT revision FROM identity_authorization_domain WHERE domain_type='MERCHANT' AND domain_id=7`); got != baselineRevision+1 {
		t.Fatalf("replay incremented revision: %d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_subject_grant`); got != afterGrants {
		t.Fatalf("replay created another grant: %d -> %d", afterGrants, got)
	}
	if err := service.ReplaceSubjectGrants(context.Background(), domain, "subject-staff-2", []int64{1}, "grant-valid", 1); !errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("same operation with changed roles must conflict, got %v", err)
	}
}

func TestGuestSessionCommitsSubjectSessionRefreshAndOutboxAtomically(t *testing.T) {
	database := integrationDatabase(t)
	for _, statement := range []string{
		`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(7,'Merchant Seven','ACTIVE',1)`,
		`INSERT INTO identity_shop(shop_id,merchant_id,name,code,status,version) VALUES(13,7,'Guest Shop','guest-shop','ACTIVE',1)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	directory, err := NewDirectoryRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	repository, err := NewAuthRepository(database, directory)
	if err != nil {
		t.Fatal(err)
	}
	refreshHash := sha256.Sum256([]byte("guest-refresh"))
	session, err := repository.Guest(context.Background(), biz.GuestCommand{
		Subject: "guest-integration", ShopCode: "guest-shop", SessionID: "guest-session", FamilyID: "guest-family",
		RefreshHash: refreshHash, ExpiresAt: time.Now().Add(time.Hour), IPAddress: "127.0.0.1", UserAgent: "integration",
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.Subject.PrincipalType != principal.TypeGuest || session.Selected.ShopID != 13 || session.Selected.MerchantID != 7 {
		t.Fatalf("unexpected guest session: %#v", session)
	}
	assertCounts(t, database, map[string]int{"identity_subject": 1, "identity_session": 1, "identity_refresh_token": 1, "identity_outbox": 2})
	var authenticationLevel string
	if err := database.QueryRow(`SELECT authentication_level FROM identity_session WHERE session_id='guest-session' AND selected_merchant_id=7 AND selected_shop_id=13`).Scan(&authenticationLevel); err != nil || authenticationLevel != "GUEST" {
		t.Fatalf("guest session binding: level=%q err=%v", authenticationLevel, err)
	}

	_, err = repository.Guest(context.Background(), biz.GuestCommand{
		Subject: "guest-rollback", ShopCode: "missing-shop", SessionID: "guest-session-rollback", FamilyID: "guest-family-rollback",
		RefreshHash: sha256.Sum256([]byte("rollback-refresh")), ExpiresAt: time.Now().Add(time.Hour),
	})
	if !errors.Is(err, biz.ErrInvalidCredentials) {
		t.Fatalf("missing shop returned %v", err)
	}
	assertCounts(t, database, map[string]int{"identity_subject": 1, "identity_session": 1, "identity_refresh_token": 1, "identity_outbox": 2})
}

func integrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	rawDSN := os.Getenv(integrationDSNEnv)
	if rawDSN == "" {
		t.Skipf("set %s to run MySQL integration tests", integrationDSNEnv)
	}
	configuration, err := driver.ParseDSN(rawDSN)
	if err != nil {
		t.Fatalf("parse test DSN: %v", err)
	}
	configuration.DBName = ""
	admin, err := sql.Open("mysql", configuration.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	databaseName := fmt.Sprintf("liveshop_identity_it_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE DATABASE ` + databaseName + ` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci`); err != nil {
		admin.Close()
		t.Fatalf("create integration database: %v", err)
	}
	t.Cleanup(func() {
		// databaseName is generated locally from decimal digits only.
		_, _ = admin.Exec(`DROP DATABASE ` + databaseName)
		_ = admin.Close()
	})
	configuration.DBName = databaseName
	configuration.MultiStatements = true
	database, err := sql.Open("mysql", configuration.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	applyIntegrationMigrations(t, database)
	return database
}

func applyIntegrationMigrations(t *testing.T, database *sql.DB) {
	t.Helper()
	directory := filepath.Join("..", "..", "..", "..", "migrations")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		contents, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(string(contents)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
}

func seedAuthorizationFixture(t *testing.T, database *sql.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO identity_permission_projection(permission_code,module_id,name,resource_code,action,description,registry_revision,active) VALUES('identity.directory.read','identity','Read directory','identity.directory','read','',1,1)`,
		`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(7,'Merchant Seven','ACTIVE',1),(8,'Merchant Eight','ACTIVE',1)`,
		`INSERT INTO identity_organization(organization_id,organization_type,merchant_id,name,status,version) VALUES(9,'MERCHANT',7,'Org Seven','ACTIVE',1),(10,'MERCHANT',8,'Org Eight','ACTIVE',1)`,
		`INSERT INTO identity_authorization_domain(domain_type,domain_id,revision,entitlement_revision) VALUES('MERCHANT',7,1,1),('MERCHANT',8,1,1)`,
		`INSERT INTO identity_entitlement_projection_state(merchant_id,entitlement_revision,snapshot_digest,source_updated_at,projected_at) VALUES(7,1,UNHEX(SHA2('identity.directory.read',256)),CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3)),(8,1,UNHEX(SHA2('identity.directory.read',256)),CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3))`,
		`INSERT INTO identity_authorization_role(domain_type,domain_id,role_id,code,name,status,system_role,version) VALUES('MERCHANT',7,1,'staff-reader','Reader','ACTIVE',0,1),('MERCHANT',7,2,'staff-second','Second','ACTIVE',0,1),('MERCHANT',8,3,'other-role','Other','ACTIVE',0,1)`,
		`INSERT INTO identity_authorization_role_permission(domain_type,domain_id,role_id,permission_code) VALUES('MERCHANT',7,1,'identity.directory.read')`,
		`INSERT INTO identity_entitlement_projection(merchant_id,permission_code,status,entitlement_revision) VALUES(7,'identity.directory.read','ACTIVE',1),(8,'identity.directory.read','ACTIVE',1)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func assertCounts(t *testing.T, database *sql.DB, expected map[string]int) {
	t.Helper()
	for table, count := range expected {
		if got := scalarInt(t, database, `SELECT COUNT(*) FROM `+table); got != count {
			t.Fatalf("%s count=%d want=%d", table, got, count)
		}
	}
}

func scalarInt(t *testing.T, database *sql.DB, query string) int {
	t.Helper()
	var result int
	if err := database.QueryRow(query).Scan(&result); err != nil {
		t.Fatal(err)
	}
	return result
}
