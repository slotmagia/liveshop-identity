// Package model owns customer-service account facts and invariants.
package model

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusDisabled Status = "DISABLED"
)

var (
	ErrUnavailable  = errors.New("customer service account repository unavailable")
	ErrNotFound     = errors.New("customer service account not found")
	ErrConflict     = errors.New("customer service account version or unique-key conflict")
	ErrIdempotency  = errors.New("customer service command key was reused with different input")
	ErrInvalid      = errors.New("invalid customer service account")
	platformPattern = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
)

type Account struct {
	ID           int64     `json:"id"`
	MerchantID   int64     `json:"merchantId"`
	ShopID       int64     `json:"shopId"`
	Platform     string    `json:"platform"`
	Account      string    `json:"account"`
	Nickname     string    `json:"nickname"`
	Status       Status    `json:"status"`
	Config       string    `json:"config"`
	Remark       string    `json:"remark"`
	Version      uint64    `json:"version"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (a Account) NormalizeEditable() (Account, error) {
	a.Platform = strings.ToLower(strings.TrimSpace(a.Platform))
	a.Account = strings.TrimSpace(a.Account)
	a.Nickname = strings.TrimSpace(a.Nickname)
	a.Config = strings.TrimSpace(a.Config)
	a.Remark = strings.TrimSpace(a.Remark)
	if !platformPattern.MatchString(a.Platform) || len([]rune(a.Account)) == 0 || len([]rune(a.Account)) > 128 ||
		len([]rune(a.Nickname)) > 64 || len(a.Config) > 4096 || (a.Config != "" && !json.Valid([]byte(a.Config))) ||
		len([]rune(a.Remark)) > 500 || (a.Status != StatusActive && a.Status != StatusDisabled) {
		return a, ErrInvalid
	}
	return a, nil
}

func (a Account) ValidatePersisted() error {
	if a.ID <= 0 || a.MerchantID <= 0 || a.ShopID <= 0 || a.Version == 0 {
		return ErrInvalid
	}
	_, err := a.NormalizeEditable()
	return err
}

type Query struct {
	MerchantID int64
	ShopID     int64
	Platform   string
	Account    string
	Status     *Status
	Page       int
	PageSize   int
}

func (q Query) Normalize() (Query, error) {
	q.Platform = strings.ToLower(strings.TrimSpace(q.Platform))
	q.Account = strings.TrimSpace(q.Account)
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 200 {
		q.PageSize = 20
	}
	if q.MerchantID < 0 || q.ShopID < 0 ||
		(q.Platform != "" && !platformPattern.MatchString(q.Platform)) || len([]rune(q.Account)) > 128 ||
		(q.Status != nil && *q.Status != StatusActive && *q.Status != StatusDisabled) {
		return q, ErrInvalid
	}
	return q, nil
}

type Page struct {
	Items    []Account
	Page     int
	PageSize int
	Total    int64
}

type SaveCommand struct {
	CommandKey      string
	ExpectedVersion uint64
	Account         Account
}

func (c SaveCommand) Normalize() (SaveCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	account, err := c.Account.NormalizeEditable()
	c.Account = account
	if err != nil || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 || c.Account.MerchantID <= 0 || c.Account.ShopID <= 0 ||
		(c.Account.ID == 0) != (c.ExpectedVersion == 0) {
		return c, ErrInvalid
	}
	return c, nil
}

func (c SaveCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"SAVE", c.CommandKey, strconv.FormatInt(c.Account.ID, 10), strconv.FormatInt(c.Account.MerchantID, 10),
		strconv.FormatInt(c.Account.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10), c.Account.Platform,
		c.Account.Account, c.Account.Nickname, string(c.Account.Status), c.Account.Config, c.Account.Remark,
	}, "\n")))
}

type DeleteCommand struct {
	AccountID       int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
}

func (c DeleteCommand) Normalize() (DeleteCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.AccountID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c DeleteCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"DELETE", c.CommandKey, strconv.FormatInt(c.AccountID, 10), strconv.FormatInt(c.MerchantID, 10),
		strconv.FormatInt(c.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}

type DeleteResult struct {
	ID      int64  `json:"id"`
	Deleted bool   `json:"deleted"`
	Version uint64 `json:"version"`
}
