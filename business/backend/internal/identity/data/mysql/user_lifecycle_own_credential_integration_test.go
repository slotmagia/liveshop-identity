//go:build integration

package mysql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

func TestChangeOwnCredentialRetainsCurrentSessionAndRevokesOthers(t *testing.T) {
	database := integrationDatabase(t)
	hash, err := HashPassword("password-old")
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(71,'Merchant Seventy One','ACTIVE',1)`,
		`INSERT INTO identity_organization(organization_id,organization_type,merchant_id,name,status,version) VALUES(71,'MERCHANT',71,'Org Seventy One','ACTIVE',1)`,
		`INSERT INTO identity_subject(subject,realm,principal_type,display_name,status,version) VALUES('owner-security-71','MERCHANT','MERCHANT_OWNER','Owner Seventy One','ACTIVE',1)`,
		`INSERT INTO identity_workforce_member(organization_id,merchant_id,subject,member_type,status,access_version) VALUES(71,71,'owner-security-71','OWNER','ACTIVE',1)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.Exec(`INSERT INTO identity_credential(credential_id,subject,namespace_type,credential_kind,normalized_identifier,secret_hash,status,version) VALUES(71,'owner-security-71','GLOBAL','USERNAME','owner71',?,'ACTIVE',1)`, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO identity_session(session_id,session_family_id,subject,selected_organization_id,selected_merchant_id,context_version,status,expires_at) VALUES('current-session-71','current-family-71','owner-security-71',71,71,1,'ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY)),('other-session-71','other-family-71','owner-security-71',71,71,1,'ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY))`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO identity_refresh_token(token_hash,session_id,status,expires_at) VALUES(UNHEX(SHA2('token-current-71',256)),'current-session-71','ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY)),(UNHEX(SHA2('token-other-71',256)),'other-session-71','ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY))`); err != nil {
		t.Fatal(err)
	}

	directory, err := NewDirectoryRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	repository, err := NewUserLifecycleRepository(database, directory, 2*time.Minute, 3*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service := biz.NewUserLifecycle(repository)
	scope := biz.UserScope{OrganizationID: 71, MerchantID: 71}

	own, err := service.OwnAccount(context.Background(), scope, "owner-security-71")
	if err != nil {
		t.Fatal(err)
	}
	if own.Subject.PrincipalType != principal.TypeMerchantOwner || own.Credential.Identifier != "owner71" || own.ActiveSessions != 2 {
		t.Fatalf("own account=%+v", own)
	}

	command := biz.ChangeOwnCredential{
		IdempotencyKey: "change-own-71", OperationID: "change-own-71", Subject: "owner-security-71", ActorSubject: "owner-security-71",
		SessionID: "current-session-71", OldPassword: "wrong-password", Password: "password-new", Scope: scope, ExpectedCredentialVersion: 1,
	}
	if _, err := service.ChangeOwnCredential(context.Background(), command); !errors.Is(err, model.ErrInvalidCredential) {
		t.Fatalf("wrong password=%v", err)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_session WHERE subject='owner-security-71' AND status='ACTIVE'`); got != 2 {
		t.Fatalf("wrong password revoked sessions=%d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_idempotency WHERE command_type='change_own_credential'`); got != 0 {
		t.Fatalf("failed password leaked idempotency=%d", got)
	}

	command.OldPassword = "password-old"
	first, err := service.ChangeOwnCredential(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if first.Replayed || !first.CurrentRetained || first.RevokedSessions != 1 || first.Credential.Version != 2 {
		t.Fatalf("first=%+v", first)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_session WHERE session_id='current-session-71' AND status='ACTIVE'`); got != 1 {
		t.Fatalf("current session not retained=%d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_session WHERE session_id='other-session-71' AND status='REVOKED' AND revoke_reason='CREDENTIAL_CHANGED'`); got != 1 {
		t.Fatalf("other session not revoked=%d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_refresh_token WHERE session_id='current-session-71' AND status='ACTIVE'`); got != 1 {
		t.Fatalf("current refresh revoked=%d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_refresh_token WHERE session_id='other-session-71' AND status='REVOKED'`); got != 1 {
		t.Fatalf("other refresh not revoked=%d", got)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_outbox WHERE event_type='identity.credential.changed'`); got != 1 {
		t.Fatalf("outbox=%d", got)
	}

	replay, err := service.ChangeOwnCredential(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Replayed || replay.Credential.Version != 2 || replay.RevokedSessions != 1 {
		t.Fatalf("replay=%+v", replay)
	}
	if got := scalarInt(t, database, `SELECT version FROM identity_credential WHERE credential_id=71`); got != 2 {
		t.Fatalf("replay mutated version=%d", got)
	}

	changed := command
	changed.Password = "password-alt"
	if _, err := service.ChangeOwnCredential(context.Background(), changed); !errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("changed payload=%v", err)
	}
}

func TestOwnerCanListOwnSessionsButNotManagedStaffSessions(t *testing.T) {
	database := integrationDatabase(t)
	for _, statement := range []string{
		`INSERT INTO identity_merchant(merchant_id,name,status,version) VALUES(72,'Merchant Seventy Two','ACTIVE',1)`,
		`INSERT INTO identity_organization(organization_id,organization_type,merchant_id,name,status,version) VALUES(72,'MERCHANT',72,'Org Seventy Two','ACTIVE',1)`,
		`INSERT INTO identity_subject(subject,realm,principal_type,display_name,status,version) VALUES('owner-device-72','MERCHANT','MERCHANT_OWNER','Owner Seventy Two','ACTIVE',1)`,
		`INSERT INTO identity_workforce_member(organization_id,merchant_id,subject,member_type,status,access_version) VALUES(72,72,'owner-device-72','OWNER','ACTIVE',1)`,
		`INSERT INTO identity_session(session_id,session_family_id,subject,selected_organization_id,selected_merchant_id,context_version,status,expires_at) VALUES('owner-session-72','owner-family-72','owner-device-72',72,72,1,'ACTIVE',DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY))`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	directory, err := NewDirectoryRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	repository, err := NewUserLifecycleRepository(database, directory, 2*time.Minute, 3*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service := biz.NewUserLifecycle(repository)
	scope := biz.UserScope{OrganizationID: 72, MerchantID: 72}
	if _, err := service.Sessions(context.Background(), scope, "owner-device-72"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("staff-only list=%v", err)
	}
	own, err := service.OwnSessions(context.Background(), scope, "owner-device-72")
	if err != nil {
		t.Fatal(err)
	}
	if len(own) != 1 || own[0].ID != "owner-session-72" {
		t.Fatalf("own sessions=%+v", own)
	}
	if err := service.RevokeSessions(context.Background(), biz.RevokeSessions{
		IdempotencyKey: "admin-72", OperationID: "admin-72", Subject: "owner-device-72", ActorSubject: "owner-device-72",
		SessionID: "owner-session-72", Scope: scope,
	}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("staff-only revoke=%v", err)
	}
	if err := service.RevokeOwnSessions(context.Background(), biz.RevokeSessions{
		IdempotencyKey: "self-72", OperationID: "self-72", Subject: "owner-device-72", ActorSubject: "owner-device-72",
		SessionID: "owner-session-72", Reason: "SELF_REVOKED", Scope: scope,
	}); err != nil {
		t.Fatal(err)
	}
	if got := scalarInt(t, database, `SELECT COUNT(*) FROM identity_session WHERE session_id='owner-session-72' AND status='REVOKED' AND revoke_reason='SELF_REVOKED'`); got != 1 {
		t.Fatalf("self revoke=%d", got)
	}
}
