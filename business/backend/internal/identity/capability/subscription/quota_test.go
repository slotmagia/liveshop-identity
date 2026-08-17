package subscription

import (
	"testing"
	"time"
)

func TestApplyQuotaCommandHashIsStableAndPayloadSensitive(t *testing.T) {
	limit := int64(50)
	command := ApplyQuotaCommand{MerchantID: 7, CommandKey: "catalog-plan-001", Code: CatalogProductsQuota, Limit: &limit, EffectiveFrom: time.Unix(10, 0).UTC()}
	if command.RequestHash() != command.RequestHash() {
		t.Fatal("hash is not stable")
	}
	other := command
	other.Limit = nil
	if command.RequestHash() == other.RequestHash() {
		t.Fatal("different payloads share a hash")
	}
}

func TestApplyQuotaCommandRejectsUnprovenZeroLimit(t *testing.T) {
	zero := int64(0)
	command := ApplyQuotaCommand{MerchantID: 1, CommandKey: "catalog-plan-001", Code: CatalogProductsQuota, Limit: &zero, EffectiveFrom: time.Now()}
	if command.Validate() == nil {
		t.Fatal("zero limit must not silently mean unlimited")
	}
}
