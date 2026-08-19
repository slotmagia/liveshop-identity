package model

import (
	"crypto/sha256"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	ErrUnavailable = errors.New("customer: unavailable")
	ErrNotFound    = errors.New("customer: not found")
	ErrConflict    = errors.New("customer: version or unique-key conflict")
	ErrIdempotency = errors.New("customer: command key was reused with different input")
	ErrInvalid     = errors.New("customer: invalid argument")
)

type Tenant struct{ MerchantID, ShopID int64 }

func (t Tenant) Valid() bool { return t.MerchantID > 0 && t.ShopID > 0 }

type Address struct {
	ID         int64
	Recipient  string
	Phone      string
	Country    string
	Province   string
	City       string
	District   string
	Detail     string
	PostalCode string
	IsDefault  bool
	Version    uint64
}

func (a Address) Normalize() (Address, error) {
	a.Recipient = strings.TrimSpace(a.Recipient)
	a.Phone = strings.TrimSpace(a.Phone)
	a.Country = strings.TrimSpace(a.Country)
	a.Province = strings.TrimSpace(a.Province)
	a.City = strings.TrimSpace(a.City)
	a.District = strings.TrimSpace(a.District)
	a.Detail = strings.TrimSpace(a.Detail)
	a.PostalCode = strings.TrimSpace(a.PostalCode)
	if utf8.RuneCountInString(a.Recipient) == 0 || utf8.RuneCountInString(a.Recipient) > 64 ||
		len(a.Phone) < 6 || len(a.Phone) > 32 ||
		utf8.RuneCountInString(a.Country) > 64 || utf8.RuneCountInString(a.Province) > 64 ||
		utf8.RuneCountInString(a.City) > 64 || utf8.RuneCountInString(a.District) > 64 ||
		utf8.RuneCountInString(a.Detail) == 0 || utf8.RuneCountInString(a.Detail) > 512 ||
		utf8.RuneCountInString(a.PostalCode) > 16 {
		return a, ErrInvalid
	}
	return a, nil
}

type SaveAddressCommand struct {
	Tenant          Tenant
	Subject         string
	CommandKey      string
	ExpectedVersion uint64
	Address         Address
}

func (c SaveAddressCommand) Normalize() (SaveAddressCommand, error) {
	c.Subject = strings.TrimSpace(c.Subject)
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	address, err := c.Address.Normalize()
	c.Address = address
	if err != nil || !c.Tenant.Valid() || c.Subject == "" || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 ||
		(c.Address.ID == 0) != (c.ExpectedVersion == 0) {
		return c, ErrInvalid
	}
	return c, nil
}

func (c SaveAddressCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"SAVE", c.CommandKey, strconv.FormatInt(c.Address.ID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
		c.Address.Recipient, c.Address.Phone, c.Address.Country, c.Address.Province, c.Address.City,
		c.Address.District, c.Address.Detail, c.Address.PostalCode, strconv.FormatBool(c.Address.IsDefault),
	}, "\n")))
}

type DeleteAddressCommand struct {
	Tenant          Tenant
	Subject         string
	AddressID       int64
	CommandKey      string
	ExpectedVersion uint64
}

func (c DeleteAddressCommand) Normalize() (DeleteAddressCommand, error) {
	c.Subject = strings.TrimSpace(c.Subject)
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if !c.Tenant.Valid() || c.Subject == "" || c.AddressID <= 0 || c.ExpectedVersion == 0 ||
		len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c DeleteAddressCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"DELETE", c.CommandKey, strconv.FormatInt(c.AddressID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}

type ReplaceDefaultCommand struct {
	Tenant          Tenant
	Subject         string
	AddressID       int64
	CommandKey      string
	ExpectedVersion uint64
}

func (c ReplaceDefaultCommand) Normalize() (ReplaceDefaultCommand, error) {
	c.Subject = strings.TrimSpace(c.Subject)
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if !c.Tenant.Valid() || c.Subject == "" || c.AddressID <= 0 || c.ExpectedVersion == 0 ||
		len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c ReplaceDefaultCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"DEFAULT", c.CommandKey, strconv.FormatInt(c.AddressID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}

type WishlistItem struct {
	ProductID int64
	CreatedAt int64
}

type AddWishlistCommand struct {
	Tenant     Tenant
	Subject    string
	ProductID  int64
	CommandKey string
}

func (c AddWishlistCommand) Normalize() (AddWishlistCommand, error) {
	c.Subject = strings.TrimSpace(c.Subject)
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if !c.Tenant.Valid() || c.Subject == "" || c.ProductID <= 0 || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrInvalid
	}
	return c, nil
}

func (c AddWishlistCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"WISH_ADD", c.CommandKey, strconv.FormatInt(c.ProductID, 10),
	}, "\n")))
}

type RemoveWishlistCommand struct {
	Tenant    Tenant
	Subject   string
	ProductID int64
}

func (c RemoveWishlistCommand) Normalize() (RemoveWishlistCommand, error) {
	c.Subject = strings.TrimSpace(c.Subject)
	if !c.Tenant.Valid() || c.Subject == "" || c.ProductID <= 0 {
		return c, ErrInvalid
	}
	return c, nil
}
