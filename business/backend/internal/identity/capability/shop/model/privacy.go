package model

import (
	"crypto/sha256"
	"errors"
	"net/mail"
	"strconv"
	"strings"
)

const (
	PrivacyRetentionMin = 1
	PrivacyRetentionMax = 3650
	PrivacyDefaultDays  = 365
)

var (
	ErrPrivacyNotFound    = errors.New("shop privacy scope not found")
	ErrPrivacyConflict    = errors.New("shop privacy version conflict")
	ErrPrivacyIdempotency = errors.New("shop privacy command key was reused with different input")
	ErrPrivacyInvalid     = errors.New("invalid shop privacy")
	ErrPrivacyRestricted  = errors.New("shop privacy is restricted by platform")
)

type Privacy struct {
	ID                int64  `json:"id,omitempty"`
	MerchantID        int64  `json:"merchantId"`
	ShopID            int64  `json:"shopId"`
	CollectConsent    bool   `json:"collectConsent"`
	MarketingConsent  bool   `json:"marketingConsent"`
	CookieBanner      bool   `json:"cookieBanner"`
	DataRetentionDays int    `json:"dataRetentionDays"`
	ContactEmail      string `json:"contactEmail"`
	Version           uint64 `json:"version"`
}

func DefaultPrivacy(merchantID, shopID int64) Privacy {
	return Privacy{
		MerchantID: merchantID, ShopID: shopID,
		CollectConsent: true, CookieBanner: true, DataRetentionDays: PrivacyDefaultDays,
	}
}

func (p Privacy) Normalize() (Privacy, error) {
	p.ContactEmail = strings.ToLower(strings.TrimSpace(p.ContactEmail))
	if p.MerchantID <= 0 || p.ShopID <= 0 ||
		p.DataRetentionDays < PrivacyRetentionMin || p.DataRetentionDays > PrivacyRetentionMax {
		return p, ErrPrivacyInvalid
	}
	if p.ContactEmail != "" {
		if _, err := mail.ParseAddress(p.ContactEmail); err != nil || strings.ContainsAny(p.ContactEmail, " <>") {
			return p, ErrPrivacyInvalid
		}
		if len(p.ContactEmail) > 254 {
			return p, ErrPrivacyInvalid
		}
	}
	return p, nil
}

func (p Privacy) ValidatePersisted() error {
	if p.ID <= 0 || p.Version == 0 {
		return ErrPrivacyInvalid
	}
	_, err := p.Normalize()
	return err
}

type SavePrivacyCommand struct {
	CommandKey      string
	ExpectedVersion uint64
	Privacy         Privacy
}

func (c SavePrivacyCommand) Normalize() (SavePrivacyCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	privacy, err := c.Privacy.Normalize()
	c.Privacy = privacy
	if err != nil || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrPrivacyInvalid
	}
	return c, nil
}

func (c SavePrivacyCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"SAVE", c.CommandKey,
		strconv.FormatInt(c.Privacy.MerchantID, 10), strconv.FormatInt(c.Privacy.ShopID, 10),
		strconv.FormatUint(c.ExpectedVersion, 10),
		strconv.FormatBool(c.Privacy.CollectConsent), strconv.FormatBool(c.Privacy.MarketingConsent),
		strconv.FormatBool(c.Privacy.CookieBanner), strconv.Itoa(c.Privacy.DataRetentionDays),
		c.Privacy.ContactEmail,
	}, "\n")))
}
