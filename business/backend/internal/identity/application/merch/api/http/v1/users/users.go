package users

import "github.com/gogf/gf/v2/frame/g"

type Credential struct {
	ID         uint64 `json:"id"`
	Version    uint64 `json:"version"`
	Kind       string `json:"kind"`
	Identifier string `json:"identifier"`
	Status     string `json:"status"`
}
type Shop struct {
	ID         int64  `json:"id"`
	MerchantID int64  `json:"merchantId"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
	Version    uint64 `json:"version"`
}
type Role struct {
	ID         int64  `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	SystemRole bool   `json:"systemRole"`
	Version    int64  `json:"version"`
}
type Unit struct {
	ID       int64  `json:"id"`
	ParentID int64  `json:"parentId"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Version  uint64 `json:"version"`
}
type Member struct {
	ID             int64      `json:"id"`
	Subject        string     `json:"subject"`
	DisplayName    string     `json:"displayName"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	MemberStatus   string     `json:"memberStatus"`
	PrincipalType  string     `json:"principalType"`
	AccessVersion  uint64     `json:"accessVersion"`
	SubjectVersion uint64     `json:"subjectVersion"`
	Credential     Credential `json:"credential"`
	RoleIDs        []int64    `json:"roleIds"`
	UnitIDs        []int64    `json:"unitIds"`
	ShopIDs        []int64    `json:"shopIds"`
	ActiveSessions int        `json:"activeSessions"`
}
type OptionsReq struct {
	g.Meta `path:"/members/options" method:"get" tags:"Identity-Members"`
}
type OptionsRes struct {
	Shops []Shop `json:"shops"`
	Roles []Role `json:"roles"`
	Units []Unit `json:"units"`
}
type ListReq struct {
	g.Meta     `path:"/members" method:"get" tags:"Identity-Members"`
	Keyword    string `json:"keyword" in:"query"`
	MemberType string `json:"type" in:"query"`
	Status     string `json:"status" in:"query"`
	ShopID     int64  `json:"shopId" in:"query"`
	Page       int    `json:"page" in:"query" d:"1"`
	PageSize   int    `json:"pageSize" in:"query" d:"20"`
}
type ListRes struct {
	Items    []Member `json:"items"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
	Total    int64    `json:"total"`
}
type CreateReq struct {
	g.Meta         `path:"/members" method:"post" tags:"Identity-Members"`
	IdempotencyKey string  `json:"idempotencyKey"`
	OperationID    string  `json:"operationId"`
	DisplayName    string  `json:"displayName"`
	MemberType     string  `json:"memberType"`
	Username       string  `json:"username"`
	Password       string  `json:"password"`
	UnitIDs        []int64 `json:"unitIds"`
	ShopIDs        []int64 `json:"shopIds"`
	RoleIDs        []int64 `json:"roleIds"`
}
type CreateRes struct {
	MemberID    int64  `json:"memberId"`
	Subject     string `json:"subject"`
	Status      string `json:"status"`
	OperationID string `json:"operationId"`
	Version     uint64 `json:"version"`
}
type DetailReq struct {
	g.Meta  `path:"/members/{subject}" method:"get" tags:"Identity-Members"`
	Subject string `json:"subject" in:"path"`
}
type DetailRes Member
type UpdateReq struct {
	g.Meta                  `path:"/members/{subject}" method:"put" tags:"Identity-Members"`
	Subject                 string  `json:"subject" in:"path"`
	IdempotencyKey          string  `json:"idempotencyKey"`
	OperationID             string  `json:"operationId"`
	DisplayName             string  `json:"displayName"`
	MemberType              string  `json:"memberType"`
	ExpectedIdentityVersion uint64  `json:"expectedIdentityVersion"`
	ExpectedAccessVersion   uint64  `json:"expectedAccessVersion"`
	UnitIDs                 []int64 `json:"unitIds"`
	ShopIDs                 []int64 `json:"shopIds"`
	RoleIDs                 []int64 `json:"roleIds"`
}
type UpdateRes struct {
	MemberID        int64  `json:"memberId"`
	Subject         string `json:"subject"`
	Status          string `json:"status"`
	OperationID     string `json:"operationId"`
	IdentityVersion uint64 `json:"identityVersion"`
	AccessVersion   uint64 `json:"accessVersion"`
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
type ResetCredentialReq struct {
	g.Meta                    `path:"/members/{subject}/credentials/{credentialId}/reset" method:"post" tags:"Identity-Members"`
	Subject                   string `json:"subject" in:"path"`
	CredentialID              uint64 `json:"credentialId" in:"path"`
	IdempotencyKey            string `json:"idempotencyKey"`
	OperationID               string `json:"operationId"`
	Password                  string `json:"password"`
	ExpectedCredentialVersion uint64 `json:"expectedCredentialVersion"`
}
type ResetCredentialRes Credential
type EnableReq struct {
	g.Meta                  `path:"/members/{subject}/enable" method:"post" tags:"Identity-Members"`
	Subject                 string `json:"subject" in:"path"`
	IdempotencyKey          string `json:"idempotencyKey"`
	OperationID             string `json:"operationId"`
	ExpectedIdentityVersion uint64 `json:"expectedIdentityVersion"`
	ExpectedAccessVersion   uint64 `json:"expectedAccessVersion"`
}
type EnableRes struct {
	Subject         string `json:"subject"`
	Status          string `json:"status"`
	IdentityVersion uint64 `json:"identityVersion"`
	AccessVersion   uint64 `json:"accessVersion"`
}
type DisableReq struct {
	g.Meta                  `path:"/members/{subject}/disable" method:"post" tags:"Identity-Members"`
	Subject                 string `json:"subject" in:"path"`
	IdempotencyKey          string `json:"idempotencyKey"`
	OperationID             string `json:"operationId"`
	ExpectedIdentityVersion uint64 `json:"expectedIdentityVersion"`
	ExpectedAccessVersion   uint64 `json:"expectedAccessVersion"`
}
type DisableRes = EnableRes
type SessionsReq struct {
	g.Meta  `path:"/members/{subject}/sessions" method:"get" tags:"Identity-Sessions"`
	Subject string `json:"subject" in:"path"`
}
type SessionsRes []Session
type RevokeSessionsReq struct {
	g.Meta         `path:"/members/{subject}/sessions/revoke-all" method:"post" tags:"Identity-Sessions"`
	Subject        string `json:"subject" in:"path"`
	IdempotencyKey string `json:"idempotencyKey"`
	OperationID    string `json:"operationId"`
}
type RevokeSessionReq struct {
	g.Meta         `path:"/members/{subject}/sessions/{sessionId}/revoke" method:"post" tags:"Identity-Sessions"`
	Subject        string `json:"subject" in:"path"`
	SessionID      string `json:"sessionId" in:"path"`
	IdempotencyKey string `json:"idempotencyKey"`
	OperationID    string `json:"operationId"`
}
type MutationRes struct{}

func (*MutationRes) NoData() bool { return true }
