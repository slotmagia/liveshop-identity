package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
)

var (
	ErrAuthorizationInvalid    = fmt.Errorf("identity: invalid authorization input")
	ErrAuthorizationDenied     = fmt.Errorf("identity: authorization denied")
	ErrAuthorizationNotFound   = fmt.Errorf("identity: authorization fact not found")
	ErrAuthorizationConflict   = fmt.Errorf("identity: authorization version conflict")
	ErrSystemRoleProtected     = fmt.Errorf("identity: system role is protected")
	ErrRegistryProjectionStale = fmt.Errorf("identity: registry projection is unavailable or stale")
	ErrEntitlementUnavailable  = fmt.Errorf("identity: entitlement projection is unavailable")
)

const (
	AuthorizationPlatform  = "PLATFORM_ORG"
	AuthorizationMerchant  = "MERCHANT"
	AuthorizationActive    = "ACTIVE"
	AuthorizationDisabled  = "DISABLED"
	AuthorizationDeleted   = "DELETED"
	ScopeSelf              = modulesession.DataScopeSelf
	ScopeAll               = modulesession.DataScopeAll
	ScopeCurrentOrgUnit    = modulesession.DataScopeCurrentOrganizationUnit
	ScopeOrgUnitSubtree    = modulesession.DataScopeOrganizationSubtree
	ScopeCurrentShop       = modulesession.DataScopeCurrentShop
	ScopeAssignedShops     = modulesession.DataScopeAssignedShops
	ScopeDelegatedBusiness = modulesession.DataScopeDelegatedBusiness
	ScopeCustomReference   = modulesession.DataScopeCustomReference
)

type AuthorizationDomain struct {
	Type           string
	ID             int64
	OrganizationID int64
}

func (d AuthorizationDomain) Valid() bool {
	return d.ID > 0 && d.OrganizationID > 0 && (d.Type == AuthorizationPlatform || d.Type == AuthorizationMerchant)
}
func (d AuthorizationDomain) Key() string { return fmt.Sprintf("%s:%d", d.Type, d.ID) }

type Permission struct {
	ModuleID, Code, Name, Resource, Action, Description string
	RegistryRevision                                    uint64
}
type Role struct {
	ID                 int64
	Code, Name, Status string
	SystemRole         bool
	Version            int64
}
type ScopeRule struct {
	Resource, Type string
	ReferenceIDs   []string
}
type RolePolicy struct {
	Permissions []string
	Scopes      []ScopeRule
}
type Authorization struct {
	Revision, IdentityVersion, OrganizationVersion, EntitlementRevision uint64
	Permissions                                                         []string
	DataScopes                                                          []modulesession.DataScope
}

func (a Authorization) Has(required ...string) bool {
	granted := make(map[string]bool, len(a.Permissions))
	for _, code := range a.Permissions {
		granted[code] = true
	}
	for _, code := range required {
		if !granted[code] {
			return false
		}
	}
	return len(required) > 0
}

type RegistryContribution struct {
	RegistryRevision    uint64                 `json:"registryRevision"`
	ModuleID            string                 `json:"moduleId"`
	ModuleVersion       string                 `json:"moduleVersion"`
	ContributionID      string                 `json:"contributionId"`
	Surface             string                 `json:"surface"`
	RequiredPermissions []string               `json:"requiredPermissions"`
	AllowedRoutes       []RegistryAllowedRoute `json:"allowedRoutes"`
	Kind                string                 `json:"kind"`
	Route               string                 `json:"route,omitempty"`
	Outlet              string                 `json:"outlet,omitempty"`
	Title               string                 `json:"title"`
	Description         string                 `json:"description"`
	Icon                string                 `json:"icon,omitempty"`
	Sort                int32                  `json:"sort,omitempty"`
	Navigation          *RegistryNavigation    `json:"navigation,omitempty"`
	Artifact            RegistryArtifact       `json:"artifact"`
	Frontend            RegistryFrontend       `json:"frontend"`
}
type RegistryNavigation struct {
	GroupID    string `json:"groupId"`
	GroupTitle string `json:"groupTitle"`
	GroupSort  int32  `json:"groupSort"`
}
type RegistryArtifact struct {
	Type       string `json:"type"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Entry      string `json:"entry"`
	ExportName string `json:"exportName,omitempty"`
	Integrity  string `json:"integrity"`
}
type RegistryFrontend struct {
	Component string                   `json:"component"`
	Props     json.RawMessage          `json:"props"`
	Events    json.RawMessage          `json:"events"`
	Actions   []RegistryFrontendAction `json:"actions"`
}
type RegistryFrontendAction struct {
	ID                  string          `json:"id"`
	Label               string          `json:"label"`
	Description         string          `json:"description"`
	Invocation          string          `json:"invocation"`
	Target              string          `json:"target"`
	Parameters          json.RawMessage `json:"parameters"`
	RequiredPermissions []string        `json:"requiredPermissions"`
}
type RegistryModule struct {
	ID, Name, Version string
	Release           json.RawMessage
}
type RegistryAllowedRoute struct {
	Methods             []string `json:"methods"`
	Prefix              string   `json:"prefix"`
	RequiredPermissions []string `json:"requiredPermissions"`
}

func NormalizePolicy(policy RolePolicy) RolePolicy {
	policy.Permissions = uniqueStrings(policy.Permissions)
	for i := range policy.Scopes {
		policy.Scopes[i].Resource = strings.TrimSpace(policy.Scopes[i].Resource)
		policy.Scopes[i].ReferenceIDs = uniqueStrings(policy.Scopes[i].ReferenceIDs)
	}
	sort.Slice(policy.Scopes, func(i, j int) bool { return policy.Scopes[i].Resource < policy.Scopes[j].Resource })
	return policy
}
func ValidatePolicy(policy RolePolicy, catalog map[string]Permission) error {
	seen := map[string]bool{}
	for _, code := range policy.Permissions {
		if _, ok := catalog[code]; !ok {
			return ErrAuthorizationInvalid
		}
	}
	for _, scope := range policy.Scopes {
		if scope.Resource == "" || seen[scope.Resource] || !validScope(scope.Type) {
			return ErrAuthorizationInvalid
		}
		seen[scope.Resource] = true
		if (scope.Type == ScopeCustomReference) != (len(scope.ReferenceIDs) > 0) {
			return ErrAuthorizationInvalid
		}
	}
	return nil
}
func validScope(value string) bool {
	switch value {
	case ScopeAll, ScopeSelf, ScopeCurrentOrgUnit, ScopeOrgUnitSubtree, ScopeCurrentShop, ScopeAssignedShops, ScopeDelegatedBusiness, ScopeCustomReference:
		return true
	}
	return false
}
func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
