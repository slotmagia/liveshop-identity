-- Identity risk owns shop-scoped visitor risk events and the current
-- visitor risk snapshot. room_id is an opaque Live room reference, not a
-- foreign key. app_id/commercial_id are not runtime identifiers.

CREATE TABLE identity_visitor_risk (
  visitor_risk_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  visitor_id VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  score INT NOT NULL,
  level VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  version BIGINT NOT NULL,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (visitor_risk_id),
  UNIQUE KEY uq_identity_visitor_risk_shop_visitor (merchant_id, shop_id, visitor_id),
  CONSTRAINT fk_identity_visitor_risk_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_visitor_risk_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_visitor_risk_scope CHECK (merchant_id > 0 AND shop_id > 0),
  CONSTRAINT ck_identity_visitor_risk_score CHECK (score >= 0),
  CONSTRAINT ck_identity_visitor_risk_level CHECK (level IN ('NONE','LOW','MEDIUM','HIGH')),
  CONSTRAINT ck_identity_visitor_risk_status CHECK (status IN ('NORMAL','WATCH','RESTRICTED','BLOCKED')),
  CONSTRAINT ck_identity_visitor_risk_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_risk_event (
  event_id BIGINT NOT NULL AUTO_INCREMENT,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  visitor_id VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  nickname VARCHAR(64) NOT NULL DEFAULT '',
  room_id BIGINT NOT NULL DEFAULT 0,
  reason VARCHAR(64) NOT NULL,
  score_before INT NOT NULL,
  score_after_decay INT NOT NULL,
  score_delta INT NOT NULL,
  score_after INT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (event_id),
  KEY idx_identity_risk_event_list (merchant_id, shop_id, created_at, event_id),
  KEY idx_identity_risk_event_visitor (merchant_id, shop_id, visitor_id, event_id),
  CONSTRAINT fk_identity_risk_event_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_risk_event_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_risk_event_scope CHECK (merchant_id > 0 AND shop_id > 0 AND room_id >= 0),
  CONSTRAINT ck_identity_risk_event_visitor CHECK (visitor_id REGEXP '^[^[:space:]]{1,64}$'),
  CONSTRAINT ck_identity_risk_event_scores CHECK (score_before >= 0 AND score_after_decay >= 0 AND score_after >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
