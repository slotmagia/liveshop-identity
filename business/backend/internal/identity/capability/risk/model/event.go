// Package model owns visitor-risk event facts and list invariants.
package model

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

type Level string
type Status string

const (
	LevelNone   Level = "NONE"
	LevelLow    Level = "LOW"
	LevelMedium Level = "MEDIUM"
	LevelHigh   Level = "HIGH"

	StatusNormal     Status = "NORMAL"
	StatusWatch      Status = "WATCH"
	StatusRestricted Status = "RESTRICTED"
	StatusBlocked    Status = "BLOCKED"
)

var (
	ErrUnavailable = errors.New("risk event repository unavailable")
	ErrNotFound    = errors.New("risk event shop not found")
	ErrInvalid     = errors.New("invalid risk event")
)

type Event struct {
	ID              int64
	MerchantID      int64
	ShopID          int64
	VisitorID       string
	Nickname        string
	RoomID          int64
	Reason          string
	ScoreBefore     int
	ScoreAfterDecay int
	ScoreDelta      int
	ScoreAfter      int
	CurrentScore    int
	CurrentLevel    Level
	VisitorStatus   Status
	CreatedAt       time.Time
}

func (e Event) ValidatePersisted() error {
	if e.ID <= 0 || e.MerchantID <= 0 || e.ShopID <= 0 || e.CreatedAt.IsZero() {
		return ErrInvalid
	}
	if err := validateVisitorID(e.VisitorID); err != nil {
		return err
	}
	if len([]rune(strings.TrimSpace(e.Nickname))) > 64 || e.RoomID < 0 ||
		len([]rune(e.Reason)) == 0 || len([]rune(e.Reason)) > 64 ||
		e.ScoreBefore < 0 || e.ScoreAfterDecay < 0 || e.ScoreAfter < 0 || e.CurrentScore < 0 {
		return ErrInvalid
	}
	if !validLevel(e.CurrentLevel) || !validStatus(e.VisitorStatus) {
		return ErrInvalid
	}
	return nil
}

type Query struct {
	MerchantID    int64
	ShopID        int64
	VisitorID     string
	RoomID        int64
	Reason        string
	VisitorStatus Status
	Page          int
	PageSize      int
}

func (q Query) Normalize() (Query, error) {
	q.VisitorID = strings.TrimSpace(q.VisitorID)
	q.Reason = strings.TrimSpace(q.Reason)
	q.VisitorStatus = Status(strings.ToUpper(strings.TrimSpace(string(q.VisitorStatus))))
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	if q.MerchantID <= 0 || q.ShopID <= 0 || q.RoomID < 0 {
		return q, ErrInvalid
	}
	if q.VisitorID != "" {
		if err := validateVisitorID(q.VisitorID); err != nil {
			return q, err
		}
	}
	if len([]rune(q.Reason)) > 64 {
		return q, ErrInvalid
	}
	if q.VisitorStatus != "" && !validStatus(q.VisitorStatus) {
		return q, ErrInvalid
	}
	return q, nil
}

type Page struct {
	Items    []Event
	Page     int
	PageSize int
	Total    int64
}

func validLevel(value Level) bool {
	switch value {
	case LevelNone, LevelLow, LevelMedium, LevelHigh:
		return true
	default:
		return false
	}
}

func validStatus(value Status) bool {
	switch value {
	case StatusNormal, StatusWatch, StatusRestricted, StatusBlocked:
		return true
	default:
		return false
	}
}

func validateVisitorID(value string) error {
	if value == "" || len([]rune(value)) > 64 {
		return ErrInvalid
	}
	for _, r := range value {
		if unicode.IsSpace(r) {
			return ErrInvalid
		}
	}
	return nil
}
