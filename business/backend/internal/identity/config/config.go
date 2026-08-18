// Package config owns the identity configuration schema. The process reads
// exactly one complete file; there is no environment overlay and no code
// default, so a deployment is fully described by what it ships.
package config

import (
	"fmt"
	"os"

	"github.com/gogf/gf/v2/encoding/gjson"
)

type Config struct {
	Service                 string                  `json:"service"`
	Log                     Log                     `json:"log"`
	Server                  Server                  `json:"server"`
	Database                Database                `json:"database"`
	ModuleCapability        ModuleCapability        `json:"module_capability"`
	AccessIdentity          AccessIdentity          `json:"access_identity"`
	GRPC                    GRPC                    `json:"grpc"`
	PlatformRegistry        PlatformRegistry        `json:"platform_registry"`
	SubscriptionEntitlement SubscriptionEntitlement `json:"subscription_entitlement"`
	Compose                 Compose                 `json:"compose"`
}

type Compose struct {
	TradeOrigin       string `json:"trade_origin"`
	PlatformOrigin    string `json:"platform_origin"`
	InternalToken     string `json:"internal_token"`
	DomainCNAMETarget string `json:"domain_cname_target"`
}

type Log struct {
	Level string `json:"level"`
	JSON  bool   `json:"json"`
}

type Server struct {
	HTTP string `json:"http"`
	GRPC string `json:"grpc"`
}

type GRPC struct {
	TLS              GRPCTLS `json:"tls"`
	PlatformSPIFFEID string  `json:"platform_spiffe_id"`
	CatalogSPIFFEID  string  `json:"catalog_spiffe_id"`
}
type GRPCTLS struct {
	CertificateFile string `json:"certificate_file"`
	PrivateKeyFile  string `json:"private_key_file"`
	ClientCAFile    string `json:"client_ca_file"`
}
type PlatformRegistry struct {
	Endpoint     string  `json:"endpoint"`
	ServerName   string  `json:"server_name"`
	MaxStaleness string  `json:"max_staleness"`
	SyncInterval string  `json:"sync_interval"`
	TLS          GRPCTLS `json:"tls"`
}
type SubscriptionEntitlement struct {
	MaxStaleness string `json:"max_staleness"`
	SyncInterval string `json:"sync_interval"`
}

type Database struct {
	URL                   string `json:"url"`
	MaxOpenConnections    int    `json:"max_open_connections"`
	MaxIdleConnections    int    `json:"max_idle_connections"`
	ConnectionMaxLifetime string `json:"connection_max_lifetime"`
	ConnectionMaxIdleTime string `json:"connection_max_idle_time"`
}

type ModuleCapability struct {
	Issuer     string `json:"issuer"`
	KeyID      string `json:"key_id"`
	PrivateKey string `json:"private_key"`
	TTL        string `json:"ttl"`
}

// AccessIdentity is private signing material owned by Identity. It is
// intentionally separate from the contribution-scoped Module Capability key.
type AccessIdentity struct {
	Issuer                string `json:"issuer"`
	KeyID                 string `json:"key_id"`
	PrivateKey            string `json:"private_key"`
	AccessTTL             string `json:"access_ttl"`
	RefreshTTL            string `json:"refresh_ttl"`
	PlatformRefreshCookie string `json:"platform_refresh_cookie"`
	MerchantRefreshCookie string `json:"merchant_refresh_cookie"`
	CustomerRefreshCookie string `json:"customer_refresh_cookie"`
	CookieSecure          bool   `json:"cookie_secure"`
}

// Load reads and validates the configuration file. A missing or incomplete file
// fails startup instead of silently degrading the process.
func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, fmt.Errorf("identity: -config is required")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("identity: cannot read %s: %w", path, err)
	}
	parsed, err := gjson.LoadContentType("yaml", content)
	if err != nil {
		return Config{}, fmt.Errorf("identity: cannot parse %s: %w", path, err)
	}
	var config Config
	if err := parsed.Scan(&config); err != nil {
		return Config{}, fmt.Errorf("identity: cannot decode %s: %w", path, err)
	}
	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("identity: invalid %s: %w", path, err)
	}
	return config, nil
}
