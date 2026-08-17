package shop

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type stubAppRepository struct {
	created model.CreateAppCommand
	query   model.AppQuery
}

func (s *stubAppRepository) ListApps(_ context.Context, query model.AppQuery) (model.AppPage, error) {
	s.query = query
	return model.AppPage{Items: []model.App{{
		ID: 1, MerchantID: query.MerchantID, ShopID: query.ShopID, Name: "订单同步",
		ClientID: "app_test", SecretHint: "abcdef", Scopes: "orders:read",
		Status: model.AppActive, Version: 1,
	}}, Page: query.Page, PageSize: query.PageSize, Total: 1}, nil
}
func (s *stubAppRepository) CreateApp(_ context.Context, command model.CreateAppCommand) (model.AppMutation, bool, error) {
	s.created = command
	return model.AppMutation{App: model.App{
		ID: 1, MerchantID: command.MerchantID, ShopID: command.ShopID, Name: command.Name,
		ClientID: "app_test", SecretHint: "abcdef", Scopes: command.Scopes, Status: model.AppActive, Version: 1,
	}, ClientSecret: "sec_test"}, false, nil
}
func (stubAppRepository) ResetAppSecret(context.Context, model.ResetAppSecretCommand) (model.AppMutation, bool, error) {
	return model.AppMutation{}, false, nil
}
func (stubAppRepository) SetAppEnabled(context.Context, model.SetAppEnabledCommand) (model.App, bool, error) {
	return model.App{}, false, nil
}

func TestPrivateAppCreateNormalizesScopesAndRejectsUnknown(t *testing.T) {
	repository := &stubAppRepository{}
	service := NewPrivateApps(repository)
	_, _, err := service.Create(context.Background(), model.CreateAppCommand{
		CommandKey: "app-create-1", MerchantID: 2001, ShopID: 3001,
		Name: " 订单同步 ", Scopes: " ORDERS:READ , products:read , orders:read ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.created.Name != "订单同步" || repository.created.Scopes != "orders:read,products:read" {
		t.Fatalf("normalized command=%+v", repository.created)
	}
	_, _, err = service.Create(context.Background(), model.CreateAppCommand{
		CommandKey: "app-create-2", MerchantID: 2001, ShopID: 3001, Name: "订单同步", Scopes: "orders:read,unknown:write",
	})
	if !errors.Is(err, model.ErrAppInvalid) {
		t.Fatalf("unknown scope error=%v", err)
	}
}

func TestPrivateAppListRequiresShopScopeAndRejectsUnknownStatus(t *testing.T) {
	service := NewPrivateApps(&stubAppRepository{})
	if _, err := service.List(context.Background(), model.AppQuery{MerchantID: 2001}); !errors.Is(err, model.ErrAppInvalid) {
		t.Fatalf("missing shop error=%v", err)
	}
	if _, err := service.List(context.Background(), model.AppQuery{MerchantID: 2001, ShopID: 3001, Status: "paused"}); !errors.Is(err, model.ErrAppInvalid) {
		t.Fatalf("unknown status error=%v", err)
	}
}

func TestPrivateAppCommandDigestSeparatesMutations(t *testing.T) {
	create := model.CreateAppCommand{CommandKey: "app-cmd-1", MerchantID: 2001, ShopID: 3001, Name: "订单同步", Scopes: "orders:read"}
	reset := model.ResetAppSecretCommand{AppID: 9, MerchantID: 2001, ShopID: 3001, CommandKey: create.CommandKey, ExpectedVersion: 1}
	enable := model.SetAppEnabledCommand{AppID: 9, MerchantID: 2001, ShopID: 3001, CommandKey: create.CommandKey, ExpectedVersion: 1, Enabled: true}
	disable := enable
	disable.Enabled = false
	if create.RequestDigest() == reset.RequestDigest() || reset.RequestDigest() == enable.RequestDigest() || enable.RequestDigest() == disable.RequestDigest() {
		t.Fatal("create, reset, enable and disable commands must not share a digest")
	}
}

func TestNormalizeAppScopesAllowsEmptyAndDeduplicates(t *testing.T) {
	value, err := model.NormalizeAppScopes("")
	if err != nil || value != "" {
		t.Fatalf("empty scopes=%q err=%v", value, err)
	}
	value, err = model.NormalizeAppScopes("live:read, live:read")
	if err != nil || value != "live:read" {
		t.Fatalf("deduped scopes=%q err=%v", value, err)
	}
}
