-- Identity merchant_governance owns the platform overlay on merchant
-- capabilities. Content of privacy/policies/domains/apps/languages/shipping
-- stays in those capabilities; this table only stores dual-status governance.

CREATE TABLE identity_merchant_capability (
  capability_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  app_id BIGINT NOT NULL,
  commercial_id BIGINT NOT NULL,
  module VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  name VARCHAR(120) NOT NULL,
  merchant_status VARCHAR(20) COLLATE utf8mb4_0900_as_cs NOT NULL,
  platform_status VARCHAR(20) COLLATE utf8mb4_0900_as_cs NOT NULL,
  platform_reason_public VARCHAR(500) NOT NULL DEFAULT '',
  version BIGINT NOT NULL,
  updated_by VARCHAR(128) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (capability_id),
  UNIQUE KEY uq_identity_merchant_capability_scope (shop_id, module),
  KEY idx_identity_merchant_capability_list (merchant_id, shop_id, module, capability_id),
  CONSTRAINT fk_identity_merchant_capability_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_merchant_capability_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_merchant_capability_scope CHECK (merchant_id > 0 AND shop_id > 0 AND app_id > 0 AND commercial_id > 0),
  CONSTRAINT ck_identity_merchant_capability_module CHECK (module IN ('privacy','policies','domains','apps','languages','shipping')),
  CONSTRAINT ck_identity_merchant_capability_merchant_status CHECK (merchant_status IN ('unset','active','draft')),
  CONSTRAINT ck_identity_merchant_capability_platform_status CHECK (platform_status IN ('active','restricted','suspended')),
  CONSTRAINT ck_identity_merchant_capability_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_merchant_capability_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_merchant_capability_audit (
  audit_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  app_id BIGINT NOT NULL,
  commercial_id BIGINT NOT NULL,
  module VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  capability_id BIGINT NOT NULL DEFAULT 0,
  action VARCHAR(40) COLLATE utf8mb4_0900_as_cs NOT NULL,
  operator VARCHAR(128) NOT NULL,
  reason_internal VARCHAR(1000) NOT NULL,
  reason_public VARCHAR(500) NOT NULL DEFAULT '',
  before_json JSON NULL,
  after_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (audit_id),
  KEY idx_identity_merchant_capability_audit (merchant_id, shop_id, module, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
