package login

import "github.com/gogf/gf/v2/frame/g"

type CreateOTPReq struct {
	g.Meta   `path:"/login/otp" method:"post" tags:"Identity-shop" summary:"Create a login OTP challenge"`
	ShopCode string `json:"shopCode"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
}

type CreateOTPRes struct {
	ChallengeID string `json:"challengeId"`
	TTLSeconds  int    `json:"ttlSeconds"`
	ExpiresAt   string `json:"expiresAt"`
}

type CreateReq struct {
	g.Meta      `path:"/login" method:"post" tags:"Identity-shop" summary:"Verify a login OTP challenge"`
	ShopCode    string `json:"shopCode"`
	ChallengeID string `json:"challengeId"`
	Code        string `json:"code"`
}

type CreateRes struct {
	ChallengeID string `json:"challengeId"`
	Verified    bool   `json:"verified"`
}
