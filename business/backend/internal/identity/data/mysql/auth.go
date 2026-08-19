package mysql

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lvtuopen-ai/kernel-go/principal"
	"golang.org/x/crypto/bcrypt"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

type AuthRepository struct {
	database  *sql.DB
	directory *DirectoryRepository
}

var _ biz.AuthRepository = (*AuthRepository)(nil)

func NewAuthRepository(database *sql.DB, directory *DirectoryRepository) (*AuthRepository, error) {
	if database == nil || directory == nil {
		return nil, model.ErrUnavailable
	}
	return &AuthRepository{database: database, directory: directory}, nil
}

func (r *AuthRepository) Login(ctx context.Context, command biz.LoginCommand) (biz.AuthenticatedSession, error) {
	if command.ChallengeID != "" {
		return r.loginWithVerifiedChallenge(ctx, command)
	}
	var subject, secretHash, subjectStatus, memberStatus string
	var credentialShopID sql.NullInt64
	var selected model.SelectedContext
	err := r.directory.transaction(ctx, func(tx *sql.Tx) error {
		query := `SELECT s.subject, c.secret_hash, s.status, COALESCE(m.status, 'ACTIVE'),
       COALESCE(m.organization_id, 0), COALESCE(m.merchant_id, 0), c.shop_id
FROM identity_credential c
JOIN identity_subject s ON s.subject=c.subject
LEFT JOIN identity_workforce_member m ON m.subject=s.subject AND m.status <> 'REVOKED'
WHERE s.realm=? AND c.normalized_identifier=? AND c.status='ACTIVE'
AND s.principal_type<>'GUEST'`
		args := []any{command.Realm, normalizeIdentifier(command.Username)}
		if command.Realm == principal.RealmCustomer {
			if command.ShopCode == "" {
				return biz.ErrInvalidCredentials
			}
			query += ` AND c.namespace_type='SHOP' AND c.shop_id=(SELECT shop_id FROM identity_shop WHERE code=? AND status='ACTIVE')`
			args = append(args, command.ShopCode)
		} else {
			query += ` AND c.namespace_type='GLOBAL'`
		}
		query += ` ORDER BY c.credential_id LIMIT 2 FOR UPDATE`
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			count++
			if err := rows.Scan(&subject, &secretHash, &subjectStatus, &memberStatus, &selected.OrganizationID, &selected.MerchantID, &credentialShopID); err != nil {
				return err
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		// Ambiguous credentials are denied rather than selecting an arbitrary tenant.
		if count != 1 || subjectStatus != "ACTIVE" || memberStatus != "ACTIVE" || bcrypt.CompareHashAndPassword([]byte(secretHash), []byte(command.Password)) != nil {
			return biz.ErrInvalidCredentials
		}
		if err := r.selectMandatoryShop(ctx, tx, subject, credentialShopID, &selected); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO identity_session
(session_id, session_family_id, subject, selected_organization_id, selected_merchant_id,
 selected_shop_id, context_version,
 device_name, ip_address, user_agent, status, expires_at)
VALUES (?, ?, ?, NULLIF(?,0), NULLIF(?,0), NULLIF(?,0), 1, ?, ?, ?, 'ACTIVE', ?)`,
			command.SessionID, command.FamilyID, subject, selected.OrganizationID, selected.MerchantID,
			selected.ShopID, command.DeviceName, command.IPAddress,
			command.UserAgent, command.ExpiresAt)
		if err != nil {
			return mapConflict(err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_refresh_token (token_hash, session_id, status, expires_at) VALUES (?, ?, 'ACTIVE', ?)`, command.RefreshHash[:], command.SessionID, command.ExpiresAt); err != nil {
			return mapConflict(err)
		}
		return appendOutbox(ctx, tx, "session", command.SessionID, 1, "identity.session.changed", map[string]any{"sessionId": command.SessionID, "subject": subject, "status": "ACTIVE"})
	})
	if err != nil {
		return biz.AuthenticatedSession{}, err
	}
	return r.sessionResult(ctx, command.SessionID, subject, selected, 1)
}

func (r *AuthRepository) loginWithVerifiedChallenge(ctx context.Context, command biz.LoginCommand) (biz.AuthenticatedSession, error) {
	var subject string
	var selected model.SelectedContext
	err := r.directory.transaction(ctx, func(tx *sql.Tx) error {
		var shopCode, phone, email, status string
		var consumedAt sql.NullTime
		var boundSession sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT merchant_id,shop_id,shop_code,phone,email,status,consumed_at,login_session_id
FROM identity_auth_otp_challenge WHERE challenge_id=? FOR UPDATE`, command.ChallengeID).
			Scan(&selected.MerchantID, &selected.ShopID, &shopCode, &phone, &email, &status, &consumedAt, &boundSession)
		if errors.Is(err, sql.ErrNoRows) {
			return biz.ErrInvalidCredentials
		}
		if err != nil {
			return err
		}
		if status != "CONSUMED" || boundSession.Valid || shopCode != command.ShopCode || !consumedAt.Valid {
			return biz.ErrInvalidCredentials
		}
		if !consumedAt.Time.Add(300 * time.Second).After(time.Now()) {
			return biz.ErrInvalidCredentials
		}
		kind, identifier := otpCredential(phone, email)
		if kind == "" {
			return biz.ErrInvalidCredentials
		}
		var existing, principalType, existingStatus string
		err = tx.QueryRowContext(ctx, `SELECT c.subject,s.principal_type,s.status
FROM identity_credential c JOIN identity_subject s ON s.subject=c.subject
WHERE c.namespace_type='SHOP' AND c.merchant_id=? AND c.shop_id=? AND c.credential_kind=? AND c.normalized_identifier=? AND c.status='ACTIVE'
FOR UPDATE`, selected.MerchantID, selected.ShopID, kind, identifier).
			Scan(&existing, &principalType, &existingStatus)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		guestSubject, guestSession, guestSameShop, err := r.lockGuestForUpgrade(ctx, tx, command.GuestRefreshToken, selected)
		if err != nil {
			return err
		}
		switch {
		case existing != "":
			if principalType != "CUSTOMER" || existingStatus != "ACTIVE" {
				return biz.ErrInvalidCredentials
			}
			subject = existing
			if guestSession != "" {
				if err := r.revokeSession(ctx, tx, guestSession, "LOGIN_UPGRADE"); err != nil {
					return err
				}
			}
			if guestSubject != "" && guestSubject != subject && guestSameShop {
				if err := appendOutbox(ctx, tx, "subject", guestSubject, 1, "identity.guest.upgraded", map[string]any{
					"fromSubject": guestSubject, "toSubject": subject,
					"merchantId": selected.MerchantID, "shopId": selected.ShopID,
				}); err != nil {
					return err
				}
			}
		case guestSubject != "" && guestSameShop:
			subject = guestSubject
			result, err := tx.ExecContext(ctx, `UPDATE identity_subject SET principal_type='CUSTOMER',display_name='顾客',version=version+1,updated_at=CURRENT_TIMESTAMP(3)
WHERE subject=? AND principal_type='GUEST' AND status='ACTIVE'`, subject)
			if err != nil {
				return err
			}
			if affected, _ := result.RowsAffected(); affected != 1 {
				return biz.ErrInvalidCredentials
			}
			if err := r.insertShopCredential(ctx, tx, subject, kind, identifier, selected); err != nil {
				return err
			}
			if err := r.revokeSession(ctx, tx, guestSession, "LOGIN_UPGRADE"); err != nil {
				return err
			}
			if err := appendOutbox(ctx, tx, "subject", subject, 2, "identity.subject.changed", map[string]any{"subject": subject, "principalType": "CUSTOMER", "status": "ACTIVE"}); err != nil {
				return err
			}
		default:
			subject = "customer-" + secureToken()
			if _, err := tx.ExecContext(ctx, `INSERT INTO identity_subject
(subject,realm,principal_type,display_name,status,version)
VALUES (?,'CUSTOMER','CUSTOMER','顾客','ACTIVE',1)`, subject); err != nil {
				return mapConflict(err)
			}
			if err := r.insertShopCredential(ctx, tx, subject, kind, identifier, selected); err != nil {
				return err
			}
			if guestSession != "" {
				if err := r.revokeSession(ctx, tx, guestSession, "LOGIN_UPGRADE"); err != nil {
					return err
				}
			}
			if err := appendOutbox(ctx, tx, "subject", subject, 1, "identity.subject.changed", map[string]any{"subject": subject, "principalType": "CUSTOMER", "status": "ACTIVE"}); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_session
(session_id,session_family_id,subject,selected_merchant_id,selected_shop_id,
 context_version,authentication_level,device_name,ip_address,user_agent,status,expires_at)
VALUES (?,?,?,?,?,1,'OTP',?,?,?,'ACTIVE',?)`, command.SessionID, command.FamilyID, subject,
			selected.MerchantID, selected.ShopID, command.DeviceName, command.IPAddress, command.UserAgent, command.ExpiresAt); err != nil {
			return mapConflict(err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_refresh_token (token_hash,session_id,status,expires_at) VALUES (?,?,'ACTIVE',?)`, command.RefreshHash[:], command.SessionID, command.ExpiresAt); err != nil {
			return mapConflict(err)
		}
		bound, err := tx.ExecContext(ctx, `UPDATE identity_auth_otp_challenge SET login_session_id=? WHERE challenge_id=? AND status='CONSUMED' AND login_session_id IS NULL`, command.SessionID, command.ChallengeID)
		if err != nil {
			return err
		}
		if affected, _ := bound.RowsAffected(); affected != 1 {
			return biz.ErrInvalidCredentials
		}
		return appendOutbox(ctx, tx, "session", command.SessionID, 1, "identity.session.changed", map[string]any{"sessionId": command.SessionID, "subject": subject, "status": "ACTIVE"})
	})
	if err != nil {
		return biz.AuthenticatedSession{}, err
	}
	return r.sessionResult(ctx, command.SessionID, subject, selected, 1)
}

func otpCredential(phone, email string) (kind, identifier string) {
	if phone != "" {
		return "PHONE", phone
	}
	if email != "" {
		return "EMAIL", email
	}
	return "", ""
}

func (r *AuthRepository) insertShopCredential(ctx context.Context, tx *sql.Tx, subject, kind, identifier string, selected model.SelectedContext) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO identity_credential
(subject,namespace_type,merchant_id,shop_id,credential_kind,normalized_identifier,secret_hash,status,verified_at,version)
VALUES (?,'SHOP',?,?,?,?,NULL,'ACTIVE',CURRENT_TIMESTAMP(3),1)`, subject, selected.MerchantID, selected.ShopID, kind, identifier)
	return mapConflict(err)
}

func (r *AuthRepository) lockGuestForUpgrade(ctx context.Context, tx *sql.Tx, refreshToken string, selected model.SelectedContext) (subject, sessionID string, sameShop bool, err error) {
	if refreshToken == "" {
		return "", "", false, nil
	}
	hash := sha256.Sum256([]byte(refreshToken))
	var principalType, status string
	var merchantID, shopID int64
	err = tx.QueryRowContext(ctx, `SELECT s.session_id,s.subject,su.principal_type,s.status,COALESCE(s.selected_merchant_id,0),COALESCE(s.selected_shop_id,0)
FROM identity_refresh_token rt
JOIN identity_session s ON s.session_id=rt.session_id
JOIN identity_subject su ON su.subject=s.subject
WHERE rt.token_hash=? AND rt.status='ACTIVE' AND su.realm='CUSTOMER' FOR UPDATE`, hash[:]).
		Scan(&sessionID, &subject, &principalType, &status, &merchantID, &shopID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	if principalType != "GUEST" || status != "ACTIVE" {
		return "", "", false, nil
	}
	return subject, sessionID, merchantID == selected.MerchantID && shopID == selected.ShopID, nil
}

func (r *AuthRepository) revokeSession(ctx context.Context, tx *sql.Tx, sessionID, reason string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE identity_session SET status='REVOKED',revoked_at=CURRENT_TIMESTAMP(3),revoke_reason=? WHERE session_id=? AND status='ACTIVE'`, reason, sessionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_refresh_token SET status='REVOKED' WHERE session_id=? AND status='ACTIVE'`, sessionID); err != nil {
		return err
	}
	return appendOutbox(ctx, tx, "session", sessionID, 1, "identity.session.changed", map[string]any{"sessionId": sessionID, "status": "REVOKED", "reason": reason})
}

func (r *AuthRepository) Guest(ctx context.Context, command biz.GuestCommand) (biz.AuthenticatedSession, error) {
	selected := model.SelectedContext{}
	err := r.directory.transaction(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT s.merchant_id,s.shop_id
FROM identity_shop s
JOIN identity_merchant m ON m.merchant_id=s.merchant_id AND m.status='ACTIVE'
WHERE s.code=? AND s.status='ACTIVE' FOR UPDATE`, command.ShopCode).
			Scan(&selected.MerchantID, &selected.ShopID)
		if errors.Is(err, sql.ErrNoRows) {
			return biz.ErrInvalidCredentials
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_subject
(subject,realm,principal_type,display_name,status,version)
VALUES (?,'CUSTOMER','GUEST','游客','ACTIVE',1)`, command.Subject); err != nil {
			return mapConflict(err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_session
(session_id,session_family_id,subject,selected_merchant_id,selected_shop_id,
 context_version,authentication_level,device_name,ip_address,user_agent,status,expires_at)
VALUES (?,?,?,?,?,1,'GUEST',?,?,?,'ACTIVE',?)`, command.SessionID, command.FamilyID, command.Subject,
			selected.MerchantID, selected.ShopID,
			command.DeviceName, command.IPAddress, command.UserAgent, command.ExpiresAt); err != nil {
			return mapConflict(err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_refresh_token (token_hash,session_id,status,expires_at) VALUES (?,?,'ACTIVE',?)`, command.RefreshHash[:], command.SessionID, command.ExpiresAt); err != nil {
			return mapConflict(err)
		}
		if err := appendOutbox(ctx, tx, "subject", command.Subject, 1, "identity.subject.changed", map[string]any{"subject": command.Subject, "principalType": "GUEST", "status": "ACTIVE"}); err != nil {
			return err
		}
		return appendOutbox(ctx, tx, "session", command.SessionID, 1, "identity.session.changed", map[string]any{"sessionId": command.SessionID, "subject": command.Subject, "status": "ACTIVE"})
	})
	if err != nil {
		return biz.AuthenticatedSession{}, err
	}
	return r.sessionResult(ctx, command.SessionID, command.Subject, selected, 1)
}

func (r *AuthRepository) selectMandatoryShop(ctx context.Context, tx *sql.Tx, subject string, credentialShopID sql.NullInt64, selected *model.SelectedContext) error {
	var principalType, memberType string
	var memberID int64
	if err := tx.QueryRowContext(ctx, `SELECT s.principal_type,COALESCE(m.member_id,0),COALESCE(m.member_type,'')
FROM identity_subject s
LEFT JOIN identity_workforce_member m ON m.subject=s.subject AND m.status='ACTIVE'
WHERE s.subject=?`, subject).Scan(&principalType, &memberID, &memberType); err != nil {
		return err
	}
	if principalType == "PLATFORM_OPERATOR" {
		return nil
	}
	if principalType == "CUSTOMER" {
		if !credentialShopID.Valid {
			return model.ErrInvalidContext
		}
		return tx.QueryRowContext(ctx, `SELECT merchant_id,shop_id FROM identity_shop WHERE shop_id=? AND status='ACTIVE'`, credentialShopID.Int64).Scan(&selected.MerchantID, &selected.ShopID)
	}
	// A browser login has no authority to invent a shop scope. Select it
	// only when Identity can prove that exactly one active shop is available.
	// Multi-shop users authenticate successfully only after the host supplies a
	// context-selection flow through /auth/context/switch; choosing the first row
	// would make ORDER BY an authorization decision.
	query := `SELECT s.merchant_id,s.shop_id
FROM identity_shop s WHERE s.merchant_id=? AND s.status='ACTIVE' ORDER BY s.shop_id LIMIT 2`
	args := []any{selected.MerchantID}
	if memberType != "OWNER" {
		query = `SELECT s.merchant_id,s.shop_id
FROM identity_member_shop ms
JOIN identity_shop s ON s.shop_id=ms.shop_id AND s.status='ACTIVE'
WHERE ms.member_id=? AND ms.status='ACTIVE' ORDER BY s.shop_id LIMIT 2`
		args = []any{memberID}
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	var only model.ShopContext
	for rows.Next() {
		count++
		if err := rows.Scan(&only.MerchantID, &only.ShopID); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != 1 {
		return model.ErrInvalidContext
	}
	selected.ShopContext = only
	return nil
}

func (r *AuthRepository) RotateRefresh(ctx context.Context, expectedRealm principal.Realm, refreshToken string, expiresAt time.Time) (biz.AuthenticatedSession, string, error) {
	oldHash := sha256.Sum256([]byte(refreshToken))
	newToken := secureToken()
	newHash := sha256.Sum256([]byte(newToken))
	var sessionID, subject string
	var selected model.SelectedContext
	var contextVersion uint64
	reuseDetected := false
	err := r.directory.transaction(ctx, func(tx *sql.Tx) error {
		var tokenStatus, sessionStatus string
		var refreshExpiresAt, sessionExpiresAt time.Time
		err := tx.QueryRowContext(ctx, `SELECT rt.session_id, rt.status, s.status, s.subject,
COALESCE(s.selected_organization_id,0), COALESCE(s.selected_merchant_id,0),
COALESCE(s.selected_shop_id,0), s.context_version, rt.expires_at, s.expires_at
FROM identity_refresh_token rt JOIN identity_session s ON s.session_id=rt.session_id JOIN identity_subject su ON su.subject=s.subject
WHERE rt.token_hash=? AND su.realm=? FOR UPDATE`, oldHash[:], expectedRealm).Scan(&sessionID, &tokenStatus, &sessionStatus, &subject,
			&selected.OrganizationID, &selected.MerchantID, &selected.ShopID, &contextVersion, &refreshExpiresAt, &sessionExpiresAt)
		if errors.Is(err, sql.ErrNoRows) {
			return biz.ErrInvalidCredentials
		}
		if err != nil {
			return err
		}
		if tokenStatus == "USED" {
			if _, err := tx.ExecContext(ctx, `UPDATE identity_session SET status='REVOKED', revoked_at=CURRENT_TIMESTAMP(3), revoke_reason='REFRESH_REUSE' WHERE session_family_id=(SELECT session_family_id FROM identity_session WHERE session_id=?) AND status='ACTIVE'`, sessionID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE identity_refresh_token rt JOIN identity_session s ON s.session_id=rt.session_id SET rt.status='REVOKED' WHERE s.session_family_id=(SELECT session_family_id FROM identity_session WHERE session_id=?) AND rt.status='ACTIVE'`, sessionID); err != nil {
				return err
			}
			reuseDetected = true
			return nil
		}
		if tokenStatus != "ACTIVE" || sessionStatus != "ACTIVE" || !refreshExpiresAt.After(time.Now()) || !sessionExpiresAt.After(time.Now()) {
			return biz.ErrInvalidCredentials
		}
		result, err := tx.ExecContext(ctx, `UPDATE identity_refresh_token SET status='USED', used_at=CURRENT_TIMESTAMP(3) WHERE token_hash=? AND status='ACTIVE'`, oldHash[:])
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return model.ErrConflict
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_refresh_token (token_hash, session_id, status, expires_at) VALUES (?, ?, 'ACTIVE', ?)`, newHash[:], sessionID, expiresAt); err != nil {
			return mapConflict(err)
		}
		_, err = tx.ExecContext(ctx, `UPDATE identity_session SET last_refreshed_at=CURRENT_TIMESTAMP(3), expires_at=? WHERE session_id=? AND status='ACTIVE'`, expiresAt, sessionID)
		return err
	})
	if err != nil {
		return biz.AuthenticatedSession{}, "", err
	}
	if reuseDetected {
		return biz.AuthenticatedSession{}, "", biz.ErrInvalidCredentials
	}
	result, err := r.sessionResult(ctx, sessionID, subject, selected, contextVersion)
	return result, newToken, err
}

func (r *AuthRepository) Logout(ctx context.Context, expectedRealm principal.Realm, refreshToken string) error {
	hash := sha256.Sum256([]byte(refreshToken))
	return r.directory.transaction(ctx, func(tx *sql.Tx) error {
		var sessionID string
		err := tx.QueryRowContext(ctx, `SELECT rt.session_id FROM identity_refresh_token rt JOIN identity_session s ON s.session_id=rt.session_id JOIN identity_subject su ON su.subject=s.subject WHERE rt.token_hash=? AND su.realm=?`, hash[:], expectedRealm).Scan(&sessionID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE identity_session SET status='REVOKED', revoked_at=CURRENT_TIMESTAMP(3), revoke_reason='LOGOUT' WHERE session_id=? AND status='ACTIVE'`, sessionID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE identity_refresh_token SET status='REVOKED' WHERE session_id=? AND status='ACTIVE'`, sessionID)
		return err
	})
}

func (r *AuthRepository) SwitchContext(ctx context.Context, command biz.SwitchContextCommand) (biz.AuthenticatedSession, error) {
	var contextVersion uint64
	err := r.directory.transaction(ctx, func(tx *sql.Tx) error {
		var principalType, memberType, memberStatus, subjectStatus string
		var memberID, organizationID, merchantID int64
		if err := tx.QueryRowContext(ctx, `SELECT s.principal_type,s.status,COALESCE(m.member_id,0),COALESCE(m.organization_id,0),COALESCE(m.merchant_id,0),COALESCE(m.member_type,''),COALESCE(m.status,'ACTIVE')
FROM identity_subject s LEFT JOIN identity_workforce_member m ON m.subject=s.subject AND m.status<>'REVOKED'
WHERE s.subject=? FOR UPDATE`, command.Subject).Scan(&principalType, &subjectStatus, &memberID, &organizationID, &merchantID, &memberType, &memberStatus); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return model.ErrNotFound
			}
			return err
		}
		if subjectStatus != "ACTIVE" || memberStatus != "ACTIVE" {
			return model.ErrInactive
		}
		if command.Selected.OrganizationID != organizationID || (merchantID > 0 && command.Selected.MerchantID != merchantID) {
			return model.ErrInvalidContext
		}
		if principalType == "PLATFORM_OPERATOR" {
			if command.Selected.ShopID != 0 || command.Selected.MerchantID != 0 {
				return model.ErrInvalidContext
			}
		} else if command.Selected.ShopID > 0 {
			var found int
			query := `SELECT 1 FROM identity_shop s WHERE s.shop_id=? AND s.merchant_id=? AND s.status='ACTIVE'`
			args := []any{command.Selected.ShopID, command.Selected.MerchantID}
			if memberType != "OWNER" {
				query += ` AND EXISTS(SELECT 1 FROM identity_member_shop ms WHERE ms.member_id=? AND ms.shop_id=s.shop_id AND ms.status='ACTIVE')`
				args = append(args, memberID)
			}
			if err := tx.QueryRowContext(ctx, query, args...).Scan(&found); errors.Is(err, sql.ErrNoRows) {
				return model.ErrInvalidContext
			} else if err != nil {
				return err
			}
		} else if principalType == "SHOP_ANCHOR" || principalType == "CUSTOMER" || principalType == "GUEST" {
			return model.ErrInvalidContext
		}
		result, err := tx.ExecContext(ctx, `UPDATE identity_session SET selected_organization_id=NULLIF(?,0), selected_merchant_id=NULLIF(?,0),
selected_shop_id=NULLIF(?,0), context_version=context_version+1
WHERE session_id=? AND subject=? AND status='ACTIVE' AND expires_at>CURRENT_TIMESTAMP(3) AND context_version=?`, command.Selected.OrganizationID, command.Selected.MerchantID,
			command.Selected.ShopID, command.SessionID, command.Subject, command.ExpectedContextVersion)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return model.ErrConflict
		}
		return nil
	})
	if err != nil {
		return biz.AuthenticatedSession{}, err
	}
	contextVersion = command.ExpectedContextVersion + 1
	return r.sessionResult(ctx, command.SessionID, command.Subject, command.Selected, contextVersion)
}

func (r *AuthRepository) sessionResult(ctx context.Context, sessionID, subject string, selected model.SelectedContext, contextVersion uint64) (biz.AuthenticatedSession, error) {
	resolved, err := r.directory.ResolvePrincipalContext(ctx, subject)
	if err != nil {
		return biz.AuthenticatedSession{}, err
	}
	return biz.AuthenticatedSession{SessionID: sessionID, Subject: resolved.Subject, Member: resolved.Member, Organization: resolved.Organization, Selected: selected, ContextVersion: contextVersion}, nil
}

func normalizeIdentifier(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func secureToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", fmt.Errorf("identity: password must contain at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func ComparePassword(hash, password string) error {
	if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return model.ErrInvalidCredential
	}
	return nil
}
