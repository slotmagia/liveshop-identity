// Package model owns identity entities, invariants and domain errors. It
// must not import GoFrame, gRPC, database/sql or any outer layer.
package model

import (
	"errors"
	"fmt"

	"github.com/lvtuopen-ai/kernel-go/principal"
)

// ErrUnavailable marks a capability whose backing dependency was not
// assembled. Transports translate it into their own unavailability signal.
var (
	ErrUnavailable         = errors.New("identity: a dependency is unavailable")
	ErrNotFound            = errors.New("identity: fact not found")
	ErrConflict            = errors.New("identity: version or unique-key conflict")
	ErrInvalidContext      = errors.New("identity: selected context is invalid")
	ErrInactive            = errors.New("identity: subject or membership is not active")
	ErrProtectedOwner      = errors.New("identity: merchant owner is protected")
	ErrInvalidAssignment   = errors.New("identity: invalid shop assignment")
	ErrInvalidShopCurrency = errors.New("identity: shop settlement currency is invalid")
	ErrIdempotencyConflict = errors.New("identity: idempotency key was reused with different input")
	ErrInvalidCredential   = errors.New("identity: credential verification failed")
)

type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusDisabled Status = "DISABLED"
	StatusClosed   Status = "CLOSED"
)

func (s Status) Valid() bool {
	return s == StatusActive || s == StatusDisabled || s == StatusClosed
}

type OrganizationType string

const (
	OrganizationPlatform OrganizationType = "PLATFORM"
	OrganizationMerchant OrganizationType = "MERCHANT"
)

type MemberType string

const (
	MemberOperator MemberType = "OPERATOR"
	MemberOwner    MemberType = "OWNER"
	MemberStaff    MemberType = "STAFF"
	MemberAnchor   MemberType = "ANCHOR"
)

type MemberStatus string

const (
	MemberActive    MemberStatus = "ACTIVE"
	MemberSuspended MemberStatus = "SUSPENDED"
	MemberRevoked   MemberStatus = "REVOKED"
)

type AssignmentKind string

const (
	AssignmentOperate AssignmentKind = "OPERATE"
	AssignmentAnchor  AssignmentKind = "ANCHOR"
)

type Subject struct {
	ID            string
	Realm         principal.Realm
	PrincipalType principal.Type
	DisplayName   string
	LegacyUID     int64
	Status        Status
	Version       uint64
}

func (s Subject) Validate() error {
	if s.ID == "" || !s.Realm.Valid() || !s.PrincipalType.Valid() ||
		!s.PrincipalType.AllowsRealm(s.Realm) || !s.Status.Valid() || s.Version == 0 {
		return fmt.Errorf("identity: invalid subject")
	}
	return nil
}

type ShopContext struct {
	MerchantID int64
	ShopID     int64
}

func (s ShopContext) Complete() bool {
	return s.MerchantID > 0 && s.ShopID > 0
}

// ShopResolution is the authoritative read model for a shop. Currency is
// intentionally not embedded in ShopContext: changing a settlement setting
// must never change the identity tuple used by authorization and isolation.
type ShopResolution struct {
	Context  ShopContext
	Currency string
	Status   Status
	Version  uint64
}

func (s ShopResolution) Validate() error {
	if !s.Context.Complete() || !s.Status.Valid() || s.Version == 0 {
		return ErrInvalidContext
	}
	if !ValidCurrency(s.Currency) {
		return ErrInvalidShopCurrency
	}
	return nil
}

// ValidCurrency validates the ISO 4217 wire representation used throughout
// LiveShop. Identity only publishes canonical uppercase three-letter values.
func ValidCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, current := range value {
		if current < 'A' || current > 'Z' {
			return false
		}
	}
	return true
}

type SelectedContext struct {
	OrganizationID int64
	ShopContext
}

type Organization struct {
	ID         int64
	Type       OrganizationType
	MerchantID int64
	Name       string
	Status     Status
	Version    uint64
}

type WorkforceMember struct {
	ID             int64
	OrganizationID int64
	MerchantID     int64
	Subject        string
	Type           MemberType
	Status         MemberStatus
	AccessVersion  uint64
	LegacyStaffID  int64
}

func (m WorkforceMember) Authorizable() bool { return m.Status == MemberActive }

type PrincipalContext struct {
	Subject             Subject
	Organization        Organization
	Member              WorkforceMember
	OrganizationUnitIDs []int64
	AvailableShops      []ShopContext
	Selected            SelectedContext
}

func (p PrincipalContext) ValidateSelected(candidate SelectedContext) error {
	if p.Subject.Status != StatusActive {
		return ErrInactive
	}
	if p.Subject.PrincipalType == principal.TypeCustomer || p.Subject.PrincipalType == principal.TypeGuest {
		return validateShop(candidate, p.AvailableShops)
	}
	if !p.Member.Authorizable() || candidate.OrganizationID != p.Organization.ID {
		return ErrInactive
	}
	switch p.Member.Type {
	case MemberOperator:
		if candidate.ShopID != 0 || candidate.MerchantID != 0 {
			return ErrInvalidContext
		}
		return nil
	case MemberOwner:
		if candidate.MerchantID != p.Member.MerchantID {
			return ErrInvalidContext
		}
		if candidate.ShopID == 0 {
			return nil
		}
		return validateShop(candidate, p.AvailableShops)
	case MemberStaff, MemberAnchor:
		if candidate.MerchantID != p.Member.MerchantID {
			return ErrInvalidContext
		}
		return validateShop(candidate, p.AvailableShops)
	default:
		return ErrInvalidContext
	}
}

func validateShop(candidate SelectedContext, available []ShopContext) error {
	if !candidate.ShopContext.Complete() {
		return ErrInvalidContext
	}
	for _, shop := range available {
		if shop == candidate.ShopContext {
			return nil
		}
	}
	return ErrInvalidContext
}

func ExpectedPrincipalType(memberType MemberType) principal.Type {
	switch memberType {
	case MemberOperator:
		return principal.TypePlatformOperator
	case MemberOwner:
		return principal.TypeMerchantOwner
	case MemberStaff:
		return principal.TypeMerchantStaff
	case MemberAnchor:
		return principal.TypeShopAnchor
	default:
		return ""
	}
}

// Health is the readiness of the facts this module owns.
type Health struct {
	Status Status
}
