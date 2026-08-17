-- Identity shop capability owns merchant-created private app credentials.
-- Platform overlay for module=apps stays in identity_merchant_capability
-- and is never copied onto this table. Plaintext secrets are not stored here.

CREATE TABLE identity_shop_app (
  private_app_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  name VARCHAR(120) NOT NULL,
  client_id VARCHAR(80) NOT NULL,
  client_secret_hash CHAR(64) NOT NULL,
  secret_hint VARCHAR(16) NOT NULL,
  scopes VARCHAR(1000) NOT NULL DEFAULT '',
  status VARCHAR(20) NOT NULL,
  version BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (private_app_id),
  UNIQUE KEY uq_identity_shop_app_client (client_id),
  KEY idx_identity_shop_app_shop (merchant_id, shop_id, private_app_id),
  CONSTRAINT fk_identity_shop_app_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_shop_app_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_shop_app_scope CHECK (merchant_id > 0 AND shop_id > 0),
  CONSTRAINT ck_identity_shop_app_status CHECK (status IN ('ACTIVE','DISABLED')),
  CONSTRAINT ck_identity_shop_app_version CHECK (version > 0),
  CONSTRAINT ck_identity_shop_app_client CHECK (client_id LIKE 'app_%'),
  CONSTRAINT ck_identity_shop_app_hint CHECK (CHAR_LENGTH(secret_hint) = 6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_shop_app_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
