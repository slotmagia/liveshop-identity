package model

import (
	"crypto/sha256"
	"errors"
	"strconv"
	"strings"
	"time"
)

type AppStatus string

const (
	AppActive   AppStatus = "ACTIVE"
	AppDisabled AppStatus = "DISABLED"

	AppNameMax    = 120
	AppHintLength = 6
)

var (
	ErrAppNotFound    = errors.New("shop private app not found")
	ErrAppConflict    = errors.New("shop private app version or unique-key conflict")
	ErrAppIdempotency = errors.New("shop private app command key was reused with different input")
	ErrAppInvalid     = errors.New("invalid shop private app")
	ErrAppRestricted  = errors.New("shop private app is restricted by platform")
)

type AppScope struct {
	Code  string
	Group string
	Label string
}

func AppScopeCatalog() []AppScope {
	return []AppScope{
		{Code: "orders:read", Group: "orders", Label: "读取订单"},
		{Code: "orders:write", Group: "orders", Label: "写入订单"},
		{Code: "products:read", Group: "products", Label: "读取商品"},
		{Code: "products:write", Group: "products", Label: "写入商品"},
		{Code: "customers:read", Group: "customers", Label: "读取客户"},
		{Code: "inventory:read", Group: "inventory", Label: "读取库存"},
		{Code: "live:read", Group: "live", Label: "读取直播"},
	}
}

func ValidAppScope(code string) bool {
	for _, item := range AppScopeCatalog() {
		if item.Code == code {
			return true
		}
	}
	return false
}

func ValidAppStatus(value AppStatus) bool {
	return value == AppActive || value == AppDisabled
}

func NormalizeAppScopes(value string) (string, error) {
	parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(value)), func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	})
	seen := map[string]struct{}{}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if !ValidAppScope(part) {
			return "", ErrAppInvalid
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	joined := strings.Join(out, ",")
	if len(joined) > 1000 {
		return "", ErrAppInvalid
	}
	return joined, nil
}

type App struct {
	ID         int64
	MerchantID int64
	ShopID     int64
	Name       string
	ClientID   string
	SecretHint string
	Scopes     string
	Status     AppStatus
	Version    uint64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (a App) NormalizeEditable() (App, error) {
	a.Name = strings.TrimSpace(a.Name)
	scopes, err := NormalizeAppScopes(a.Scopes)
	a.Scopes = scopes
	nameRunes := []rune(a.Name)
	if err != nil || a.MerchantID <= 0 || a.ShopID <= 0 ||
		len(nameRunes) == 0 || len(nameRunes) > AppNameMax {
		return a, ErrAppInvalid
	}
	return a, nil
}

func (a App) ValidatePersisted() error {
	if a.ID <= 0 || a.Version == 0 || a.ClientID == "" || a.SecretHint == "" || !ValidAppStatus(a.Status) {
		return ErrAppInvalid
	}
	_, err := a.NormalizeEditable()
	return err
}

type AppQuery struct {
	MerchantID int64
	ShopID     int64
	Status     AppStatus
	Page       int
	PageSize   int
}

func (q AppQuery) Normalize() (AppQuery, error) {
	q.Status = AppStatus(strings.ToUpper(strings.TrimSpace(string(q.Status))))
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	if q.MerchantID <= 0 || q.ShopID <= 0 || (q.Status != "" && !ValidAppStatus(q.Status)) {
		return q, ErrAppInvalid
	}
	return q, nil
}

type AppPage struct {
	Items    []App
	Page     int
	PageSize int
	Total    int64
}

type AppMutation struct {
	App          App
	ClientSecret string
}

func commandKeyValid(value string) bool {
	return len(value) >= 8 && len(value) <= 128
}

type CreateAppCommand struct {
	CommandKey string
	MerchantID int64
	ShopID     int64
	Name       string
	Scopes     string
}

func (c CreateAppCommand) Normalize() (CreateAppCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	app, err := App{MerchantID: c.MerchantID, ShopID: c.ShopID, Name: c.Name, Scopes: c.Scopes}.NormalizeEditable()
	c.MerchantID, c.ShopID, c.Name, c.Scopes = app.MerchantID, app.ShopID, app.Name, app.Scopes
	if err != nil || !commandKeyValid(c.CommandKey) {
		return c, ErrAppInvalid
	}
	return c, nil
}

func (c CreateAppCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"CREATE", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatInt(c.ShopID, 10), c.Name, c.Scopes,
	}, "\n")))
}

type ResetAppSecretCommand struct {
	AppID           int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
}

func (c ResetAppSecretCommand) Normalize() (ResetAppSecretCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.AppID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 || !commandKeyValid(c.CommandKey) {
		return c, ErrAppInvalid
	}
	return c, nil
}

func (c ResetAppSecretCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"RESET", c.CommandKey, strconv.FormatInt(c.AppID, 10), strconv.FormatInt(c.MerchantID, 10),
		strconv.FormatInt(c.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}

type SetAppEnabledCommand struct {
	AppID           int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
	Enabled         bool
}

func (c SetAppEnabledCommand) Normalize() (SetAppEnabledCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.AppID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 || !commandKeyValid(c.CommandKey) {
		return c, ErrAppInvalid
	}
	return c, nil
}

func (c SetAppEnabledCommand) RequestDigest() [32]byte {
	action := "DISABLE"
	if c.Enabled {
		action = "ENABLE"
	}
	return sha256.Sum256([]byte(strings.Join([]string{
		action, c.CommandKey, strconv.FormatInt(c.AppID, 10), strconv.FormatInt(c.MerchantID, 10),
		strconv.FormatInt(c.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}
