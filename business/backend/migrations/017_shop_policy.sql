-- Identity owns shop policy versions. Merchant status is the publication
-- lifecycle; platform overlay remains in identity_merchant_capability and is
-- not copied onto these rows. app_id/commercial_id are not runtime identifiers.

CREATE TABLE identity_shop_policy (
  policy_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  policy_type VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  title VARCHAR(255) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  version_no INT NOT NULL,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  version BIGINT NOT NULL,
  published_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (policy_id),
  UNIQUE KEY uq_identity_shop_policy_version (shop_id, policy_type, version_no),
  KEY idx_identity_shop_policy_list (merchant_id, shop_id, policy_type, status, version_no),
  CONSTRAINT fk_identity_shop_policy_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_shop_policy_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_shop_policy_scope CHECK (merchant_id > 0 AND shop_id > 0),
  CONSTRAINT ck_identity_shop_policy_type CHECK (policy_type IN ('privacy','terms','refund','shipping')),
  CONSTRAINT ck_identity_shop_policy_status CHECK (status IN ('DRAFT','PUBLISHED','ARCHIVED')),
  CONSTRAINT ck_identity_shop_policy_version_no CHECK (version_no > 0),
  CONSTRAINT ck_identity_shop_policy_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_shop_policy_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  policy_id BIGINT NOT NULL DEFAULT 0,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
