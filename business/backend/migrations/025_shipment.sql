-- Identity fulfillment owns shop-scoped shipments, traces, and the write
-- command ledger. order_id is an opaque Trade order reference, not a foreign
-- key. app_id/commercial_id are not runtime identifiers.

CREATE TABLE identity_shipment (
  shipment_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  order_id BIGINT NOT NULL,
  carrier VARCHAR(64) NOT NULL,
  tracking_no VARCHAR(64) NOT NULL,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  traces JSON NOT NULL,
  version BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (shipment_id),
  UNIQUE KEY uk_identity_shipment_order (merchant_id, shop_id, order_id),
  KEY idx_identity_shipment_list (merchant_id, shop_id, created_at, shipment_id),
  KEY idx_identity_shipment_status (merchant_id, shop_id, status, shipment_id),
  CONSTRAINT fk_identity_shipment_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_shipment_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_shipment_scope CHECK (merchant_id > 0 AND shop_id > 0 AND order_id > 0),
  CONSTRAINT ck_identity_shipment_status CHECK (status IN ('SHIPPED','DELIVERED')),
  CONSTRAINT ck_identity_shipment_carrier CHECK (CHAR_LENGTH(carrier) BETWEEN 1 AND 64),
  CONSTRAINT ck_identity_shipment_tracking CHECK (CHAR_LENGTH(tracking_no) BETWEEN 1 AND 64),
  CONSTRAINT ck_identity_shipment_traces CHECK (JSON_VALID(traces) AND JSON_TYPE(traces)='ARRAY'),
  CONSTRAINT ck_identity_shipment_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_shipment_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash VARBINARY(32) NOT NULL,
  shipment_id BIGINT NULL,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  completed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
