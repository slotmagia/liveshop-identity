package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/kernel-go/modulesession"
	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type stubCategoryRepo struct{}

func (stubCategoryRepo) ListCategories(context.Context) ([]shopmodel.Category, error) {
	return []shopmodel.Category{
		{ID: 1, Code: "apparel", Name: "服装服饰", Icon: "👗", Sort: 1, Status: shopmodel.CategoryActive, Version: 1},
		{ID: 2, Code: "retired_cat", Name: "停用品类", Sort: 2, Status: shopmodel.CategoryDisabled, Version: 1},
	}, nil
}
func (stubCategoryRepo) SaveCategory(context.Context, shopmodel.SaveCategoryCommand) (shopmodel.Category, bool, error) {
	return shopmodel.Category{}, false, nil
}
func (stubCategoryRepo) SetCategoryEnabled(context.Context, shopmodel.SetCategoryEnabledCommand) (shopmodel.Category, bool, error) {
	return shopmodel.Category{}, false, nil
}
func (stubCategoryRepo) RetireCategory(context.Context, shopmodel.RetireCategoryCommand) (shopmodel.Category, bool, error) {
	return shopmodel.Category{}, false, nil
}

func merchShopLogic() *Logic {
	return New(nil, nil, nil, nil, shop.NewDirectory(stubShopRepo{}), nil, nil, nil, nil, Subscription{}, nil, shop.NewCategories(stubCategoryRepo{}), nil)
}

func TestShopCategoriesRequiresMerchantContext(t *testing.T) {
	if _, err := merchShopLogic().ShopCategories(context.Background()); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestShopCategoriesReturnsActiveOnly(t *testing.T) {
	values, err := merchShopLogic().ShopCategories(merchOwnerContext())
	if err != nil || len(values) != 1 || values[0].Code != "apparel" {
		t.Fatalf("values=%+v err=%v", values, err)
	}
}

func TestManagedShopsUsesSessionMerchant(t *testing.T) {
	page, err := merchShopLogic().ManagedShops(merchOwnerContext(), appmodel.ShopQuery{Page: 1, PageSize: 20})
	if err != nil || !page.Owner || page.Total != 1 || page.Items[0].MerchantID != 7 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	staffPage, err := merchShopLogic().ManagedShops(merchStaffContext(), appmodel.ShopQuery{Page: 1, PageSize: 20})
	if err != nil || staffPage.Owner {
		t.Fatalf("staff page=%+v err=%v", staffPage, err)
	}
}

func merchShopSession(owner bool) context.Context {
	claims := modulesession.Claims{Subject: "owner-1", PrincipalType: principal.TypeMerchantOwner, OrganizationID: 9, MerchantID: 7, ShopID: 3001}
	if !owner {
		claims.Subject = "staff-1"
		claims.PrincipalType = principal.TypeMerchantStaff
	}
	return authctx.With(context.Background(), claims)
}

func TestCurrentShopUsesSessionShop(t *testing.T) {
	if _, err := merchShopLogic().CurrentShop(merchOwnerContext()); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("missing shop error=%v", err)
	}
	value, err := merchShopLogic().CurrentShop(merchShopSession(true))
	if err != nil || !value.Owner || value.Shop.ShopID != 3001 || value.Shop.MerchantID != 7 {
		t.Fatalf("owner=%+v err=%v", value, err)
	}
	staff, err := merchShopLogic().CurrentShop(merchShopSession(false))
	if err != nil || staff.Owner || staff.Shop.ShopID != 3001 {
		t.Fatalf("staff=%+v err=%v", staff, err)
	}
	missing, err := merchShopLogic().CurrentShop(authctx.With(context.Background(), modulesession.Claims{
		Subject: "owner-1", PrincipalType: principal.TypeMerchantOwner, OrganizationID: 9, MerchantID: 7, ShopID: 999,
	}))
	if err == nil || missing.Shop.ShopID != 0 {
		t.Fatalf("missing=%+v err=%v", missing, err)
	}
}

func TestShopWritesAreOwnerOnly(t *testing.T) {
	logic := merchShopLogic()
	if _, err := logic.CreateShop(merchStaffContext(), appmodel.CreateShop{CommandKey: "shop-create-0001", Name: "二店", Subdomain: "second-shop"}); !errors.Is(err, model.ErrProtectedOwner) {
		t.Fatalf("create error=%v", err)
	}
	created, err := logic.CreateShop(merchOwnerContext(), appmodel.CreateShop{CommandKey: "shop-create-0001", Name: "二店", Subdomain: "second-shop"})
	if err != nil || created.Shop.ShopID != 3002 || created.Shop.MerchantID != 7 {
		t.Fatalf("created=%+v err=%v", created, err)
	}
}
