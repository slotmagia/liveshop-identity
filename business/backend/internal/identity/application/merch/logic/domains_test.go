package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/compose"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance"
	governancemodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type stubDomainRepo struct {
	items        []shopmodel.Domain
	query        shopmodel.DomainQuery
	created      shopmodel.CreateDomainCommand
	createCalled bool
	testCalled   bool
	activate     shopmodel.DomainWriteCommand
	deleted      shopmodel.DomainWriteCommand
}

func sampleLiveDomain() shopmodel.Domain {
	host := "live.example.com"
	return shopmodel.Domain{
		ID: 31, MerchantID: 2001, ShopID: 3001, Host: host, Scene: shopmodel.DomainSceneLive,
		Status: shopmodel.DomainPending, TxtName: shopmodel.ChallengeName(host), TxtValue: "token-1",
		Version: 1, CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}
}

func (s *stubDomainRepo) ListDomains(_ context.Context, query shopmodel.DomainQuery) (shopmodel.DomainPage, error) {
	s.query = query
	items := make([]shopmodel.Domain, 0, len(s.items))
	for _, item := range s.items {
		if query.Scene == "" || item.Scene == query.Scene {
			items = append(items, item)
		}
	}
	return shopmodel.DomainPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: int64(len(items))}, nil
}
func (s *stubDomainRepo) GetDomain(context.Context, int64, int64, int64) (shopmodel.Domain, error) {
	return sampleLiveDomain(), nil
}
func (s *stubDomainRepo) GetDomainByHost(_ context.Context, host string) (shopmodel.Domain, error) {
	value := sampleLiveDomain()
	if host != "" && host != value.Host {
		return shopmodel.Domain{}, shopmodel.ErrDomainNotFound
	}
	return value, nil
}
func (s *stubDomainRepo) CreateDomain(_ context.Context, command shopmodel.CreateDomainCommand) (shopmodel.Domain, bool, error) {
	s.createCalled = true
	s.created = command
	value := sampleLiveDomain()
	value.Host = command.Host
	value.Scene = command.Scene
	value.TxtName = shopmodel.ChallengeName(command.Host)
	return value, false, nil
}
func (s *stubDomainRepo) TestDomain(context.Context, shopmodel.DomainWriteCommand, bool) (shopmodel.Domain, bool, error) {
	s.testCalled = true
	value := sampleLiveDomain()
	value.Status = shopmodel.DomainVerified
	value.Version = 2
	return value, false, nil
}
func (s *stubDomainRepo) ActivateDomain(_ context.Context, command shopmodel.DomainWriteCommand) (shopmodel.Domain, bool, error) {
	s.activate = command
	value := sampleLiveDomain()
	value.Status = shopmodel.DomainVerified
	value.IsPrimary = true
	value.Version = command.ExpectedVersion + 1
	return value, false, nil
}
func (s *stubDomainRepo) DeleteDomain(_ context.Context, command shopmodel.DomainWriteCommand) (shopmodel.Domain, bool, error) {
	s.deleted = command
	value := sampleLiveDomain()
	value.Status = shopmodel.DomainDeleted
	value.Version = command.ExpectedVersion + 1
	return value, false, nil
}

func merchDomainLogic(items []governancemodel.Capability) (*Logic, *stubDomainRepo) {
	repo := &stubDomainRepo{items: []shopmodel.Domain{sampleLiveDomain()}}
	lookup := func(context.Context, string) ([]string, error) { return []string{"token-1"}, nil }
	return New(nil, nil, nil, nil, shop.NewDirectory(stubShopRepo{}), nil, nil, nil, merchant_governance.NewCapabilities(stubGovernanceRepo{items: items}), Subscription{}, nil, nil, nil, nil, nil, shop.NewCustomDomains(repo, lookup, "edge.liveshop.example"), nil, nil, nil), repo
}

func TestDomainShopsRequiresMerchantContext(t *testing.T) {
	logic, _ := merchDomainLogic(nil)
	if _, err := logic.DomainShops(context.Background()); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestDomainsRejectsForeignShopWhenSessionIsBound(t *testing.T) {
	logic, _ := merchDomainLogic(nil)
	if _, err := logic.Domains(merchPolicyContext(), appmodel.DomainQuery{ShopID: 4001}); !errors.Is(err, model.ErrInvalidContext) {
		t.Fatalf("error=%v", err)
	}
}

func TestDomainsAttachesSyntheticActiveOverlayAndCNAME(t *testing.T) {
	logic, _ := merchDomainLogic(nil)
	page, err := logic.Domains(merchPolicyContext(), appmodel.DomainQuery{ShopID: 3001, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].PlatformStatus != "active" || page.Items[0].Scene != "LIVE" || page.CnameTarget != "edge.liveshop.example" {
		t.Fatalf("page=%+v", page)
	}
}

func TestCreateTestActivateAreDeniedWhenOverlayRestricts(t *testing.T) {
	logic, repo := merchDomainLogic([]governancemodel.Capability{{
		ID: 8, MerchantID: 2001, ShopID: 3001, Module: "domains", PlatformStatus: governancemodel.PlatformRestricted,
		PlatformReasonPublic: "平台限制域名", Version: 1,
	}})
	_, err := logic.CreateDomain(merchPolicyContext(), appmodel.CreateDomain{
		CommandKey: "domain-create-0001", ShopID: 3001, Host: "live.example.com",
	})
	if !errors.Is(err, shopmodel.ErrDomainRestricted) || repo.createCalled {
		t.Fatalf("create error=%v called=%v", err, repo.createCalled)
	}
	_, err = logic.TestDomain(merchPolicyContext(), appmodel.DomainWrite{
		DomainID: 31, ShopID: 3001, CommandKey: "domain-test-0001", ExpectedVersion: 1,
	})
	if !errors.Is(err, shopmodel.ErrDomainRestricted) || repo.testCalled {
		t.Fatalf("test error=%v called=%v", err, repo.testCalled)
	}
	_, err = logic.ActivateDomain(merchPolicyContext(), appmodel.DomainWrite{
		DomainID: 31, ShopID: 3001, CommandKey: "domain-activate-0001", ExpectedVersion: 1,
	})
	if !errors.Is(err, shopmodel.ErrDomainRestricted) {
		t.Fatalf("activate error=%v", err)
	}
}

func TestDeleteDomainRemainsAllowedWhenOverlayRestricts(t *testing.T) {
	logic, repo := merchDomainLogic([]governancemodel.Capability{{
		ID: 8, MerchantID: 2001, ShopID: 3001, Module: "domains", PlatformStatus: governancemodel.PlatformRestricted,
		PlatformReasonPublic: "平台限制域名", Version: 1,
	}})
	result, err := logic.DeleteDomain(merchPolicyContext(), appmodel.DomainWrite{
		DomainID: 31, ShopID: 3001, CommandKey: "domain-delete-0001", ExpectedVersion: 1,
	})
	if err != nil || result.Domain.Status != "DELETED" || result.Domain.PlatformStatus != "restricted" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if repo.deleted.DomainID != 31 {
		t.Fatalf("delete command=%+v", repo.deleted)
	}
}

func TestDomainsDefaultsSceneToLive(t *testing.T) {
	logic, repo := merchDomainLogic(nil)
	page, err := logic.Domains(merchPolicyContext(), appmodel.DomainQuery{ShopID: 3001, Page: 1, PageSize: 20})
	if err != nil || page.Total != 1 || repo.query.Scene != shopmodel.DomainSceneLive {
		t.Fatalf("page=%+v query=%+v err=%v", page, repo.query, err)
	}
}

func TestDomainsPassesShopScene(t *testing.T) {
	logic, repo := merchDomainLogic(nil)
	page, err := logic.Domains(merchPolicyContext(), appmodel.DomainQuery{ShopID: 3001, Scene: "shop", Page: 1, PageSize: 20})
	if err != nil || page.Total != 0 || repo.query.Scene != shopmodel.DomainSceneShop {
		t.Fatalf("page=%+v query=%+v err=%v", page, repo.query, err)
	}
}

func TestDomainsRejectsInvalidScene(t *testing.T) {
	logic, _ := merchDomainLogic(nil)
	if _, err := logic.Domains(merchPolicyContext(), appmodel.DomainQuery{ShopID: 3001, Scene: "ROOM"}); !errors.Is(err, shopmodel.ErrDomainInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func TestCreateDomainPassesShopScene(t *testing.T) {
	logic, repo := merchDomainLogic(nil)
	result, err := logic.CreateDomain(merchPolicyContext(), appmodel.CreateDomain{
		CommandKey: "domain-create-shop-1", ShopID: 3001, Host: "shop.example.com", Scene: "SHOP",
	})
	if err != nil || result.Domain.Scene != "SHOP" || repo.created.Scene != shopmodel.DomainSceneShop {
		t.Fatalf("result=%+v created=%+v err=%v", result, repo.created, err)
	}
}

func TestCreateDomainRejectsInvalidScene(t *testing.T) {
	logic, repo := merchDomainLogic(nil)
	_, err := logic.CreateDomain(merchPolicyContext(), appmodel.CreateDomain{
		CommandKey: "domain-create-bad-1", ShopID: 3001, Host: "shop.example.com", Scene: "ROOM",
	})
	if !errors.Is(err, shopmodel.ErrDomainInvalid) || repo.createCalled {
		t.Fatalf("error=%v called=%v", err, repo.createCalled)
	}
}

func TestDomainWriteShopSceneMismatchIsNotFound(t *testing.T) {
	logic, _ := merchDomainLogic(nil)
	_, err := logic.TestDomain(merchPolicyContext(), appmodel.DomainWrite{
		DomainID: 31, ShopID: 3001, CommandKey: "domain-test-shop-1", ExpectedVersion: 1, Scene: "SHOP",
	})
	if !errors.Is(err, shopmodel.ErrDomainNotFound) {
		t.Fatalf("error=%v", err)
	}
}

func TestDeleteDomainForwardsShopScene(t *testing.T) {
	logic, repo := merchDomainLogic(nil)
	_, err := logic.DeleteDomain(merchPolicyContext(), appmodel.DomainWrite{
		DomainID: 31, ShopID: 3001, CommandKey: "domain-delete-shop-1", ExpectedVersion: 1, Scene: "SHOP",
	})
	if err != nil || repo.deleted.Scene != shopmodel.DomainSceneShop {
		t.Fatalf("deleted=%+v err=%v", repo.deleted, err)
	}
}

func TestCreateShopSceneDeniedWhenOverlayRestricts(t *testing.T) {
	logic, repo := merchDomainLogic([]governancemodel.Capability{{
		ID: 8, MerchantID: 2001, ShopID: 3001, Module: "domains", PlatformStatus: governancemodel.PlatformRestricted,
		PlatformReasonPublic: "平台限制域名", Version: 1,
	}})
	_, err := logic.CreateDomain(merchPolicyContext(), appmodel.CreateDomain{
		CommandKey: "domain-create-shop-2", ShopID: 3001, Host: "shop.example.com", Scene: "SHOP",
	})
	if !errors.Is(err, shopmodel.ErrDomainRestricted) || repo.createCalled {
		t.Fatalf("create error=%v called=%v", err, repo.createCalled)
	}
}

type stubEdgeGrants struct{ compose.Unavailable }

func (stubEdgeGrants) EdgeSnapshot(context.Context) (compose.EdgeSnapshot, error) {
	return compose.EdgeSnapshot{CNAMETarget: "from-platform.example", ReservedHosts: []string{"shop.wopays.com"}}, nil
}

func TestDomainsPrefersPlatformCNAMESnapshot(t *testing.T) {
	logic, _ := merchDomainLogic(nil)
	logic.UseGrants(stubEdgeGrants{})
	page, err := logic.Domains(merchPolicyContext(), appmodel.DomainQuery{ShopID: 3001, Page: 1, PageSize: 20})
	if err != nil || page.CnameTarget != "from-platform.example" || page.Items[0].CnameTarget != "from-platform.example" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}

func TestCreateDomainRejectsPlatformReservedHost(t *testing.T) {
	logic, repo := merchDomainLogic(nil)
	logic.UseGrants(stubEdgeGrants{})
	_, err := logic.CreateDomain(merchPolicyContext(), appmodel.CreateDomain{
		CommandKey: "domain-create-reserved-1", ShopID: 3001, Host: "shop.wopays.com",
	})
	if !errors.Is(err, shopmodel.ErrDomainInvalid) || repo.createCalled {
		t.Fatalf("error=%v called=%v", err, repo.createCalled)
	}
}
