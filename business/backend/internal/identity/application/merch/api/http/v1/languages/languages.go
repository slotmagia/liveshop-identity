package languages

import "github.com/gogf/gf/v2/frame/g"

type Item struct {
	Locale            string `json:"locale"`
	Label             string `json:"label"`
	Published         bool   `json:"published"`
	IsDefault         bool   `json:"isDefault"`
	SortOrder         int    `json:"sortOrder"`
	CompletionPercent int    `json:"completionPercent"`
	PlatformStatus    string `json:"platformStatus"`
}

type GetReq struct {
	g.Meta `path:"/languages" method:"get" tags:"Identity-merch"`
}
type GetRes struct {
	DefaultLocale string `json:"defaultLocale"`
	Version       uint64 `json:"version"`
	Items         []Item `json:"items"`
}

type UpdateReq struct {
	g.Meta           `path:"/languages" method:"put" tags:"Identity-merch"`
	CommandKey       string   `json:"commandKey"`
	ExpectedVersion  uint64   `json:"expectedVersion"`
	DefaultLocale    string   `json:"defaultLocale"`
	PublishedLocales []string `json:"publishedLocales"`
}
type UpdateRes struct {
	DefaultLocale string `json:"defaultLocale"`
	Version       uint64 `json:"version"`
	Items         []Item `json:"items"`
	Replayed      bool   `json:"replayed"`
}
