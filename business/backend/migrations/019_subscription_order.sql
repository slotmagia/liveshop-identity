-- Merchant self-service SaaS purchase orders. Payment facts stay in Trade;
-- this table only snapshots the plan and records PENDING/PAID/CANCELLED.

CREATE TABLE subscription_order (
  order_id BIGINT NOT NULL AUTO_INCREMENT,
  order_no VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  merchant_id BIGINT NOT NULL,
  plan_id BIGINT NOT NULL,
  plan_code VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  plan_name VARCHAR(191) NOT NULL,
  price_minor BIGINT NOT NULL,
  duration_days INT NOT NULL,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  pay_no VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT '',
  channel_code VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT '',
  version BIGINT NOT NULL,
  paid_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  pending_slot BIGINT GENERATED ALWAYS AS (
    CASE WHEN status = 'PENDING' THEN plan_id ELSE NULL END
  ) STORED,
  PRIMARY KEY (order_id),
  UNIQUE KEY uq_subscription_order_no (order_no),
  UNIQUE KEY uq_subscription_order_pending_plan (merchant_id, pending_slot),
  KEY idx_subscription_order_merchant (merchant_id, created_at),
  CONSTRAINT ck_subscription_order_price CHECK (price_minor > 0),
  CONSTRAINT ck_subscription_order_duration CHECK (duration_days >= 0),
  CONSTRAINT ck_subscription_order_status CHECK (status IN ('PENDING','PAID','CANCELLED')),
  CONSTRAINT ck_subscription_order_version CHECK (version > 0),
  CONSTRAINT fk_subscription_order_plan
    FOREIGN KEY (plan_id) REFERENCES subscription_plan(plan_id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE subscription_order_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  merchant_id BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
