-- Identity fulfillment owns shop-scoped shipping rules and shipping presets.
-- Zones and rates are stored on the preset aggregate. Platform overlay is not
-- copied onto these rows. app_id/commercial_id are not runtime identifiers.

CREATE TABLE identity_shipping_rule (
  shipping_rule_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  name VARCHAR(120) NOT NULL,
  regions VARCHAR(2000) NOT NULL,
  fee_fen BIGINT NOT NULL,
  free_over_fen BIGINT NOT NULL DEFAULT 0,
  min_days INT NOT NULL,
  max_days INT NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  version BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (shipping_rule_id),
  KEY idx_identity_shipping_rule_list (merchant_id, shop_id, status, sort_order, shipping_rule_id),
  CONSTRAINT fk_identity_shipping_rule_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_shipping_rule_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_shipping_rule_scope CHECK (merchant_id > 0 AND shop_id > 0),
  CONSTRAINT ck_identity_shipping_rule_fee CHECK (fee_fen >= 0 AND free_over_fen >= 0),
  CONSTRAINT ck_identity_shipping_rule_days CHECK (min_days >= 0 AND max_days >= min_days AND max_days <= 365),
  CONSTRAINT ck_identity_shipping_rule_status CHECK (status IN ('ACTIVE','DISABLED','RETIRED')),
  CONSTRAINT ck_identity_shipping_rule_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_shipping_rule_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash VARBINARY(32) NOT NULL,
  shipping_rule_id BIGINT NULL,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  completed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_shipping_preset (
  shipping_preset_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  name VARCHAR(120) NOT NULL,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  product_scope VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  product_ids_json JSON NOT NULL,
  origin_name VARCHAR(120) NOT NULL,
  origin_region_code VARCHAR(32) NOT NULL,
  origin_region_name VARCHAR(120) NOT NULL,
  origin_country_code CHAR(2) COLLATE utf8mb4_0900_as_cs NOT NULL,
  origin_country_name VARCHAR(120) NOT NULL,
  origin_subdivision_code VARCHAR(32) NOT NULL DEFAULT '',
  origin_subdivision_name VARCHAR(120) NOT NULL DEFAULT '',
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  zones_json JSON NOT NULL,
  version BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (shipping_preset_id),
  KEY idx_identity_shipping_preset_list (merchant_id, shop_id, status, shipping_preset_id),
  CONSTRAINT fk_identity_shipping_preset_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_shipping_preset_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_shipping_preset_scope CHECK (merchant_id > 0 AND shop_id > 0),
  CONSTRAINT ck_identity_shipping_preset_default CHECK (is_default IN (0,1)),
  CONSTRAINT ck_identity_shipping_preset_product CHECK (product_scope IN ('ALL','SELECTED')),
  CONSTRAINT ck_identity_shipping_preset_country CHECK (origin_country_code REGEXP '^[A-Z]{2}$'),
  CONSTRAINT ck_identity_shipping_preset_status CHECK (status IN ('ACTIVE','DISABLED','RETIRED')),
  CONSTRAINT ck_identity_shipping_preset_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_shipping_preset_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash VARBINARY(32) NOT NULL,
  shipping_preset_id BIGINT NULL,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  completed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
