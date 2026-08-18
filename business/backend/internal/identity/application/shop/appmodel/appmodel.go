package appmodel

import "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"

type Health struct {
	Status model.Status
}

type CreateLoginOTP struct {
	ShopCode string
	Phone    string
	Email    string
}

type LoginOTP struct {
	ChallengeID string
	TTLSeconds  int
	ExpiresAt   string
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
