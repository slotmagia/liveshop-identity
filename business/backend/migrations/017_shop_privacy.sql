-- Identity shop capability owns the merchant-editable customer privacy
-- document. Platform overlay for module=privacy stays in
-- identity_merchant_capability and is never copied onto this table.

CREATE TABLE identity_shop_privacy (
  privacy_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  collect_consent TINYINT(1) NOT NULL,
  marketing_consent TINYINT(1) NOT NULL,
  cookie_banner TINYINT(1) NOT NULL,
  data_retention_days INT NOT NULL,
  contact_email VARCHAR(254) NOT NULL DEFAULT '',
  version BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (privacy_id),
  UNIQUE KEY uq_identity_shop_privacy_shop (shop_id),
  KEY idx_identity_shop_privacy_merchant (merchant_id, shop_id),
  CONSTRAINT fk_identity_shop_privacy_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_shop_privacy_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_shop_privacy_scope CHECK (merchant_id > 0 AND shop_id > 0),
  CONSTRAINT ck_identity_shop_privacy_retention CHECK (data_retention_days BETWEEN 1 AND 3650),
  CONSTRAINT ck_identity_shop_privacy_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_shop_privacy_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
