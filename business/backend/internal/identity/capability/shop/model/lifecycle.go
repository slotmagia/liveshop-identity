package model

import (
	"crypto/sha256"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrNotFound         = errors.New("shop not found")
	ErrConflict         = errors.New("shop version or unique-key conflict")
	ErrIdempotency      = errors.New("shop command key was reused with different input")
	ErrInvalid          = errors.New("invalid shop command")
	ErrLastShop         = errors.New("merchant must keep at least one shop")
	ErrMerchantClosed   = errors.New("closed merchant cannot accept shop writes")
	ErrCategoryInactive = errors.New("shop category is not active")
	subdomainPattern    = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	localePattern       = regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`)
	currencyPattern     = regexp.MustCompile(`^[A-Z]{3}$`)
)

const (
	StatusClosed     Status = "CLOSED"
	DefaultLocale           = "zh-CN"
	DefaultCurrency         = "CNY"
	maxShopNameRunes        = 191
)

func (s Shop) ValidatePersisted() error {
	if s.ID <= 0 || s.MerchantID <= 0 || s.Code == "" || s.Name == "" ||
		s.Currency == "" || s.Version == 0 ||
		(s.Status != StatusActive && s.Status != StatusDisabled && s.Status != StatusClosed) {
		return ErrInvalidShop
	}
	if s.CategoryCode != "" && !categoryCodePattern.MatchString(s.CategoryCode) {
		return ErrInvalidShop
	}
	return nil
}

type Query struct {
	MerchantID int64
	Keyword    string
	Status     Status
	Page       int
	PageSize   int
}

type Page struct {
	Items    []Shop
	Page     int
	PageSize int
	Total    int64
}

func (q Query) Normalize() (Query, error) {
	q.Keyword = strings.TrimSpace(q.Keyword)
	if q.MerchantID <= 0 {
		return q, ErrInvalidMerchantID
	}
	if q.Status != "" && q.Status != StatusActive && q.Status != StatusDisabled {
		return q, ErrInvalid
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	if len([]rune(q.Keyword)) > 64 {
		return q, ErrInvalid
	}
	return q, nil
}

type CreateCommand struct {
	CommandKey    string
	MerchantID    int64
	Name          string
	Subdomain     string
	Currency      string
	DefaultLocale string
	CategoryCode  string
	Status        Status
}

func (c CreateCommand) Normalize() (CreateCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.Name = strings.TrimSpace(c.Name)
	c.Subdomain = strings.ToLower(strings.TrimSpace(c.Subdomain))
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.DefaultLocale = strings.TrimSpace(c.DefaultLocale)
	c.CategoryCode = strings.TrimSpace(c.CategoryCode)
	if c.DefaultLocale == "" {
		c.DefaultLocale = DefaultLocale
	}
	if c.Currency == "" {
		c.Currency = DefaultCurrency
	}
	if c.Status == "" {
		c.Status = StatusActive
	}
	if len(c.CommandKey) < 8 || len(c.CommandKey) > 128 || c.MerchantID <= 0 ||
		len([]rune(c.Name)) == 0 || len([]rune(c.Name)) > maxShopNameRunes ||
		!subdomainPattern.MatchString(c.Subdomain) || !currencyPattern.MatchString(c.Currency) ||
		!localePattern.MatchString(c.DefaultLocale) ||
		(c.Status != StatusActive && c.Status != StatusDisabled) ||
		(c.CategoryCode != "" && !categoryCodePattern.MatchString(c.CategoryCode)) {
		return c, ErrInvalid
	}
	return c, nil
}

func (c CreateCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"CREATE", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), c.Name, c.Subdomain,
		c.Currency, c.DefaultLocale, c.CategoryCode, string(c.Status),
	}, "\n")))
}

type UpdateCommand struct {
	ShopID          int64
	MerchantID      int64
	CommandKey      string
	ExpectedVersion uint64
	Name            string
	Subdomain       string
}

func (c UpdateCommand) Normalize() (UpdateCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.Name = strings.TrimSpace(c.Name)
	c.Subdomain = strings.ToLower(strings.TrimSpace(c.Subdomain))
	if c.ShopID <= 0 || c.MerchantID <= 0 || c.ExpectedVersion == 0 ||
		len(c.CommandKey) < 8 || len(c.CommandKey) > 128 ||
		len([]rune(c.Name)) == 0 || len([]rune(c.Name)) > maxShopNameRunes ||
		!subdomainPattern.MatchString(c.Subdomain) {
		return c, ErrInvalid
	}
	return c, nil
}

func (c UpdateCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"UPDATE", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatInt(c.ShopID, 10),
		strconv.FormatUint(c.ExpectedVersion, 10), c.Name, c.Subdomain,
	}, "\n")))
}

type SetEnabledCommand struct {
	ShopID          int64
	MerchantID      int64
	CommandKey      string
	ExpectedVersion uint64
	Enabled         bool
}

func (c SetEnabledCommand) Normalize() (SetEnabledCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.ShopID <= 0 || c.MerchantID <= 0 || c.ExpectedVersion == 0 ||
		len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c SetEnabledCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"SET_ENABLED", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatInt(c.ShopID, 10),
		strconv.FormatUint(c.ExpectedVersion, 10), strconv.FormatBool(c.Enabled),
	}, "\n")))
}

type CloseCommand struct {
	ShopID          int64
	MerchantID      int64
	CommandKey      string
	ExpectedVersion uint64
}

func (c CloseCommand) Normalize() (CloseCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.ShopID <= 0 || c.MerchantID <= 0 || c.ExpectedVersion == 0 ||
		len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c CloseCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"CLOSE", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatInt(c.ShopID, 10),
		strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}

func ShopCodeForID(shopID int64) string {
	return "shop-" + strconv.FormatInt(shopID, 10)
}
