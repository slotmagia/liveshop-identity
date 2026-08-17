package authorization

import "github.com/gogf/gf/v2/frame/g"

type Permission struct {
	ModuleID         string `json:"moduleId"`
	Code             string `json:"code"`
	Name             string `json:"name"`
	Resource         string `json:"resource"`
	Action           string `json:"action"`
	Description      string `json:"description"`
	RegistryRevision uint64 `json:"registryRevision"`
}
type Role struct {
	ID         int64  `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	SystemRole bool   `json:"systemRole"`
	Version    int64  `json:"version"`
}
type ScopeRule struct {
	Resource     string   `json:"resource"`
	Type         string   `json:"type"`
	ReferenceIDs []string `json:"referenceIds"`
}
type PermissionsReq struct {
	g.Meta `path:"/authorization/permissions" method:"get" tags:"Identity-Authorization"`
}
type PermissionsRes []Permission
type RolesReq struct {
	g.Meta `path:"/authorization/roles" method:"get" tags:"Identity-Authorization"`
}
type RolesRes []Role
type PutRoleReq struct {
	g.Meta          `path:"/authorization/roles/{roleId}" method:"put" tags:"Identity-Authorization"`
	RoleID          int64  `json:"roleId" in:"path"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Status          string `json:"status"`
}
type PutRoleRes Role
type PutPolicyReq struct {
	g.Meta          `path:"/authorization/roles/{roleId}/policy" method:"put" tags:"Identity-Authorization"`
	RoleID          int64       `json:"roleId" in:"path"`
	ExpectedVersion int64       `json:"expectedVersion"`
	Permissions     []string    `json:"permissions"`
	Scopes          []ScopeRule `json:"scopes"`
}
type PutPolicyRes Role
type PutGrantsReq struct {
	g.Meta        `path:"/authorization/subjects/{subject}/grants" method:"put" tags:"Identity-Authorization"`
	Subject       string  `json:"subject" in:"path"`
	RoleIDs       []int64 `json:"roleIds"`
	OperationID   string  `json:"operationId"`
	AccessVersion uint64  `json:"accessVersion"`
}
type PutGrantsRes struct{}

func (*PutGrantsRes) NoData() bool { return true }
