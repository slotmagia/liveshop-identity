-- Identity fulfillment owns shop-scoped complaint tickets and the review
-- command ledger. target_id is an opaque reference to Trade/Live/Catalog,
-- not a foreign key. app_id/commercial_id are not runtime identifiers.

CREATE TABLE identity_complaint (
  complaint_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  customer_subject VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  target_type VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  target_id BIGINT NOT NULL,
  reason_code VARCHAR(64) NOT NULL,
  content VARCHAR(4000) NOT NULL,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  handle_note VARCHAR(2000) NOT NULL DEFAULT '',
  version BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  handled_at DATETIME(3) NULL,
  PRIMARY KEY (complaint_id),
  KEY idx_identity_complaint_list (merchant_id, shop_id, created_at, complaint_id),
  KEY idx_identity_complaint_customer (merchant_id, shop_id, customer_subject, complaint_id),
  KEY idx_identity_complaint_status (merchant_id, shop_id, status, complaint_id),
  CONSTRAINT fk_identity_complaint_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_complaint_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_complaint_scope CHECK (merchant_id > 0 AND shop_id > 0 AND target_id >= 0),
  CONSTRAINT ck_identity_complaint_customer CHECK (customer_subject REGEXP '^[^[:space:]]{1,128}$'),
  CONSTRAINT ck_identity_complaint_status CHECK (status IN ('OPEN','ACCEPTED','REJECTED')),
  CONSTRAINT ck_identity_complaint_target CHECK (target_type IN ('ORDER','AFTERSALE','LIVE','PRODUCT','OTHER')),
  CONSTRAINT ck_identity_complaint_reason CHECK (CHAR_LENGTH(reason_code) BETWEEN 1 AND 64),
  CONSTRAINT ck_identity_complaint_content CHECK (CHAR_LENGTH(content) BETWEEN 1 AND 4000),
  CONSTRAINT ck_identity_complaint_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_complaint_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash VARBINARY(32) NOT NULL,
  complaint_id BIGINT NULL,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  completed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
