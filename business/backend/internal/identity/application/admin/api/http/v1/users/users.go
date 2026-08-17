package users

import "github.com/gogf/gf/v2/frame/g"

type Credential struct {
	ID         uint64 `json:"id"`
	Version    uint64 `json:"version"`
	Kind       string `json:"kind"`
	Identifier string `json:"identifier"`
	Status     string `json:"status"`
}
type User struct {
	Subject        string     `json:"subject"`
	DisplayName    string     `json:"displayName"`
	Realm          string     `json:"realm"`
	PrincipalType  string     `json:"principalType"`
	SubjectStatus  string     `json:"subjectStatus"`
	SubjectVersion uint64     `json:"subjectVersion"`
	MemberID       int64      `json:"memberId"`
	OrganizationID int64      `json:"organizationId"`
	MemberType     string     `json:"memberType"`
	MemberStatus   string     `json:"memberStatus"`
	AccessVersion  uint64     `json:"accessVersion"`
	Credential     Credential `json:"credential"`
	RoleIDs        []int64    `json:"roleIds"`
	ActiveSessions int        `json:"activeSessions"`
}
type Session struct {
	ID              string `json:"id"`
	DeviceName      string `json:"deviceName"`
	IPAddress       string `json:"ipAddress"`
	UserAgent       string `json:"userAgent"`
	Status          string `json:"status"`
	CreatedAt       string `json:"createdAt"`
	LastRefreshedAt string `json:"lastRefreshedAt"`
	ExpiresAt       string `json:"expiresAt"`
}
type ListReq struct {
	g.Meta `path:"/users" method:"get" tags:"Identity-Users"`
}
type ListRes []User
type DetailReq struct {
	g.Meta  `path:"/users/{subject}" method:"get" tags:"Identity-Users"`
	Subject string `json:"subject" in:"path"`
}
type DetailRes User
type CreateReq struct {
	g.Meta         `path:"/users" method:"post" tags:"Identity-Users"`
	IdempotencyKey string  `json:"idempotencyKey"`
	OperationID    string  `json:"operationId"`
	DisplayName    string  `json:"displayName"`
	Username       string  `json:"username"`
	Password       string  `json:"password"`
	RoleIDs        []int64 `json:"roleIds"`
}
type CreateRes User
type EnableReq struct {
	g.Meta                  `path:"/users/{subject}/enable" method:"post" tags:"Identity-Users"`
	Subject                 string `json:"subject" in:"path"`
	IdempotencyKey          string `json:"idempotencyKey"`
	OperationID             string `json:"operationId"`
	ExpectedIdentityVersion uint64 `json:"expectedIdentityVersion"`
	ExpectedAccessVersion   uint64 `json:"expectedAccessVersion"`
}
type EnableRes User
type DisableReq struct {
	g.Meta                  `path:"/users/{subject}/disable" method:"post" tags:"Identity-Users"`
	Subject                 string `json:"subject" in:"path"`
	IdempotencyKey          string `json:"idempotencyKey"`
	OperationID             string `json:"operationId"`
	ExpectedIdentityVersion uint64 `json:"expectedIdentityVersion"`
	ExpectedAccessVersion   uint64 `json:"expectedAccessVersion"`
}
type DisableRes User
type ResetCredentialReq struct {
	g.Meta                    `path:"/users/{subject}/credentials/{credentialId}/reset" method:"post" tags:"Identity-Users"`
	Subject                   string `json:"subject" in:"path"`
	CredentialID              uint64 `json:"credentialId" in:"path"`
	IdempotencyKey            string `json:"idempotencyKey"`
	OperationID               string `json:"operationId"`
	Password                  string `json:"password"`
	ExpectedCredentialVersion uint64 `json:"expectedCredentialVersion"`
}
type ResetCredentialRes Credential
type SessionsReq struct {
	g.Meta  `path:"/users/{subject}/sessions" method:"get" tags:"Identity-Sessions"`
	Subject string `json:"subject" in:"path"`
}
type SessionsRes []Session
type RevokeSessionsReq struct {
	g.Meta         `path:"/users/{subject}/sessions/revoke-all" method:"post" tags:"Identity-Sessions"`
	Subject        string `json:"subject" in:"path"`
	IdempotencyKey string `json:"idempotencyKey"`
	OperationID    string `json:"operationId"`
}
type RevokeSessionReq struct {
	g.Meta         `path:"/users/{subject}/sessions/{sessionId}/revoke" method:"post" tags:"Identity-Sessions"`
	Subject        string `json:"subject" in:"path"`
	SessionID      string `json:"sessionId" in:"path"`
	IdempotencyKey string `json:"idempotencyKey"`
	OperationID    string `json:"operationId"`
}
type MutationRes struct{}

func (*MutationRes) NoData() bool { return true }
