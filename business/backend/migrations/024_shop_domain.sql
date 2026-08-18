-- Identity shop capability owns merchant-bound custom hosts, TXT challenges
-- and the per-shop per-scene primary mark. Platform overlay for module=domains
-- stays in identity_merchant_capability and is never copied onto this table.
-- CNAME targets and certificates are not stored here.

CREATE TABLE identity_shop_domain (
  domain_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  host VARCHAR(253) NOT NULL,
  scene VARCHAR(16) NOT NULL,
  status VARCHAR(20) NOT NULL,
  is_primary TINYINT NOT NULL DEFAULT 0,
  txt_name VARCHAR(300) NOT NULL,
  txt_value VARCHAR(128) NOT NULL,
  version BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  active_host VARCHAR(253) GENERATED ALWAYS AS (IF(status = 'DELETED', NULL, host)) STORED,
  primary_slot VARCHAR(80) GENERATED ALWAYS AS (
    IF(status = 'DELETED' OR is_primary = 0, NULL, CONCAT(merchant_id, ':', shop_id, ':', scene))
  ) STORED,
  PRIMARY KEY (domain_id),
  UNIQUE KEY uq_identity_shop_domain_active_host (active_host),
  UNIQUE KEY uq_identity_shop_domain_primary (primary_slot),
  KEY idx_identity_shop_domain_shop (merchant_id, shop_id, scene, domain_id),
  CONSTRAINT fk_identity_shop_domain_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_shop_domain_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_shop_domain_scope CHECK (merchant_id > 0 AND shop_id > 0),
  CONSTRAINT ck_identity_shop_domain_scene CHECK (scene IN ('LIVE','SHOP')),
  CONSTRAINT ck_identity_shop_domain_status CHECK (status IN ('PENDING','VERIFIED','FAILED','DELETED')),
  CONSTRAINT ck_identity_shop_domain_primary CHECK (is_primary IN (0,1)),
  CONSTRAINT ck_identity_shop_domain_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_shop_domain_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
