package model

import (
	"crypto/sha256"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ErrUnavailable     = errors.New("merchant directory unavailable")
	ErrInvalidMerchant = errors.New("invalid merchant directory item")
	ErrNotFound        = errors.New("merchant not found")
	ErrConflict        = errors.New("merchant version or unique-key conflict")
	ErrIdempotency     = errors.New("merchant command key was reused with different input")
	ErrInvalid         = errors.New("invalid merchant command")
	ErrClosed          = errors.New("merchant is closed")
	usernamePattern    = regexp.MustCompile(`^[a-z][a-z0-9._-]{2,63}$`)
	emailPattern       = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,24}$`)
)

type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusDisabled Status = "DISABLED"
	StatusClosed   Status = "CLOSED"
)

type Merchant struct {
	ID         int64
	Name       string
	ExternalID string
	Status     Status
	Version    uint64
}

func (m Merchant) Validate() error {
	if m.ID <= 0 || m.Name == "" || m.Version == 0 || (m.Status != StatusActive && m.Status != StatusDisabled) {
		return ErrInvalidMerchant
	}
	return nil
}

type Record struct {
	ID                  int64
	Name                string
	ExternalID          string
	Account             string
	ContactName         string
	ContactPhone        string
	MarketingEmailOptIn bool
	MarketingSMSOptIn   bool
	Status              Status
	Version             uint64
	ShopID              int64
	ShopCode            string
}

type Query struct {
	Keyword  string
	Status   string
	Page     int
	PageSize int
}

func (q Query) Normalize() Query {
	q.Keyword = strings.TrimSpace(q.Keyword)
	q.Status = strings.TrimSpace(q.Status)
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	return q
}

type Page struct {
	Items    []Record
	Page     int
	PageSize int
	Total    int64
}

type CreateCommand struct {
	CommandKey   string
	Account      string
	Password     string
	Name         string
	ContactName  string
	ContactPhone string
}

func validMerchantAccount(account string) bool {
	if usernamePattern.MatchString(account) {
		return true
	}
	return len(account) <= 191 && emailPattern.MatchString(account)
}

func (c CreateCommand) Normalize() (CreateCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.Account = strings.ToLower(strings.TrimSpace(c.Account))
	c.Name = strings.TrimSpace(c.Name)
	c.ContactName = strings.TrimSpace(c.ContactName)
	c.ContactPhone = strings.TrimSpace(c.ContactPhone)
	if len(c.CommandKey) < 8 || len(c.CommandKey) > 128 || !validMerchantAccount(c.Account) ||
		len(c.Password) < 8 || len([]rune(c.Name)) == 0 || len([]rune(c.Name)) > 191 ||
		len([]rune(c.ContactName)) > 128 || len([]rune(c.ContactPhone)) > 32 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c CreateCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"CREATE", c.CommandKey, c.Account, c.Password, c.Name, c.ContactName, c.ContactPhone,
	}, "\n")))
}

type CreateResult struct {
	Merchant Record
	ShopID   int64
	ShopCode string
	Account  string
}

type UpdateCommand struct {
	MerchantID      int64
	CommandKey      string
	ExpectedVersion uint64
	Name            string
	Status          Status
	ContactName     string
	ContactPhone    string
}

func (c UpdateCommand) Normalize() (UpdateCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.Name = strings.TrimSpace(c.Name)
	c.ContactName = strings.TrimSpace(c.ContactName)
	c.ContactPhone = strings.TrimSpace(c.ContactPhone)
	if c.MerchantID <= 0 || c.ExpectedVersion == 0 || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 ||
		len([]rune(c.Name)) == 0 || len([]rune(c.Name)) > 191 ||
		(c.Status != StatusActive && c.Status != StatusDisabled) ||
		len([]rune(c.ContactName)) > 128 || len([]rune(c.ContactPhone)) > 32 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c UpdateCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"UPDATE", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
		c.Name, string(c.Status), c.ContactName, c.ContactPhone,
	}, "\n")))
}

type ResetPasswordCommand struct {
	MerchantID int64
	CommandKey string
	Password   string
}

func (c ResetPasswordCommand) Normalize() (ResetPasswordCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.MerchantID <= 0 || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 || len(c.Password) < 8 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c ResetPasswordCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"RESET_PASSWORD", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), c.Password,
	}, "\n")))
}

type CloseCommand struct {
	MerchantID      int64
	CommandKey      string
	ExpectedVersion uint64
}

func (c CloseCommand) Normalize() (CloseCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.MerchantID <= 0 || c.ExpectedVersion == 0 || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c CloseCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"CLOSE", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}

type UpdateProfileCommand struct {
	MerchantID          int64
	CommandKey          string
	ExpectedVersion     uint64
	ExternalID          string
	ContactName         string
	ContactPhone        string
	MarketingEmailOptIn bool
	MarketingSMSOptIn   bool
}

func (c UpdateProfileCommand) Normalize() (UpdateProfileCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.ExternalID = strings.TrimSpace(c.ExternalID)
	c.ContactName = strings.TrimSpace(c.ContactName)
	c.ContactPhone = strings.TrimSpace(c.ContactPhone)
	if c.MerchantID <= 0 || c.ExpectedVersion == 0 || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 ||
		len([]rune(c.ExternalID)) > 64 || len([]rune(c.ContactName)) > 128 || len([]rune(c.ContactPhone)) > 32 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c UpdateProfileCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"UPDATE_PROFILE", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
		c.ExternalID, c.ContactName, c.ContactPhone, strconv.FormatBool(c.MarketingEmailOptIn), strconv.FormatBool(c.MarketingSMSOptIn),
	}, "\n")))
}

type ShopOption struct {
	ID     int64
	Name   string
	Code   string
	Status Status
}

func SubjectForMerchant(merchantID int64) string {
	return "merchant-owner-" + strconv.FormatInt(merchantID, 10)
}

func ShopCodeForMerchant(merchantID int64) string {
	return "shop-" + strconv.FormatInt(merchantID, 10)
}

func ExpiresAt(durationDays int, from time.Time) *time.Time {
	if durationDays <= 0 {
		return nil
	}
	value := from.UTC().AddDate(0, 0, durationDays)
	return &value
}
