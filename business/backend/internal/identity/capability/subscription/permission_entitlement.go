package subscription

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var permissionCodePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*(?:\.[a-z][a-z0-9-]*)+$`)

var ErrPermissionEntitlementNotConfigured = errors.New("permission entitlement is not configured")

type PermissionEntitlement struct {
	MerchantID      int64
	PermissionCodes []string
	Revision        uint64
	SnapshotDigest  string
	UpdatedAt       time.Time
}

type ApplyPermissionEntitlementCommand struct {
	MerchantID       int64
	CommandKey       string
	PermissionCodes  []string
	ExpectedRevision uint64
}

func (c ApplyPermissionEntitlementCommand) Normalize() (ApplyPermissionEntitlementCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.MerchantID <= 0 {
		return c, errors.New("merchant_id must be positive")
	}
	if len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, errors.New("command_key must contain 8 to 128 characters")
	}
	seen := make(map[string]struct{}, len(c.PermissionCodes))
	normalized := make([]string, 0, len(c.PermissionCodes))
	for _, code := range c.PermissionCodes {
		code = strings.TrimSpace(code)
		if len(code) > 191 || !permissionCodePattern.MatchString(code) {
			return c, errors.New("permission code is invalid")
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		normalized = append(normalized, code)
	}
	sort.Strings(normalized)
	c.PermissionCodes = normalized
	return c, nil
}

func (c ApplyPermissionEntitlementCommand) RequestDigest() [32]byte {
	canonical := strings.Join([]string{
		strconv.FormatInt(c.MerchantID, 10),
		c.CommandKey,
		strconv.FormatUint(c.ExpectedRevision, 10),
		strings.Join(c.PermissionCodes, "\n"),
	}, "\n")
	return sha256.Sum256([]byte(canonical))
}

func PermissionSnapshotDigest(permissionCodes []string) [32]byte {
	return sha256.Sum256([]byte(strings.Join(permissionCodes, "\n")))
}

func HexDigest(value [32]byte) string { return hex.EncodeToString(value[:]) }

type PermissionEntitlementRepository interface {
	GetPermissionEntitlement(context.Context, int64) (PermissionEntitlement, error)
	ApplyPermissionEntitlement(context.Context, ApplyPermissionEntitlementCommand) (PermissionEntitlement, bool, error)
}

type PermissionEntitlements struct {
	repository PermissionEntitlementRepository
}

func NewPermissionEntitlements(repository PermissionEntitlementRepository) *PermissionEntitlements {
	return &PermissionEntitlements{repository: repository}
}

func (p *PermissionEntitlements) Get(ctx context.Context, merchantID int64) (PermissionEntitlement, error) {
	if merchantID <= 0 {
		return PermissionEntitlement{}, errors.New("merchant_id must be positive")
	}
	return p.repository.GetPermissionEntitlement(ctx, merchantID)
}

func (p *PermissionEntitlements) Apply(ctx context.Context, command ApplyPermissionEntitlementCommand) (PermissionEntitlement, bool, error) {
	normalized, err := command.Normalize()
	if err != nil {
		return PermissionEntitlement{}, false, err
	}
	return p.repository.ApplyPermissionEntitlement(ctx, normalized)
}
