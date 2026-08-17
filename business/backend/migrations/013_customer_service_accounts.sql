-- Identity customer_service owns shop-scoped external customer-service
-- accounts. app_id/commercial_id are retained as legacy business dimensions,
-- but are always derived from the authoritative identity_shop row.

CREATE TABLE identity_customer_service_account (
  account_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  app_id BIGINT NOT NULL,
  commercial_id BIGINT NOT NULL,
  platform VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  account VARCHAR(128) NOT NULL,
  nickname VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  config TEXT NOT NULL,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  version BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (account_id),
  UNIQUE KEY uq_identity_customer_service_shop_platform_account (shop_id, platform, account),
  KEY idx_identity_customer_service_list (merchant_id, shop_id, status, account_id),
  CONSTRAINT fk_identity_customer_service_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_customer_service_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_customer_service_scope CHECK (merchant_id > 0 AND shop_id > 0 AND app_id > 0 AND commercial_id > 0),
  CONSTRAINT ck_identity_customer_service_status CHECK (status IN ('ACTIVE','DISABLED')),
  CONSTRAINT ck_identity_customer_service_platform CHECK (platform REGEXP '^[a-z0-9_-]{1,32}$'),
  CONSTRAINT ck_identity_customer_service_config CHECK (config = '' OR JSON_VALID(config)),
  CONSTRAINT ck_identity_customer_service_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_customer_service_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
