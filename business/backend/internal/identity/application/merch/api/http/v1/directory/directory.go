package directory

import "github.com/gogf/gf/v2/frame/g"

type ReadReq struct {
	g.Meta `path:"/directory" method:"get" tags:"Identity-merch"`
}
type ReadRes struct {
	Organization any `json:"organization"`
	Units        any `json:"units"`
	Members      any `json:"members"`
	Shops        any `json:"shops"`
}
type CreateUnitReq struct {
	g.Meta          `path:"/organization-units" method:"post" tags:"Identity-merch"`
	IdempotencyKey  string `json:"idempotencyKey"`
	UnitID          int64  `json:"unitId"`
	ParentID        int64  `json:"parentId"`
	Name            string `json:"name"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type ReplaceAccessReq struct {
	g.Meta                `path:"/members/{memberId}/access" method:"put" tags:"Identity-merch"`
	MemberID              int64   `json:"memberId" in:"path"`
	IdempotencyKey        string  `json:"idempotencyKey"`
	OperationID           string  `json:"operationId"`
	MemberType            string  `json:"memberType"`
	ExpectedAccessVersion uint64  `json:"expectedAccessVersion"`
	UnitIDs               []int64 `json:"unitIds"`
	ShopIDs               []int64 `json:"shopIds"`
}
type MutationRes struct {
	MemberID    int64  `json:"memberId,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Status      string `json:"status,omitempty"`
	OperationID string `json:"operationId,omitempty"`
	Version     uint64 `json:"version"`
}
