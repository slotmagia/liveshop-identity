package shop

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type CategoryRepository interface {
	ListCategories(context.Context) ([]model.Category, error)
	SaveCategory(context.Context, model.SaveCategoryCommand) (model.Category, bool, error)
	SetCategoryEnabled(context.Context, model.SetCategoryEnabledCommand) (model.Category, bool, error)
	RetireCategory(context.Context, model.RetireCategoryCommand) (model.Category, bool, error)
}

type Categories struct{ repository CategoryRepository }

func NewCategories(repository CategoryRepository) *Categories {
	return &Categories{repository: repository}
}

func (c *Categories) List(ctx context.Context) ([]model.Category, error) {
	if c == nil || c.repository == nil {
		return nil, model.ErrUnavailable
	}
	values, err := c.repository.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if _, err := value.Normalize(); err != nil || value.Status == model.CategoryRetired || value.Version == 0 || value.ID <= 0 || value.UsedShopCount < 0 {
			return nil, model.ErrCategoryInvalid
		}
	}
	return values, nil
}

func (c *Categories) Save(ctx context.Context, command model.SaveCategoryCommand) (model.Category, bool, error) {
	if c == nil || c.repository == nil {
		return model.Category{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Category{}, false, err
	}
	return c.repository.SaveCategory(ctx, normalized)
}

func (c *Categories) SetEnabled(ctx context.Context, command model.SetCategoryEnabledCommand) (model.Category, bool, error) {
	if c == nil || c.repository == nil {
		return model.Category{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Category{}, false, err
	}
	return c.repository.SetCategoryEnabled(ctx, normalized)
}

func (c *Categories) Retire(ctx context.Context, command model.RetireCategoryCommand) (model.Category, bool, error) {
	if c == nil || c.repository == nil {
		return model.Category{}, false, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Category{}, false, err
	}
	return c.repository.RetireCategory(ctx, normalized)
}
