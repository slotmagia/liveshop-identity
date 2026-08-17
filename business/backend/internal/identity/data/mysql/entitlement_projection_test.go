package mysql

import (
	"errors"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

func TestEntitlementProjectionOrderingAndDuplicateRules(t *testing.T) {
	digestA := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	digestB := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	for name, test := range map[string]struct {
		current, incoming uint64
		currentDigest     []byte
		incomingDigest    []byte
		want              entitlementProjectionDecision
		conflict          bool
	}{
		"new revision replaces":                  {1, 2, digestA, digestB, entitlementReplace, false},
		"same revision same digest refreshes":    {2, 2, digestA, digestA, entitlementRefresh, false},
		"same revision changed digest conflicts": {2, 2, digestA, digestB, 0, true},
		"older revision conflicts":               {3, 2, digestA, digestA, 0, true},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := decideEntitlementProjection(test.current, test.currentDigest, test.incoming, test.incomingDigest)
			if errors.Is(err, model.ErrAuthorizationConflict) != test.conflict || got != test.want {
				t.Fatalf("decision=%d err=%v", got, err)
			}
		})
	}
}
