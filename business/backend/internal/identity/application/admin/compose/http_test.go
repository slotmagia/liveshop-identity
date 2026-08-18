package compose

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/config"
)

func TestHTTPComposeReadsEdgeSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/internal/v1/edge/snapshot" {
			t.Fatalf("path=%s", request.URL.Path)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": 0,
			"data": EdgeSnapshot{CNAMETarget: "edge.example", ShopDomain: "shop.example", ReservedHosts: []string{"shop.example"}},
		})
	}))
	t.Cleanup(server.Close)
	client := NewHTTP(config.Compose{PlatformOrigin: server.URL, InternalToken: "secret-token"})
	value, err := client.EdgeSnapshot(context.Background())
	if err != nil || value.CNAMETarget != "edge.example" || value.ShopDomain != "shop.example" {
		t.Fatalf("value=%+v err=%v", value, err)
	}
}

func TestUnavailableGrantsSurfaceMerchantError(t *testing.T) {
	var grants Grants = Unavailable{}
	if _, err := grants.PaymentChannels(context.Background(), 2001, 3001); !errors.Is(err, model.ErrUnavailable) {
		t.Fatalf("error=%v", err)
	}
}

func TestHTTPComposeUsesInternalGrantAndCamelCaseBody(t *testing.T) {
	var gotPath, gotToken, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotPath = request.URL.Path
		gotToken = request.Header.Get("X-Liveshop-Internal-Grant")
		payload, _ := io.ReadAll(request.Body)
		gotBody = string(payload)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": 0,
			"data": appmodel.MerchantPaymentChannels{MerchantID: 2001, ShopID: 3001, Version: 2, Channels: []appmodel.MerchantPaymentGrant{{ChannelCode: "alipay", Enabled: true}}},
		})
	}))
	t.Cleanup(server.Close)
	client := NewHTTP(config.Compose{TradeOrigin: server.URL, InternalToken: "secret-token"})
	value, err := client.PutPaymentChannels(context.Background(), appmodel.PutMerchantPaymentChannels{
		MerchantID: 2001, ShopID: 3001, CommandKey: "command-key-1", ExpectedVersion: 1,
		Channels: []appmodel.MerchantPaymentGrant{{ChannelCode: "alipay", Enabled: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if value.Version != 2 || gotToken != "secret-token" || gotPath != "/internal/v1/payment/merchant-channels" {
		t.Fatalf("value=%+v path=%s token=%s", value, gotPath, gotToken)
	}
	for _, part := range []string{`"merchantId":2001`, `"shopId":3001`, `"commandKey":"command-key-1"`} {
		if !strings.Contains(gotBody, part) {
			t.Fatalf("body=%s missing %s", gotBody, part)
		}
	}
}
