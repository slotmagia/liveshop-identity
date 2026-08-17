// Package app is the identity composition root. It builds every dependency
// explicitly and injects it; nothing here reaches a global accessor, so tests
// can assemble their own process without fighting shared state.
package app

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/lvtuopen-ai/kernel-go/accessidentity"
	"github.com/lvtuopen-ai/kernel-go/modulesession"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/app/authendpoint"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/app/capabilityendpoint"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/app/entitlementsync"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/app/grpcdirectory"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/app/grpcentitlement"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/app/registrysync"
	admincompose "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/compose"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/risk"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/middleware"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/config"
	customerservicedata "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/data/customer_service"
	merchantdata "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/data/merchant"
	governancedata "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/data/merchant_governance"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/data/mysql"
	riskdata "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/data/risk"
	shopdata "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/data/shop"
	subscriptiondata "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/data/subscription"
)

// Dependencies holds the use cases every surface is built from, plus the
// credentials its transports verify.
type Dependencies struct {
	Config             config.Config
	Health             *biz.Health
	Directory          *biz.Directory
	Authentication     *biz.Authentication
	AuthEndpoint       *authendpoint.Endpoint
	CapabilityEndpoint *capabilityendpoint.Endpoint
	Authorization      *biz.AuthorizationService
	Users              *biz.UserLifecycle
	Capability         *biz.CapabilityService
	SubscriptionQuotas *subscription.Quotas
	PermissionPlans    *subscription.PermissionEntitlements
	Plans              *subscription.Plans
	Merchants          *merchant.Directory
	Shops              *shop.Directory
	ShopCategories     *shop.Categories
	Privacy            *shop.PrivacySettings
	Policies           *shop.Policies
	Apps               *shop.PrivateApps
	CustomerService    *customer_service.Accounts
	RiskEvents         *risk.Events
	MerchantGovernance *merchant_governance.Capabilities
	Assignments        *subscription.Assignments
	Orders             *subscription.Orders
	Grants             admincompose.Grants
	RegistrySync       *registrysync.Syncer
	EntitlementSync    *entitlementsync.Syncer
	RegistryProjection *mysql.AuthorizationRepository
	GRPCServer         *grpcdirectory.Server
	ModuleSessions     *modulesession.Verifier

	database *sql.DB
}

// NewDependencies fails fast when a backing dependency is missing, so the
// process never starts in a silently degraded mode.
func NewDependencies(ctx context.Context, settings config.Config) (*Dependencies, error) {
	sessions, err := newModuleCapabilityVerifier(settings.ModuleCapability)
	if err != nil {
		return nil, err
	}
	database, err := openDatabase(ctx, settings.Database)
	if err != nil {
		return nil, err
	}
	quotas := subscription.New(subscriptiondata.NewQuotaRepository(database))
	permissionPlans := subscription.NewPermissionEntitlements(subscriptiondata.NewPermissionEntitlementRepository(database))
	plans := subscription.NewPlans(subscriptiondata.NewPlanRepository(database))
	merchants := merchant.NewDirectory(merchantdata.NewRepository(database))
	shops := shop.NewDirectory(shopdata.NewRepository(database))
	shopCategories := shop.NewCategories(shopdata.NewCategoryRepository(database))
	privacy := shop.NewPrivacySettings(shopdata.NewPrivacyRepository(database))
	policies := shop.NewPolicies(shopdata.NewPolicyRepository(database))
	apps := shop.NewPrivateApps(shopdata.NewAppRepository(database))
	customerService := customer_service.NewAccounts(customerservicedata.NewRepository(database))
	riskEvents := risk.NewEvents(riskdata.NewRepository(database))
	merchantGovernance := merchant_governance.NewCapabilities(governancedata.NewRepository(database))
	assignments := subscription.NewAssignments(subscriptiondata.NewAssignmentRepository(database))
	orders := subscription.NewOrders(subscriptiondata.NewOrderRepository(database))
	healthRepository, err := mysql.NewHealthRepository(database)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	directoryRepository, err := mysql.NewDirectoryRepository(database)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	entitlementStaleness, err := settings.SubscriptionEntitlement.Staleness()
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	registryStaleness, err := settings.PlatformRegistry.Staleness()
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	authorizationRepository, err := mysql.NewAuthorizationRepository(database, entitlementStaleness)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	authRepository, err := mysql.NewAuthRepository(database, directoryRepository)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	userRepository, err := mysql.NewUserLifecycleRepository(database, directoryRepository, entitlementStaleness, registryStaleness)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	directory := biz.NewDirectory(directoryRepository)
	authentication := biz.NewAuthentication(authRepository, directory)
	accessIssuer, accessVerifier, err := newAccessIdentity(settings.AccessIdentity)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	authEndpoint, err := authendpoint.New(authentication, directory, accessIssuer, accessVerifier, settings.AccessIdentity)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	capabilityIssuer, err := modulesession.NewIssuer(settings.ModuleCapability.PrivateKey, settings.ModuleCapability.KeyID, settings.ModuleCapability.Issuer)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("identity: cannot build module capability issuer: %w", err)
	}
	authorization := biz.NewAuthorization(authorizationRepository)
	capabilityTTL, err := settings.ModuleCapability.Duration()
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	capability := biz.NewCapability(authorization, authorizationRepository, capabilityIssuer, capabilityTTL, registryStaleness)
	capabilityEndpoint, err := capabilityendpoint.New(directory, capability, accessVerifier, biz.NewHealth(healthRepository), func(call context.Context) error {
		if err := authorizationRepository.RegistryReady(call, registryStaleness); err != nil {
			return err
		}
		return authorizationRepository.EntitlementsReady(call)
	})
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	grpcServer, err := grpcdirectory.New(settings, directory, grpcentitlement.New(quotas, permissionPlans))
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	registrySync, err := registrysync.New(settings.PlatformRegistry, authorizationRepository)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	if err = registrySync.Once(ctx); err != nil {
		_ = registrySync.Close()
		_ = database.Close()
		return nil, fmt.Errorf("identity: initial registry projection sync failed: %w", err)
	}
	entitlementSync, err := entitlementsync.New(settings.SubscriptionEntitlement, permissionPlans, authorizationRepository)
	if err != nil {
		_ = registrySync.Close()
		_ = database.Close()
		return nil, err
	}
	if err = entitlementSync.Once(ctx); err != nil {
		_ = entitlementSync.Close()
		_ = registrySync.Close()
		_ = database.Close()
		return nil, fmt.Errorf("identity: initial Subscription entitlement sync failed: %w", err)
	}
	return &Dependencies{
		Config:             settings,
		Health:             biz.NewHealth(healthRepository),
		Directory:          directory,
		Authentication:     authentication,
		AuthEndpoint:       authEndpoint,
		CapabilityEndpoint: capabilityEndpoint,
		Authorization:      authorization,
		Users:              biz.NewUserLifecycle(userRepository),
		Capability:         capability,
		SubscriptionQuotas: quotas,
		PermissionPlans:    permissionPlans,
		Plans:              plans,
		Merchants:          merchants,
		Shops:              shops,
		ShopCategories:     shopCategories,
		Privacy:            privacy,
		Policies:           policies,
		Apps:               apps,
		CustomerService:    customerService,
		RiskEvents:         riskEvents,
		MerchantGovernance: merchantGovernance,
		Assignments:        assignments,
		Orders:             orders,
		Grants:             admincompose.NewHTTP(settings.Compose),
		RegistrySync:       registrySync,
		EntitlementSync:    entitlementSync,
		RegistryProjection: authorizationRepository,
		GRPCServer:         grpcServer,
		ModuleSessions:     sessions,
		database:           database,
	}, nil
}

func newAccessIdentity(settings config.AccessIdentity) (*accessidentity.Issuer, *accessidentity.Verifier, error) {
	issuer, err := accessidentity.NewIssuer(settings.PrivateKey, settings.KeyID, settings.Issuer)
	if err != nil {
		return nil, nil, fmt.Errorf("identity: cannot build access identity issuer: %w", err)
	}
	keyMaterial, err := base64.RawURLEncoding.DecodeString(settings.PrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("identity: invalid access identity private key")
	}
	var privateKey ed25519.PrivateKey
	switch len(keyMaterial) {
	case ed25519.SeedSize:
		privateKey = ed25519.NewKeyFromSeed(keyMaterial)
	case ed25519.PrivateKeySize:
		privateKey = ed25519.PrivateKey(keyMaterial)
	default:
		return nil, nil, fmt.Errorf("identity: invalid access identity private key")
	}
	publicKey := ed25519.PrivateKey(privateKey).Public().(ed25519.PublicKey)
	verifier, err := accessidentity.NewVerifier(map[string]string{settings.KeyID: base64.RawURLEncoding.EncodeToString(publicKey)}, settings.Issuer, accessidentity.AudienceRuntime)
	if err != nil {
		return nil, nil, fmt.Errorf("identity: cannot build access identity verifier: %w", err)
	}
	return issuer, verifier, nil
}

func (d *Dependencies) Close() error {
	if d.RegistrySync != nil {
		_ = d.RegistrySync.Close()
	}
	if d.EntitlementSync != nil {
		_ = d.EntitlementSync.Close()
	}
	if d.database == nil {
		return nil
	}
	return d.database.Close()
}

// newModuleCapabilityVerifier derives the local verification key from the
// Identity-owned signing key. There is one issuer and no legacy Platform path.
func newModuleCapabilityVerifier(settings config.ModuleCapability) (*modulesession.Verifier, error) {
	keyMaterial, err := base64.RawURLEncoding.DecodeString(settings.PrivateKey)
	if err != nil {
		return nil, err
	}
	var privateKey ed25519.PrivateKey
	switch len(keyMaterial) {
	case ed25519.SeedSize:
		privateKey = ed25519.NewKeyFromSeed(keyMaterial)
	case ed25519.PrivateKeySize:
		privateKey = ed25519.PrivateKey(keyMaterial)
	default:
		return nil, fmt.Errorf("identity: invalid module capability private key")
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	verifier, err := modulesession.NewVerifier(map[string]string{settings.KeyID: base64.RawURLEncoding.EncodeToString(publicKey)}, settings.Issuer, middleware.Audience)
	if err != nil {
		return nil, fmt.Errorf("identity: cannot build module capability verifier: %w", err)
	}
	return verifier, nil
}

func openDatabase(ctx context.Context, settings config.Database) (*sql.DB, error) {
	database, err := sql.Open("mysql", settings.URL)
	if err != nil {
		return nil, fmt.Errorf("identity: cannot open database: %w", err)
	}
	lifetime, err := settings.Lifetime()
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	idleTime, err := settings.IdleTime()
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	database.SetMaxOpenConns(settings.MaxOpenConnections)
	database.SetMaxIdleConns(settings.MaxIdleConnections)
	database.SetConnMaxLifetime(lifetime)
	database.SetConnMaxIdleTime(idleTime)
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("identity: database is unreachable: %w", err)
	}
	return database, nil
}
