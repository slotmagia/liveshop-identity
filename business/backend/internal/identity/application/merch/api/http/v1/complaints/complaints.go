package complaints

import "github.com/gogf/gf/v2/frame/g"

type Complaint struct {
	ID              int64   `json:"id"`
	CustomerSubject string  `json:"customerSubject"`
	TargetType      string  `json:"targetType"`
	TargetID        int64   `json:"targetId"`
	ReasonCode      string  `json:"reasonCode"`
	Content         string  `json:"content"`
	Status          string  `json:"status"`
	HandleNote      string  `json:"handleNote"`
	Version         uint64  `json:"version"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
	HandledAt       *string `json:"handledAt,omitempty"`
}

type ListReq struct {
	g.Meta          `path:"/complaints" method:"get" tags:"Identity-merch"`
	CustomerSubject string `json:"customerSubject" in:"query"`
	Status          string `json:"status" in:"query"`
	TargetType      string `json:"targetType" in:"query"`
	Page            int    `json:"page" in:"query" d:"1"`
	PageSize        int    `json:"pageSize" in:"query" d:"20"`
}

type ListRes struct {
	Items    []Complaint `json:"items"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	Total    int64       `json:"total"`
}

type GetReq struct {
	g.Meta      `path:"/complaints/{complaintId}" method:"get" tags:"Identity-merch"`
	ComplaintId int64 `json:"complaintId" in:"path"`
}

type GetRes struct {
	Complaint Complaint `json:"complaint"`
}

type ReviewReq struct {
	g.Meta          `path:"/complaints/{complaintId}/review" method:"post" tags:"Identity-merch"`
	ComplaintId     int64  `json:"complaintId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	Status          string `json:"status"`
	HandleNote      string `json:"handleNote"`
}

type ReviewRes struct {
	Complaint Complaint `json:"complaint"`
	Replayed  bool      `json:"replayed"`
}
