package logic

import (
	"context"
	"errors"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/compose"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

func (l *Logic) Languages(ctx context.Context) (appmodel.Languages, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.Languages{}, err
	}
	if l.shops == nil {
		return appmodel.Languages{}, model.ErrUnavailable
	}
	value, err := l.shops.Languages(ctx, merchantID, shopID)
	if err != nil {
		return appmodel.Languages{}, err
	}
	return l.projectLanguages(ctx, value), nil
}

func (l *Logic) UpdateLanguages(ctx context.Context, input appmodel.UpdateLanguages) (appmodel.LanguagesMutation, error) {
	merchantID, shopID, err := l.shopScope(ctx)
	if err != nil {
		return appmodel.LanguagesMutation{}, err
	}
	if l.shops == nil {
		return appmodel.LanguagesMutation{}, model.ErrUnavailable
	}
	catalog, _ := l.languageCatalog(ctx)
	allowed := make([]string, 0, len(catalog))
	for _, item := range catalog {
		allowed = append(allowed, item.Code)
	}
	value, replayed, err := l.shops.ReplaceLanguages(ctx, shopmodel.ReplaceLanguagesCommand{
		MerchantID: merchantID, ShopID: shopID, CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
		DefaultLocale: input.DefaultLocale, PublishedLocales: input.PublishedLocales, AllowedLocales: allowed,
	})
	if err != nil {
		return appmodel.LanguagesMutation{}, err
	}
	return appmodel.LanguagesMutation{Languages: l.projectLanguages(ctx, value), Replayed: replayed}, nil
}

func (l *Logic) projectLanguages(ctx context.Context, value shopmodel.Languages) appmodel.Languages {
	catalog, platformOK := l.languageCatalog(ctx)
	labels := map[string]string{}
	for _, item := range catalog {
		labels[item.Code] = item.Label
	}
	published := map[string]shopmodel.LocaleRow{}
	for _, item := range value.Items {
		if item.Published {
			published[item.Locale] = item
		}
	}
	items := make([]appmodel.LanguageItem, 0, len(catalog))
	for index, item := range catalog {
		row, ok := published[item.Code]
		sortOrder := index
		if ok {
			sortOrder = row.SortOrder
		}
		status := "unavailable"
		percent := 0
		if platformOK {
			status = "available"
			percent = l.languageCompletion(ctx, value.MerchantID, value.ShopID, item.Code)
		}
		if item.Code == shopmodel.SourceLocale {
			percent = 100
		}
		items = append(items, appmodel.LanguageItem{
			Locale: item.Code, Label: item.Label, Published: ok, IsDefault: item.Code == value.DefaultLocale,
			SortOrder: sortOrder, CompletionPercent: percent, PlatformStatus: status,
		})
	}
	return appmodel.Languages{DefaultLocale: value.DefaultLocale, Version: value.Version, Items: items}
}

func (l *Logic) languageCatalog(ctx context.Context) ([]compose.I18nLocale, bool) {
	catalog := []compose.I18nLocale{{Code: shopmodel.SourceLocale, Label: "简体中文"}}
	if l.grants == nil {
		return append(catalog, compose.I18nLocale{Code: "en-US", Label: "English"}), false
	}
	items, err := l.grants.I18nLocales(ctx)
	if err != nil {
		if !errors.Is(err, compose.ErrUnavailable) {
			return append(catalog, compose.I18nLocale{Code: "en-US", Label: "English"}), false
		}
		return append(catalog, compose.I18nLocale{Code: "en-US", Label: "English"}), false
	}
	seen := map[string]struct{}{shopmodel.SourceLocale: {}}
	for _, item := range items {
		if item.Code == "" || item.Code == shopmodel.SourceLocale {
			continue
		}
		if _, ok := seen[item.Code]; ok {
			continue
		}
		seen[item.Code] = struct{}{}
		label := item.Label
		if label == "" {
			label = item.Code
		}
		catalog = append(catalog, compose.I18nLocale{Code: item.Code, Label: label})
	}
	if len(catalog) == 1 {
		catalog = append(catalog, compose.I18nLocale{Code: "en-US", Label: "English"})
	}
	return catalog, true
}

func (l *Logic) languageCompletion(ctx context.Context, merchantID, shopID int64, locale string) int {
	if locale == shopmodel.SourceLocale || l.grants == nil {
		if locale == shopmodel.SourceLocale {
			return 100
		}
		return 0
	}
	entities, err := l.grants.I18nEntities(ctx)
	if err != nil || len(entities) == 0 {
		return 0
	}
	filled := 0
	for _, entity := range entities {
		items, err := l.grants.I18nTexts(ctx, entity.EntityType, locale, merchantID, shopID)
		if err == nil && len(items) > 0 {
			filled++
		}
	}
	return filled * 100 / len(entities)
}
