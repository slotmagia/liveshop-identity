package biz

import (
	"context"
	"strings"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

type AuthorizationRepository interface {
	Permissions(context.Context, model.AuthorizationDomain) ([]model.Permission, error)
	Roles(context.Context, model.AuthorizationDomain) ([]model.Role, error)
	PutRole(context.Context, model.AuthorizationDomain, model.Role, int64) (model.Role, error)
	SetRolePolicy(context.Context, model.AuthorizationDomain, int64, int64, model.RolePolicy) (model.Role, error)
	ReplaceSubjectGrants(context.Context, model.AuthorizationDomain, string, []int64, string, uint64) error
	Effective(context.Context, model.AuthorizationDomain, model.PrincipalContext) (model.Authorization, error)
	CapabilitySnapshot(context.Context, model.AuthorizationDomain, model.PrincipalContext, string, string, string, string, time.Duration) (model.RegistryContribution, model.Authorization, error)
	RuntimeSnapshot(context.Context, model.AuthorizationDomain, model.PrincipalContext, string, time.Duration) (uint64, []model.RegistryContribution, model.Authorization, error)
	CatalogSnapshot(context.Context, model.AuthorizationDomain, model.PrincipalContext, time.Duration) (uint64, []model.RegistryModule, model.Authorization, error)
}

type AuthorizationService struct{ repository AuthorizationRepository }

func NewAuthorization(repository AuthorizationRepository) *AuthorizationService {
	return &AuthorizationService{repository: repository}
}
func (a *AuthorizationService) Permissions(ctx context.Context, d model.AuthorizationDomain) ([]model.Permission, error) {
	if a == nil || a.repository == nil {
		return nil, model.ErrUnavailable
	}
	if !d.Valid() {
		return nil, model.ErrAuthorizationInvalid
	}
	return a.repository.Permissions(ctx, d)
}
func (a *AuthorizationService) Roles(ctx context.Context, d model.AuthorizationDomain) ([]model.Role, error) {
	if a == nil || a.repository == nil {
		return nil, model.ErrUnavailable
	}
	if !d.Valid() {
		return nil, model.ErrAuthorizationInvalid
	}
	return a.repository.Roles(ctx, d)
}
func (a *AuthorizationService) PutRole(ctx context.Context, d model.AuthorizationDomain, r model.Role, expected int64) (model.Role, error) {
	if a == nil || a.repository == nil {
		return model.Role{}, model.ErrUnavailable
	}
	r.Code = strings.TrimSpace(r.Code)
	r.Name = strings.TrimSpace(r.Name)
	if !d.Valid() || r.ID <= 0 || r.Code == "" || r.Name == "" || (r.Status != model.AuthorizationActive && r.Status != model.AuthorizationDisabled) || expected < 0 {
		return model.Role{}, model.ErrAuthorizationInvalid
	}
	if r.SystemRole {
		return model.Role{}, model.ErrSystemRoleProtected
	}
	return a.repository.PutRole(ctx, d, r, expected)
}
func (a *AuthorizationService) SetRolePolicy(ctx context.Context, d model.AuthorizationDomain, roleID, expected int64, p model.RolePolicy) (model.Role, error) {
	if a == nil || a.repository == nil {
		return model.Role{}, model.ErrUnavailable
	}
	if !d.Valid() || roleID <= 0 || expected <= 0 {
		return model.Role{}, model.ErrAuthorizationInvalid
	}
	p = model.NormalizePolicy(p)
	return a.repository.SetRolePolicy(ctx, d, roleID, expected, p)
}
func (a *AuthorizationService) ReplaceSubjectGrants(ctx context.Context, d model.AuthorizationDomain, subject string, roles []int64, operation string, accessVersion uint64) error {
	if a == nil || a.repository == nil {
		return model.ErrUnavailable
	}
	roles = normalizedIDs(roles)
	if !d.Valid() || strings.TrimSpace(subject) == "" || strings.TrimSpace(operation) == "" || accessVersion == 0 || len(roles) == 0 {
		return model.ErrAuthorizationInvalid
	}
	return a.repository.ReplaceSubjectGrants(ctx, d, subject, roles, operation, accessVersion)
}
func (a *AuthorizationService) Effective(ctx context.Context, d model.AuthorizationDomain, p model.PrincipalContext) (model.Authorization, error) {
	if a == nil || a.repository == nil {
		return model.Authorization{}, model.ErrUnavailable
	}
	if !d.Valid() || p.Subject.ID == "" || p.Subject.Status != model.StatusActive || p.Subject.Version == 0 || p.Member.AccessVersion == 0 {
		return model.Authorization{}, model.ErrAuthorizationInvalid
	}
	return a.repository.Effective(ctx, d, p)
}
