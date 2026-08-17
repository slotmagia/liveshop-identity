package grpcdirectory

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lvtuopen-ai/kernel-go/grpcx"
	"github.com/lvtuopen-ai/kernel-go/principal"
	identityv1 "github.com/lvtuopen-ai/liveshop-identity/protocol/gen/go/identity/v1"
	subscriptionv1 "github.com/lvtuopen-ai/liveshop-identity/protocol/gen/go/subscription/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/config"
)

type Server struct {
	transport *grpcx.Server
	directory *biz.Directory
	identityv1.UnimplementedIdentityDirectoryServiceServer
}

func New(settings config.Config, directory *biz.Directory, entitlements subscriptionv1.SubscriptionServiceServer) (*Server, error) {
	if directory == nil {
		return nil, model.ErrUnavailable
	}
	if entitlements == nil {
		return nil, fmt.Errorf("identity: entitlement gRPC service is required")
	}
	creds, err := serverCredentials(settings.GRPC.TLS)
	if err != nil {
		return nil, err
	}
	transport, err := grpcx.NewServer(settings.Server.GRPC, grpcx.ServerOptions{TransportCredentials: creds, UnaryInterceptors: []grpc.UnaryServerInterceptor{authorize(settings.GRPC)}})
	if err != nil {
		return nil, fmt.Errorf("identity: create gRPC server: %w", err)
	}
	server := &Server{transport: transport, directory: directory}
	identityv1.RegisterIdentityDirectoryServiceServer(transport.Engine(), server)
	subscriptionv1.RegisterSubscriptionServiceServer(transport.Engine(), entitlements)
	return server, nil
}
func (s *Server) Serve() error                   { return s.transport.Serve() }
func (s *Server) Stop(ctx context.Context) error { return s.transport.Stop(ctx) }

func (s *Server) ResolvePrincipalContext(ctx context.Context, request *identityv1.ResolvePrincipalContextRequest) (*identityv1.ResolvePrincipalContextResponse, error) {
	selected := selectedContext(request.GetSelectedContext())
	resolved, err := s.directory.ResolveAuthenticatedPrincipalContext(ctx, request.GetSessionId(), request.GetSubject(), selected, uint64(request.GetExpectedContextVersion()))
	if err != nil {
		return nil, grpcError(err)
	}
	return principalResponse(resolved), nil
}
func (s *Server) ResolveShopContext(ctx context.Context, request *identityv1.ResolveShopContextRequest) (*identityv1.ResolveShopContextResponse, error) {
	var resolved model.ShopResolution
	var err error
	switch selector := request.Selector.(type) {
	case *identityv1.ResolveShopContextRequest_ShopId:
		resolved, err = s.directory.ResolveShopByID(ctx, selector.ShopId)
	default:
		return nil, status.Error(codes.InvalidArgument, "supported shop selector is required")
	}
	if err != nil {
		return nil, grpcError(err)
	}
	return shopResolutionResponse(resolved), nil
}
func (s *Server) ValidateSelectedContext(ctx context.Context, request *identityv1.ValidateSelectedContextRequest) (*identityv1.ValidateSelectedContextResponse, error) {
	resolved, err := s.directory.ValidateSelectedContext(ctx, request.GetSubject(), selectedContext(request.GetSelectedContext()), uint64(request.GetExpectedIdentityVersion()), uint64(request.GetExpectedAccessVersion()))
	if err != nil {
		return &identityv1.ValidateSelectedContextResponse{Valid: false, DenialReason: err.Error()}, nil
	}
	return &identityv1.ValidateSelectedContextResponse{Valid: true, SelectedContext: selectedContextMessage(resolved.Selected), IdentityVersion: int64(resolved.Subject.Version), OrganizationVersion: int64(resolved.Organization.Version), AccessVersion: int64(resolved.Member.AccessVersion)}, nil
}
func (s *Server) GetOrganizationSubtree(ctx context.Context, request *identityv1.GetOrganizationSubtreeRequest) (*identityv1.GetOrganizationSubtreeResponse, error) {
	ids, version, err := s.directory.OrganizationSubtree(ctx, request.GetOrganizationId(), request.GetRootUnitId())
	if err != nil {
		return nil, grpcError(err)
	}
	return &identityv1.GetOrganizationSubtreeResponse{OrganizationId: request.GetOrganizationId(), OrganizationUnitIds: ids, OrganizationVersion: int64(version)}, nil
}
func (s *Server) BatchGetSubjects(ctx context.Context, request *identityv1.BatchGetSubjectsRequest) (*identityv1.BatchGetSubjectsResponse, error) {
	items, err := s.directory.Subjects(ctx, request.GetSubjects())
	if err != nil {
		return nil, grpcError(err)
	}
	response := &identityv1.BatchGetSubjectsResponse{}
	for _, item := range items {
		response.Subjects = append(response.Subjects, subjectMessage(item))
	}
	return response, nil
}
func (s *Server) ResolveLegacySubjects(ctx context.Context, request *identityv1.ResolveLegacySubjectsRequest) (*identityv1.ResolveLegacySubjectsResponse, error) {
	items, err := s.directory.LegacySubjects(ctx, principal.Realm(request.GetRealm()), request.GetLegacyUids())
	if err != nil {
		return nil, grpcError(err)
	}
	response := &identityv1.ResolveLegacySubjectsResponse{}
	for _, item := range items {
		response.Subjects = append(response.Subjects, subjectMessage(item))
	}
	return response, nil
}

func principalResponse(value model.PrincipalContext) *identityv1.ResolvePrincipalContextResponse {
	response := &identityv1.ResolvePrincipalContextResponse{Subject: subjectMessage(value.Subject), Member: &identityv1.WorkforceMember{MemberId: value.Member.ID, OrganizationId: value.Member.OrganizationID, MerchantId: value.Member.MerchantID, MemberType: string(value.Member.Type), Status: string(value.Member.Status), AccessVersion: int64(value.Member.AccessVersion), LegacyStaffId: value.Member.LegacyStaffID}, Organization: &identityv1.Organization{OrganizationId: value.Organization.ID, OrganizationType: string(value.Organization.Type), MerchantId: value.Organization.MerchantID, Name: value.Organization.Name, Status: string(value.Organization.Status), Version: int64(value.Organization.Version)}, OrganizationUnitIds: value.OrganizationUnitIDs, SelectedContext: selectedContextMessage(value.Selected), IdentityVersion: int64(value.Subject.Version), OrganizationVersion: int64(value.Organization.Version), AccessVersion: int64(value.Member.AccessVersion)}
	for _, shop := range value.AvailableShops {
		response.AvailableShops = append(response.AvailableShops, shopContext(shop))
	}
	return response
}
func subjectMessage(value model.Subject) *identityv1.Subject {
	return &identityv1.Subject{Subject: value.ID, Realm: value.Realm.String(), PrincipalType: value.PrincipalType.String(), DisplayName: value.DisplayName, Status: string(value.Status), Version: int64(value.Version), LegacyUid: value.LegacyUID}
}
func shopContext(value model.ShopContext) *identityv1.ShopContext {
	return &identityv1.ShopContext{MerchantId: value.MerchantID, ShopId: value.ShopID}
}
func shopResolutionResponse(value model.ShopResolution) *identityv1.ResolveShopContextResponse {
	return &identityv1.ResolveShopContextResponse{
		Context:  shopContext(value.Context),
		Status:   string(value.Status),
		Version:  int64(value.Version),
		Currency: value.Currency,
	}
}
func selectedContext(value *identityv1.SelectedContext) model.SelectedContext {
	if value == nil {
		return model.SelectedContext{}
	}
	return model.SelectedContext{OrganizationID: value.GetOrganizationId(), ShopContext: model.ShopContext{MerchantID: value.GetMerchantId(), ShopID: value.GetShopId()}}
}
func selectedContextMessage(value model.SelectedContext) *identityv1.SelectedContext {
	return &identityv1.SelectedContext{OrganizationId: value.OrganizationID, MerchantId: value.MerchantID, ShopId: value.ShopID}
}
func grpcError(err error) error {
	switch {
	case errors.Is(err, model.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, model.ErrConflict):
		return status.Error(codes.Aborted, err.Error())
	case errors.Is(err, model.ErrInactive), errors.Is(err, model.ErrInvalidContext):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Unavailable, "identity directory failed")
	}
}

func serverCredentials(settings config.GRPCTLS) (credentials.TransportCredentials, error) {
	certificate, err := tls.LoadX509KeyPair(settings.CertificateFile, settings.PrivateKeyFile)
	if err != nil {
		return nil, err
	}
	pem, err := os.ReadFile(settings.ClientCAFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("identity: gRPC client CA invalid")
	}
	return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}), nil
}
func authorize(settings config.GRPC) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		isDirectory := strings.HasPrefix(info.FullMethod, "/liveshop.identity.v1.IdentityDirectoryService/")
		isCatalogEntitlement := info.FullMethod == subscriptionv1.SubscriptionService_Check_FullMethodName ||
			info.FullMethod == subscriptionv1.SubscriptionService_GetQuotaLimit_FullMethodName ||
			info.FullMethod == subscriptionv1.SubscriptionService_GetQuotaLimits_FullMethodName
		if !isDirectory && !isCatalogEntitlement {
			return nil, status.Error(codes.PermissionDenied, "gRPC method is not authorized")
		}
		remote, ok := peer.FromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "client certificate required")
		}
		tlsInfo, ok := remote.AuthInfo.(credentials.TLSInfo)
		if !ok || len(tlsInfo.State.VerifiedChains) == 0 {
			return nil, status.Error(codes.Unauthenticated, "verified client certificate required")
		}
		var found *url.URL
		for _, uri := range tlsInfo.State.VerifiedChains[0][0].URIs {
			if uri.Scheme == "spiffe" {
				found = uri
				break
			}
		}
		expectedSPIFFE := settings.CatalogSPIFFEID
		if isDirectory {
			expectedSPIFFE = settings.PlatformSPIFFEID
		}
		if found == nil || found.String() != expectedSPIFFE {
			return nil, status.Error(codes.PermissionDenied, "workload SPIFFE identity is not trusted")
		}
		return handler(ctx, request)
	}
}

var _ = time.Second
