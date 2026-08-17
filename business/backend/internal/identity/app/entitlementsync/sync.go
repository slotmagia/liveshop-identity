package entitlementsync

import (
	"context"
	"fmt"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/config"
	mysqlrepo "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/data/mysql"
	subscriptionv1 "github.com/lvtuopen-ai/liveshop-identity/protocol/gen/go/subscription/v1"
)

// Syncer maintains Identity's authorization read model from the Subscription
// capability in the same database. It has no network client and therefore no
// cross-service projection failure window; revision/digest checks remain in the
// read-model repository.
type Syncer struct {
	permissions *subscription.PermissionEntitlements
	repo        *mysqlrepo.AuthorizationRepository
	interval    time.Duration
}

func New(settings config.SubscriptionEntitlement, permissions *subscription.PermissionEntitlements, repo *mysqlrepo.AuthorizationRepository) (*Syncer, error) {
	if permissions == nil || repo == nil {
		return nil, fmt.Errorf("identity: subscription capability dependencies are required")
	}
	interval, err := settings.Interval()
	if err != nil {
		return nil, err
	}
	return &Syncer{permissions: permissions, repo: repo, interval: interval}, nil
}

func (s *Syncer) Once(ctx context.Context) error {
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	merchantIDs, err := s.repo.MerchantDomainIDs(call)
	if err != nil {
		return err
	}
	for _, merchantID := range merchantIDs {
		snapshot, err := s.permissions.Get(call, merchantID)
		if err != nil {
			return fmt.Errorf("identity: read local permission entitlement for merchant %d: %w", merchantID, err)
		}
		message := &subscriptionv1.GetMerchantPermissionEntitlementSnapshotResponse{
			MerchantId: snapshot.MerchantID, PermissionCodes: snapshot.PermissionCodes,
			Revision: int64(snapshot.Revision), SnapshotDigest: snapshot.SnapshotDigest,
			UpdatedAtUnixMs: snapshot.UpdatedAt.UnixMilli(),
		}
		if err := s.repo.ReplaceEntitlementSnapshot(call, message); err != nil {
			return fmt.Errorf("identity: update local entitlement read model for merchant %d: %w", merchantID, err)
		}
	}
	return nil
}

func (s *Syncer) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.Once(ctx)
		}
	}
}

func (s *Syncer) Close() error { return nil }
