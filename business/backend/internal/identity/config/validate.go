package config

import (
	"errors"
	"fmt"
	"time"
)

var logLevels = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}

// Validate rejects every configuration a running process could not honour.
// It runs before any dependency is built so failures name the field, not a
// downstream symptom.
func (c Config) Validate() error {
	if c.Service == "" {
		return errors.New("service is required")
	}
	if !logLevels[c.Log.Level] {
		return errors.New("log.level must be one of debug, info, warn, error")
	}
	if c.Server.HTTP == "" {
		return errors.New("server.http is required")
	}
	if err := c.Database.validate(); err != nil {
		return err
	}
	if c.ModuleCapability.Issuer == "" || c.ModuleCapability.KeyID == "" || c.ModuleCapability.PrivateKey == "" {
		return errors.New("module_capability issuer, key_id and private_key are required")
	}
	capabilityTTL, err := duration("module_capability.ttl", c.ModuleCapability.TTL)
	if err != nil {
		return err
	}
	if capabilityTTL > 5*time.Minute {
		return errors.New("module_capability.ttl must not exceed 5m")
	}
	if err := c.AccessIdentity.validate(); err != nil {
		return err
	}
	if c.Server.GRPC == "" || c.GRPC.TLS.CertificateFile == "" || c.GRPC.TLS.PrivateKeyFile == "" || c.GRPC.TLS.ClientCAFile == "" || c.GRPC.PlatformSPIFFEID == "" || c.GRPC.CatalogSPIFFEID == "" {
		return errors.New("server.grpc, grpc TLS, platform_spiffe_id and catalog_spiffe_id are required")
	}
	if c.PlatformRegistry.Endpoint == "" || c.PlatformRegistry.ServerName == "" || c.PlatformRegistry.TLS.CertificateFile == "" || c.PlatformRegistry.TLS.PrivateKeyFile == "" || c.PlatformRegistry.TLS.ClientCAFile == "" {
		return errors.New("platform_registry endpoint, server_name and TLS files are required")
	}
	if _, err := duration("platform_registry.max_staleness", c.PlatformRegistry.MaxStaleness); err != nil {
		return err
	}
	if _, err := duration("platform_registry.sync_interval", c.PlatformRegistry.SyncInterval); err != nil {
		return err
	}
	if _, err := duration("subscription_entitlement.max_staleness", c.SubscriptionEntitlement.MaxStaleness); err != nil {
		return err
	}
	if _, err := duration("subscription_entitlement.sync_interval", c.SubscriptionEntitlement.SyncInterval); err != nil {
		return err
	}
	return nil
}

func (a AccessIdentity) validate() error {
	if a.Issuer == "" || a.KeyID == "" || a.PrivateKey == "" || a.PlatformRefreshCookie == "" || a.MerchantRefreshCookie == "" || a.CustomerRefreshCookie == "" {
		return errors.New("access_identity issuer, key_id, private_key and all realm refresh cookies are required")
	}
	if a.PlatformRefreshCookie == a.MerchantRefreshCookie || a.PlatformRefreshCookie == a.CustomerRefreshCookie || a.MerchantRefreshCookie == a.CustomerRefreshCookie {
		return errors.New("access_identity realm refresh cookie names must be distinct")
	}
	access, err := duration("access_identity.access_ttl", a.AccessTTL)
	if err != nil {
		return err
	}
	if access > 12*time.Hour {
		return errors.New("access_identity.access_ttl must not exceed 12h")
	}
	_, err = duration("access_identity.refresh_ttl", a.RefreshTTL)
	return err
}

func (a AccessIdentity) AccessDuration() (time.Duration, error) {
	return duration("access_identity.access_ttl", a.AccessTTL)
}

func (a AccessIdentity) RefreshDuration() (time.Duration, error) {
	return duration("access_identity.refresh_ttl", a.RefreshTTL)
}
func (m ModuleCapability) Duration() (time.Duration, error) {
	return duration("module_capability.ttl", m.TTL)
}
func (p PlatformRegistry) Staleness() (time.Duration, error) {
	return duration("platform_registry.max_staleness", p.MaxStaleness)
}
func (p PlatformRegistry) Interval() (time.Duration, error) {
	return duration("platform_registry.sync_interval", p.SyncInterval)
}
func (s SubscriptionEntitlement) Staleness() (time.Duration, error) {
	return duration("subscription_entitlement.max_staleness", s.MaxStaleness)
}
func (s SubscriptionEntitlement) Interval() (time.Duration, error) {
	return duration("subscription_entitlement.sync_interval", s.SyncInterval)
}

func (d Database) validate() error {
	if d.URL == "" {
		return errors.New("database.url is required")
	}
	if d.MaxOpenConnections <= 0 {
		return errors.New("database.max_open_connections must be positive")
	}
	if d.MaxIdleConnections < 0 || d.MaxIdleConnections > d.MaxOpenConnections {
		return errors.New("database.max_idle_connections must be between 0 and max_open_connections")
	}
	if _, err := d.Lifetime(); err != nil {
		return err
	}
	if _, err := d.IdleTime(); err != nil {
		return err
	}
	return nil
}

func (d Database) Lifetime() (time.Duration, error) {
	return duration("database.connection_max_lifetime", d.ConnectionMaxLifetime)
}

func (d Database) IdleTime() (time.Duration, error) {
	return duration("database.connection_max_idle_time", d.ConnectionMaxIdleTime)
}

func duration(field, value string) (time.Duration, error) {
	if value == "" {
		return 0, fmt.Errorf("%s is required", field)
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s is not a duration: %w", field, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be positive", field)
	}
	return parsed, nil
}
