package model

import "testing"

func TestRequestRequiresDestinationAndShop(t *testing.T) {
	if _, err := (RequestCommand{ShopCode: "local-shop"}).Normalize(); err != ErrInvalid {
		t.Fatalf("err=%v", err)
	}
	got, err := (RequestCommand{ShopCode: " local-shop ", Phone: "13800000000"}).Normalize()
	if err != nil || got.ShopCode != "local-shop" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestHashCodeIsStableAndNotPlaintext(t *testing.T) {
	hash := HashCode("abc", "123456")
	if hash == "123456" || len(hash) != 64 || HashCode("abc", "123456") != hash {
		t.Fatalf("hash=%s", hash)
	}
	if Delivered(nil) || Delivered([]Delivery{{Status: "FAILED_PERMANENT"}}) {
		t.Fatal("expected no successful delivery")
	}
	if !Delivered([]Delivery{{Status: "FAILED_PERMANENT"}, {Status: StatusSent}}) {
		t.Fatal("expected SENT to win")
	}
	if DeliveryKey("c1") != EventKey+":c1" {
		t.Fatalf("deliveryKey=%s", DeliveryKey("c1"))
	}
}
