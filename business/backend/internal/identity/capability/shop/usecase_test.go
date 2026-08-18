package shop

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type stubRepository struct {
	values []model.Shop
	page   model.Page
}

func (s stubRepository) ListShops(context.Context, int64) ([]model.Shop, error) {
	return s.values, nil
}
func (s stubRepository) ListShopsByMerchant(ctx context.Context, merchantID int64) ([]model.Shop, error) {
	if merchantID <= 0 {
		return nil, model.ErrInvalidMerchantID
	}
	return s.ListShops(ctx, merchantID)
}
func (s stubRepository) ListManagedShops(context.Context, model.Query) (model.Page, error) {
	return s.page, nil
}
func (s stubRepository) GetManagedShop(_ context.Context, merchantID, shopID int64) (model.Shop, error) {
	for _, value := range s.values {
		if value.MerchantID == merchantID && value.ID == shopID {
			return value, nil
		}
	}
	return model.Shop{}, model.ErrNotFound
}
func (s stubRepository) GetShopByCode(_ context.Context, code string) (model.Shop, error) {
	for _, value := range s.values {
		if value.Code == code {
			return value, nil
		}
	}
	return model.Shop{}, model.ErrNotFound
}
func (s stubRepository) GetShopBySubdomain(_ context.Context, subdomain string) (model.Shop, error) {
	for _, value := range s.values {
		if value.Subdomain == subdomain {
			return value, nil
		}
	}
	return model.Shop{}, model.ErrNotFound
}
func (stubRepository) CreateShop(context.Context, model.CreateCommand) (model.Shop, bool, error) {
	return model.Shop{}, false, nil
}
func (stubRepository) UpdateShop(context.Context, model.UpdateCommand) (model.Shop, bool, error) {
	return model.Shop{}, false, nil
}
func (stubRepository) SetShopEnabled(context.Context, model.SetEnabledCommand) (model.Shop, bool, error) {
	return model.Shop{}, false, nil
}
func (stubRepository) CloseShop(context.Context, model.CloseCommand) (model.Shop, bool, error) {
	return model.Shop{}, false, nil
}

func TestDirectoryRequiresRealMerchantID(t *testing.T) {
	directory := NewDirectory(stubRepository{})
	if _, err := directory.ListByMerchant(context.Background(), 0); !errors.Is(err, model.ErrInvalidMerchantID) {
		t.Fatalf("error=%v", err)
	}
}

func TestDirectoryListsAllShopsWhenMerchantIDIsEmpty(t *testing.T) {
	directory := NewDirectory(stubRepository{values: []model.Shop{{
		ID: 1, MerchantID: 8, Code: "shop", Name: "Shop", Currency: "CNY", Status: model.StatusActive, Version: 1,
	}}})
	values, err := directory.List(context.Background(), 0)
	if err != nil || len(values) != 1 || values[0].MerchantID != 8 {
		t.Fatalf("values=%v err=%v", values, err)
	}
}

func TestDirectoryRejectsCrossMerchantLeak(t *testing.T) {
	directory := NewDirectory(stubRepository{values: []model.Shop{{
		ID: 1, MerchantID: 8, Code: "shop", Name: "Shop", Currency: "CNY", Status: model.StatusActive, Version: 1,
	}}})
	if _, err := directory.ListByMerchant(context.Background(), 7); !errors.Is(err, model.ErrInvalidShop) {
		t.Fatalf("error=%v", err)
	}
}

func TestCreateCommandNormalizesAndRejectsInvalidSubdomain(t *testing.T) {
	command, err := model.CreateCommand{CommandKey: "shop-create-0001", MerchantID: 7, Name: " 二店 ", Subdomain: "Second-Shop", Currency: "cny"}.Normalize()
	if err != nil || command.Subdomain != "second-shop" || command.Currency != "CNY" || command.DefaultLocale != model.DefaultLocale || command.Status != model.StatusActive {
		t.Fatalf("command=%+v err=%v", command, err)
	}
	if _, err := (model.CreateCommand{CommandKey: "shop-create-0001", MerchantID: 7, Name: "二店", Subdomain: "Bad_Domain"}).Normalize(); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func TestManagedListRejectsClosedStatusFilter(t *testing.T) {
	directory := NewDirectory(stubRepository{})
	if _, err := directory.ListManaged(context.Background(), model.Query{MerchantID: 7, Status: model.StatusClosed}); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func TestGetManagedRequiresMerchantAndShop(t *testing.T) {
	directory := NewDirectory(stubRepository{values: []model.Shop{{
		ID: 101, MerchantID: 7, Code: "shop", Name: "Shop", Currency: "CNY", Status: model.StatusActive, Version: 1,
	}}})
	if _, err := directory.GetManaged(context.Background(), 0, 101); !errors.Is(err, model.ErrInvalidMerchantID) {
		t.Fatalf("merchant error=%v", err)
	}
	if _, err := directory.GetManaged(context.Background(), 7, 0); !errors.Is(err, model.ErrInvalidShop) {
		t.Fatalf("shop error=%v", err)
	}
	value, err := directory.GetManaged(context.Background(), 7, 101)
	if err != nil || value.ID != 101 || value.MerchantID != 7 {
		t.Fatalf("value=%+v err=%v", value, err)
	}
	if _, err := directory.GetManaged(context.Background(), 7, 999); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("missing error=%v", err)
	}
}

func TestGetBySlugPrefersCodeThenSubdomain(t *testing.T) {
	directory := NewDirectory(stubRepository{values: []model.Shop{
		{ID: 1, MerchantID: 7, Code: "acme", Subdomain: "other", Name: "Code Shop", Currency: "CNY", Status: model.StatusActive, Version: 1},
		{ID: 2, MerchantID: 8, Code: "other", Subdomain: "brand", Name: "Sub Shop", Currency: "CNY", Status: model.StatusActive, Version: 1},
	}})
	byCode, err := directory.GetBySlug(context.Background(), "acme")
	if err != nil || byCode.ID != 1 {
		t.Fatalf("code=%+v err=%v", byCode, err)
	}
	bySub, err := directory.GetBySlug(context.Background(), "brand")
	if err != nil || bySub.ID != 2 {
		t.Fatalf("subdomain=%+v err=%v", bySub, err)
	}
	if _, err := directory.GetBySlug(context.Background(), "missing"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("missing error=%v", err)
	}
}
