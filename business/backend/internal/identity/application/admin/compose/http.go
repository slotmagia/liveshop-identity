package compose

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/config"
)

type HTTP struct {
	trade    string
	platform string
	token    string
	client   *http.Client
}

func NewHTTP(settings config.Compose) Grants {
	if settings.TradeOrigin == "" && settings.PlatformOrigin == "" {
		return Unavailable{}
	}
	return &HTTP{trade: settings.TradeOrigin, platform: settings.PlatformOrigin, token: settings.InternalToken, client: &http.Client{Timeout: 8 * time.Second}}
}

func (h *HTTP) PaymentChannels(ctx context.Context, merchantID, shopID int64) (appmodel.MerchantPaymentChannels, error) {
	var out appmodel.MerchantPaymentChannels
	err := h.get(ctx, h.trade, "/internal/v1/payment/merchant-channels", url.Values{"merchantId": {fmt.Sprint(merchantID)}, "shopId": {fmt.Sprint(shopID)}}, &out)
	return out, err
}

func (h *HTTP) PutPaymentChannels(ctx context.Context, input appmodel.PutMerchantPaymentChannels) (appmodel.MerchantPaymentChannels, error) {
	var out appmodel.MerchantPaymentChannels
	err := h.put(ctx, h.trade, "/internal/v1/payment/merchant-channels", input, &out)
	return out, err
}

func (h *HTTP) SMSRegions(ctx context.Context, merchantID, shopID int64) (appmodel.MerchantSMSRegions, error) {
	var out appmodel.MerchantSMSRegions
	err := h.get(ctx, h.platform, "/internal/v1/sms/merchant-grants", url.Values{"merchantId": {fmt.Sprint(merchantID)}, "shopId": {fmt.Sprint(shopID)}}, &out)
	return out, err
}

func (h *HTTP) PutSMSRegions(ctx context.Context, input appmodel.PutMerchantSMSRegions) (appmodel.MerchantSMSRegions, error) {
	var out appmodel.MerchantSMSRegions
	err := h.put(ctx, h.platform, "/internal/v1/sms/merchant-grants", input, &out)
	return out, err
}

func (h *HTTP) LiveProviders(ctx context.Context, merchantID int64) (appmodel.MerchantLiveProviders, error) {
	var out appmodel.MerchantLiveProviders
	err := h.get(ctx, h.platform, "/internal/v1/live-providers/assignments", url.Values{"merchantId": {fmt.Sprint(merchantID)}}, &out)
	return out, err
}

func (h *HTTP) PutLiveProviders(ctx context.Context, input appmodel.PutMerchantLiveProviders) (appmodel.MerchantLiveProviders, error) {
	var out appmodel.MerchantLiveProviders
	err := h.put(ctx, h.platform, "/internal/v1/live-providers/assignments", input, &out)
	return out, err
}

func (h *HTTP) EdgeSnapshot(ctx context.Context) (EdgeSnapshot, error) {
	var out EdgeSnapshot
	err := h.get(ctx, h.platform, "/internal/v1/edge/snapshot", nil, &out)
	return out, err
}

func (h *HTTP) get(ctx context.Context, origin, path string, query url.Values, out any) error {
	if origin == "" {
		return ErrUnavailable
	}
	target := origin + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	return h.do(request, out)
}

func (h *HTTP) put(ctx context.Context, origin, path string, body, out any) error {
	if origin == "" {
		return ErrUnavailable
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, origin+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	return h.do(request, out)
}

func (h *HTTP) do(request *http.Request, out any) error {
	if h.token != "" {
		request.Header.Set("X-Liveshop-Internal-Grant", h.token)
	}
	response, err := h.client.Do(request)
	if err != nil {
		return fmt.Errorf("identity compose: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode >= 300 {
		return fmt.Errorf("identity compose: upstream %d", response.StatusCode)
	}
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(payload, &envelope) == nil && envelope.Code == 0 && len(envelope.Data) > 0 {
		return json.Unmarshal(envelope.Data, out)
	}
	return json.Unmarshal(payload, out)
}
