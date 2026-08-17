package grpcentitlement

import (
	"context"
	"errors"
	"time"

	biz "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
	subscriptionv1 "github.com/lvtuopen-ai/liveshop-identity/protocol/gen/go/subscription/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	subscriptionv1.UnimplementedSubscriptionServiceServer
	quotas      *biz.Quotas
	permissions *biz.PermissionEntitlements
}

func New(quotas *biz.Quotas, permissions *biz.PermissionEntitlements) *Server {
	return &Server{quotas: quotas, permissions: permissions}
}

func (s *Server) Check(ctx context.Context, _ *subscriptionv1.CheckRequest) (*subscriptionv1.CheckResponse, error) {
	if err := s.quotas.Ready(ctx); err != nil {
		return nil, status.Error(codes.Unavailable, "subscription database is not ready")
	}
	return &subscriptionv1.CheckResponse{Status: "ready"}, nil
}

func (s *Server) GetMerchantPermissionEntitlementSnapshot(ctx context.Context, request *subscriptionv1.GetMerchantPermissionEntitlementSnapshotRequest) (*subscriptionv1.GetMerchantPermissionEntitlementSnapshotResponse, error) {
	if request == nil || request.GetMerchantId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "positive merchant_id is required")
	}
	snapshot, err := s.permissions.Get(ctx, request.GetMerchantId())
	if err != nil {
		return nil, transportError(err)
	}
	return &subscriptionv1.GetMerchantPermissionEntitlementSnapshotResponse{
		MerchantId: snapshot.MerchantID, PermissionCodes: snapshot.PermissionCodes,
		Revision: int64(snapshot.Revision), SnapshotDigest: snapshot.SnapshotDigest,
		UpdatedAtUnixMs: snapshot.UpdatedAt.UnixMilli(),
	}, nil
}

func (s *Server) GetQuotaLimit(ctx context.Context, request *subscriptionv1.GetQuotaLimitRequest) (*subscriptionv1.GetQuotaLimitResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	quota, err := s.quotas.Get(ctx, request.GetMerchantId(), request.GetQuotaCode(), evaluationTime(request.GetEvaluatedAtUnixMs()))
	if err != nil {
		return nil, transportError(err)
	}
	return &subscriptionv1.GetQuotaLimitResponse{Quota: quotaMessage(quota)}, nil
}

func (s *Server) GetQuotaLimits(ctx context.Context, request *subscriptionv1.GetQuotaLimitsRequest) (*subscriptionv1.GetQuotaLimitsResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	quotas, err := s.quotas.GetMany(ctx, request.GetMerchantId(), request.GetQuotaCodes(), evaluationTime(request.GetEvaluatedAtUnixMs()))
	if err != nil {
		return nil, transportError(err)
	}
	response := &subscriptionv1.GetQuotaLimitsResponse{Quotas: make([]*subscriptionv1.QuotaLimit, 0, len(quotas))}
	for _, quota := range quotas {
		response.Quotas = append(response.Quotas, quotaMessage(quota))
	}
	return response, nil
}

func evaluationTime(milliseconds int64) time.Time {
	if milliseconds == 0 {
		return time.Time{}
	}
	return time.UnixMilli(milliseconds).UTC()
}

func quotaMessage(quota biz.QuotaLimit) *subscriptionv1.QuotaLimit {
	message := &subscriptionv1.QuotaLimit{
		QuotaCode: quota.Code, Unlimited: quota.Unlimited(), Revision: quota.Revision,
		EffectiveFromUnixMs: quota.EffectiveFrom.UnixMilli(),
	}
	if quota.Limit != nil {
		message.Limit = *quota.Limit
	}
	if quota.EffectiveUntil != nil {
		message.EffectiveUntilUnixMs = quota.EffectiveUntil.UnixMilli()
	}
	return message
}

func transportError(err error) error {
	switch {
	case errors.Is(err, biz.ErrNotConfigured):
		return status.Error(codes.FailedPrecondition, "no explicit effective quota entitlement")
	case errors.Is(err, biz.ErrPermissionEntitlementNotConfigured):
		return status.Error(codes.FailedPrecondition, "no explicit merchant permission entitlement")
	default:
		return status.Error(codes.Internal, "subscription entitlement lookup failed")
	}
}
