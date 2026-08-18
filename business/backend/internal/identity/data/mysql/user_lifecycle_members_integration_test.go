//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

func TestListMembersPagesStaffAndHidesOwnerAndRevoked(t *testing.T) {
	database := integrationDatabase(t)
	seedAuthorizationFixture(t, database)
	if _, err := database.Exec(`INSERT INTO identity_shop(shop_id,merchant_id,app_id,commercial_id,code,name,status,version) VALUES(101,7,1,101,'shop-seven-101','Shop 101','ACTIVE',1),(102,7,1,102,'shop-seven-102','Shop 102','ACTIVE',1),(201,8,1,201,'shop-eight-201','Shop 201','ACTIVE',1)`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO identity_subject(subject,realm,principal_type,display_name,status,version) VALUES('owner-seven','MERCHANT','MERCHANT_OWNER','Owner Seven','ACTIVE',1)`,
		`INSERT INTO identity_workforce_member(organization_id,merchant_id,subject,member_type,status,access_version) VALUES(9,7,'owner-seven','OWNER','ACTIVE',1)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	directory, err := NewDirectoryRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	users, err := NewUserLifecycleRepository(database, directory, 2*time.Minute, 3*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service := biz.NewDirectory(directory)
	lifecycle := biz.NewUserLifecycle(users)
	staff, err := service.ProvisionMember(context.Background(), biz.ProvisionMember{
		OperationID: "list-staff", IdempotencyKey: "list-staff", Subject: "staff-seven",
		Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff, DisplayName: "Ada Staff",
		OrganizationID: 9, MerchantID: 7, MemberType: model.MemberStaff, AssignmentKind: model.AssignmentOperate,
		ShopIDs: []int64{101}, CredentialKind: "USERNAME", CredentialNamespace: "GLOBAL", NormalizedIdentifier: "ada-staff",
		Password: "password-one", RoleIDs: []int64{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProvisionMember(context.Background(), biz.ProvisionMember{
		OperationID: "list-anchor", IdempotencyKey: "list-anchor", Subject: "anchor-seven",
		Realm: principal.RealmMerchant, PrincipalType: principal.TypeShopAnchor, DisplayName: "Ben Anchor",
		OrganizationID: 9, MerchantID: 7, MemberType: model.MemberAnchor, AssignmentKind: model.AssignmentAnchor,
		ShopIDs: []int64{102}, CredentialKind: "USERNAME", CredentialNamespace: "SHOP", NormalizedIdentifier: "ben-anchor",
		Password: "password-one", RoleIDs: []int64{1},
	}); err != nil {
		t.Fatal(err)
	}
	revoked, err := service.ProvisionMember(context.Background(), biz.ProvisionMember{
		OperationID: "list-revoked", IdempotencyKey: "list-revoked", Subject: "revoked-seven",
		Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff, DisplayName: "Revoked Staff",
		OrganizationID: 9, MerchantID: 7, MemberType: model.MemberStaff, AssignmentKind: model.AssignmentOperate,
		ShopIDs: []int64{101}, CredentialKind: "USERNAME", CredentialNamespace: "GLOBAL", NormalizedIdentifier: "revoked-staff",
		Password: "password-one", RoleIDs: []int64{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE identity_workforce_member SET status='REVOKED' WHERE member_id=?`, revoked.MemberID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProvisionMember(context.Background(), biz.ProvisionMember{
		OperationID: "list-other", IdempotencyKey: "list-other", Subject: "staff-eight",
		Realm: principal.RealmMerchant, PrincipalType: principal.TypeMerchantStaff, DisplayName: "Other Staff",
		OrganizationID: 10, MerchantID: 8, MemberType: model.MemberStaff, AssignmentKind: model.AssignmentOperate,
		ShopIDs: []int64{201}, CredentialKind: "USERNAME", CredentialNamespace: "GLOBAL", NormalizedIdentifier: "other-staff",
		Password: "password-one", RoleIDs: []int64{3},
	}); err != nil {
		t.Fatal(err)
	}

	page, err := lifecycle.ListMembers(context.Background(), biz.MemberQuery{Scope: biz.UserScope{OrganizationID: 9, MerchantID: 7}, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("page=%+v", page)
	}
	subjects := map[string]struct{}{}
	for _, item := range page.Items {
		subjects[item.Subject.ID] = struct{}{}
	}
	if _, ok := subjects["staff-seven"]; !ok {
		t.Fatal("missing staff")
	}
	if _, ok := subjects["anchor-seven"]; !ok {
		t.Fatal("missing anchor")
	}
	if _, ok := subjects["owner-seven"]; ok {
		t.Fatal("owner listed")
	}
	if _, ok := subjects["revoked-seven"]; ok {
		t.Fatal("revoked listed")
	}
	if _, ok := subjects["staff-eight"]; ok {
		t.Fatal("cross-merchant listed")
	}

	keyword, err := lifecycle.ListMembers(context.Background(), biz.MemberQuery{Scope: biz.UserScope{OrganizationID: 9, MerchantID: 7}, Keyword: "Ada", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if keyword.Total != 1 || keyword.Items[0].Subject.ID != "staff-seven" {
		t.Fatalf("keyword=%+v", keyword)
	}
	anchors, err := lifecycle.ListMembers(context.Background(), biz.MemberQuery{Scope: biz.UserScope{OrganizationID: 9, MerchantID: 7}, MemberType: "ANCHOR", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if anchors.Total != 1 || anchors.Items[0].Subject.ID != "anchor-seven" {
		t.Fatalf("anchors=%+v", anchors)
	}
	shop, err := lifecycle.ListMembers(context.Background(), biz.MemberQuery{Scope: biz.UserScope{OrganizationID: 9, MerchantID: 7}, ShopID: 101, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if shop.Total != 1 || shop.Items[0].Member.ID != staff.MemberID {
		t.Fatalf("shop filter=%+v", shop)
	}
	paged, err := lifecycle.ListMembers(context.Background(), biz.MemberQuery{Scope: biz.UserScope{OrganizationID: 9, MerchantID: 7}, Page: 2, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if paged.Total != 2 || paged.Page != 2 || paged.PageSize != 1 || len(paged.Items) != 1 {
		t.Fatalf("paged=%+v", paged)
	}
}
