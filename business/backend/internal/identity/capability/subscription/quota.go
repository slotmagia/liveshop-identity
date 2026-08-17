package subscription

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const CatalogProductsQuota = "catalog.products"

var quotaCodePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z][a-z0-9]*)+$`)

var (
	ErrNotConfigured       = errors.New("quota entitlement is not configured")
	ErrVersionConflict     = errors.New("quota entitlement revision conflict")
	ErrIdempotencyConflict = errors.New("idempotency key was used with another command")
)

type QuotaLimit struct {
	MerchantID     int64
	Code           string
	Limit          *int64
	Revision       int64
	EffectiveFrom  time.Time
	EffectiveUntil *time.Time
}

func (q QuotaLimit) Unlimited() bool { return q.Limit == nil }

type ApplyQuotaCommand struct {
	MerchantID       int64
	CommandKey       string
	Code             string
	Limit            *int64
	ExpectedRevision int64
	EffectiveFrom    time.Time
	EffectiveUntil   *time.Time
}

func (c ApplyQuotaCommand) Validate() error {
	if c.MerchantID <= 0 {
		return errors.New("merchant_id must be positive")
	}
	if len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return errors.New("command_key must contain 8 to 128 characters")
	}
	if !quotaCodePattern.MatchString(c.Code) || len(c.Code) > 96 {
		return errors.New("quota_code is invalid")
	}
	if c.Limit != nil && *c.Limit <= 0 {
		return errors.New("limit must be positive or null for unlimited")
	}
	if c.ExpectedRevision < 0 {
		return errors.New("expected_revision cannot be negative")
	}
	if c.EffectiveFrom.IsZero() {
		return errors.New("effective_from is required")
	}
	if c.EffectiveUntil != nil && !c.EffectiveUntil.After(c.EffectiveFrom) {
		return errors.New("effective_until must be after effective_from")
	}
	return nil
}

func (c ApplyQuotaCommand) RequestHash() string {
	limit := "unlimited"
	if c.Limit != nil {
		limit = strconv.FormatInt(*c.Limit, 10)
	}
	until := ""
	if c.EffectiveUntil != nil {
		until = c.EffectiveUntil.UTC().Format(time.RFC3339Nano)
	}
	canonical := strings.Join([]string{
		strconv.FormatInt(c.MerchantID, 10), c.Code, limit,
		strconv.FormatInt(c.ExpectedRevision, 10),
		c.EffectiveFrom.UTC().Format(time.RFC3339Nano), until,
	}, "\n")
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:])
}

type QuotaRepository interface {
	GetEffective(context.Context, int64, string, time.Time) (QuotaLimit, error)
	GetEffectiveMany(context.Context, int64, []string, time.Time) ([]QuotaLimit, error)
	Apply(context.Context, ApplyQuotaCommand) (QuotaLimit, bool, error)
	Ready(context.Context) error
}

type Quotas struct{ repository QuotaRepository }

func New(repository QuotaRepository) *Quotas { return &Quotas{repository: repository} }

func (q *Quotas) Get(ctx context.Context, merchantID int64, code string, at time.Time) (QuotaLimit, error) {
	if merchantID <= 0 || !quotaCodePattern.MatchString(code) {
		return QuotaLimit{}, fmt.Errorf("invalid quota lookup")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return q.repository.GetEffective(ctx, merchantID, code, at.UTC())
}

func (q *Quotas) GetMany(ctx context.Context, merchantID int64, codes []string, at time.Time) ([]QuotaLimit, error) {
	if merchantID <= 0 || len(codes) == 0 || len(codes) > 32 {
		return nil, fmt.Errorf("invalid quota lookup")
	}
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if !quotaCodePattern.MatchString(code) {
			return nil, fmt.Errorf("invalid quota code %q", code)
		}
		if _, exists := seen[code]; exists {
			return nil, fmt.Errorf("duplicate quota code %q", code)
		}
		seen[code] = struct{}{}
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return q.repository.GetEffectiveMany(ctx, merchantID, codes, at.UTC())
}

func (q *Quotas) Apply(ctx context.Context, command ApplyQuotaCommand) (QuotaLimit, bool, error) {
	if err := command.Validate(); err != nil {
		return QuotaLimit{}, false, err
	}
	return q.repository.Apply(ctx, command)
}

func (q *Quotas) Ready(ctx context.Context) error { return q.repository.Ready(ctx) }
