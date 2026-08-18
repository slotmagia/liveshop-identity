package model

import "testing"

func TestAftersaleQueryRejectsMissingShop(t *testing.T) {
	if _, err := (AftersaleQuery{MerchantID: 2001, Page: 1, PageSize: 20}).Normalize(); err != ErrAftersaleInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestAftersaleQueryDefaultsPage(t *testing.T) {
	query, err := (AftersaleQuery{MerchantID: 2001, ShopID: 3001, Status: "pending", Type: "return_refund"}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if query.Page != 1 || query.PageSize != 20 || query.Status != AftersalePending || query.Type != AftersaleReturnRefund {
		t.Fatalf("query=%+v", query)
	}
}

func TestReviewAftersaleRejectsPendingStatus(t *testing.T) {
	if _, err := (ReviewAftersaleCommand{
		AftersaleID: 11, MerchantID: 2001, ShopID: 3001, CommandKey: "review-01", ExpectedVersion: 1,
		Status: AftersalePending, HandleNote: "说明足够长",
	}).Normalize(); err != ErrAftersaleInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestReceiveAftersaleRequiresCommandKey(t *testing.T) {
	if _, err := (ReceiveAftersaleCommand{AftersaleID: 11, MerchantID: 2001, ShopID: 3001, ExpectedVersion: 1}).Normalize(); err != ErrAftersaleInvalid {
		t.Fatalf("error=%v", err)
	}
}
