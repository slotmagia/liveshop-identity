package model

import (
	"crypto/sha256"
	"errors"
	"strconv"
	"strings"
	"time"
)

type PolicyType string
type PolicyStatus string

const (
	PolicyPrivacy  PolicyType = "privacy"
	PolicyTerms    PolicyType = "terms"
	PolicyRefund   PolicyType = "refund"
	PolicyShipping PolicyType = "shipping"

	PolicyDraft     PolicyStatus = "DRAFT"
	PolicyPublished PolicyStatus = "PUBLISHED"
	PolicyArchived  PolicyStatus = "ARCHIVED"
)

var (
	ErrPolicyNotFound    = errors.New("shop policy not found")
	ErrPolicyConflict    = errors.New("shop policy version or unique-key conflict")
	ErrPolicyIdempotency = errors.New("shop policy command key was reused with different input")
	ErrPolicyInvalid     = errors.New("invalid shop policy")
	ErrPolicyRestricted  = errors.New("shop policy is restricted by platform")
)

func ValidPolicyType(value PolicyType) bool {
	return value == PolicyPrivacy || value == PolicyTerms || value == PolicyRefund || value == PolicyShipping
}

func ValidPolicyStatus(value PolicyStatus) bool {
	return value == PolicyDraft || value == PolicyPublished || value == PolicyArchived
}

type Policy struct {
	ID             int64
	MerchantID     int64
	ShopID         int64
	PolicyType     PolicyType
	Title          string
	Content        string
	VersionNo      int
	Status         PolicyStatus
	Version        uint64
	PublishedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	PlatformStatus string
	PlatformReason string
}

func (p Policy) NormalizeEditable() (Policy, error) {
	p.PolicyType = PolicyType(strings.ToLower(strings.TrimSpace(string(p.PolicyType))))
	p.Title = strings.TrimSpace(p.Title)
	p.Content = strings.TrimSpace(p.Content)
	titleRunes := []rune(p.Title)
	contentRunes := []rune(p.Content)
	if p.MerchantID <= 0 || p.ShopID <= 0 || !ValidPolicyType(p.PolicyType) ||
		len(titleRunes) == 0 || len(titleRunes) > 255 ||
		len(contentRunes) < 10 || len(contentRunes) > 20000 {
		return p, ErrPolicyInvalid
	}
	return p, nil
}

func (p Policy) ValidatePersisted() error {
	if p.ID <= 0 || p.VersionNo <= 0 || p.Version == 0 || !ValidPolicyStatus(p.Status) {
		return ErrPolicyInvalid
	}
	_, err := p.NormalizeEditable()
	return err
}

type PolicyQuery struct {
	MerchantID int64
	ShopID     int64
	PolicyType PolicyType
	Status     PolicyStatus
	Page       int
	PageSize   int
}

func (q PolicyQuery) Normalize() (PolicyQuery, error) {
	q.PolicyType = PolicyType(strings.ToLower(strings.TrimSpace(string(q.PolicyType))))
	q.Status = PolicyStatus(strings.ToUpper(strings.TrimSpace(string(q.Status))))
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	if q.MerchantID <= 0 || q.ShopID <= 0 ||
		(q.PolicyType != "" && !ValidPolicyType(q.PolicyType)) ||
		(q.Status != "" && !ValidPolicyStatus(q.Status)) {
		return q, ErrPolicyInvalid
	}
	return q, nil
}

type PolicyPage struct {
	Items    []Policy
	Page     int
	PageSize int
	Total    int64
}

type SavePolicyCommand struct {
	CommandKey string
	MerchantID int64
	ShopID     int64
	PolicyType PolicyType
	Title      string
	Content    string
	Publish    bool
}

func (c SavePolicyCommand) Normalize() (SavePolicyCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	policy, err := Policy{MerchantID: c.MerchantID, ShopID: c.ShopID, PolicyType: c.PolicyType, Title: c.Title, Content: c.Content}.NormalizeEditable()
	c.MerchantID, c.ShopID, c.PolicyType, c.Title, c.Content = policy.MerchantID, policy.ShopID, policy.PolicyType, policy.Title, policy.Content
	if err != nil || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrPolicyInvalid
	}
	return c, nil
}

func (c SavePolicyCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"SAVE", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatInt(c.ShopID, 10),
		string(c.PolicyType), c.Title, c.Content, strconv.FormatBool(c.Publish),
	}, "\n")))
}

type PublishPolicyCommand struct {
	PolicyID        int64
	MerchantID      int64
	ShopID          int64
	CommandKey      string
	ExpectedVersion uint64
}

func (c PublishPolicyCommand) Normalize() (PublishPolicyCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.PolicyID <= 0 || c.MerchantID <= 0 || c.ShopID <= 0 || c.ExpectedVersion == 0 ||
		len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrPolicyInvalid
	}
	return c, nil
}

func (c PublishPolicyCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"PUBLISH", c.CommandKey, strconv.FormatInt(c.PolicyID, 10), strconv.FormatInt(c.MerchantID, 10),
		strconv.FormatInt(c.ShopID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}
