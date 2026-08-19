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

type SMSRegion struct {
	DialCode string `json:"dialCode"`
	Name     string `json:"name"`
	ISO2     string `json:"iso2"`
	Emoji    string `json:"emoji"`
}

type ListSMSRegionsReq struct {
	g.Meta   `path:"/login/sms-regions" method:"get" tags:"Identity-shop" summary:"List shop login SMS regions"`
	ShopCode string `json:"shopCode" in:"query"`
}

type ListSMSRegionsRes struct {
	Items        []SMSRegion `json:"items"`
	Unrestricted bool        `json:"unrestricted"`
}
