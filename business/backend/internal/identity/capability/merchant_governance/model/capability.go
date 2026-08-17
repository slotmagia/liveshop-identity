package model

import (
	"crypto/sha256"
	"errors"
	"strconv"
	"strings"
	"time"
)

type PlatformStatus string
type MerchantStatus string

const (
	PlatformActive     PlatformStatus = "active"
	PlatformRestricted PlatformStatus = "restricted"
	PlatformSuspended  PlatformStatus = "suspended"

	MerchantUnset  MerchantStatus = "unset"
	MerchantActive MerchantStatus = "active"
	MerchantDraft  MerchantStatus = "draft"
)

var (
	ErrUnavailable = errors.New("merchant governance repository unavailable")
	ErrNotFound    = errors.New("merchant capability scope not found")
	ErrConflict    = errors.New("merchant capability version conflict")
	ErrIdempotency = errors.New("merchant governance command key was reused with different input")
	ErrInvalid     = errors.New("invalid merchant governance request")
)

type Module struct {
	Key   string
	Label string
}

func Catalog() []Module {
	return []Module{
		{Key: "privacy", Label: "隐私"},
		{Key: "policies", Label: "政策"},
		{Key: "domains", Label: "域名"},
		{Key: "apps", Label: "私有应用"},
		{Key: "languages", Label: "语言"},
		{Key: "shipping", Label: "配送"},
	}
}

func ValidModule(value string) bool {
	for _, item := range Catalog() {
		if item.Key == value {
			return true
		}
	}
	return false
}

func ModuleLabel(value string) string {
	for _, item := range Catalog() {
		if item.Key == value {
			return item.Label
		}
	}
	return value
}

func ValidPlatformStatus(value PlatformStatus) bool {
	return value == PlatformActive || value == PlatformRestricted || value == PlatformSuspended
}

type Capability struct {
	ID                   int64          `json:"id"`
	MerchantID           int64          `json:"merchantId"`
	ShopID               int64          `json:"shopId"`
	Module               string         `json:"module"`
	ModuleLabel          string         `json:"moduleLabel"`
	Name                 string         `json:"name"`
	MerchantStatus       MerchantStatus `json:"merchantStatus"`
	PlatformStatus       PlatformStatus `json:"platformStatus"`
	PlatformReasonPublic string         `json:"platformReasonPublic"`
	Version              uint64         `json:"version"`
	UpdatedBy            string         `json:"updatedBy"`
	UpdatedAt            time.Time      `json:"updatedAt"`
}

func (c Capability) ValidatePersisted() error {
	if c.ID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 ||
		!ValidModule(c.Module) || !ValidPlatformStatus(c.PlatformStatus) || c.Version == 0 {
		return ErrInvalid
	}
	return nil
}

type Query struct {
	MerchantID int64
	ShopID     int64
	Module     string
}

func (q Query) Normalize() (Query, error) {
	q.Module = strings.ToLower(strings.TrimSpace(q.Module))
	if q.MerchantID <= 0 || q.ShopID <= 0 || (q.Module != "" && !ValidModule(q.Module)) {
		return q, ErrInvalid
	}
	return q, nil
}

type Page struct {
	Items []Capability
}

type AuditQuery struct {
	MerchantID int64
	ShopID     int64
	Module     string
	Limit      int
}

func (q AuditQuery) Normalize() (AuditQuery, error) {
	q.Module = strings.ToLower(strings.TrimSpace(q.Module))
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Limit > 200 || q.MerchantID <= 0 || q.ShopID <= 0 || (q.Module != "" && !ValidModule(q.Module)) {
		return q, ErrInvalid
	}
	return q, nil
}

type AuditItem struct {
	ID             int64
	MerchantID     int64
	ShopID         int64
	Module         string
	CapabilityID   int64
	Action         string
	Operator       string
	ReasonInternal string
	ReasonPublic   string
	CreatedAt      time.Time
}

type InterveneCommand struct {
	CommandKey      string
	ExpectedVersion uint64
	MerchantID      int64
	ShopID          int64
	Module          string
	PlatformStatus  PlatformStatus
	ReasonInternal  string
	ReasonPublic    string
	Operator        string
}

func (c InterveneCommand) Normalize() (InterveneCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.Module = strings.ToLower(strings.TrimSpace(c.Module))
	c.ReasonInternal = strings.TrimSpace(c.ReasonInternal)
	c.ReasonPublic = strings.TrimSpace(c.ReasonPublic)
	c.Operator = strings.TrimSpace(c.Operator)
	if c.PlatformStatus == PlatformActive {
		c.ReasonPublic = ""
	}
	if len(c.CommandKey) < 8 || len(c.CommandKey) > 128 || c.MerchantID <= 0 || c.ShopID <= 0 ||
		!ValidModule(c.Module) || !ValidPlatformStatus(c.PlatformStatus) ||
		c.ReasonInternal == "" || len([]rune(c.ReasonInternal)) > 1000 ||
		len([]rune(c.ReasonPublic)) > 500 || c.Operator == "" ||
		(c.PlatformStatus != PlatformActive && c.ReasonPublic == "") {
		return c, ErrInvalid
	}
	return c, nil
}

func (c InterveneCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"INTERVENE", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatInt(c.ShopID, 10),
		c.Module, string(c.PlatformStatus), strconv.FormatUint(c.ExpectedVersion, 10),
		c.ReasonInternal, c.ReasonPublic, c.Operator,
	}, "\n")))
}
