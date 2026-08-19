package http

import (
	"context"

	api "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/api/http/v1/languages"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type LanguagesQueryController struct{ service service.Merch }

func NewLanguagesQuery(s service.Merch) *LanguagesQueryController {
	return &LanguagesQueryController{service: s}
}

func (c *LanguagesQueryController) Get(ctx context.Context, _ *api.GetReq) (*api.GetRes, error) {
	value, err := c.service.Languages(ctx)
	if err != nil {
		return nil, web.Failure(err)
	}
	res := api.GetRes(wireLanguages(value))
	return &res, nil
}

type LanguagesWriteController struct{ service service.Merch }

func NewLanguagesWrite(s service.Merch) *LanguagesWriteController {
	return &LanguagesWriteController{service: s}
}

func (c *LanguagesWriteController) Update(ctx context.Context, request *api.UpdateReq) (*api.UpdateRes, error) {
	value, err := c.service.UpdateLanguages(ctx, appmodel.UpdateLanguages{
		CommandKey: request.CommandKey, ExpectedVersion: request.ExpectedVersion,
		DefaultLocale: request.DefaultLocale, PublishedLocales: request.PublishedLocales,
	})
	if err != nil {
		return nil, web.Failure(err)
	}
	res := api.UpdateRes{DefaultLocale: value.Languages.DefaultLocale, Version: value.Languages.Version, Items: wireItems(value.Languages.Items), Replayed: value.Replayed}
	return &res, nil
}

func wireLanguages(value appmodel.Languages) api.GetRes {
	return api.GetRes{DefaultLocale: value.DefaultLocale, Version: value.Version, Items: wireItems(value.Items)}
}

func wireItems(items []appmodel.LanguageItem) []api.Item {
	out := make([]api.Item, 0, len(items))
	for _, item := range items {
		out = append(out, api.Item{
			Locale: item.Locale, Label: item.Label, Published: item.Published, IsDefault: item.IsDefault,
			SortOrder: item.SortOrder, CompletionPercent: item.CompletionPercent, PlatformStatus: item.PlatformStatus,
		})
	}
	return out
}
