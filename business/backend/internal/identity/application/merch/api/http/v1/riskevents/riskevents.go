package riskevents

import "github.com/gogf/gf/v2/frame/g"

type Event struct {
	ID              int64  `json:"id"`
	VisitorID       string `json:"visitorId"`
	Nickname        string `json:"nickname"`
	RoomID          int64  `json:"roomId"`
	Reason          string `json:"reason"`
	ScoreBefore     int    `json:"scoreBefore"`
	ScoreAfterDecay int    `json:"scoreAfterDecay"`
	ScoreDelta      int    `json:"scoreDelta"`
	ScoreAfter      int    `json:"scoreAfter"`
	CurrentScore    int    `json:"currentScore"`
	CurrentLevel    string `json:"currentLevel"`
	VisitorStatus   string `json:"visitorStatus"`
	CreatedAt       string `json:"createdAt"`
}

type ListReq struct {
	g.Meta        `path:"/risk-events" method:"get" tags:"Identity-merch"`
	VisitorID     string `json:"visitorId" in:"query"`
	RoomID        int64  `json:"roomId" in:"query"`
	Reason        string `json:"reason" in:"query"`
	VisitorStatus string `json:"visitorStatus" in:"query"`
	Page          int    `json:"page" in:"query" d:"1"`
	PageSize      int    `json:"pageSize" in:"query" d:"20"`
}

type ListRes struct {
	Items    []Event `json:"items"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
	Total    int64   `json:"total"`
}
