package compose

import (
	"context"
	"fmt"
	"net/url"
)

type I18nLocale struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type I18nEntity struct {
	EntityType  string `json:"entityType"`
	Label       string `json:"label"`
	OwnerModule string `json:"ownerModule"`
	Field       string `json:"field"`
}

type I18nPublishedText struct {
	EntityID string `json:"entityId"`
	Value    string `json:"value"`
	Version  int64  `json:"version"`
}

type I18nCatalog struct {
	Locales  []I18nLocale
	Entities []I18nEntity
}

func (Unavailable) I18nLocales(context.Context) ([]I18nLocale, error) {
	return nil, ErrUnavailable
}
func (Unavailable) I18nEntities(context.Context) ([]I18nEntity, error) {
	return nil, ErrUnavailable
}
func (Unavailable) I18nTexts(context.Context, string, string, int64, int64) ([]I18nPublishedText, error) {
	return nil, ErrUnavailable
}

func (h *HTTP) I18nLocales(ctx context.Context) ([]I18nLocale, error) {
	var out struct {
		Items []I18nLocale `json:"items"`
	}
	if err := h.get(ctx, h.platform, "/internal/platform/i18n/locales", nil, &out); err != nil {
		return nil, err
	}
	if out.Items == nil {
		out.Items = []I18nLocale{}
	}
	return out.Items, nil
}

func (h *HTTP) I18nEntities(ctx context.Context) ([]I18nEntity, error) {
	var out struct {
		Items []I18nEntity `json:"items"`
	}
	if err := h.get(ctx, h.platform, "/internal/platform/i18n/entities", nil, &out); err != nil {
		return nil, err
	}
	if out.Items == nil {
		out.Items = []I18nEntity{}
	}
	return out.Items, nil
}

func (h *HTTP) I18nTexts(ctx context.Context, entityType, locale string, merchantID, shopID int64) ([]I18nPublishedText, error) {
	query := url.Values{"entityType": {entityType}, "locale": {locale}, "merchantId": {fmt.Sprint(merchantID)}, "shopId": {fmt.Sprint(shopID)}}
	var out struct {
		Items []I18nPublishedText `json:"items"`
	}
	if err := h.get(ctx, h.platform, "/internal/platform/i18n/texts", query, &out); err != nil {
		return nil, err
	}
	if out.Items == nil {
		out.Items = []I18nPublishedText{}
	}
	return out.Items, nil
}
