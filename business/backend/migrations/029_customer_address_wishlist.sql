-- Identity customer owns shop-scoped shipping addresses and product wishlist
-- relations. Catalog product facts stay in Catalog; this table only stores the
-- customer-to-product relationship.

CREATE TABLE identity_customer_address (
  address_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  customer_subject VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  recipient VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  phone VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  country VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT '',
  province VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT '',
  city VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT '',
  district VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT '',
  detail VARCHAR(512) COLLATE utf8mb4_0900_as_cs NOT NULL,
  postal_code VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT '',
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (address_id),
  KEY idx_identity_customer_address_subject (merchant_id, shop_id, customer_subject, is_default, address_id),
  CONSTRAINT fk_identity_customer_address_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_customer_address_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_customer_address_scope CHECK (merchant_id > 0 AND shop_id > 0),
  CONSTRAINT ck_identity_customer_address_default CHECK (is_default IN (0, 1)),
  CONSTRAINT ck_identity_customer_address_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_customer_address_command (
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  customer_subject VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  address_id BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (merchant_id, shop_id, customer_subject, command_key),
  CONSTRAINT fk_identity_customer_address_command_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_customer_address_command_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_customer_wishlist (
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  customer_subject VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  product_id BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (merchant_id, shop_id, customer_subject, product_id),
  KEY idx_identity_customer_wishlist_created (merchant_id, shop_id, customer_subject, created_at, product_id),
  CONSTRAINT fk_identity_customer_wishlist_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_customer_wishlist_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_customer_wishlist_scope CHECK (merchant_id > 0 AND shop_id > 0 AND product_id > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_customer_wishlist_command (
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  customer_subject VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  product_id BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (merchant_id, shop_id, customer_subject, command_key),
  CONSTRAINT fk_identity_customer_wishlist_command_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_customer_wishlist_command_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
