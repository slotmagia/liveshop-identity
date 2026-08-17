package model

import (
	"crypto/sha256"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

type CategoryStatus string

const (
	CategoryActive   CategoryStatus = "ACTIVE"
	CategoryDisabled CategoryStatus = "DISABLED"
	CategoryRetired  CategoryStatus = "RETIRED"
)

var (
	ErrCategoryNotFound    = errors.New("shop category not found")
	ErrCategoryConflict    = errors.New("shop category version or unique-key conflict")
	ErrCategoryIdempotency = errors.New("shop category command key was reused with different input")
	ErrCategoryInvalid     = errors.New("invalid shop category")
	categoryCodePattern    = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)
)

type Category struct {
	ID            int64
	Code          string
	Name          string
	Icon          string
	Sort          int
	Status        CategoryStatus
	Version       uint64
	UsedShopCount int64
}

func (c Category) Normalize() (Category, error) {
	c.Code = strings.TrimSpace(c.Code)
	c.Name = strings.TrimSpace(c.Name)
	c.Icon = strings.TrimSpace(c.Icon)
	if !categoryCodePattern.MatchString(c.Code) || len([]rune(c.Name)) == 0 || len([]rune(c.Name)) > 64 ||
		len([]rune(c.Icon)) > 16 || c.Sort < 0 || c.Sort > 1_000_000 ||
		(c.Status != CategoryActive && c.Status != CategoryDisabled && c.Status != CategoryRetired) {
		return c, ErrCategoryInvalid
	}
	return c, nil
}

type SaveCategoryCommand struct {
	CommandKey      string
	ExpectedVersion uint64
	Category        Category
}

func (c SaveCategoryCommand) Normalize() (SaveCategoryCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if len(c.CommandKey) < 8 || len(c.CommandKey) > 128 || (c.Category.ID == 0) != (c.ExpectedVersion == 0) {
		return c, ErrCategoryInvalid
	}
	category, err := c.Category.Normalize()
	c.Category = category
	if err != nil || c.Category.Status == CategoryRetired {
		return c, ErrCategoryInvalid
	}
	return c, nil
}

func (c SaveCategoryCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"SAVE", c.CommandKey, strconv.FormatInt(c.Category.ID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
		c.Category.Code, c.Category.Name, c.Category.Icon, strconv.Itoa(c.Category.Sort), string(c.Category.Status),
	}, "\n")))
}

type SetCategoryEnabledCommand struct {
	CategoryID      int64
	CommandKey      string
	ExpectedVersion uint64
	Enabled         bool
}

func (c SetCategoryEnabledCommand) Normalize() (SetCategoryEnabledCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.CategoryID <= 0 || c.ExpectedVersion == 0 || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrCategoryInvalid
	}
	return c, nil
}

func (c SetCategoryEnabledCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"SET_ENABLED", c.CommandKey, strconv.FormatInt(c.CategoryID, 10),
		strconv.FormatUint(c.ExpectedVersion, 10), strconv.FormatBool(c.Enabled),
	}, "\n")))
}

type RetireCategoryCommand struct {
	CategoryID      int64
	CommandKey      string
	ExpectedVersion uint64
}

func (c RetireCategoryCommand) Normalize() (RetireCategoryCommand, error) {
	c.CommandKey = strings.TrimSpace(c.CommandKey)
	if c.CategoryID <= 0 || c.ExpectedVersion == 0 || len(c.CommandKey) < 8 || len(c.CommandKey) > 128 {
		return c, ErrCategoryInvalid
	}
	return c, nil
}

func (c RetireCategoryCommand) RequestDigest() [32]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"RETIRE", c.CommandKey, strconv.FormatInt(c.CategoryID, 10), strconv.FormatUint(c.ExpectedVersion, 10),
	}, "\n")))
}
