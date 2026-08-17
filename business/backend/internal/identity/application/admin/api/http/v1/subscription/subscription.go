package subscription

import "github.com/gogf/gf/v2/frame/g"

type Plan struct {
	ID           int64  `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Level        int    `json:"level"`
	PriceMinor   int64  `json:"priceMinor"`
	DurationDays int    `json:"durationDays"`
	Description  string `json:"description"`
	Default      bool   `json:"default"`
	Sort         int    `json:"sort"`
	Status       string `json:"status"`
	Version      uint64 `json:"version"`
}

type Permission struct {
	ModuleID         string `json:"moduleId"`
	Code             string `json:"code"`
	Name             string `json:"name"`
	Resource         string `json:"resource"`
	Action           string `json:"action"`
	Description      string `json:"description"`
	RegistryRevision uint64 `json:"registryRevision"`
}

type ListPlansReq struct {
	g.Meta `path:"/subscription/plans" method:"get" tags:"Identity-Subscription"`
}
type ListPlansRes []Plan

type CreatePlanReq struct {
	g.Meta          `path:"/subscription/plans" method:"post" tags:"Identity-Subscription"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Level           int    `json:"level"`
	PriceMinor      int64  `json:"priceMinor"`
	DurationDays    int    `json:"durationDays"`
	Description     string `json:"description"`
	Default         bool   `json:"default"`
	Sort            int    `json:"sort"`
	Status          string `json:"status"`
}

type UpdatePlanReq struct {
	g.Meta          `path:"/subscription/plans/{planId}" method:"put" tags:"Identity-Subscription"`
	PlanID          int64  `json:"planId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Level           int    `json:"level"`
	PriceMinor      int64  `json:"priceMinor"`
	DurationDays    int    `json:"durationDays"`
	Description     string `json:"description"`
	Default         bool   `json:"default"`
	Sort            int    `json:"sort"`
	Status          string `json:"status"`
}

type SavePlanRes struct {
	Plan     Plan `json:"plan"`
	Replayed bool `json:"replayed"`
}

type RetirePlanReq struct {
	g.Meta          `path:"/subscription/plans/{planId}/retire" method:"post" tags:"Identity-Subscription"`
	PlanID          int64  `json:"planId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type RetirePlanRes SavePlanRes

type ListPermissionsReq struct {
	g.Meta `path:"/subscription/permissions" method:"get" tags:"Identity-Subscription"`
}
type ListPermissionsRes []Permission

type PlanPermissionPolicy struct {
	PlanID          int64    `json:"planID"`
	PlanCode        string   `json:"planCode"`
	PlanName        string   `json:"planName"`
	PermissionCodes []string `json:"permissionCodes"`
	ProductLimit    *int64   `json:"productLimit"`
	Revision        uint64   `json:"revision"`
}

type GetPlanPermissionsReq struct {
	g.Meta `path:"/subscription/plans/{planId}/permissions" method:"get" tags:"Identity-Subscription"`
	PlanID int64 `json:"planId" in:"path"`
}
type GetPlanPermissionsRes PlanPermissionPolicy

type PutPlanPermissionsReq struct {
	g.Meta           `path:"/subscription/plans/{planId}/permissions" method:"put" tags:"Identity-Subscription"`
	PlanID           int64    `json:"planId" in:"path"`
	CommandKey       string   `json:"commandKey"`
	ExpectedRevision uint64   `json:"expectedRevision"`
	PermissionCodes  []string `json:"permissionCodes"`
	ProductLimit     *int64   `json:"productLimit"`
}
type PutPlanPermissionsRes struct {
	Policy   PlanPermissionPolicy `json:"policy"`
	Replayed bool                 `json:"replayed"`
}
