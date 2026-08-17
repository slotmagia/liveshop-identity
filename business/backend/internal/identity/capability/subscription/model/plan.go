package model

import (
	"crypto/sha256"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type PlanStatus string

const (
	PlanActive   PlanStatus = "ACTIVE"
	PlanDisabled PlanStatus = "DISABLED"
	PlanRetired  PlanStatus = "RETIRED"
)

const CatalogProductsQuota = "catalog.products"

var (
	ErrPlanNotFound           = errors.New("subscription: plan not found")
	ErrPlanConflict           = errors.New("subscription: plan version or unique-key conflict")
	ErrPlanIdempotency        = errors.New("subscription: command key was reused with different input")
	ErrPlanInvalid            = errors.New("subscription: invalid plan")
	ErrPlanDefaultRequired    = errors.New("subscription: an active default plan is required")
	ErrPlanPermissionInactive = errors.New("subscription: plan references an inactive permission")
	ErrAssignmentNotFound     = errors.New("subscription: merchant assignment not found")
	ErrAssignmentConflict     = errors.New("subscription: merchant assignment version conflict")
	ErrAssignmentIdempotency  = errors.New("subscription: merchant assignment command key was reused with different input")
	ErrAssignmentInvalid      = errors.New("subscription: invalid merchant assignment")
	ErrOrderNotFound          = errors.New("subscription: purchase order not found")
	ErrOrderConflict          = errors.New("subscription: purchase order conflict")
	ErrOrderIdempotency       = errors.New("subscription: purchase command key was reused with different input")
	ErrOrderInvalid           = errors.New("subscription: invalid purchase order")
	ErrOrderNotBuyable        = errors.New("subscription: plan is not available for merchant purchase")
	ErrPaymentUnavailable     = errors.New("subscription: payment collection is unavailable")
	planCodePattern           = regexp.MustCompile(`^[a-z][a-z0-9-]{1,63}$`)
	permissionCodePattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*(?:\.[a-z][a-z0-9-]*)+$`)
)

type Plan struct {
	ID           int64
	Code         string
	Name         string
	Level        int
	PriceMinor   int64
	DurationDays int
	Description  string
	Default      bool
	Sort         int
	Status       PlanStatus
	Version      uint64
}

func (p Plan) Validate() error {
	if !planCodePattern.MatchString(p.Code) || strings.TrimSpace(p.Name) == "" || len([]rune(p.Name)) > 191 {
		return ErrPlanInvalid
	}
	if p.PriceMinor < 0 || p.DurationDays < 0 || len(p.Description) > 65535 {
		return ErrPlanInvalid
	}
	if p.Status != PlanActive && p.Status != PlanDisabled && p.Status != PlanRetired {
		return ErrPlanInvalid
	}
	if p.Default && p.Status != PlanActive {
		return ErrPlanInvalid
	}
	return nil
}

func (p Plan) Normalize() (Plan, error) {
	p.Code = strings.TrimSpace(p.Code)
	p.Name = strings.TrimSpace(p.Name)
	p.Description = strings.TrimSpace(p.Description)
	return p, p.Validate()
}

type SavePlanCommand struct {
	CommandKey      string
	ExpectedVersion uint64
	Plan            Plan
}

func (c SavePlanCommand) Normalize() (SavePlanCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrPlanInvalid
	}
	plan, err := c.Plan.Normalize()
	c.Plan = plan
	if err != nil {
		return c, err
	}
	if (c.Plan.ID == 0) != (c.ExpectedVersion == 0) || c.Plan.Status == PlanRetired {
		return c, ErrPlanInvalid
	}
	return c, nil
}

func (c SavePlanCommand) RequestDigest() [32]byte {
	canonical := strings.Join([]string{
		"SAVE", c.CommandKey, strconv.FormatInt(c.Plan.ID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
		c.Plan.Code, c.Plan.Name, strconv.Itoa(c.Plan.Level), strconv.FormatInt(c.Plan.PriceMinor, 10),
		strconv.Itoa(c.Plan.DurationDays), c.Plan.Description, strconv.FormatBool(c.Plan.Default),
		strconv.Itoa(c.Plan.Sort), string(c.Plan.Status),
	}, "\n")
	return sha256.Sum256([]byte(canonical))
}

type PlanPolicy struct {
	PlanID          int64
	PlanCode        string
	PlanName        string
	PermissionCodes []string
	ProductLimit    *int64
	Revision        uint64
}

func (p PlanPolicy) Normalize() (PlanPolicy, error) {
	if p.PlanID <= 0 || p.Revision == 0 {
		return p, ErrPlanInvalid
	}
	if p.ProductLimit != nil && *p.ProductLimit <= 0 {
		return p, ErrPlanInvalid
	}
	seen := make(map[string]struct{}, len(p.PermissionCodes))
	codes := make([]string, 0, len(p.PermissionCodes))
	for _, code := range p.PermissionCodes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if len(code) > 191 || !permissionCodePattern.MatchString(code) {
			return p, ErrPlanInvalid
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	sort.Strings(codes)
	p.PermissionCodes = codes
	return p, nil
}

type SavePlanPolicyCommand struct {
	CommandKey       string
	ExpectedRevision uint64
	Policy           PlanPolicy
}

func (c SavePlanPolicyCommand) Normalize() (SavePlanPolicyCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if len(c.CommandKey) < 8 || len(c.CommandKey) > 128 || c.ExpectedRevision == 0 {
		return c, ErrPlanInvalid
	}
	c.Policy.Revision = c.ExpectedRevision
	policy, err := c.Policy.Normalize()
	c.Policy = policy
	return c, err
}

func (c SavePlanPolicyCommand) RequestDigest() [32]byte {
	limit := "unlimited"
	if c.Policy.ProductLimit != nil {
		limit = strconv.FormatInt(*c.Policy.ProductLimit, 10)
	}
	return sha256.Sum256([]byte(strings.Join([]string{
		"SAVE_POLICY", c.CommandKey, strconv.FormatInt(c.Policy.PlanID, 10),
		strconv.FormatUint(c.ExpectedRevision, 10), strings.Join(c.Policy.PermissionCodes, "\n"), limit,
	}, "\n")))
}

type RetirePlanCommand struct {
	PlanID          int64
	CommandKey      string
	ExpectedVersion uint64
}

func (c RetirePlanCommand) Validate() error {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.PlanID <= 0 || c.ExpectedVersion == 0 || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return ErrPlanInvalid
	}
	return nil
}

func (c RetirePlanCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"RETIRE", strings.TrimSpace(c.CommandKey), strconv.FormatInt(c.PlanID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}

type Assignment struct {
	MerchantID int64
	PlanID     int64
	PlanCode   string
	PlanName   string
	ExpiresAt  string
	Version    uint64
}

type AssignCommand struct {
	MerchantID      int64
	CommandKey      string
	ExpectedVersion uint64
	PlanID          int64
	ExpiresAt       *string
}

func (c AssignCommand) Normalize() (AssignCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.MerchantID <= 0 || c.PlanID <= 0 || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrAssignmentInvalid
	}
	return c, nil
}

func (c AssignCommand) RequestDigest() [32]byte {
	expires := ""
	if c.ExpiresAt != nil {
		expires = *c.ExpiresAt
	}
	return sha256.Sum256([]byte(strings.Join([]string{
		"ASSIGN", c.CommandKey, strconv.FormatInt(c.MerchantID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
		strconv.FormatInt(c.PlanID, 10), expires,
	}, "\n")))
}
