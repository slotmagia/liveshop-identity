package biz

import (
	"context"
	"strings"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

type UserScope struct {
	OrganizationID int64
	MerchantID     int64
}

func (s UserScope) Valid() bool { return s.OrganizationID > 0 && s.MerchantID >= 0 }

type ManagedCredential struct {
	ID, Version      uint64
	Kind, Identifier string
	Status           model.Status
}

type ManagedSession struct {
	ID, DeviceName, IPAddress, UserAgent, Status, CreatedAt, LastRefreshedAt, ExpiresAt string
}

type ManagedUser struct {
	Subject        model.Subject
	Member         model.WorkforceMember
	Credential     ManagedCredential
	RoleIDs        []int64
	ShopIDs        []int64
	UnitIDs        []int64
	ActiveSessions int
}

type MemberQuery struct {
	Scope      UserScope
	Keyword    string
	MemberType string
	Status     string
	ShopID     int64
	Page       int
	PageSize   int
}

type MemberPage struct {
	Items    []ManagedUser
	Page     int
	PageSize int
	Total    int64
}

type CreateOperator struct {
	IdempotencyKey, OperationID, Subject, DisplayName, Username, Password string
	OrganizationID                                                        int64
	RoleIDs                                                               []int64
	RequestHash                                                           [32]byte
}

type ChangeUserStatus struct {
	IdempotencyKey, OperationID, Subject, ActorSubject string
	Scope                                              UserScope
	ExpectedIdentityVersion, ExpectedAccessVersion     uint64
	Target                                             model.Status
	RequestHash                                        [32]byte
}

type ResetCredential struct {
	IdempotencyKey, OperationID, Subject, ActorSubject, Password string
	Scope                                                        UserScope
	CredentialID, ExpectedCredentialVersion                      uint64
	RequestHash                                                  [32]byte
}

type RevokeSessions struct {
	IdempotencyKey, OperationID, Subject, ActorSubject, SessionID string
	Reason                                                        string `json:"reason,omitempty"`
	Scope                                                         UserScope
	RequestHash                                                   [32]byte
}

type ChangeOwnCredential struct {
	IdempotencyKey, OperationID, Subject, ActorSubject, SessionID, OldPassword, Password string
	Scope                                                                                UserScope
	ExpectedCredentialVersion                                                            uint64
	RequestHash                                                                          [32]byte
}

type ChangeOwnCredentialResult struct {
	Credential      ManagedCredential
	RevokedSessions int64
	CurrentRetained bool
	Replayed        bool
}

type UserLifecycleRepository interface {
	ListUsers(context.Context, UserScope) ([]ManagedUser, error)
	ListMembers(context.Context, MemberQuery) (MemberPage, error)
	GetUser(context.Context, UserScope, string) (ManagedUser, error)
	OwnAccount(context.Context, UserScope, string) (ManagedUser, error)
	CreatePlatformOperator(context.Context, CreateOperator) (ManagedUser, error)
	ChangeUserStatus(context.Context, ChangeUserStatus) (ManagedUser, error)
	ResetCredential(context.Context, ResetCredential) (ManagedCredential, error)
	ChangeOwnCredential(context.Context, ChangeOwnCredential) (ChangeOwnCredentialResult, error)
	ListSessions(context.Context, UserScope, string) ([]ManagedSession, error)
	ListOwnSessions(context.Context, UserScope, string) ([]ManagedSession, error)
	RevokeSessions(context.Context, RevokeSessions) error
	RevokeOwnSessions(context.Context, RevokeSessions) error
	ValidateCurrentAuthorization(context.Context, modulesession.Claims) error
}

func (u *UserLifecycle) ValidateCurrentAuthorization(ctx context.Context, claims modulesession.Claims) error {
	if u == nil || u.repository == nil {
		return model.ErrUnavailable
	}
	if claims.Subject == "" || claims.SessionID == "" || claims.IdentityVersion == 0 || claims.OrganizationVersion == 0 || claims.AuthorizationRevision == 0 || claims.EntitlementRevision == 0 || claims.ContextVersion == 0 || claims.RegistryRevision == 0 {
		return model.ErrAuthorizationDenied
	}
	return u.repository.ValidateCurrentAuthorization(ctx, claims)
}

type UserLifecycle struct{ repository UserLifecycleRepository }

func NewUserLifecycle(repository UserLifecycleRepository) *UserLifecycle {
	return &UserLifecycle{repository: repository}
}

func (q MemberQuery) Normalize() (MemberQuery, error) {
	q.Keyword = strings.TrimSpace(q.Keyword)
	q.MemberType = strings.ToUpper(strings.TrimSpace(q.MemberType))
	q.Status = strings.ToUpper(strings.TrimSpace(q.Status))
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	if !q.Scope.Valid() || q.Scope.MerchantID <= 0 {
		return q, model.ErrInvalidContext
	}
	if q.MemberType != "" && q.MemberType != string(model.MemberStaff) && q.MemberType != string(model.MemberAnchor) {
		return q, model.ErrConflict
	}
	if q.Status != "" && q.Status != string(model.StatusActive) && q.Status != string(model.StatusDisabled) {
		return q, model.ErrConflict
	}
	if q.ShopID < 0 {
		return q, model.ErrConflict
	}
	return q, nil
}

func (u *UserLifecycle) List(ctx context.Context, scope UserScope) ([]ManagedUser, error) {
	if u == nil || u.repository == nil {
		return nil, model.ErrUnavailable
	}
	if !scope.Valid() {
		return nil, model.ErrInvalidContext
	}
	return u.repository.ListUsers(ctx, scope)
}

func (u *UserLifecycle) ListMembers(ctx context.Context, query MemberQuery) (MemberPage, error) {
	if u == nil || u.repository == nil {
		return MemberPage{}, model.ErrUnavailable
	}
	normalized, err := query.Normalize()
	if err != nil {
		return MemberPage{}, err
	}
	return u.repository.ListMembers(ctx, normalized)
}

func (u *UserLifecycle) Detail(ctx context.Context, scope UserScope, subject string) (ManagedUser, error) {
	if u == nil || u.repository == nil {
		return ManagedUser{}, model.ErrUnavailable
	}
	if !scope.Valid() || strings.TrimSpace(subject) == "" {
		return ManagedUser{}, model.ErrInvalidContext
	}
	return u.repository.GetUser(ctx, scope, subject)
}

func (u *UserLifecycle) OwnAccount(ctx context.Context, scope UserScope, subject string) (ManagedUser, error) {
	if u == nil || u.repository == nil {
		return ManagedUser{}, model.ErrUnavailable
	}
	if !scope.Valid() || scope.MerchantID <= 0 || strings.TrimSpace(subject) == "" {
		return ManagedUser{}, model.ErrInvalidContext
	}
	return u.repository.OwnAccount(ctx, scope, subject)
}

func (u *UserLifecycle) CreateOperator(ctx context.Context, command CreateOperator) (ManagedUser, error) {
	if u == nil || u.repository == nil {
		return ManagedUser{}, model.ErrUnavailable
	}
	command.RoleIDs = normalizedIDs(command.RoleIDs)
	command.DisplayName = strings.TrimSpace(command.DisplayName)
	command.Username = strings.ToLower(strings.TrimSpace(command.Username))
	if command.IdempotencyKey == "" || command.OperationID == "" || command.Subject == "" || command.OrganizationID <= 0 || command.DisplayName == "" || command.Username == "" || len(command.Password) < 8 || len(command.RoleIDs) == 0 {
		return ManagedUser{}, model.ErrConflict
	}
	stable := command
	stable.RequestHash = [32]byte{}
	hash, err := CommandHash(stable)
	if err != nil {
		return ManagedUser{}, err
	}
	command.RequestHash = hash
	return u.repository.CreatePlatformOperator(ctx, command)
}

func (u *UserLifecycle) ChangeStatus(ctx context.Context, command ChangeUserStatus) (ManagedUser, error) {
	if u == nil || u.repository == nil {
		return ManagedUser{}, model.ErrUnavailable
	}
	if command.IdempotencyKey == "" || command.OperationID == "" || command.Subject == "" || command.ActorSubject == "" || !command.Scope.Valid() || command.ExpectedIdentityVersion == 0 || command.ExpectedAccessVersion == 0 || (command.Target != model.StatusActive && command.Target != model.StatusDisabled) {
		return ManagedUser{}, model.ErrConflict
	}
	stable := command
	stable.RequestHash = [32]byte{}
	hash, err := CommandHash(stable)
	if err != nil {
		return ManagedUser{}, err
	}
	command.RequestHash = hash
	return u.repository.ChangeUserStatus(ctx, command)
}

func (u *UserLifecycle) ResetCredential(ctx context.Context, command ResetCredential) (ManagedCredential, error) {
	if u == nil || u.repository == nil {
		return ManagedCredential{}, model.ErrUnavailable
	}
	if command.IdempotencyKey == "" || command.OperationID == "" || command.Subject == "" || command.ActorSubject == "" || !command.Scope.Valid() || command.CredentialID == 0 || command.ExpectedCredentialVersion == 0 || len(command.Password) < 8 {
		return ManagedCredential{}, model.ErrConflict
	}
	stable := command
	stable.RequestHash = [32]byte{}
	stable.Password = "sha256:" + passwordDigest(command.Password)
	hash, err := CommandHash(stable)
	if err != nil {
		return ManagedCredential{}, err
	}
	command.RequestHash = hash
	return u.repository.ResetCredential(ctx, command)
}

func (u *UserLifecycle) ChangeOwnCredential(ctx context.Context, command ChangeOwnCredential) (ChangeOwnCredentialResult, error) {
	if u == nil || u.repository == nil {
		return ChangeOwnCredentialResult{}, model.ErrUnavailable
	}
	command.Subject = strings.TrimSpace(command.Subject)
	command.ActorSubject = strings.TrimSpace(command.ActorSubject)
	command.SessionID = strings.TrimSpace(command.SessionID)
	if command.IdempotencyKey == "" || command.OperationID == "" || command.Subject == "" || command.ActorSubject == "" || command.Subject != command.ActorSubject || command.SessionID == "" || !command.Scope.Valid() || command.Scope.MerchantID <= 0 || command.ExpectedCredentialVersion == 0 || command.OldPassword == "" || len(command.Password) < 8 {
		return ChangeOwnCredentialResult{}, model.ErrConflict
	}
	if command.Password == command.OldPassword {
		return ChangeOwnCredentialResult{}, model.ErrConflict
	}
	stable := command
	stable.RequestHash = [32]byte{}
	stable.OldPassword = "sha256:" + passwordDigest(command.OldPassword)
	stable.Password = "sha256:" + passwordDigest(command.Password)
	hash, err := CommandHash(stable)
	if err != nil {
		return ChangeOwnCredentialResult{}, err
	}
	command.RequestHash = hash
	return u.repository.ChangeOwnCredential(ctx, command)
}

func (u *UserLifecycle) Sessions(ctx context.Context, scope UserScope, subject string) ([]ManagedSession, error) {
	if u == nil || u.repository == nil {
		return nil, model.ErrUnavailable
	}
	if !scope.Valid() || subject == "" {
		return nil, model.ErrInvalidContext
	}
	return u.repository.ListSessions(ctx, scope, subject)
}

func (u *UserLifecycle) OwnSessions(ctx context.Context, scope UserScope, subject string) ([]ManagedSession, error) {
	if u == nil || u.repository == nil {
		return nil, model.ErrUnavailable
	}
	if !scope.Valid() || scope.MerchantID <= 0 || subject == "" {
		return nil, model.ErrInvalidContext
	}
	return u.repository.ListOwnSessions(ctx, scope, subject)
}

func (u *UserLifecycle) RevokeSessions(ctx context.Context, command RevokeSessions) error {
	if u == nil || u.repository == nil {
		return model.ErrUnavailable
	}
	return u.revokeSessions(ctx, command, u.repository.RevokeSessions)
}

func (u *UserLifecycle) RevokeOwnSessions(ctx context.Context, command RevokeSessions) error {
	if u == nil || u.repository == nil {
		return model.ErrUnavailable
	}
	if command.Scope.MerchantID <= 0 {
		return model.ErrInvalidContext
	}
	return u.revokeSessions(ctx, command, u.repository.RevokeOwnSessions)
}

func (u *UserLifecycle) revokeSessions(ctx context.Context, command RevokeSessions, persist func(context.Context, RevokeSessions) error) error {
	if u == nil || u.repository == nil || persist == nil {
		return model.ErrUnavailable
	}
	if command.IdempotencyKey == "" || command.OperationID == "" || command.Subject == "" || command.ActorSubject == "" || !command.Scope.Valid() {
		return model.ErrConflict
	}
	stable := command
	stable.RequestHash = [32]byte{}
	hash, err := CommandHash(stable)
	if err != nil {
		return err
	}
	command.RequestHash = hash
	return persist(ctx, command)
}

func passwordDigest(password string) string {
	hash, _ := CommandHash(password)
	const hex = "0123456789abcdef"
	out := make([]byte, len(hash)*2)
	for i, b := range hash {
		out[i*2] = hex[b>>4]
		out[i*2+1] = hex[b&15]
	}
	return string(out)
}
