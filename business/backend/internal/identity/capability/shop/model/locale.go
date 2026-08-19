package model

import (
	"crypto/sha256"
	"strconv"
	"strings"
)

const SourceLocale = DefaultLocale

type LocaleRow struct {
	Locale    string
	Published bool
	SortOrder int
}

type Languages struct {
	MerchantID    int64
	ShopID        int64
	DefaultLocale string
	Version       uint64
	Items         []LocaleRow
}

type ReplaceLanguagesCommand struct {
	MerchantID        int64
	ShopID            int64
	CommandKey        string
	ExpectedVersion   uint64
	DefaultLocale     string
	PublishedLocales  []string
	AllowedLocales    []string
}

func (c ReplaceLanguagesCommand) Normalize() (ReplaceLanguagesCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	c.DefaultLocale = strings.TrimSpace(c.DefaultLocale)
	published := uniqueLocales(c.PublishedLocales)
	allowed := uniqueLocales(c.AllowedLocales)
	if c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 ||
		len(c.CommandKey) < 8 || len(c.CommandKey) > 128 ||
		c.DefaultLocale == "" || len(published) == 0 || len(allowed) == 0 {
		return c, ErrInvalid
	}
	if !localePattern.MatchString(c.DefaultLocale) {
		return c, ErrInvalid
	}
	allowedSet := map[string]struct{}{}
	for _, locale := range allowed {
		if !localePattern.MatchString(locale) {
			return c, ErrInvalid
		}
		allowedSet[locale] = struct{}{}
	}
	foundDefault := false
	for _, locale := range published {
		if !localePattern.MatchString(locale) {
			return c, ErrInvalid
		}
		if _, ok := allowedSet[locale]; !ok {
			return c, ErrInvalid
		}
		if locale == c.DefaultLocale {
			foundDefault = true
		}
	}
	if !foundDefault {
		return c, ErrInvalid
	}
	c.PublishedLocales = published
	c.AllowedLocales = allowed
	return c, nil
}

func (c ReplaceLanguagesCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"LANGUAGES", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatInt(c.ShopID, 10),
		strconv.FormatUint(c.ExpectedVersion, 10), c.DefaultLocale, strings.Join(c.PublishedLocales, ","),
	}, "\n")))
}

func uniqueLocales(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		locale := strings.TrimSpace(value)
		if locale == "" {
			continue
		}
		if _, ok := seen[locale]; ok {
			continue
		}
		seen[locale] = struct{}{}
		out = append(out, locale)
	}
	return out
}

func DefaultLanguageRows(defaultLocale string) []LocaleRow {
	if strings.TrimSpace(defaultLocale) == "" {
		defaultLocale = SourceLocale
	}
	return []LocaleRow{{Locale: defaultLocale, Published: true, SortOrder: 0}}
}

func PublishedFromRows(defaultLocale string, rows []LocaleRow) (string, []string) {
	if strings.TrimSpace(defaultLocale) == "" {
		defaultLocale = SourceLocale
	}
	if len(rows) == 0 {
		return defaultLocale, []string{defaultLocale}
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Published && row.Locale != "" {
			out = append(out, row.Locale)
		}
	}
	if len(out) == 0 {
		out = []string{defaultLocale}
	}
	found := false
	for _, locale := range out {
		if locale == defaultLocale {
			found = true
			break
		}
	}
	if !found {
		defaultLocale = out[0]
	}
	return defaultLocale, out
}
