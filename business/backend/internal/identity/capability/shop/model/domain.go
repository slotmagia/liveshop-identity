package model

import (
	"crypto/sha256"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type DomainStatus string
type DomainScene string

const (
	DomainPending  DomainStatus = "PENDING"
	DomainVerified DomainStatus = "VERIFIED"
	DomainFailed   DomainStatus = "FAILED"
	DomainDeleted  DomainStatus = "DELETED"

	DomainSceneLive DomainScene = "LIVE"
	DomainSceneShop DomainScene = "SHOP"

	DomainHostMax = 253
	DomainTXTMax  = 128
)

var (
	ErrDomainNotFound    = errors.New("shop domain not found")
	ErrDomainConflict    = errors.New("shop domain version or unique-key conflict")
	ErrDomainIdempotency = errors.New("shop domain command key was reused with different input")
	ErrDomainInvalid     = errors.New("invalid shop domain")
	ErrDomainRestricted  = errors.New("shop domain is restricted by platform")
	ErrDomainUnavailable = errors.New("shop domain dns lookup unavailable")
)

func ValidDomainStatus(value DomainStatus) bool {
	return value == DomainPending || value == DomainVerified || value == DomainFailed || value == DomainDeleted
}

func ValidLiveDomainStatusFilter(value DomainStatus) bool {
	return value == "" || value == DomainPending || value == DomainVerified || value == DomainFailed
}

func ValidDomainScene(value DomainScene) bool {
	return value == DomainSceneLive || value == DomainSceneShop
}

func NormalizeHost(value string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(value))
	if host == "" || strings.Contains(host, "://") || strings.ContainsAny(host, "/:\\ ") || strings.Contains(host, "..") {
		return "", ErrDomainInvalid
	}
	if len(host) < 4 || len(host) > DomainHostMax || !strings.Contains(host, ".") {
		return "", ErrDomainInvalid
	}
	if netHostLooksLikeIP(host) {
		return "", ErrDomainInvalid
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrDomainInvalid
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return "", ErrDomainInvalid
			}
		}
	}
	return host, nil
}

func ChallengeName(host string) string {
	return "_liveshop-challenge." + host
}

func netHostLooksLikeIP(host string) bool {
	dotCount := 0
	digitOnly := true
	for _, r := range host {
		if r == '.' {
			dotCount++
			continue
		}
		if !unicode.IsDigit(r) {
			digitOnly = false
			break
		}
	}
	return digitOnly && dotCount == 3
}

type Domain struct {
	ID         int64
	MerchantID int64
	ShopID     int64
	Host       string
	Scene      DomainScene
	Status     DomainStatus
	IsPrimary  bool
	TxtName    string
	TxtValue   string
	Version    uint64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (d Domain) ValidatePersisted() error {
	host, err := NormalizeHost(d.Host)
	if err != nil || d.ID <= 0 || d.MerchantID <= 0 || d.ShopID <= 0 || d.Version == 0 ||
		!ValidDomainScene(d.Scene) || !ValidDomainStatus(d.Status) ||
		d.TxtName != ChallengeName(host) || d.TxtValue == "" || len(d.TxtValue) > DomainTXTMax {
		return ErrDomainInvalid
	}
	return nil
}

type DomainQuery struct {
	MerchantID int64
	ShopID     int64
	Scene      DomainScene
	Status     DomainStatus
	Page       int
	PageSize   int
}

func (q DomainQuery) Normalize() (DomainQuery, error) {
	q.Status = DomainStatus(strings.ToUpper(strings.TrimSpace(string(q.Status))))
	q.Scene = DomainScene(strings.ToUpper(strings.TrimSpace(string(q.Scene))))
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	if q.MerchantID <= 0 || q.ShopID <= 0 || !ValidDomainScene(q.Scene) || !ValidLiveDomainStatusFilter(q.Status) {
		return q, ErrDomainInvalid
	}
	return q, nil
}

type DomainPage struct {
	Items    []Domain
	Page     int
	PageSize int
	Total    int64
}

type CreateDomainCommand struct {
	CommandKey string
	MerchantID int64
	ShopID     int64
	Host       string
	Scene      DomainScene
}

func (c CreateDomainCommand) Normalize() (CreateDomainCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	host, err := NormalizeHost(c.Host)
	c.Host = host
	c.Scene = DomainScene(strings.ToUpper(strings.TrimSpace(string(c.Scene))))
	if err != nil || c.MerchantID <= 0 || c.ShopID <= 0 || !commandKeyValid(c.CommandKey) || !ValidDomainScene(c.Scene) {
		return c, ErrDomainInvalid
	}
	return c, nil
}

func (c CreateDomainCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"CREATE", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatInt(c.ShopID, 10), c.Host, string(c.Scene),
	}, "\n")))
}

type DomainWriteCommand struct {
	DomainID        int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
	Scene           DomainScene
}

func (c DomainWriteCommand) Normalize() (DomainWriteCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.Scene = DomainScene(strings.ToUpper(strings.TrimSpace(string(c.Scene))))
	if c.DomainID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 || !commandKeyValid(c.CommandKey) {
		return c, ErrDomainInvalid
	}
	if c.Scene != "" && !ValidDomainScene(c.Scene) {
		return c, ErrDomainInvalid
	}
	return c, nil
}

func (c DomainWriteCommand) RequestDigest(action string) [32]byte {
	parts := []string{
		action, c.CommandKey, strconv.FormatInt(c.DomainID, 10), strconv.FormatInt(c.MerchantID, 10),
		strconv.FormatInt(c.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}
	if c.Scene != "" {
		parts = append(parts, string(c.Scene))
	}
	return sha256.Sum256([]byte(strings.Join(parts, "\n")))
}

func OptionalDomainScene(value string) (DomainScene, error) {
	scene := DomainScene(strings.ToUpper(strings.TrimSpace(value)))
	if scene == "" {
		return "", nil
	}
	if !ValidDomainScene(scene) {
		return "", ErrDomainInvalid
	}
	return scene, nil
}

func DefaultDomainScene(value string) (DomainScene, error) {
	scene, err := OptionalDomainScene(value)
	if err != nil {
		return "", err
	}
	if scene == "" {
		return DomainSceneLive, nil
	}
	return scene, nil
}
