package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	EventKey       = "identity.auth.otp.requested"
	TTLSeconds     = 300
	MaxAttempts    = 5
	StatusPending  = "PENDING"
	StatusConsumed = "CONSUMED"
	StatusExpired  = "EXPIRED"
	StatusSent     = "SENT"
	CodeLength     = 6
)

var (
	ErrUnavailable    = errors.New("auth otp unavailable")
	ErrInvalid        = errors.New("invalid login otp")
	ErrNotFound       = errors.New("login otp challenge not found")
	ErrExpired        = errors.New("login otp challenge expired")
	ErrDeliveryFailed = errors.New("login otp delivery failed")
	phonePattern      = regexp.MustCompile(`^\+?[0-9]{8,20}$`)
	shopCodePattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	challengePattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	codePattern       = regexp.MustCompile(`^[0-9]{6}$`)
)

type RequestCommand struct {
	ShopCode string
	Phone    string
	Email    string
}

type VerifyCommand struct {
	ShopCode    string
	ChallengeID string
	Code        string
}

type Challenge struct {
	ID         string
	TTLSeconds int
	ExpiresAt  time.Time
}

type Record struct {
	ID           string
	MerchantID   int64
	ShopID       int64
	ShopCode     string
	Phone        string
	Email        string
	CodeHash     string
	TTLSeconds   int
	Status       string
	AttemptCount int
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

type Delivery struct {
	Channel string
	Status  string
}

func (c RequestCommand) Normalize() (RequestCommand, error) {
	c.ShopCode = strings.TrimSpace(c.ShopCode)
	c.Phone = strings.TrimSpace(c.Phone)
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	if !shopCodePattern.MatchString(c.ShopCode) {
		return RequestCommand{}, ErrInvalid
	}
	if c.Phone != "" && !phonePattern.MatchString(c.Phone) {
		return RequestCommand{}, ErrInvalid
	}
	if c.Email != "" {
		address, err := mail.ParseAddress(c.Email)
		if err != nil || address.Address != c.Email || len(c.Email) > 254 {
			return RequestCommand{}, ErrInvalid
		}
	}
	if c.Phone == "" && c.Email == "" {
		return RequestCommand{}, ErrInvalid
	}
	return c, nil
}

func (c VerifyCommand) Normalize() (VerifyCommand, error) {
	c.ShopCode = strings.TrimSpace(c.ShopCode)
	c.ChallengeID = strings.ToLower(strings.TrimSpace(c.ChallengeID))
	c.Code = strings.TrimSpace(c.Code)
	if !shopCodePattern.MatchString(c.ShopCode) || !challengePattern.MatchString(c.ChallengeID) || !codePattern.MatchString(c.Code) {
		return VerifyCommand{}, ErrInvalid
	}
	return c, nil
}

func HashCode(challengeID, code string) string {
	sum := sha256.Sum256(append(append([]byte(challengeID), 0), code...))
	return hex.EncodeToString(sum[:])
}

func DeliveryKey(challengeID string) string {
	return EventKey + ":" + challengeID
}

func Delivered(deliveries []Delivery) bool {
	for _, item := range deliveries {
		if item.Status == StatusSent {
			return true
		}
	}
	return false
}

func ValidCode(code string) bool {
	if len(code) != CodeLength {
		return false
	}
	for _, r := range code {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
