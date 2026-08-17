package shop

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
)

type stubPolicyRepository struct {
	saved   model.SavePolicyCommand
	query   model.PolicyQuery
	publish model.PublishPolicyCommand
}

func (s *stubPolicyRepository) ListPolicies(_ context.Context, query model.PolicyQuery) (model.PolicyPage, error) {
	s.query = query
	return model.PolicyPage{Items: []model.Policy{{
		ID: 1, MerchantID: query.MerchantID, ShopID: query.ShopID, PolicyType: model.PolicyPrivacy,
		Title: "隐私政策", Content: "这是一份足够长的店铺隐私政策正文。", VersionNo: 1,
		Status: model.PolicyDraft, Version: 1,
	}}, Page: query.Page, PageSize: query.PageSize, Total: 1}, nil
}
func (s *stubPolicyRepository) SavePolicy(_ context.Context, command model.SavePolicyCommand) (model.Policy, bool, error) {
	s.saved = command
	return model.Policy{
		ID: 1, MerchantID: command.MerchantID, ShopID: command.ShopID, PolicyType: command.PolicyType,
		Title: command.Title, Content: command.Content, VersionNo: 1, Status: model.PolicyDraft, Version: 1,
	}, false, nil
}
func (s *stubPolicyRepository) PublishPolicy(_ context.Context, command model.PublishPolicyCommand) (model.Policy, bool, error) {
	s.publish = command
	return model.Policy{ID: command.PolicyID, MerchantID: command.MerchantID, ShopID: command.ShopID, Version: command.ExpectedVersion + 1}, false, nil
}

func TestPolicySaveNormalizesTypeAndRejectsShortContent(t *testing.T) {
	repository := &stubPolicyRepository{}
	service := NewPolicies(repository)
	_, _, err := service.Save(context.Background(), model.SavePolicyCommand{
		CommandKey: "policy-create-1", MerchantID: 2001, ShopID: 3001,
		PolicyType: " PRIVACY ", Title: " 隐私政策 ", Content: " 这是一份足够长的店铺隐私政策正文。 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.saved.PolicyType != model.PolicyPrivacy || repository.saved.Title != "隐私政策" {
		t.Fatalf("normalized command=%+v", repository.saved)
	}
	_, _, err = service.Save(context.Background(), model.SavePolicyCommand{
		CommandKey: "policy-create-2", MerchantID: 2001, ShopID: 3001,
		PolicyType: model.PolicyTerms, Title: "条款", Content: "太短",
	})
	if !errors.Is(err, model.ErrPolicyInvalid) {
		t.Fatalf("short content error=%v", err)
	}
}

func TestPolicyListRequiresShopScopeAndRejectsUnknownType(t *testing.T) {
	service := NewPolicies(&stubPolicyRepository{})
	if _, err := service.List(context.Background(), model.PolicyQuery{MerchantID: 2001}); !errors.Is(err, model.ErrPolicyInvalid) {
		t.Fatalf("missing shop error=%v", err)
	}
	if _, err := service.List(context.Background(), model.PolicyQuery{MerchantID: 2001, ShopID: 3001, PolicyType: "cookie"}); !errors.Is(err, model.ErrPolicyInvalid) {
		t.Fatalf("unknown type error=%v", err)
	}
}

func TestPolicyCommandDigestSeparatesMutations(t *testing.T) {
	base := model.SavePolicyCommand{CommandKey: "policy-save-1", MerchantID: 2001, ShopID: 3001, PolicyType: model.PolicyRefund, Title: "退款", Content: "这是一份足够长的退款政策正文。"}
	published := base
	published.Publish = true
	if base.RequestDigest() == published.RequestDigest() {
		t.Fatal("draft and publish-on-save commands must not share a digest")
	}
	publish := model.PublishPolicyCommand{PolicyID: 9, MerchantID: 2001, ShopID: 3001, CommandKey: base.CommandKey, ExpectedVersion: 1}
	if base.RequestDigest() == publish.RequestDigest() {
		t.Fatal("save and publish commands must not share a digest")
	}
}
