package compose

import (
	"context"
	"errors"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/appmodel"
	merchantmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
)

var ErrUnavailable = errors.New("merchant grant compose unavailable")

type Grants interface {
	PaymentChannels(context.Context, int64, int64) (appmodel.MerchantPaymentChannels, error)
	PutPaymentChannels(context.Context, appmodel.PutMerchantPaymentChannels) (appmodel.MerchantPaymentChannels, error)
	SMSRegions(context.Context, int64, int64) (appmodel.MerchantSMSRegions, error)
	PutSMSRegions(context.Context, appmodel.PutMerchantSMSRegions) (appmodel.MerchantSMSRegions, error)
	LiveProviders(context.Context, int64) (appmodel.MerchantLiveProviders, error)
	PutLiveProviders(context.Context, appmodel.PutMerchantLiveProviders) (appmodel.MerchantLiveProviders, error)
	EdgeSnapshot(context.Context) (EdgeSnapshot, error)
}

type EdgeSnapshot struct {
	CNAMETarget    string   `json:"cnameTarget"`
	RootDomain     string   `json:"rootDomain"`
	ShopDomain     string   `json:"shopDomain"`
	LiveDomain     string   `json:"liveDomain"`
	RTSDomain      string   `json:"rtsDomain"`
	AdminDomain    string   `json:"adminDomain"`
	MerchantDomain string   `json:"merchantDomain"`
	ForceHTTPS     bool     `json:"forceHttps"`
	ReservedHosts  []string `json:"reservedHosts"`
}

type Unavailable struct{}

func (Unavailable) PaymentChannels(context.Context, int64, int64) (appmodel.MerchantPaymentChannels, error) {
	return appmodel.MerchantPaymentChannels{}, merchantmodel.ErrUnavailable
}
func (Unavailable) PutPaymentChannels(context.Context, appmodel.PutMerchantPaymentChannels) (appmodel.MerchantPaymentChannels, error) {
	return appmodel.MerchantPaymentChannels{}, merchantmodel.ErrUnavailable
}
func (Unavailable) SMSRegions(context.Context, int64, int64) (appmodel.MerchantSMSRegions, error) {
	return appmodel.MerchantSMSRegions{}, merchantmodel.ErrUnavailable
}
func (Unavailable) PutSMSRegions(context.Context, appmodel.PutMerchantSMSRegions) (appmodel.MerchantSMSRegions, error) {
	return appmodel.MerchantSMSRegions{}, merchantmodel.ErrUnavailable
}
func (Unavailable) LiveProviders(context.Context, int64) (appmodel.MerchantLiveProviders, error) {
	return appmodel.MerchantLiveProviders{}, merchantmodel.ErrUnavailable
}
func (Unavailable) PutLiveProviders(context.Context, appmodel.PutMerchantLiveProviders) (appmodel.MerchantLiveProviders, error) {
	return appmodel.MerchantLiveProviders{}, merchantmodel.ErrUnavailable
}
func (Unavailable) EdgeSnapshot(context.Context) (EdgeSnapshot, error) {
	return EdgeSnapshot{}, merchantmodel.ErrUnavailable
}
