-- Identity fulfillment owns shop-scoped aftersale tickets, line items and the
-- review/return command ledger. order_id/sku_id/payment_no are opaque references
-- to Trade/Catalog, not foreign keys. app_id/commercial_id are not runtime identifiers.

CREATE TABLE identity_aftersale (
  aftersale_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  customer_subject VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  order_id BIGINT NOT NULL,
  payment_no VARCHAR(64) NOT NULL DEFAULT '',
  type VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  requested_amount BIGINT NOT NULL,
  amount BIGINT NOT NULL,
  reason VARCHAR(255) NOT NULL,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  return_status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  handle_note VARCHAR(2000) NOT NULL DEFAULT '',
  version BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  reviewed_at DATETIME(3) NULL,
  received_at DATETIME(3) NULL,
  PRIMARY KEY (aftersale_id),
  KEY idx_identity_aftersale_list (merchant_id, shop_id, created_at, aftersale_id),
  KEY idx_identity_aftersale_customer (merchant_id, shop_id, customer_subject, aftersale_id),
  KEY idx_identity_aftersale_status (merchant_id, shop_id, status, aftersale_id),
  CONSTRAINT fk_identity_aftersale_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_aftersale_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_aftersale_scope CHECK (merchant_id > 0 AND shop_id > 0 AND order_id > 0),
  CONSTRAINT ck_identity_aftersale_customer CHECK (customer_subject REGEXP '^[^[:space:]]{1,128}$'),
  CONSTRAINT ck_identity_aftersale_type CHECK (type IN ('REFUND_ONLY','RETURN_REFUND')),
  CONSTRAINT ck_identity_aftersale_status CHECK (status IN ('PENDING','APPROVED','REJECTED','REFUNDED','CLOSED')),
  CONSTRAINT ck_identity_aftersale_return CHECK (return_status IN ('NOT_REQUIRED','PENDING','RECEIVED')),
  CONSTRAINT ck_identity_aftersale_amount CHECK (requested_amount > 0 AND amount > 0 AND amount <= requested_amount),
  CONSTRAINT ck_identity_aftersale_reason CHECK (CHAR_LENGTH(reason) BETWEEN 1 AND 255),
  CONSTRAINT ck_identity_aftersale_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_aftersale_item (
  item_id BIGINT NOT NULL AUTO_INCREMENT,
  aftersale_id BIGINT NOT NULL,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  sku_id BIGINT NOT NULL,
  title VARCHAR(200) NOT NULL DEFAULT '',
  quantity BIGINT NOT NULL,
  refund_amount BIGINT NOT NULL,
  received_quantity BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (item_id),
  UNIQUE KEY uk_identity_aftersale_item (aftersale_id, sku_id),
  KEY idx_identity_aftersale_item_shop (merchant_id, shop_id, aftersale_id),
  CONSTRAINT fk_identity_aftersale_item_ticket FOREIGN KEY (aftersale_id)
    REFERENCES identity_aftersale(aftersale_id),
  CONSTRAINT ck_identity_aftersale_item_scope CHECK (merchant_id > 0 AND shop_id > 0 AND sku_id > 0),
  CONSTRAINT ck_identity_aftersale_item_qty CHECK (quantity > 0 AND refund_amount >= 0 AND received_quantity >= 0 AND received_quantity <= quantity)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_aftersale_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash VARBINARY(32) NOT NULL,
  aftersale_id BIGINT NULL,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  completed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
