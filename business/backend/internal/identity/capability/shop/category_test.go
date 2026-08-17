package shop

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type stubCategoryRepository struct {
	saved model.SaveCategoryCommand
}

func (s *stubCategoryRepository) ListCategories(context.Context) ([]model.Category, error) {
	return []model.Category{{ID: 1, Code: "mother_baby", Name: "母婴用品", Icon: "🍼", Sort: 6, Status: model.CategoryActive, Version: 1}}, nil
}
func (s *stubCategoryRepository) SaveCategory(_ context.Context, command model.SaveCategoryCommand) (model.Category, bool, error) {
	s.saved = command
	return command.Category, false, nil
}
func (s *stubCategoryRepository) SetCategoryEnabled(context.Context, model.SetCategoryEnabledCommand) (model.Category, bool, error) {
	return model.Category{}, false, nil
}
func (s *stubCategoryRepository) RetireCategory(context.Context, model.RetireCategoryCommand) (model.Category, bool, error) {
	return model.Category{}, false, nil
}

func TestCategorySaveNormalizesAndValidatesStableCode(t *testing.T) {
	repository := &stubCategoryRepository{}
	service := NewCategories(repository)
	_, _, err := service.Save(context.Background(), model.SaveCategoryCommand{
		CommandKey: "category-create-1",
		Category:   model.Category{Code: " mother_baby ", Name: " 母婴用品 ", Icon: " 🍼 ", Sort: 6, Status: model.CategoryActive},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.saved.Category.Code != "mother_baby" || repository.saved.Category.Name != "母婴用品" || repository.saved.Category.Icon != "🍼" {
		t.Fatalf("normalized command=%+v", repository.saved)
	}
	_, _, err = service.Save(context.Background(), model.SaveCategoryCommand{
		CommandKey: "category-create-2",
		Category:   model.Category{Code: "Product.Category", Name: "错误", Status: model.CategoryActive},
	})
	if !errors.Is(err, model.ErrCategoryInvalid) {
		t.Fatalf("invalid code error=%v", err)
	}
}

func TestCategoryCommandDigestSeparatesMutations(t *testing.T) {
	base := model.SetCategoryEnabledCommand{CategoryID: 7, CommandKey: "category-toggle-1", ExpectedVersion: 2, Enabled: true}
	changed := base
	changed.Enabled = false
	if base.RequestDigest() == changed.RequestDigest() {
		t.Fatal("enable and disable commands must not share a digest")
	}
	retire := model.RetireCategoryCommand{CategoryID: 7, CommandKey: base.CommandKey, ExpectedVersion: 2}
	if base.RequestDigest() == retire.RequestDigest() {
		t.Fatal("toggle and retire commands must not share a digest")
	}
}
