package appmodel

import "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"

type Health struct {
	Status model.Status
}

type CreateLoginOTP struct {
	ShopCode string
	Channel  string
	Phone    string
	Email    string
}

type LoginOTP struct {
	ChallengeID        string
	TTLSeconds         int
	ExpiresAt          string
	ResendAfterSeconds int
	NextSendAt         string
}

type CreateLogin struct {
	ShopCode    string
	ChallengeID string
	Code        string
}

type Login struct {
	ChallengeID string
	Verified    bool
}

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

type SaveAddress struct {
	ID              int64
	Recipient       string
	Phone           string
	Country         string
	Province        string
	City            string
	District        string
	Detail          string
	PostalCode      string
	IsDefault       bool
	CommandKey      string
	ExpectedVersion uint64
}

type DeleteAddress struct {
	ID              int64
	CommandKey      string
	ExpectedVersion uint64
}

type ReplaceDefault struct {
	ID              int64
	CommandKey      string
	ExpectedVersion uint64
}

type WishlistItem struct {
	ProductID int64
	CreatedAt int64
}

type AddWishlist struct {
	ProductID  int64
	CommandKey string
}

type Profile struct {
	Subject       string
	PrincipalType string
	SignedIn      bool
	DisplayName   string
}

type SMSRegion struct {
	DialCode string
	Name     string
	ISO2     string
	Emoji    string
}

type SMSRegions struct {
	Items        []SMSRegion
	Unrestricted bool
}

type AftersaleItem struct {
	ID               int64
	SKUID            int64
	Title            string
	Quantity         int64
	RefundAmount     int64
	ReceivedQuantity int64
}

type Aftersale struct {
	ID              int64
	OrderID         int64
	PaymentNo       string
	Type            string
	RequestedAmount int64
	Amount          int64
	Reason          string
	Status          string
	ReturnStatus    string
	HandleNote      string
	Items           []AftersaleItem
	Version         uint64
	CreatedAt       string
	UpdatedAt       string
	ReviewedAt      *string
	ReceivedAt      *string
}

type AftersaleQuery struct {
	Status   string
	Type     string
	Page     int
	PageSize int
}

type AftersalePage struct {
	Items    []Aftersale
	Page     int
	PageSize int
	Total    int64
}
