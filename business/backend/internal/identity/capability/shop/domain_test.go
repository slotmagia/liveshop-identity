package shop

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type stubDomainRepository struct {
	created model.CreateDomainCommand
	tested  model.DomainWriteCommand
	query   model.DomainQuery
}

func sampleDomain(query model.DomainQuery) model.Domain {
	host := "live.example.com"
	return model.Domain{
		ID: 31, MerchantID: query.MerchantID, ShopID: query.ShopID, Host: host, Scene: model.DomainSceneLive,
		Status: model.DomainPending, TxtName: model.ChallengeName(host), TxtValue: "abc123", Version: 1,
	}
}

func (s *stubDomainRepository) ListDomains(_ context.Context, query model.DomainQuery) (model.DomainPage, error) {
	s.query = query
	return model.DomainPage{Items: []model.Domain{sampleDomain(query)}, Page: query.Page, PageSize: query.PageSize, Total: 1}, nil
}
func (s *stubDomainRepository) GetDomain(_ context.Context, id, merchantID, shopID int64) (model.Domain, error) {
	return model.Domain{
		ID: id, MerchantID: merchantID, ShopID: shopID, Host: "live.example.com", Scene: model.DomainSceneLive,
		Status: model.DomainPending, TxtName: model.ChallengeName("live.example.com"), TxtValue: "token-1", Version: 1,
	}, nil
}
func (s *stubDomainRepository) GetDomainByHost(_ context.Context, host string) (model.Domain, error) {
	return model.Domain{
		ID: 31, MerchantID: 2001, ShopID: 3001, Host: host, Scene: model.DomainSceneLive,
		Status: model.DomainVerified, TxtName: model.ChallengeName(host), TxtValue: "token-1", Version: 1,
	}, nil
}
func (s *stubDomainRepository) CreateDomain(_ context.Context, command model.CreateDomainCommand) (model.Domain, bool, error) {
	s.created = command
	return model.Domain{
		ID: 31, MerchantID: command.MerchantID, ShopID: command.ShopID, Host: command.Host, Scene: command.Scene,
		Status: model.DomainPending, TxtName: model.ChallengeName(command.Host), TxtValue: "token-1", Version: 1,
	}, false, nil
}
func (s *stubDomainRepository) TestDomain(_ context.Context, command model.DomainWriteCommand, matched bool) (model.Domain, bool, error) {
	s.tested = command
	status := model.DomainFailed
	if matched {
		status = model.DomainVerified
	}
	return model.Domain{
		ID: command.DomainID, MerchantID: command.MerchantID, ShopID: command.ShopID, Host: "live.example.com",
		Scene: model.DomainSceneLive, Status: status, TxtName: model.ChallengeName("live.example.com"), TxtValue: "token-1", Version: 2,
	}, false, nil
}
func (stubDomainRepository) ActivateDomain(context.Context, model.DomainWriteCommand) (model.Domain, bool, error) {
	return model.Domain{}, false, nil
}
func (stubDomainRepository) DeleteDomain(context.Context, model.DomainWriteCommand) (model.Domain, bool, error) {
	return model.Domain{}, false, nil
}

func TestCustomDomainCreateNormalizesHostAndFixesScene(t *testing.T) {
	repository := &stubDomainRepository{}
	service := NewCustomDomains(repository, nil, "")
	_, _, err := service.Create(context.Background(), model.CreateDomainCommand{
		CommandKey: "domain-create-1", MerchantID: 2001, ShopID: 3001, Host: " Live.Example.COM ", Scene: model.DomainSceneLive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.created.Host != "live.example.com" || repository.created.Scene != model.DomainSceneLive {
		t.Fatalf("normalized command=%+v", repository.created)
	}
}

func TestCustomDomainCreateRejectsSchemeAndPath(t *testing.T) {
	service := NewCustomDomains(&stubDomainRepository{}, nil, "")
	if _, _, err := service.Create(context.Background(), model.CreateDomainCommand{
		CommandKey: "domain-create-2", MerchantID: 2001, ShopID: 3001, Host: "https://live.example.com", Scene: model.DomainSceneLive,
	}); !errors.Is(err, model.ErrDomainInvalid) {
		t.Fatalf("scheme error=%v", err)
	}
	if _, _, err := service.Create(context.Background(), model.CreateDomainCommand{
		CommandKey: "domain-create-3", MerchantID: 2001, ShopID: 3001, Host: "live.example.com/path", Scene: model.DomainSceneLive,
	}); !errors.Is(err, model.ErrDomainInvalid) {
		t.Fatalf("path error=%v", err)
	}
}

func TestCustomDomainListRequiresShopScope(t *testing.T) {
	service := NewCustomDomains(&stubDomainRepository{}, nil, "")
	if _, err := service.List(context.Background(), model.DomainQuery{MerchantID: 2001, Scene: model.DomainSceneLive}); !errors.Is(err, model.ErrDomainInvalid) {
		t.Fatalf("missing shop error=%v", err)
	}
}

func TestCustomDomainTestFailsClosedWithoutResolver(t *testing.T) {
	service := NewCustomDomains(&stubDomainRepository{}, nil, "")
	if _, _, err := service.Test(context.Background(), model.DomainWriteCommand{
		DomainID: 31, MerchantID: 2001, ShopID: 3001, CommandKey: "domain-test-1", ExpectedVersion: 1,
	}); !errors.Is(err, model.ErrDomainUnavailable) {
		t.Fatalf("missing resolver error=%v", err)
	}
}

func TestCustomDomainTestMatchesTrimmedTXT(t *testing.T) {
	repository := &stubDomainRepository{}
	service := NewCustomDomains(repository, func(context.Context, string) ([]string, error) {
		return []string{`"token-1"`}, nil
	}, "cname.liveshop.example")
	value, _, err := service.Test(context.Background(), model.DomainWriteCommand{
		DomainID: 31, MerchantID: 2001, ShopID: 3001, CommandKey: "domain-test-2", ExpectedVersion: 1,
	})
	if err != nil || value.Status != model.DomainVerified {
		t.Fatalf("value=%+v err=%v", value, err)
	}
}

func TestDomainWriteDigestsSeparateMutations(t *testing.T) {
	base := model.DomainWriteCommand{DomainID: 9, MerchantID: 2001, ShopID: 3001, CommandKey: "domain-cmd-1", ExpectedVersion: 1}
	if base.RequestDigest("TEST") == base.RequestDigest("ACTIVATE") || base.RequestDigest("ACTIVATE") == base.RequestDigest("DELETE") {
		t.Fatal("test, activate and delete commands must not share a digest")
	}
}

func TestCustomDomainTestRejectsSceneMismatch(t *testing.T) {
	service := NewCustomDomains(&stubDomainRepository{}, func(context.Context, string) ([]string, error) {
		return []string{"token-1"}, nil
	}, "")
	if _, _, err := service.Test(context.Background(), model.DomainWriteCommand{
		DomainID: 31, MerchantID: 2001, ShopID: 3001, CommandKey: "domain-test-shop-1", ExpectedVersion: 1, Scene: model.DomainSceneShop,
	}); !errors.Is(err, model.ErrDomainNotFound) {
		t.Fatalf("scene mismatch error=%v", err)
	}
}

func TestDefaultDomainScene(t *testing.T) {
	live, err := model.DefaultDomainScene("")
	if err != nil || live != model.DomainSceneLive {
		t.Fatalf("empty scene=%q err=%v", live, err)
	}
	shop, err := model.DefaultDomainScene("shop")
	if err != nil || shop != model.DomainSceneShop {
		t.Fatalf("shop scene=%q err=%v", shop, err)
	}
	if _, err := model.DefaultDomainScene("ROOM"); !errors.Is(err, model.ErrDomainInvalid) {
		t.Fatalf("invalid scene error=%v", err)
	}
}

func TestCustomDomainGetByHostNormalizes(t *testing.T) {
	service := NewCustomDomains(&stubDomainRepository{}, nil, "")
	value, err := service.GetByHost(context.Background(), " Live.Brand.COM ")
	if err != nil || value.Host != "live.brand.com" {
		t.Fatalf("value=%+v err=%v", value, err)
	}
}
